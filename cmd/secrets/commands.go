// Package secrets registers an optional native SOPS command. Secret values
// never flow through Smoke's composition or environment snapshot metadata.
package secretscmd

import (
 "encoding/json"
 "io"
 "errors"
 "fmt"
 "os"
 "os/exec"
 "strings"
 "syscall"

 "github.com/xd-dash/smoke/command"
 "github.com/xd-dash/smoke/secrets"
)

func init() { command.Register("secrets", Run) }

func Run(args []string) error {
 if len(args) < 2 { return errors.New("usage: smoke secrets <seal-json|verify-json|inspect|exec-env|exec-file> <file> [-- command args...]") }
 path := args[1]
 switch args[0] {

 case "seal-json":
  if err := secrets.ValidateSecretPath(path); err != nil { return err }
  if len(args) != 2 { return errors.New("usage: smoke secrets seal-json <output.enc.json>") }
  plaintext, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
  if err != nil { return errors.New("cannot read JSON input") }
  encrypted, err := secrets.EncryptJSON(plaintext, os.Getenv("SOPS_AGE_KEY_FILE"))
  clear(plaintext)
  if err != nil { return errors.New("cannot encrypt SOPS document") }
  if err := secrets.SaveExclusive(path, encrypted); err != nil { return err }
  fmt.Printf("SOPS_SEALED=PASS sha256=%s\n",secrets.DigestCiphertext(encrypted))
  return nil
 case "verify-json":
  if len(args) != 2 { return errors.New("usage: smoke secrets verify-json <encrypted.json>") }
  raw,err:=os.ReadFile(path); if err != nil { return err }
  plain,err:=secrets.DecryptJSON(raw,os.Getenv("SOPS_AGE_KEY_FILE"))
  if err != nil {return errors.New("SOPS verification failed")}
  clear(plain)
  fmt.Println("SOPS_VERIFIED=PASS")
  return nil
 case "inspect":
  if len(args) != 2 { return errors.New("usage: smoke secrets inspect <encrypted.json>") }
  raw, err := os.ReadFile(path)
  if err != nil { return err }
  var envelope struct { SOPS json.RawMessage `json:"sops"` }
  if json.Unmarshal(raw,&envelope)!=nil || len(envelope.SOPS)==0 { return errors.New("not a SOPS JSON document") }
  fmt.Printf("SOPS_CIPHERTEXT=VALID sha256=%s\n",secrets.DigestCiphertext(raw))
  return nil
 case "exec-env","exec-file":
  if len(args) < 4 || args[2] != "--" { return errors.New("expected -- followed by an external command") }
  identity:=os.Getenv("SOPS_AGE_KEY_FILE")
  raw,err:=os.ReadFile(path); if err!=nil{return err}
  plain,err:=secrets.DecryptJSON(raw,identity); if err!=nil{return err}
  defer clear(plain)
  if args[0]=="exec-file" { return runFile(plain,args[3:]) }
  var values map[string]string
  if err=json.Unmarshal(plain,&values);err!=nil{return errors.New("exec-env expects a JSON object of string values")}
  environ:=os.Environ()
  for k,v:=range values {
    if !validEnvName(k) {return fmt.Errorf("invalid environment variable name")}
    prefix:=k+"="
    next:=environ[:0]
    for _,existing:=range environ{if !strings.HasPrefix(existing,prefix){next=append(next,existing)}}
    environ=append(next,prefix+v)
  }
  return replaceProcess(args[3:],environ)
 default: return errors.New("unknown secrets operation")
 }
}

func validEnvName(s string) bool {
 if s=="" {return false}
 for i,r:=range s {if !((r>='A'&&r<='Z')||(r>='a'&&r<='z')||r=='_'||(i>0&&r>='0'&&r<='9')) {return false}}
 return true
}

func replaceProcess(args []string, env []string) error {
 if len(args)==0 || args[0]=="" {return errors.New("missing external command")}
 binary,err:=exec.LookPath(args[0]); if err!=nil{return err}
 return syscall.Exec(binary,args,env)
}

