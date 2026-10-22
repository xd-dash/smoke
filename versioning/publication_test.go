package versioning

import (
    "errors"
    "sync"
    "testing"
)

type acceptVerifier struct{ allowed bool }
func (v acceptVerifier) Verify(Grant) error {
    if !v.allowed { return errors.New("revoked") }
    return nil
}
type memoryStore struct{
    mu sync.Mutex
    published map[string]string
    channels map[string]string
}
func newMemoryStore() *memoryStore { return &memoryStore{published: map[string]string{}, channels: map[string]string{}} }
func (s *memoryStore) LookupImmutable(ref Reference) (*ImmutableBinding,error) {
    s.mu.Lock(); defer s.mu.Unlock()
    digest,ok:=s.published[ref.String()]
    if !ok { return nil,nil }
    return &ImmutableBinding{Reference:ref,ArtifactDigest:digest},nil
}
func (s *memoryStore) PublishImmutable(b ImmutableBinding) error {
    s.mu.Lock(); defer s.mu.Unlock()
    old,exists:=s.published[b.Reference.String()]
    if exists && old!=b.ArtifactDigest { return ErrBindingConflict }
    s.published[b.Reference.String()]=b.ArtifactDigest
    return nil
}
func (s *memoryStore) CompareAndSwapChannel(stream,channel,expected,newDigest string) error {
    s.mu.Lock(); defer s.mu.Unlock()
    key:=stream+":"+channel
    if s.channels[key]!=expected { return ErrChannelConflict }
    s.channels[key]=newDigest
    return nil
}
func TestPublicationRequiresAuthority(t *testing.T) {
    ref,_:=ParseReference("artifact:2026.10.21")
    store:=newMemoryStore()
    grant:=Grant{Reference:ref,ArtifactDigest:"sha256:aaa",EvidenceDigest:"evidence",PolicyDigest:"policy"}
    if err:=(Publisher{Store:store}).Publish(grant);!errors.Is(err,ErrUnauthorized) { t.Fatalf("missing verifier: %v",err) }
    if err:=(Publisher{Store:store,Verifier:acceptVerifier{}}).Publish(grant);!errors.Is(err,ErrUnauthorized) {t.Fatalf("denied grant: %v",err)}
    p:=Publisher{Store:store,Verifier:acceptVerifier{true}}
    if err:=p.Publish(grant);err!=nil {t.Fatal(err)}
    if err:=p.Publish(grant);err!=nil {t.Fatalf("idempotent: %v",err)}
    if err:=p.PromoteChannel(grant,"qualified","");err!=nil {t.Fatal(err)}
    if err:=p.PromoteChannel(grant,"qualified","");!errors.Is(err,ErrChannelConflict) {t.Fatalf("CAS: %v",err)}
    next:=grant;next.ArtifactDigest="sha256:bbb"
    if err:=p.Publish(next);!errors.Is(err,ErrBindingConflict) {t.Fatalf("immutable: %v",err)}
    if err:=p.PromoteChannel(next,"qualified","sha256:aaa");!errors.Is(err,ErrBindingConflict) {t.Fatalf("mismatch: %v",err)}
}
func TestConcurrentPublication(t *testing.T) {
    ref,_:=ParseReference("artifact:2026.10.21.1")
    store:=newMemoryStore()
    p:=Publisher{Store:store,Verifier:acceptVerifier{true}}
    var wg sync.WaitGroup
    results:=make(chan error,2)
    for _,digest:=range []string{"sha256:aaa","sha256:bbb"} {
        wg.Add(1);go func(d string){defer wg.Done();results<-p.Publish(Grant{Reference:ref,ArtifactDigest:d,EvidenceDigest:"e",PolicyDigest:"p"})}(digest)
    }
    wg.Wait();close(results)
    successes,conflicts:=0,0
    for err:=range results {if err==nil {successes++} else if errors.Is(err,ErrBindingConflict) {conflicts++} else {t.Fatal(err)}}
    if successes!=1 || conflicts!=1 {t.Fatalf("success=%d conflicts=%d",successes,conflicts)}
}
