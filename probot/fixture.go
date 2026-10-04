package probot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	probotruntime "github.com/xd-dash/probot-runtime"
)

type Fixture struct {
	RoutingKey string `json:"routingKey"`
	AppID int64 `json:"appId"`
	WebhookSecret string `json:"webhookSecret"`
	Profile string `json:"profile"`
}

func SeedFixture(ctx context.Context, routingKey string) (Fixture,error) {
	runtime,err:=probotruntime.Prepare(ctx); if err!=nil { return Fixture{},err }
	script:=filepath.Join(runtime.Dir,"scripts","fixture.js")
	cmd:=exec.CommandContext(ctx,runtime.Node,script,"seed",routingKey)
	cmd.Dir=runtime.Dir; cmd.Env=os.Environ()
	out,err:=cmd.Output(); if err!=nil { return Fixture{},fmt.Errorf("seed Probot fixture: %w",err) }
	var fixture Fixture
	if err=json.Unmarshal(out,&fixture); err!=nil { return Fixture{},fmt.Errorf("decode Probot fixture: %w",err) }
	return fixture,nil
}
