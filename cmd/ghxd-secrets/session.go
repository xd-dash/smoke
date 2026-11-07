package ghxdsecrets

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "os"
 "path/filepath"
 "syscall"

 "github.com/xd-dash/smoke/secrets"
)

func credentialPath() string {
 if p:=os.Getenv("GHXD_SOPS_FILE"); p!="" {return p}
 return "/workspaces/.private/sops/ghxd.enc.json"
}

func readCredential() (accessToken,[]byte,error) {
 var t accessToken
 ciphertext,err:=os.ReadFile(credentialPath())
 if err!=nil{return t,nil,errors.New("encrypted GitHub credential unavailable")}
 plain,err:=secrets.DecryptJSON(ciphertext,os.Getenv("SOPS_AGE_KEY_FILE"))
 if err!=nil{return t,nil,errors.New("cannot decrypt saved GitHub credential")}
 defer clear(plain)
 if err=json.Unmarshal(plain,&t);err!=nil{return t,nil,errors.New("saved GitHub credential JSON invalid")}
 if t.AccessToken==""||t.RefreshToken=="" {return t,nil,errors.New("saved credential must include access and refresh tokens")}
 return t,ciphertext,nil
}

func verifyGitHubAccess(ctx context.Context,token string) error {
 for _,endpoint:=range []string{"user","repos/xd-dash/skymill"}{
  req,err:=http.NewRequestWithContext(ctx,http.MethodGet,"https://api.github.com/"+endpoint,nil)
  if err!=nil{return errors.New("cannot create GitHub verification request")}
  req.Header.Set("Authorization","Bearer "+token)
  req.Header.Set("Accept","application/vnd.github+json")
  resp,err:=githubClient.Do(req)
  if err!=nil{return errors.New("GitHub access verification request failed")}
  _,_=io.Copy(io.Discard,io.LimitReader(resp.Body,8192))
  _=resp.Body.Close()
  if resp.StatusCode!=http.StatusOK{
   return fmt.Errorf("GitHub access verification failed: resource=%s HTTP %d",endpoint,resp.StatusCode)
  }
 }
 return nil
}

// Probe performs a fresh real authenticated request; it never exposes token bytes.
func probeStoredCredential(ctx context.Context) error {
 t,_,err:=readCredential()
 if err!=nil{return err}
 if err:=verifyGitHubAccess(ctx,t.AccessToken);err!=nil{return err}
 fmt.Println("GHXD_LIVE_ACCESS=PASS resource=user refresh=false")
 return nil
}

// Refresh is an explicit, serialized, writer-only credential rotation.
// A failed post-exchange publish is fatal: the old refresh token may be invalid.
func refreshStoredCredential(ctx context.Context) error {
 path:=credentialPath()
 if err:=secrets.ValidateSecretPath(path);err!=nil{return err}
 dir:=filepath.Dir(path)
 lock,err:=os.OpenFile(path+".refresh.lock",os.O_CREATE|os.O_RDWR,0600)
 if err!=nil{return errors.New("cannot open credential refresh lock (writer required)")}
 defer lock.Close()
 if err=syscall.Flock(int(lock.Fd()),syscall.LOCK_EX);err!=nil{return errors.New("cannot lock refresh transaction")}
 defer syscall.Flock(int(lock.Fd()),syscall.LOCK_UN)
 t,prior,err:=readCredential()
 if err!=nil{return err}
 // Fail before the irrevocable remote rotation if the checkpoint cannot
 // safely be replaced on this filesystem.
 info,err:=os.Lstat(path)
 if err!=nil||!info.Mode().IsRegular()||info.Mode().Perm()!=0600 {
  return errors.New("encrypted credential must be a mode-0600 regular file")
 }
 stage,err:=os.CreateTemp(dir,".ghxd-preflight-*")
 if err!=nil{return errors.New("credential filesystem is not writable")}
 if err=stage.Sync();err!=nil{stage.Close();os.Remove(stage.Name());return errors.New("credential filesystem cannot sync")}
 _=stage.Close()
 _=os.Remove(stage.Name())
 clientID:=os.Getenv("GITHUB_CLIENT_ID")
 if clientID=="" {return errors.New("GITHUB_CLIENT_ID required for refresh")}
 refreshed:=accessToken{}
 err=requestOAuth(ctx,"https://github.com/login/oauth/access_token",url.Values{
  "client_id":{clientID},
  "grant_type":{"refresh_token"},
  "refresh_token":{t.RefreshToken},
 },&refreshed)
 if err!=nil{return err}
 if refreshed.Error!="" {return fmt.Errorf("GitHub refresh rejected: %s",refreshed.Error)}
 if refreshed.AccessToken==""||refreshed.RefreshToken=="" {
  return errors.New("GitHub refresh did not return a complete replacement token pair")
 }
 if refreshed.AccessToken==t.AccessToken||refreshed.RefreshToken==t.RefreshToken {
  return errors.New("GitHub refresh did not rotate both tokens")
 }
 // Prove the replacement access token works before publishing it.
 if err:=verifyGitHubAccess(ctx,refreshed.AccessToken);err!=nil {
  return errors.New("refreshed GitHub credential failed live verification")
 }
 data,err:=json.Marshal(refreshed)
 if err!=nil{return errors.New("cannot encode refreshed credential")}
 encrypted,err:=secrets.EncryptJSON(data,os.Getenv("SOPS_AGE_KEY_FILE"))
 clear(data)
 if err!=nil{return errors.New("cannot encrypt replacement credential")}
 current,err:=os.ReadFile(path)
 if err!=nil{return errors.New("credential disappeared during refresh")}
 if !bytes.Equal(prior,current) {return errors.New("credential changed during refresh; replacement not published")}
 info,err=os.Lstat(path)
 if err!=nil||!info.Mode().IsRegular()||info.Mode().Perm()!=0600 {
  return errors.New("encrypted credential must be a mode-0600 regular file")
 }
 tmp,err:=os.CreateTemp(dir,".ghxd-refresh-*.enc.json")
 if err!=nil{return errors.New("cannot stage encrypted replacement")}
 defer os.Remove(tmp.Name())
 if err:=tmp.Chmod(0600);err!=nil{tmp.Close();return err}
 if _,err:=tmp.Write(encrypted);err!=nil{tmp.Close();return err}
 if err:=tmp.Sync();err!=nil{tmp.Close();return err}
 if err:=tmp.Close();err!=nil{return err}
 if err:=os.Rename(tmp.Name(),path);err!=nil{
  return errors.New("CRITICAL: GitHub rotated credentials but encrypted checkpoint replacement failed")
 }
 if d,err:=os.Open(dir);err==nil {_=d.Sync();_ = d.Close()}
 fmt.Println("GHXD_REFRESH=PASS rotated_pair=true sops_replaced=true live_api=true")
 return nil
}
