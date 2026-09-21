package probot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestLifecycleStateRoundTrip(t *testing.T) {
	l:=Lifecycle{Dir:t.TempDir(),URL:"http://127.0.0.1:3000",Backend:"direct"}
	want:=State{Backend:"direct",PID:123,URL:l.URL,StartedAt:time.Unix(1,0).UTC()}
	if err:=l.save(want); err!=nil { t.Fatal(err) }
	got,err:=l.load(); if err!=nil { t.Fatal(err) }
	if got.Backend!=want.Backend || got.PID!=want.PID || got.URL!=want.URL || !got.StartedAt.Equal(want.StartedAt) { t.Fatalf("got=%+v want=%+v",got,want) }
	if info,err:=os.Stat(filepath.Join(l.Dir,"state.json")); err!=nil { t.Fatal(err) } else if info.Mode().Perm()!=0600 { t.Fatalf("mode=%o",info.Mode().Perm()) }
}

func TestPodmanRequiresExplicitImage(t *testing.T) {
	l:=Lifecycle{Dir:t.TempDir(),URL:"http://127.0.0.1:3000",Backend:"podman"}
	t.Setenv("PATH",t.TempDir())
	_,err:=l.Run(context.Background())
	if err==nil { t.Fatal("expected podman availability error") }
}

func TestWaitReadyRequiresHTTPResponse(t *testing.T) {
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ w.WriteHeader(http.StatusNotFound) }))
	defer server.Close()
	l:=Lifecycle{Dir:t.TempDir(),URL:server.URL,Backend:"direct"}
	ctx,cancel:=context.WithTimeout(context.Background(),time.Second); defer cancel()
	if err:=l.WaitReady(ctx,500*time.Millisecond); err!=nil { t.Fatalf("ready server rejected: %v",err) }
}

func TestWaitReadyTimesOutWithoutListener(t *testing.T) {
	l:=Lifecycle{Dir:t.TempDir(),URL:"http://127.0.0.1:1",Backend:"direct"}
	start:=time.Now()
	if err:=l.WaitReady(context.Background(),150*time.Millisecond); err==nil { t.Fatal("expected readiness timeout") }
	if time.Since(start)>time.Second { t.Fatal("readiness timeout was not bounded") }
}

func TestRouterEnvDerivesListenerFromURL(t *testing.T) {
	t.Setenv("HOST","")
	t.Setenv("PORT","")
	_ = os.Unsetenv("HOST")
	_ = os.Unsetenv("PORT")
	env:=routerEnv("http://127.0.0.1:43127")
	foundHost,foundPort:=false,false
	for _,entry:=range env { if entry=="HOST=127.0.0.1" { foundHost=true }; if entry=="PORT=43127" { foundPort=true } }
	if !foundHost || !foundPort { t.Fatalf("derived listener missing: host=%v port=%v",foundHost,foundPort) }
}

func TestStopWaitsForDirectProcessDestruction(t *testing.T) {
	if os.Getenv("SMOKE_LIFECYCLE_HELPER") == "1" {
		signal := make(chan os.Signal, 1)
		_ = signal
		for { time.Sleep(time.Second) }
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestStopWaitsForDirectProcessDestruction")
	cmd.Env = append(os.Environ(), "SMOKE_LIFECYCLE_HELPER=1")
	if err := cmd.Start(); err != nil { t.Fatal(err) }
	defer func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() }()

	l := Lifecycle{Dir:t.TempDir(), URL:"http://127.0.0.1:3000", Backend:"direct"}
	if err := l.save(State{Backend:"direct", PID:cmd.Process.Pid, URL:l.URL, StartedAt:time.Now().UTC()}); err != nil { t.Fatal(err) }
	if err := l.Stop(context.Background()); err != nil { t.Fatal(err) }
	if processAlive(cmd.Process.Pid) { t.Fatalf("process %d remains live after Stop",cmd.Process.Pid) }
	if _,err := os.Stat(l.statePath()); !os.IsNotExist(err) { t.Fatalf("state file remains after Stop: %v",err) }
}

func TestProcessAliveTreatsZombieAsDeadOnLinux(t *testing.T) {
	if _,err:=os.Stat("/proc/self/stat"); err!=nil { t.Skip("requires procfs") }
	cmd:=exec.Command("sh","-c","exit 0")
	if err:=cmd.Start(); err!=nil { t.Fatal(err) }
	pid:=cmd.Process.Pid
	deadline:=time.Now().Add(time.Second)
	for processAlive(pid) && time.Now().Before(deadline) { time.Sleep(10*time.Millisecond) }
	if processAlive(pid) { t.Fatalf("exited child %s still considered alive",strconv.Itoa(pid)) }
	_,_ = cmd.Process.Wait()
	_ = syscall.Signal(0)
}
