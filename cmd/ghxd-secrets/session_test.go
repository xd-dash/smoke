package ghxdsecrets

import (
 "context"
 "encoding/json"
 "fmt"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "filippo.io/age"
 "github.com/xd-dash/smoke/secrets"
)

type roundTripFunc func(*http.Request)(*http.Response,error)
func (f roundTripFunc) RoundTrip(r *http.Request)(*http.Response,error){return f(r)}

func TestSavedCredentialProbeAndRefresh(t *testing.T){
 tmp:=t.TempDir()
 id,err:=age.GenerateX25519Identity()
 if err!=nil {t.Fatal(err)}
 key:=filepath.Join(tmp,"identity.txt")
 if err:=os.WriteFile(key,[]byte(id.String()+"\n"),0600);err!=nil{t.Fatal(err)}
 path:=filepath.Join(tmp,"ghxd.enc.json")
 t.Setenv("SOPS_AGE_KEY_FILE",key)
 t.Setenv("GHXD_SOPS_FILE",path)
 t.Setenv("GITHUB_CLIENT_ID","Iv1.test-client")
 old:=accessToken{AccessToken:"access-before",RefreshToken:"refresh-before"}
 plain,_:=json.Marshal(old)
 encrypted,err:=secrets.EncryptJSON(plain,key)
 if err!=nil{t.Fatal(err)}
 if err=secrets.SaveExclusive(path,encrypted);err!=nil{t.Fatal(err)}
 originalClient:=githubClient
 t.Cleanup(func(){githubClient=originalClient})
 calls:=0
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path=="/user"{
   if r.Header.Get("Authorization")!="Bearer access-before"&&r.Header.Get("Authorization")!="Bearer access-after"{
    http.Error(w,"invalid",401);return
   }
   w.WriteHeader(200);fmt.Fprint(w, "{}");return
  }
  if r.URL.Path=="/login/oauth/access_token"{
   calls++
   if r.FormValue("grant_type")!="refresh_token"||r.FormValue("refresh_token")!="refresh-before"{
    http.Error(w,"invalid",401);return
   }
   w.Header().Set("Content-Type","application/json")
   fmt.Fprint(w,`{"access_token":"access-after","refresh_token":"refresh-after"}`)
   return
  }
  http.NotFound(w,r)
 }))
 defer server.Close()
 githubClient=&http.Client{Transport:roundTripFunc(func(r *http.Request)(*http.Response,error){
  clone:=r.Clone(r.Context());clone.URL.Scheme="http";clone.URL.Host=strings.TrimPrefix(server.URL,"http://");clone.Host=clone.URL.Host
  return http.DefaultTransport.RoundTrip(clone)
 })}
 if err=probeStoredCredential(context.Background());err!=nil{t.Fatal(err)}
 if err=refreshStoredCredential(context.Background());err!=nil{t.Fatal(err)}
 if calls!=1{t.Fatalf("refresh requests=%d want 1",calls)}
 if err=probeStoredCredential(context.Background());err!=nil{t.Fatal(err)}
 out,_,err:=readCredential()
 if err!=nil{t.Fatal(err)}
 if out.AccessToken!="access-after"||out.RefreshToken!="refresh-after"{t.Fatal("credential pair was not atomically replaced")}
 info,err:=os.Stat(path);if err!=nil{t.Fatal(err)}
 if info.Mode().Perm()!=0600 {t.Fatal("credential mode must be 0600")}
}
