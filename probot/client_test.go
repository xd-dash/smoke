package probot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeliveryRequestEncodesGitHubProtocol(t *testing.T) {
	var gotPath string
	var gotBody []byte
	var gotHeaders http.Header
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
		gotPath=r.URL.Path; gotHeaders=r.Header.Clone(); gotBody,_=io.ReadAll(r.Body)
		w.Header().Set("Content-Type","application/json"); w.WriteHeader(http.StatusAccepted); _,_=w.Write([]byte(`{"accepted":true,"deliveryId":"guid-1","providerDeliveryId":"opaque-1"}`))
	}))
	defer server.Close()
	c:=Client{BaseURL:server.URL}
	_,status,err:=c.Delivery(context.Background(),DeliveryRequest{RoutingKey:"app-a",DeliveryID:"guid-1",Event:"push",Signature:"sha256=abc",Body:[]byte(`{"x":1}`)})
	if err!=nil { t.Fatal(err) }
	if status!=http.StatusAccepted { t.Fatalf("status=%d",status) }
	if gotPath!="/probot/apps/app-a/deliveries" { t.Fatalf("path=%q",gotPath) }
	if gotHeaders.Get("X-GitHub-Delivery")!="guid-1" || gotHeaders.Get("X-GitHub-Event")!="push" || gotHeaders.Get("X-Hub-Signature-256")!="sha256=abc" { t.Fatalf("headers=%v",gotHeaders) }
	if string(gotBody)!=`{"x":1}` { t.Fatalf("body=%q",gotBody) }
}

func TestOperatorRequestsUseBearerToken(t *testing.T) {
	var auth string
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ auth=r.Header.Get("Authorization"); _,_=w.Write([]byte(`{"processed":0}`)) }))
	defer server.Close()
	c:=Client{BaseURL:server.URL,Token:"secret"}
	if _,_,err:=c.DeliveryRun(context.Background(),4); err!=nil { t.Fatal(err) }
	if auth!="Bearer secret" { t.Fatalf("authorization=%q",auth) }
}

func TestPublishDeliveryRequiresDurableIdentity(t *testing.T) {
	server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ _,_=w.Write([]byte(`{"accepted":true,"deliveryId":"guid-1"}`)) }))
	defer server.Close()
	c:=Client{BaseURL:server.URL}
	_,err:=c.PublishDelivery(context.Background(),DeliveryRequest{RoutingKey:"app-a",DeliveryID:"guid-1",Event:"push",Signature:"sha256=abc",Body:[]byte(`{}`)})
	if err==nil { t.Fatal("expected missing provider delivery identity error") }
}
