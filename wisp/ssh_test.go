package wisp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func endpoint(t *testing.T) Config {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
			return nil, nil
		}
		return nil, errors.New("key rejected")
	}}
	cfg.AddHostKey(signer)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherSigner, err := ssh.NewSignerFromKey(other)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AddHostKey(otherSigner)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); listener.Close(); wg.Wait() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				stop := context.AfterFunc(ctx, func() { conn.Close() })
				defer stop()
				_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for incoming := range chans {
					channel, requests, err := incoming.Accept()
					if err != nil {
						continue
					}
					switch incoming.ChannelType() {
					case "session":
						go func() {
							defer channel.Close()
							for req := range requests {
								if req.Type != "exec" {
									req.Reply(false, nil)
									continue
								}
								var command struct{ Command string }
								ssh.Unmarshal(req.Payload, &command)
								req.Reply(true, nil)
								switch command.Command {
								case "ok":
									channel.Write([]byte("out"))
									channel.Stderr().Write([]byte("err"))
									channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{0}))
									return
								case "fail":
									channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Code uint32 }{7}))
									return
								default:
									<-ctx.Done()
									return
								}
							}
						}()
					case "direct-tcpip", "direct-streamlocal@openssh.com":
						go func() { defer channel.Close(); go ssh.DiscardRequests(requests); io.Copy(channel, channel) }()
					default:
						channel.Close()
					}
				}
			}()
		}
	}()
	dir := t.TempDir()
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	identity := filepath.Join(dir, "key")
	os.WriteFile(identity, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600)
	hosts := filepath.Join(dir, "known_hosts")
	os.WriteFile(hosts, []byte(knownhosts.Line([]string{knownhosts.Normalize(listener.Addr().String())}, signer.PublicKey())+"\n"), 0600)
	return Config{Address: listener.Addr().String(), User: "test", IdentityFile: identity, KnownHostsFile: hosts}
}

func TestExecAndExitStatus(t *testing.T) {
	cfg := endpoint(t)
	var stdout, stderr bytes.Buffer
	if err := cfg.Exec(context.Background(), "ok", nil, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "out" || stderr.String() != "err" {
		t.Fatal("streams not preserved")
	}
	var exit *ssh.ExitError
	if err := cfg.Exec(context.Background(), "fail", nil, io.Discard, io.Discard); !errors.As(err, &exit) || exit.ExitStatus() != 7 {
		t.Fatalf("exit status: %v", err)
	}
}
func TestRejectUnknownHost(t *testing.T) {
	cfg := endpoint(t)
	os.WriteFile(cfg.KnownHostsFile, nil, 0600)
	if client, err := cfg.Dial(context.Background()); err == nil {
		client.Close()
		t.Fatal("accepted unknown host")
	}
}
func TestExecCancellation(t *testing.T) {
	cfg := endpoint(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := cfg.Exec(ctx, "hang", nil, io.Discard, io.Discard); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
}
func TestUnixForwardTargetsAndCleanup(t *testing.T) {
	for _, network := range []string{"tcp", "unix"} {
		t.Run(network, func(t *testing.T) {
			cfg := endpoint(t)
			dir, err := os.MkdirTemp("", "wisp-test-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)
			path := filepath.Join(dir, "socket")
			listener, err := ListenUnix(path)
			if errors.Is(err, syscall.EPERM) {
				if os.Getenv("WISP_REQUIRE_UNIX") == "1" {
					t.Fatal("AF_UNIX listener required for local qualification")
				}
				t.Skip("runner prohibits AF_UNIX listeners; run on local host")
			}
			if err != nil {
				t.Fatal(err)
			}
			mode, _ := os.Stat(path)
			if mode.Mode().Perm() != 0600 {
				t.Fatal("socket is not private")
			}
			if other, err := ListenUnix(path); err == nil {
				other.Close()
				t.Fatal("replaced existing socket")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan struct{})
			done := make(chan error, 1)
			go func() { done <- cfg.Forward(ctx, listener, network, "127.0.0.1:1234", func() { close(ready) }) }()
			select {
			case <-ready:
			case <-time.After(3 * time.Second):
				t.Fatal("not ready")
			}
			conn, err := net.Dial("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			conn.Write([]byte("hello"))
			data := make([]byte, 5)
			if _, err := io.ReadFull(conn, data); err != nil || string(data) != "hello" {
				t.Fatalf("forward: %q %v", data, err)
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("forward did not stop")
			}
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("socket not removed")
			}
		})
	}
}

func TestForwardOverSSH(t *testing.T) {
	for _, network := range []string{"tcp", "unix"} {
		t.Run(network, func(t *testing.T) {
			cfg := endpoint(t)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan struct{})
			done := make(chan error, 1)
			go func() { done <- cfg.Forward(ctx, listener, network, "127.0.0.1:1234", func() { close(ready) }) }()
			select {
			case <-ready:
			case <-time.After(3 * time.Second):
				t.Fatal("not ready")
			}
			conn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(3 * time.Second))
			conn.Write([]byte("hello"))
			data := make([]byte, 5)
			if _, err := io.ReadFull(conn, data); err != nil || string(data) != "hello" {
				t.Fatalf("forward: %q %v", data, err)
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("did not stop")
			}
		})
	}
}

func TestKnownHostAlgorithmPreference(t *testing.T) {
	cfg := endpoint(t)
	verify, err := knownhosts.New(cfg.KnownHostsFile)
	if err != nil {
		t.Fatal(err)
	}
	remote, err := net.ResolveTCPAddr("tcp", cfg.Address)
	if err != nil {
		t.Fatal(err)
	}
	algorithms, err := trustedHostAlgorithms(cfg.KnownHostsFile, cfg.Address, remote, verify)
	if err != nil || len(algorithms) != 1 || algorithms[0] != ssh.KeyAlgoED25519 {
		t.Fatalf("trusted algorithms: %v %v", algorithms, err)
	}
	if err := cfg.Exec(context.Background(), "ok", nil, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
}
func TestChangedKnownHostRejected(t *testing.T) {
	cfg := endpoint(t)
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	other, _ := ssh.NewSignerFromKey(key)
	os.WriteFile(cfg.KnownHostsFile, []byte(knownhosts.Line([]string{knownhosts.Normalize(cfg.Address)}, other.PublicKey())+"\n"), 0600)
	if client, err := cfg.Dial(context.Background()); err == nil {
		client.Close()
		t.Fatal("accepted changed host key")
	}
}
