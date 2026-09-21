package probot

import (
	"context"
	"encoding/json"
	"fmt"

	probotruntime "github.com/xd-dash/probot-runtime"
)

type RestartVerification struct {
	Backend string `json:"backend"`
	DeliveryID string `json:"delivery_id"`
	ProviderDeliveryID string `json:"provider_delivery_id"`
	RouterAStopped bool `json:"router_a_stopped"`
	ProcessedAfterRestart bool `json:"processed_after_restart"`
	NotReprocessed bool `json:"not_reprocessed"`
	DuplicateDetected bool `json:"duplicate_detected"`
}

type deliveryRunResponse struct {
	Processed int `json:"processed"`
	Results []struct {
		DeliveryID string `json:"deliveryId"`
		Status string `json:"status"`
	} `json:"results"`
}

func VerifyRestart(ctx context.Context, lifecycle Lifecycle, client Client) (result RestartVerification, err error) {
	fixture,err:=probotruntime.PrepareRestartFixture(ctx,"")
	if err!=nil { return result,fmt.Errorf("prepare restart fixture: %w",err) }

	started:=false
	defer func(){ if started { _=lifecycle.Stop(context.Background()) } }()

	a,err:=lifecycle.Run(ctx)
	if err!=nil { return result,fmt.Errorf("start router A: %w",err) }
	started=true
	result.Backend=a.Backend

	request:=DeliveryRequest{RoutingKey:fixture.RoutingKey,DeliveryID:fixture.DeliveryID,Event:fixture.Event,Signature:fixture.Signature,Body:fixture.RawBody}
	first,err:=client.PublishDelivery(ctx,request)
	if err!=nil { return result,fmt.Errorf("publish fixture delivery: %w",err) }
	if first.Duplicate { return result,fmt.Errorf("fresh fixture delivery was reported duplicate") }
	if first.DeliveryID!="" && first.DeliveryID!=fixture.DeliveryID {
		return result,fmt.Errorf("delivery identity changed at ingress: %s -> %s",fixture.DeliveryID,first.DeliveryID)
	}
	result.DeliveryID=fixture.DeliveryID
	result.ProviderDeliveryID=first.ProviderDeliveryID

	if err=lifecycle.Stop(ctx); err!=nil { return result,fmt.Errorf("destroy router A: %w",err) }
	started=false
	result.RouterAStopped=true
	if _,err=lifecycle.Status(ctx); err==nil { return result,fmt.Errorf("router A still reports running after stop") }

	if _,err=lifecycle.Run(ctx); err!=nil { return result,fmt.Errorf("start router B: %w",err) }
	started=true

	runBody,_,err:=client.DeliveryRun(ctx,25)
	if err!=nil { return result,fmt.Errorf("process delivery after restart: %w",err) }
	run,err:=decodeDeliveryRun(runBody)
	if err!=nil { return result,err }
	result.ProcessedAfterRestart=containsDone(run,fixture.DeliveryID)
	if !result.ProcessedAfterRestart { return result,fmt.Errorf("delivery %s was not completed by router B",fixture.DeliveryID) }

	secondBody,_,err:=client.DeliveryRun(ctx,25)
	if err!=nil { return result,fmt.Errorf("verify settled delivery: %w",err) }
	second,err:=decodeDeliveryRun(secondBody)
	if err!=nil { return result,err }
	result.NotReprocessed=!containsDelivery(second,fixture.DeliveryID)
	if !result.NotReprocessed { return result,fmt.Errorf("settled delivery %s was processed again",fixture.DeliveryID) }

	return result,nil
}

func decodeDeliveryRun(body []byte) (deliveryRunResponse,error) {
	var out deliveryRunResponse
	if err:=json.Unmarshal(body,&out); err!=nil { return out,fmt.Errorf("decode delivery-run response: %w",err) }
	return out,nil
}
func containsDone(run deliveryRunResponse,id string) bool {
	for _,r:=range run.Results { if r.DeliveryID==id && r.Status=="done" { return true } }
	return false
}
func containsDelivery(run deliveryRunResponse,id string) bool {
	for _,r:=range run.Results { if r.DeliveryID==id { return true } }
	return false
}
