# Versioning and qualification identity idioms

This is a **general-purpose** Smoke contract for versioning build artifacts, releases and compositions. It is not specific to Agni, Fatline, Podman, Nix or systemd.

## Boundaries

A version is a human-readable release identity. A tag is a name-to-artifact mapping. An artifact digest identifies exact bytes. Qualification evidence describes what was tested; **external workflow authority** decides what may be published or deployed. Smoke must not infer authorization merely from a tag or its own runtime identity.

Smoke's existing composition digest and workspace digest remain observational identities, not release versions or authorization. See `SMOKE_IDIOMS.md`.

## Canonical CalVer

Use UTC `YYYY.0M.0D[.MICRO]` for published release versions. The optional MICRO is a nonnegative decimal integer without leading zeros, for multiple releases on the same UTC day. Example: `2026.10.21`, `2026.10.21.1`, `2026.10.21.2`. Release versions must be unique within a named artifact stream. A later date is not a compatibility guarantee.

## Names and lifecycle

An adapter maps these logical labels into its backend (OCI tags, Git tags, artifact metadata, etc.):

- `<version>`: immutable release identity.
- `candidate-<version>`: immutable candidate identity, bound to a specific artifact.
- `qualified-<version>`: immutable evidence-backed qualification marker. The marker itself is not authority.
- `candidate`: optional mutable convenience alias for local iteration, never deployable.
- `qualified`: optional mutable discovery channel, not a deployment pin.
- Additional channels, e.g. `stable` or `testing`, must be explicitly defined and are mutable pointers only.

Never reuse or rebind an immutable versioned name. A failed, superseded or revoked version remains reserved and auditable. A qualification record may be revoked without mutating historical artifact identity.

## Promotion and deployment

1. Select exact source inputs and produce an artifact; record content identity and provenance.
2. Allocate an unused candidate version; reject conflicting mappings.
3. Run appropriate tests and record evidence, policy revision and immutable artifact digest.
4. The **owning external workflow** decides whether evidence grants publication authority.
5. Publish versioned references without overwriting existing mappings. Promotion of mutable channels must use compare-and-swap or an equivalent serialized guarded update.
6. For a multi-artifact composition, publish an independent immutable manifest listing exact member identities, configuration and policy digests. Qualify the composition separately.
7. Resolve aliases to exact artifacts before deployment. Verify qualification and authority; pin the deployed artifact digest and composition identity so restart does not silently follow a moving channel.
8. Roll back by selecting an earlier authorized immutable composition, never by rewriting versioned references.

Publishing multiple tags is not an atomic deployment transaction. An adapter must provide explicit rollback/recovery semantics for partial publication.

## Adapter ownership

Smoke core defines the general contract and may expose pure parsing/validation and small interfaces. Optional command/provider packages own backend-specific operations; the composition root decides what is linked. No mandatory OCI registry, Podman runtime, Nix environment, daemon, or separate persistent control plane belongs in Smoke core.

**Infra consumers**, including Agni/Fatline, supply artifact namespaces and release manifests, and own their runtime configurations. They reference this policy rather than copying it.

## Current implementation status

Smoke now provides a backend-neutral Publisher, Grant/Verifier and Store contract, with tests for denied publication, immutable conflicts, idempotence, CAS failures and concurrent publications. These interfaces do not constitute a durable backend, an external authorization verifier, a qualified composition, an OCI/Podman adapter, revocation enforcement, or activation. Concrete adapters must atomically enforce the specified invariants; owning-workflow reconciliation and integration qualification remain outstanding.
