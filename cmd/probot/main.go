package probot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/xd-dash/smoke/command"
	probotclient "github.com/xd-dash/smoke/probot"
)

func init() { command.Register("probot", Run) }

func Run(args []string) error {
	ctx := context.Background()
	if len(args)==0 { return usage() }
	lifecycle:=probotclient.DefaultLifecycle()
	switch args[0] {
	case "run":
		state,err:=lifecycle.Run(ctx); if err!=nil { return err }; return printJSON(state)
	case "stop":
		return lifecycle.Stop(ctx)
	case "status":
		state,err:=lifecycle.Status(ctx); if err!=nil { return err }; return printJSON(state)
	case "verify-restart":
		result,err:=probotclient.VerifyRestart(ctx,lifecycle,probotclient.Client{BaseURL:os.Getenv("PROBOT_URL"),Token:os.Getenv("PROBOT_RUNTIME_OPERATOR_TOKEN")}); if err!=nil { return err }; return printJSON(result)
	case "serve-foreground":
		return probotclient.ServeForeground(ctx,args[1:])
	case "request":
		return request(ctx,args[1:])
	default:
		return usage()
	}
}

func printJSON(value any) error {
	data,err:=json.MarshalIndent(value,"","  "); if err!=nil { return err }
	fmt.Println(string(data)); return nil
}

func request(ctx context.Context,args []string) error {
	if len(args)==0 { return usage() }
	client:=probotclient.Client{BaseURL:os.Getenv("PROBOT_URL"),Token:os.Getenv("PROBOT_RUNTIME_OPERATOR_TOKEN")}
	var body []byte; var status int; var err error
	switch args[0] {
	case "registration":
		body,status,err=client.Registration(ctx)
	case "delivery-run":
		limit:=25
		if len(args)>1 { limit,err=strconv.Atoi(args[1]); if err!=nil { return fmt.Errorf("limit: %w",err) } }
		body,status,err=client.DeliveryRun(ctx,limit)
	case "reconciliation":
		if len(args)!=2 { return fmt.Errorf("usage: smoke probot request reconciliation <routing-key>") }
		body,status,err=client.Reconciliation(ctx,args[1])
	case "delivery":
		if len(args)!=6 { return fmt.Errorf("usage: smoke probot request delivery <routing-key> <delivery-id> <event> <signature> <json-file|->") }
		var payload []byte
		if args[5]=="-" { payload,err=io.ReadAll(os.Stdin) } else { payload,err=os.ReadFile(args[5]) }
		if err!=nil { return err }
		body,status,err=client.Delivery(ctx,probotclient.DeliveryRequest{RoutingKey:args[1],DeliveryID:args[2],Event:args[3],Signature:args[4],Body:payload})
	default:
		return usage()
	}
	if len(body)>0 { fmt.Println(probotclient.Pretty(body)) } else { fmt.Println(status) }
	return err
}

func usage() error {
	return errors.New("usage: smoke probot <run|status|stop|request|verify-restart> ...")
}
