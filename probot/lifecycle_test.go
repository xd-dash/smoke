package probot

import (
	"context"
	"os"
	"path/filepath"
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
