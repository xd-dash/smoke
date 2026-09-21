package probot

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"
)

type RestartVerification struct {
	Backend string `json:"backend"`
	DeliveryID string `json:"delivery_id"`
	ProviderDeliveryID string `json:"provider_delivery_id"`
	FirstRun string `json:"first_run"`
	SecondRunProcessed int `json:"second_run_processed"`
	Duplicate bool `json:"duplicate"`
}

type deliveryRunResponse struct {
	Processed int `json:"processed"`
	Results []struct { DeliveryID string `json:"deliveryId"`; Status string `json:"status"`; Error string `json:"error,omitempty"` } `json:"results"`
}

func VerifyRestart(ctx context.Context,l Lifecycle,client Client) (RestartVerification,error) {
	if _,err:=l.Status(ctx); err==nil { return RestartVerification{},fmt.Errorf("stop the existing Probot router before verify-restart") }
	fixture,err:=SeedFixture(ctx,"smoke-restart-"+fmt.Sprint(time.Now().UnixNano())); if err!=nil { return RestartVerification{},err }
	state,err:=l.Run(ctx); if err!=nil { return RestartVerification{},err }
	cleanup:=true
	defer func(){ if cleanup { _=l.Stop(context.Background()) } }()
	if err=waitReady(ctx,client,10*time.Second); err!=nil { return RestartVerification{},err }

	body,_:=json.Marshal(map[string]any{"action":"smoke","installation":map[string]any{"id":900000002}})
	deliveryID:="smoke-"+fmt.Sprint(time.Now().UnixNano())
	mac:=hmac.New(sha256.New,[]byte(fixture.WebhookSecret)); _,_=mac.Write(body)
	signature:="sha256="+hex.EncodeToString(mac.Sum(nil))
	published,err:=client.PublishDelivery(ctx,DeliveryRequest{RoutingKey:fixture.RoutingKey,DeliveryID:deliveryID,Event:"push",Signature:signature,Body:body})
	if err!=nil { return RestartVerification{},err }

	if err=l.Stop(ctx); err!=nil { return RestartVerification{},fmt.Errorf("destroy router A: %w",err) }
	state,err=l.Run(ctx); if err!=nil { return RestartVerification{},fmt.Errorf("start router B: %w",err) }
	if err=waitReady(ctx,client,10*time.Second); err!=nil { return RestartVerification{},err }

	run,err:=deliveryRun(ctx,client,25); if err!=nil { return RestartVerification{},err }
	status:=""
	for _,result:=range run.Results { if result.DeliveryID==deliveryID { status=result.Status; if status!="done" { return RestartVerification{},fmt.Errorf("delivery %s settled as %s: %s",deliveryID,status,result.Error) } } }
	if status=="" { return RestartVerification{},fmt.Errorf("router B did not process durable delivery %s",deliveryID) }

	second,err:=deliveryRun(ctx,client,25); if err!=nil { return RestartVerification{},err }
	for _,result:=range second.Results { if result.DeliveryID==deliveryID { return RestartVerification{},fmt.Errorf("settled delivery %s was processed again",deliveryID) } }
	duplicate,err:=client.PublishDelivery(ctx,DeliveryRequest{RoutingKey:fixture.RoutingKey,DeliveryID:deliveryID,Event:"push",Signature:signature,Body:body}); if err!=nil { return RestartVerification{},err }
	if !duplicate.Duplicate { return RestartVerification{},fmt.Errorf("duplicate GitHub delivery %s was not reported duplicate",deliveryID) }
	if err=l.Stop(ctx); err!=nil { return RestartVerification{},err }
	cleanup=false
	return RestartVerification{Backend:state.Backend,DeliveryID:deliveryID,ProviderDeliveryID:published.ProviderDeliveryID,FirstRun:status,SecondRunProcessed:second.Processed,Duplicate:duplicate.Duplicate},nil
}

func waitReady(ctx context.Context,client Client,timeout time.Duration) error {
	base:=client.BaseURL; if base=="" { base="http://127.0.0.1:3000" }
	u,err:=url.Parse(base); if err!=nil { return err }
	host:=u.Host
	if _,_,err:=net.SplitHostPort(host); err!=nil {
		if u.Scheme=="https" { host=net.JoinHostPort(u.Hostname(),"443") } else { host=net.JoinHostPort(u.Hostname(),"80") }
	}
	deadline:=time.Now().Add(timeout)
	var last error
	for {
		conn,err:=net.DialTimeout("tcp",host,250*time.Millisecond)
		if err==nil { _=conn.Close(); return nil }; last=err
		if time.Now().After(deadline) { return fmt.Errorf("Probot router did not become ready: %w",last) }
		select { case <-ctx.Done(): return ctx.Err(); case <-time.After(100*time.Millisecond): }
	}
}

func deliveryRun(ctx context.Context,client Client,limit int) (deliveryRunResponse,error) {
	body,_,err:=client.DeliveryRun(ctx,limit); if err!=nil { return deliveryRunResponse{},err }
	var out deliveryRunResponse
	if err=json.Unmarshal(body,&out); err!=nil { return out,err }
	return out,nil
}
