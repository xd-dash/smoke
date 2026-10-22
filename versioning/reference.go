package versioning

import (
 "errors"
 "fmt"
 "strings"
)

var ErrInvalidReference = errors.New("invalid versioned reference")

type Kind string
const (
 Release Kind = "release"
 Candidate Kind = "candidate"
 Qualified Kind = "qualified"
)

// Reference describes a versioned, immutable logical name. It does not
// imply that an artifact exists or that qualification was authorized.
type Reference struct {
 Stream string
 Kind Kind
 Version Version
}

// ParseReference accepts "<stream>:<version>", "<stream>:candidate-<version>",
// or "<stream>:qualified-<version>". Stream is opaque to Smoke; the adapter
// determines which namespaces are valid.
func ParseReference(s string) (Reference,error) {
 pos:=strings.LastIndexByte(s, ':')
 if pos<=0 || pos==len(s)-1 { return Reference{},fmt.Errorf("%w: %q",ErrInvalidReference,s) }
 stream,tag:=s[:pos],s[pos+1:]
 if strings.TrimSpace(stream)!=stream || strings.ContainsAny(stream," \t\n\r") {
  return Reference{},fmt.Errorf("%w: invalid stream",ErrInvalidReference)
 }
 kind:=Release
 if strings.HasPrefix(tag,"candidate-") { kind=Candidate;tag=strings.TrimPrefix(tag,"candidate-") } else if strings.HasPrefix(tag,"qualified-") { kind=Qualified;tag=strings.TrimPrefix(tag,"qualified-") }
 v,err:=Parse(tag)
 if err!=nil { return Reference{},fmt.Errorf("%w: %v",ErrInvalidReference,err) }
 return Reference{Stream:stream,Kind:kind,Version:v},nil
}

func (r Reference) String() string {
 prefix:=""
 switch r.Kind {
 case Candidate: prefix="candidate-"
 case Qualified: prefix="qualified-"
 case Release:
 default: return ""
 }
 return r.Stream+":"+prefix+r.Version.String()
}

// ImmutableBinding is an observed name-to-artifact association. An adapter
// must reject re-binding a previously published reference to another digest.
// Qualification authorization remains external to this contract.
type ImmutableBinding struct {
 Reference Reference
 ArtifactDigest string
}

var ErrBindingConflict = errors.New("immutable reference already bound to another artifact")

// CheckBinding validates an existing binding against a proposed binding.
// A nil existing binding means the reference is unoccupied, not authorized.
func CheckBinding(existing *ImmutableBinding, proposed ImmutableBinding) error {
 if proposed.Reference.String()=="" || proposed.Reference.Stream=="" || strings.TrimSpace(proposed.ArtifactDigest)=="" {
  return ErrInvalidReference
 }
 if existing==nil { return nil }
 if existing.Reference.String()!=proposed.Reference.String() { return ErrInvalidReference }
 if existing.ArtifactDigest!=proposed.ArtifactDigest { return ErrBindingConflict }
 return nil
}
