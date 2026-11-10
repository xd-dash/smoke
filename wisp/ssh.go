// Package wisp executes work through an explicitly trusted SSH endpoint.
package wisp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Config struct {
	Address, User, IdentityFile, KnownHostsFile string
	ConnectTimeout                              time.Duration
}

func (c Config) Dial(ctx context.Context) (*ssh.Client, error) {
	if c.Address == "" || c.User == "" || c.IdentityFile == "" || c.KnownHostsFile == "" {
		return nil, errors.New("address, user, identity and known-hosts file are required")
	}
	key, err := os.ReadFile(c.IdentityFile)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	verify, err := knownhosts.New(c.KnownHostsFile)
	if err != nil {
		return nil, err
	}
	timeout := c.ConnectTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(handshakeCtx, "tcp", c.Address)
	if err != nil {
		return nil, err
	}
	algorithms, err := trustedHostAlgorithms(c.KnownHostsFile, c.Address, conn.RemoteAddr(), verify)
	if err != nil {
		conn.Close()
		return nil, err
	}
	stop := context.AfterFunc(handshakeCtx, func() { conn.Close() })
	cc, chans, reqs, err := ssh.NewClientConn(conn, c.Address, &ssh.ClientConfig{User: c.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: verify, HostKeyAlgorithms: algorithms})
	if !stop() {
		conn.Close()
		return nil, handshakeCtx.Err()
	}
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(cc, chans, reqs), nil
}

// Exec preserves stdout, stderr and SSH exit errors. Cancellation closes this
// operation's connection; persistent workloads must be owned by guest systemd.
func (c Config) Exec(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) error {
	client, err := c.Dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	session.Stdin, session.Stdout, session.Stderr = stdin, stdout, stderr
	err = session.Run(command)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// Forward serves a caller-owned listener until cancellation. Both TCP and Unix
// targets are resolved at the SSH server, never on the client.
func (c Config) Forward(ctx context.Context, listener net.Listener, network, target string, ready func()) error {
	defer listener.Close()
	if network != "tcp" && network != "unix" {
		return errors.New("target network must be tcp or unix")
	}
	client, err := c.Dial(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { client.Wait(); cancel() }()
	stop := context.AfterFunc(ctx, func() { listener.Close(); client.Close() })
	defer stop()
	var workers sync.WaitGroup
	defer workers.Wait()
	if ready != nil {
		ready()
	}
	for {
		local, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			client.Close()
			return err
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer local.Close()
			stopLocal := context.AfterFunc(ctx, func() { local.Close() })
			defer stopLocal()
			remote, err := client.DialContext(ctx, network, target)
			if err != nil {
				return
			}
			defer remote.Close()
			done := make(chan struct{})
			go func() {
				io.Copy(remote, local)
				if cw, ok := remote.(interface{ CloseWrite() error }); ok {
					cw.CloseWrite()
				}
				close(done)
			}()
			io.Copy(local, remote)
			local.Close()
			remote.Close()
			<-done
		}()
	}
}

// ListenUnix refuses existing paths. The private parent prevents access during
// the brief interval between bind and chmod; no process-global umask is changed.
func ListenUnix(path string) (net.Listener, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("Unix socket path must be absolute")
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	if !parent.IsDir() || parent.Mode().Perm()&0077 != 0 {
		return nil, errors.New("Unix socket parent must be a private directory")
	}
	if _, err = os.Lstat(path); !os.IsNotExist(err) {
		return nil, fmt.Errorf("refusing existing or inaccessible socket path: %s", path)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

// Prefer verified endpoint keys before negotiation; the callback still verifies
// the actual key presented by the server. In particular RSA keys use SHA-2.
func trustedHostAlgorithms(path, address string, remote net.Addr, verify ssh.HostKeyCallback) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trusted := map[string]bool{}
	for len(data) > 0 {
		_, _, key, _, rest, err := ssh.ParseKnownHosts(data)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		data = rest
		if verify(address, remote, key) != nil {
			continue
		}
		if key.Type() == ssh.KeyAlgoRSA {
			trusted[ssh.KeyAlgoRSASHA256] = true
			trusted[ssh.KeyAlgoRSASHA512] = true
		} else {
			trusted[key.Type()] = true
		}
	}
	var algorithms []string
	for _, algorithm := range ssh.SupportedAlgorithms().HostKeys {
		if trusted[algorithm] {
			algorithms = append(algorithms, algorithm)
		}
	}
	// CA-only trust retains the library's certificate negotiation and verification.
	return algorithms, nil
}
