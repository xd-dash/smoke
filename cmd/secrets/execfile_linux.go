//go:build linux

package secretscmd

import (
 "fmt"
 "io"
 "os"
 "strings"

 "golang.org/x/sys/unix"
)

func runFile(plaintext []byte, args []string) error {
 fd, err := unix.MemfdCreate("smoke-secret", unix.MFD_ALLOW_SEALING)
 if err != nil { return err }
 f := os.NewFile(uintptr(fd), "smoke-secret")
 defer f.Close()
 if _,err= f.Write(plaintext);err !=nil{return err}
 if _,err= f.Seek(0,io.SeekStart);err !=nil{return err}
 if _,err=unix.FcntlInt(f.Fd(), unix.F_ADD_SEALS,
   unix.F_SEAL_WRITE|unix.F_SEAL_GROW|unix.F_SEAL_SHRINK|unix.F_SEAL_SEAL);err !=nil{return err}
 path:=fmt.Sprintf("/proc/self/fd/%d",fd)
 resolved:=make([]string,len(args))
 found:=false
 for i,arg:=range args {
  if strings.Contains(arg,"{}"){found=true}
  resolved[i]=strings.ReplaceAll(arg,"{}",path)
 }
 if !found{return fmt.Errorf("exec-file requires a {} placeholder for the ephemeral memfd path")}
 return replaceProcess(resolved,os.Environ())
}
