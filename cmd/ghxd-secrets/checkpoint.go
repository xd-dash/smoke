package ghxdsecrets

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "net/http"
 "os"
 "strings"
 "time"

 "github.com/xd-dash/smoke/secrets"
)

type checkpoint struct {
 Version int `json:"version"`
 ClientID string `json:"client_id"`
 AccessToken string `json:"access_token"`
 RefreshToken string `json:"refresh_token"`
 AccessExpiresAt string `json:"access_token_expires_at"`
 RefreshExpiresAt string `json:"refresh_token_expires_at"`
}

func loadCheckpoint() (checkpoint,error) {
 var b checkpoint
 path:=strings.TrimSpace(os.Getenv("GHXD_SOPS_FILE"))
 if path=="" {path="/workspaces/.private/sops/ghxd.enc.json"}
 encrypted,err:=os.ReadFile(path)
 if err!=nil{return b,errors.New("encrypted Huram checkpoint unavailable")}
 plain,err:=secrets.DecryptJSON(encrypted,os.Getenv("SOPS_AGE_KEY_FILE"))
 if err!=nil{return b,errors.New("SOPS checkpoint authentication failed")}
 defer clear(plain)
 if err:=json.Unmarshal(plain,&b);err!=nil{return b,errors.New("checkpoint JSON invalid")}
 if b.Version!=1||b.ClientID==""||b.AccessToken==""||b.RefreshToken==""||b.AccessExpiresAt==""||b.RefreshExpiresAt==""{
  return b,errors.New("incomplete Huram v1 credential bundle")
 }
 if _,err:=time.Parse(time.RFC3339,b.AccessExpiresAt);err!=nil{return b,errors.New("invalid access token expiry")}
 if _,err:=time.Parse(time.RFC3339,b.RefreshExpiresAt);err!=nil{return b,errors.New("invalid refresh token expiry")}
 return b,nil
}

func probeCheckpoint(ctx context.Context) error {
 b,err:=loadCheckpoint(); if err!=nil{return err}
 expires,_:=time.Parse(time.RFC3339,b.AccessExpiresAt)
 if !time.Now().Before(expires) {
  fmt.Println("GHXD_LIVE=STALE reason=access-token-expired refresh=not-attempted")
  return errors.New("existing access token expired; canonical checkpoint refresh required")
 }
 for _,resource:=range []string{"user","repos/xd-dash/huram-abi-master"}{
  req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"https://api.github.com/"+resource,nil)
  if err!=nil{return err}
  req.Header.Set("Accept","application/vnd.github+json")
  req.Header.Set("Authorization","Bearer "+b.AccessToken)
  resp,err:=githubClient.Do(req)
  if err!=nil{return errors.New("GitHub API request failed")}
  _=resp.Body.Close()
  if resp.StatusCode!=http.StatusOK{
   fmt.Printf("GHXD_LIVE=FAIL resource=%s http=%d refresh=not-attempted\n",resource,resp.StatusCode)
   return errors.New("stored access token cannot access required GitHub resource")
  }
  fmt.Printf("GHXD_API=PASS resource=%s http=200\n",resource)
 }
 fmt.Println("GHXD_LIVE=PASS token_reused=true refresh=false")
 return nil
}
