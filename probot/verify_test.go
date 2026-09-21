package probot

import (
	"strings"
	"testing"
)

func runWith(id,status string) deliveryRunResponse {
	var out deliveryRunResponse
	out.Processed=1
	out.Results=append(out.Results,struct {
		DeliveryID string `json:"deliveryId"`
		Status string `json:"status"`
	}{DeliveryID:id,Status:status})
	return out
}

func TestValidateRestartRuns(t *testing.T) {
	if err:=validateRestartRuns("g-1",runWith("g-1","done"),deliveryRunResponse{}); err!=nil { t.Fatal(err) }
}

func TestValidateRestartRunsRequiresCompletionAfterRestart(t *testing.T) {
	err:=validateRestartRuns("g-1",runWith("other","done"),deliveryRunResponse{})
	if err==nil || !strings.Contains(err.Error(),"not completed") { t.Fatalf("got %v",err) }
}

func TestValidateRestartRunsRejectsReprocessing(t *testing.T) {
	err:=validateRestartRuns("g-1",runWith("g-1","done"),runWith("g-1","done"))
	if err==nil || !strings.Contains(err.Error(),"processed again") { t.Fatalf("got %v",err) }
}

func TestDecodeDeliveryRun(t *testing.T) {
	run,err:=decodeDeliveryRun([]byte(`{"processed":1,"results":[{"deliveryId":"g-1","status":"done"}]}`))
	if err!=nil { t.Fatal(err) }
	if !containsDone(run,"g-1") { t.Fatalf("decoded run=%+v",run) }
}
