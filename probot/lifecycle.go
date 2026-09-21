package probot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	probotrouter "github.com/xd-dash/probot-runtime/router"
)

type State struct {
	Backend string `json:"backend"`
	PID int `json:"pid,omitempty"`
	Container string `json:"container,omitempty"`
	URL string `json:"url"`
	StartedAt time.Time `json:"started_at"`
}

type Lifecycle struct {
	Dir string
	URL string
	Backend string
}

func DefaultLifecycle() Lifecycle {
	dir:=os.Getenv("SMOKE_PROBOT_STATE_DIR")
	if dir=="" { if cache,err:=os.UserCacheDir(); err==nil { dir=filepath.Join(cache,"smoke","probot") } else { dir=filepath.Join(os.TempDir(),"smoke-probot") } }
	url:=os.Getenv("PROBOT_URL"); if url=="" { url="http://127.0.0.1:3000" }
	backend:=os.Getenv("SMOKE_PROBOT_BACKEND"); if backend=="" { backend="direct" }
	return Lifecycle{Dir:dir,URL:url,Backend:backend}
}

func (l Lifecycle) statePath() string { return filepath.Join(l.Dir,"state.json") }
func (l Lifecycle) logPath() string { return filepath.Join(l.Dir,"router.log") }

func (l Lifecycle) load() (State,error) {
	data,err:=os.ReadFile(l.statePath()); if err!=nil { return State{},err }
	var state State; if err=json.Unmarshal(data,&state); err!=nil { return State{},err }; return state,nil
}
func (l Lifecycle) save(state State) error {
	if err:=os.MkdirAll(l.Dir,0700); err!=nil { return err }
	data,err:=json.MarshalIndent(state,"","  "); if err!=nil { return err }
	return os.WriteFile(l.statePath(),append(data,'\n'),0600)
}

func (l Lifecycle) Run(ctx context.Context) (State,error) {
	if state,err:=l.Status(ctx); err==nil { return state,fmt.Errorf("probot already running via %s",state.Backend) }
	var state State
	var err error
	switch l.Backend {
	case "direct": state,err=l.runDirect(ctx)
	case "podman": state,err=l.runPodman(ctx)
	default: return State{},fmt.Errorf("unsupported Probot backend %q (want direct or podman)",l.Backend)
	}
	if err!=nil { return State{},err }
	if err=l.WaitReady(ctx,10*time.Second); err!=nil { _=l.Stop(context.Background()); return State{},err }
	return state,nil
}

func (l Lifecycle) runDirect(ctx context.Context) (State,error) {
	if err:=os.MkdirAll(l.Dir,0700); err!=nil { return State{},err }
	exe,err:=os.Executable(); if err!=nil { return State{},err }
	log,err:=os.OpenFile(l.logPath(),os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600); if err!=nil { return State{},err }; defer log.Close()
	cmd:=exec.Command(exe,"probot","serve-foreground")
	cmd.Stdout=log; cmd.Stderr=log; cmd.Env=routerEnv(l.URL)
	if err:=cmd.Start(); err!=nil { return State{},err }
	go func() { _ = cmd.Wait() }()
	state:=State{Backend:"direct",PID:cmd.Process.Pid,URL:l.URL,StartedAt:time.Now().UTC()}
	if err:=l.save(state); err!=nil { _=cmd.Process.Kill(); return State{},err }
	return state,nil
}

func routerEnv(rawURL string) []string {
	env:=os.Environ()
	u,err:=url.Parse(rawURL); if err!=nil || u.Hostname()=="" { return env }
	host:=u.Hostname(); port:=u.Port()
	if port=="" { if u.Scheme=="https" { port="443" } else { port="80" } }
	if _,ok:=os.LookupEnv("HOST"); !ok { env=append(env,"HOST="+host) }
	if _,ok:=os.LookupEnv("PORT"); !ok { env=append(env,"PORT="+port) }
	return env
}

func (l Lifecycle) runPodman(ctx context.Context) (State,error) {
	if _,err:=exec.LookPath("podman"); err!=nil { return State{},fmt.Errorf("podman backend requested but podman is unavailable: %w",err) }
	image:=os.Getenv("SMOKE_PROBOT_IMAGE"); if image=="" { return State{},errors.New("SMOKE_PROBOT_IMAGE is required for podman backend") }
	name:="smoke-probot-"+strconv.Itoa(os.Getpid())
	args:=[]string{"run","-d","--rm","--name",name,"--network","host"}
	for _,key:=range []string{"PROBOT_RUNTIME_BASE_URL","PROBOT_RUNTIME_GITHUB_ORG","PROBOT_RUNTIME_ORG_ID","PROBOT_RUNTIME_TENANT_ID","PROBOT_RUNTIME_BINDING_JSON","PROBOT_RUNTIME_OPERATOR_TOKEN","PROBOT_RUNTIME_RECONCILE_EVERY","PROBOT_RUNTIME_MANIFEST_PROFILE","REDIS_URL","REDIS_DB","REDIS_USERNAME","REDIS_PASSWORD","REDIS_PREFIX","SKYMILL_URL","SKYMILL_TOKEN","SKYMILL_STREAM","SKYMILL_CONSUMER_GROUP","SKYMILL_CONSUMER","HOST","PORT"} {
		if _,ok:=os.LookupEnv(key); ok { args=append(args,"--env",key) }
	}
	args=append(args,image,"npm","run","serve:router")
	out,err:=exec.CommandContext(ctx,"podman",args...).CombinedOutput(); if err!=nil { return State{},fmt.Errorf("podman run: %w: %s",err,out) }
	state:=State{Backend:"podman",Container:name,URL:l.URL,StartedAt:time.Now().UTC()}
	if err:=l.save(state); err!=nil { _,_=exec.Command("podman","rm","-f",name).CombinedOutput(); return State{},err }
	return state,nil
}

func (l Lifecycle) Stop(ctx context.Context) error {
	state,err:=l.load(); if errors.Is(err,os.ErrNotExist) { return nil }; if err!=nil { return err }
	switch state.Backend {
	case "direct":
		p,err:=os.FindProcess(state.PID); if err==nil {
			_ = p.Signal(syscall.SIGTERM)
			deadline:=time.Now().Add(5*time.Second)
			for processAlive(state.PID) && time.Now().Before(deadline) { time.Sleep(50*time.Millisecond) }
			if processAlive(state.PID) {
				_ = p.Kill()
				killDeadline:=time.Now().Add(2*time.Second)
				for processAlive(state.PID) && time.Now().Before(killDeadline) { time.Sleep(25*time.Millisecond) }
				if processAlive(state.PID) { return fmt.Errorf("Probot process %d is still running after kill",state.PID) }
			}
		}
	case "podman":
		if _,err:=exec.LookPath("podman"); err!=nil { return err }
		out,err:=exec.CommandContext(ctx,"podman","rm","-f",state.Container).CombinedOutput(); if err!=nil { return fmt.Errorf("podman rm: %w: %s",err,out) }
		if podmanRunning(ctx,state.Container) { return fmt.Errorf("Probot container %s is still running after removal",state.Container) }
	}
	return os.Remove(l.statePath())
}

func processAlive(pid int) bool {
	if pid<=0 { return false }
	// On Linux a terminated child may remain as a zombie briefly. signal 0 still
	// succeeds for zombies, but they no longer own listeners or runtime state.
	if data,err:=os.ReadFile(fmt.Sprintf("/proc/%d/stat",pid)); err==nil {
		if end:=strings.LastIndexByte(string(data), ')'); end>=0 && end+2<len(data) && data[end+2]=='Z' { return false }
	}
	p,err:=os.FindProcess(pid); return err==nil && p.Signal(syscall.Signal(0))==nil
}

func podmanRunning(ctx context.Context,name string) bool {
	out,err:=exec.CommandContext(ctx,"podman","inspect","-f","{{.State.Running}}",name).CombinedOutput(); return err==nil && string(out)=="true\\n"
}

func (l Lifecycle) Status(ctx context.Context) (State,error) {
	state,err:=l.load(); if err!=nil { return State{},err }
	switch state.Backend {
	case "direct":
		if state.PID<=0 { return State{},errors.New("invalid Probot PID") }
		p,err:=os.FindProcess(state.PID); if err!=nil { return State{},err }
		if err=p.Signal(syscall.Signal(0)); err!=nil { return State{},err }
	case "podman":
		out,err:=exec.CommandContext(ctx,"podman","inspect","-f","{{.State.Running}}",state.Container).CombinedOutput(); if err!=nil || string(out)!="true\n" { return State{},errors.New("Probot container is not running") }
	default: return State{},fmt.Errorf("unknown Probot backend %q",state.Backend)
	}
	return state,nil
}

func (l Lifecycle) WaitReady(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 { timeout = 15*time.Second }
	deadline:=time.Now().Add(timeout)
	client:=&http.Client{Timeout:500*time.Millisecond}
	for {
		req,err:=http.NewRequestWithContext(ctx,http.MethodGet,strings.TrimRight(l.URL,"/")+"/",nil)
		if err!=nil { return err }
		resp,err:=client.Do(req)
		if err==nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 { return nil }
		}
		if time.Now().After(deadline) { return fmt.Errorf("Probot router at %s did not become ready within %s",l.URL,timeout) }
		select { case <-ctx.Done(): return ctx.Err(); case <-time.After(100*time.Millisecond): }
	}
}

func ServeForeground(ctx context.Context,args []string) error { return probotrouter.Run(ctx,append([]string{"serve"},args...)) }
