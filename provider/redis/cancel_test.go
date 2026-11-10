package redisprovider

import (
	"bufio"
	"context"
	"fmt"
	"github.com/xd-dash/smoke/callback"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSubscriptionCancellationClosesBlockedRead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var workers sync.WaitGroup
	t.Cleanup(func() { cancel(); listener.Close(); workers.Wait() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				stop := context.AfterFunc(ctx, func() { conn.Close() })
				defer stop()
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					count, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "*")))
					var command []string
					for i := 0; i < count; i++ {
						size, err := r.ReadString('\n')
						if err != nil {
							return
						}
						n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(size, "$")))
						if n < 0 || n > 65536 {
							return
						}
						data := make([]byte, n+2)
						if _, err := io.ReadFull(r, data); err != nil {
							return
						}
						command = append(command, string(data[:n]))
					}
					if len(command) == 0 {
						return
					}
					switch strings.ToUpper(command[0]) {
					case "HELLO":
						fmt.Fprint(conn, "-ERR unknown command 'hello'\r\n")
					case "PING":
						fmt.Fprint(conn, "+PONG\r\n")
					case "SUBSCRIBE":
						fmt.Fprint(conn, "*3\r\n$9\r\nsubscribe\r\n$6\r\nevents\r\n:1\r\n")
					default:
						fmt.Fprint(conn, "+OK\r\n")
					}
				}
			}()
		}
	}()
	host, portString, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portString)
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- New().RunSubscription(ctx, Subscription{Target: Target{Network: "tcp", Host: host, Port: port}, Channels: []string{"events"}, Ready: func() { close(ready) }}, callback.New(callback.Stdout{Writer: io.Discard}))
	}()
	select {
	case <-ready:
	case err := <-done:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("subscribe not ready")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation left subscription blocked")
	}
}
