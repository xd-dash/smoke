package versioning

import (
    "errors"
    "fmt"
)

var (
    ErrUnauthorized = errors.New("publication not authorized")
    ErrChannelConflict = errors.New("channel compare-and-swap conflict")
    ErrInvalidChannel = errors.New("invalid channel")
)

// Grant is an external authority decision. Smoke neither issues nor validates
// signatures for grants: the owning workflow supplies a verifier.
type Grant struct {
    Reference Reference
    ArtifactDigest string
    EvidenceDigest string
    PolicyDigest string
}

// Verifier verifies a grant using the owning workflow's authority.
// Implementations MUST reject revoked and mismatched grants.
type Verifier interface {
    Verify(Grant) error
}

// Store is a backend-neutral publication store. PublishImmutable MUST be
// atomic create-if-absent or idempotent for an identical digest.
// CompareAndSwapChannel MUST atomically compare-and-swap channel state.
// Neither method may silently overwrite immutable history.
type Store interface {
    LookupImmutable(Reference) (*ImmutableBinding, error)
    PublishImmutable(ImmutableBinding) error
    CompareAndSwapChannel(stream, channel, expectedDigest, newDigest string) error
}

// Publisher enforces the verification boundary before delegating publication.
type Publisher struct {
    Store Store
    Verifier Verifier
}

// Publish verifies external authority then publishes one immutable binding.
// A previous binding cannot be changed, including after revocation.
func (p Publisher) Publish(grant Grant) error {
    if p.Store == nil || p.Verifier == nil {
        return ErrUnauthorized
    }
    binding := ImmutableBinding{Reference: grant.Reference, ArtifactDigest: grant.ArtifactDigest}
    if grant.Reference.String() == "" || grant.ArtifactDigest == "" || grant.EvidenceDigest == "" || grant.PolicyDigest == "" {
        return ErrUnauthorized
    }
    if err := p.Verifier.Verify(grant); err != nil {
        return fmt.Errorf("%w: %v", ErrUnauthorized, err)
    }
    existing, err := p.Store.LookupImmutable(grant.Reference)
    if err != nil { return err }
    if err := CheckBinding(existing, binding); err != nil { return err }
    // Lookup is advisory; Store.PublishImmutable owns the concurrent write guard.
    return p.Store.PublishImmutable(binding)
}

// PromoteChannel requires verified authority and an already published,
// exact immutable reference. The channel is an alias, never deploy authority.
func (p Publisher) PromoteChannel(grant Grant, channel, expectedDigest string) error {
    if p.Store == nil || p.Verifier == nil {
        return ErrUnauthorized
    }
    if !validChannel(channel) { return ErrInvalidChannel }
    if grant.Reference.String() == "" || grant.ArtifactDigest == "" || grant.EvidenceDigest == "" || grant.PolicyDigest == "" {
        return ErrUnauthorized
    }
    if err := p.Verifier.Verify(grant); err != nil {
        return fmt.Errorf("%w: %v", ErrUnauthorized, err)
    }
    existing, err := p.Store.LookupImmutable(grant.Reference)
    if err != nil { return err }
    if existing == nil || existing.ArtifactDigest != grant.ArtifactDigest {
        return ErrBindingConflict
    }
    return p.Store.CompareAndSwapChannel(grant.Reference.Stream, channel, expectedDigest, grant.ArtifactDigest)
}

func validChannel(s string) bool {
    if len(s) == 0 { return false }
    for _, c := range s {
        if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') { return false }
    }
    return true
}
