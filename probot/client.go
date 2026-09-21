package probot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Client struct {
	BaseURL string
	Token string
	HTTP *http.Client
}

type DeliveryResponse struct {
	Accepted bool `json:"accepted"`
	Duplicate bool `json:"duplicate"`
	DeliveryID string `json:"deliveryId"`
	ProviderDeliveryID string `json:"providerDeliveryId"`
}

type DeliveryRequest struct {
	RoutingKey string
	DeliveryID string
	Event string
	Signature string
	Body []byte
}

func (c Client) endpoint(path string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" { base = "http://127.0.0.1:3000" }
	u, err := url.Parse(base + "/probot" + path)
	if err != nil { return "", err }
	return u.String(), nil
}

func (c Client) do(ctx context.Context, method, path string, body []byte, headers map[string]string) ([]byte, int, error) {
	target, err := c.endpoint(path); if err != nil { return nil, 0, err }
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body)); if err != nil { return nil, 0, err }
	for k,v := range headers { req.Header.Set(k,v) }
	if c.Token != "" { req.Header.Set("Authorization", "Bearer "+c.Token) }
	h := c.HTTP; if h == nil { h = http.DefaultClient }
	resp, err := h.Do(req); if err != nil { return nil,0,err }
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body); if err != nil { return nil,resp.StatusCode,err }
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return data,resp.StatusCode,fmt.Errorf("probot: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data))) }
	return data,resp.StatusCode,nil
}

func (c Client) Registration(ctx context.Context) ([]byte,int,error) {
	return c.do(ctx,http.MethodPost,"/apps/registrations",nil,map[string]string{"Accept":"application/json"})
}
func (c Client) Delivery(ctx context.Context, in DeliveryRequest) ([]byte,int,error) {
	if in.RoutingKey=="" || in.DeliveryID=="" || in.Event=="" || in.Signature=="" { return nil,0,fmt.Errorf("routing key, delivery id, event, and signature are required") }
	return c.do(ctx,http.MethodPost,"/apps/"+url.PathEscape(in.RoutingKey)+"/deliveries",in.Body,map[string]string{
		"Content-Type":"application/json","X-GitHub-Delivery":in.DeliveryID,"X-GitHub-Event":in.Event,"X-Hub-Signature-256":in.Signature,
	})
}
func (c Client) PublishDelivery(ctx context.Context, in DeliveryRequest) (DeliveryResponse, error) {
	body,_,err:=c.Delivery(ctx,in); if err!=nil { return DeliveryResponse{},err }
	var out DeliveryResponse
	if err=json.Unmarshal(body,&out); err!=nil { return DeliveryResponse{},fmt.Errorf("decode Probot delivery response: %w",err) }
	if out.ProviderDeliveryID=="" { return DeliveryResponse{},fmt.Errorf("Probot delivery response is missing providerDeliveryId") }
	return out,nil
}

func (c Client) DeliveryRun(ctx context.Context, limit int) ([]byte,int,error) {
	if limit<=0 { limit=25 }
	return c.do(ctx,http.MethodPost,"/delivery-runs?limit="+strconv.Itoa(limit),nil,nil)
}
func (c Client) Reconciliation(ctx context.Context, routingKey string) ([]byte,int,error) {
	if routingKey=="" { return nil,0,fmt.Errorf("routing key is required") }
	return c.do(ctx,http.MethodPost,"/apps/"+url.PathEscape(routingKey)+"/reconciliations",nil,nil)
}

func Pretty(body []byte) string {
	var value any
	if json.Unmarshal(body,&value)!=nil { return string(body) }
	out,err:=json.MarshalIndent(value,"","  "); if err!=nil { return string(body) }
	return string(out)
}
