# Security-cell qualification

Smoke may qualify a security-cell composition, but it does not own the cell's production authority, recovery policy, durable backup state, or deployment lifecycle.

The reusable Smoke responsibility is deliberately narrow:

```text
exact component worktrees / immutable refs
        |
        v
local composition + exact test environment
        |
        v
behavioral qualification
        |
        v
evidence returned to the caller
```

The caller, normally Huram, remains responsible for selecting exact candidate refs, materializing credentials/recovery authority, deciding retained versus disposable lifecycle, and recording deployment evidence.

## Marai lifecycle qualification

For Marai, the minimum lifecycle proof is:

```text
fresh process
    -> BOOTSTRAP
    -> create key
    -> ACTIVE
    -> encrypt known plaintext
    -> rotate key
    -> old ciphertext still decrypts
    -> QUIESCE
    -> application crypto denied
    -> optional MRS1 terminal export
    -> ZEROIZE
    -> DEAD
```

For checkpoint-capable qualification, recovery must cross a real process boundary:

```text
Marai A
    -> ACTIVE
    -> produce MRA1 ciphertext
    -> QUIESCED
    -> terminal MRS1 export
    -> DEAD

fresh Marai B
    -> BOOTSTRAP
    -> tampered MRS1 rejected without state mutation
    -> authorized MRS1 import
    -> ACTIVE in a new authority era
    -> old MRA1 ciphertext decrypts
```

The test must prove that recovered authority identity is not continuity of the old process:

```text
instance(A) != instance(B)
current_era(B) > origin_era(A)
sequence(B) begins in the new era
```

## Capability qualification

A production-shaped qualification should use distinct Redis ACL identities:

```text
application
  KMS.STATUS
  kms_encrypt
  kms_decrypt
  kms_generate_data_key
  exact native commands required by those Functions

administrator
  KMS.CREATE
  KMS.ROTATE
  KMS.STATUS
  KMS.QUIESCE
  KMS.EXPORT
  KMS.ZEROIZE

recovery
  KMS.STATUS
  KMS.IMPORT
```

The ordinary application identity must not create, rotate, quiesce, export, import, or zeroize authority. The steady-state administrator must not receive `KMS.IMPORT`; the recovery identity should be materialized only for an explicit bootstrap recovery operation.

## Atman qualification

Atman remains an authenticated application ingress adapter, not a lifecycle controller.

Smoke should prove:

```text
Marai ACTIVE
    -> Atman /healthz ready
    -> authorized encrypt/decrypt succeeds

Marai QUIESCED or DEAD
    -> Redis process may still be alive
    -> Atman /healthz is not ready
    -> application KMS traffic fails
```

There must be no ordinary Atman HTTP route for Marai create/rotate/quiesce/export/import/zeroize. Graceful draining is orchestration above Marai: stop new application admission, wait for in-flight work, then issue the local privileged `KMS.QUIESCE` transition.

## What Smoke does not persist

Smoke must not become a recovery store. In particular it must not persist as canonical state:

- MRS1 blobs;
- recovery private keys;
- production Marai ACL credentials;
- tenant/caller policy;
- lifecycle ownership or Terraform state locators.

Temporary test artifacts may exist inside one qualification workspace and are destroyed with that workspace unless the caller explicitly collects them as evidence.

## Relationship to Smoke environments

Security-cell qualification follows the normal Smoke environment model. Exact component worktrees and tools are composed into a normal Go workspace; long-running tests execute against an immutable environment snapshot. Do not invent a second security-cell dependency graph inside Smoke.

The composition/workspace identity pair remains observational evidence only:

```text
Smoke composition digest
+ workspace snapshot digest
+ exact candidate SHAs supplied by caller
```

Exact repository SHA/build qualification remains caller-owned.

## Target-backed qualification

Local/container qualification should be exhausted before acquiring cloud authority. If a behavior specifically requires CoreOS/GCE/IAM/network placement, Smoke may run the same qualification recipe on a caller-provided target, but target creation, credentials, and destruction remain Huram/Agni concerns.

Preferred escalation:

```text
repository-local tests
    -> production container image tests
    -> Smoke cross-repository composition
    -> Huram disposable target-backed smoke
    -> retained deployment qualification
```

A failed target-backed test must not trigger discovery-based cleanup. The caller must destroy only the exact deployment/state identity it created.

## NQC remains a later layer

Smoke should not introduce checkpoint propagation while local MRS1 export/import is still under qualification. Once the local contract is green, a later test may add one active Marai mutation authority, immutable checkpoint storage, best-effort Logma/NQC hints, and reconciliation.

That later test must not claim consensus or active-active mutation. NQC-style propagation is a convergence/repair mechanism around one authoritative mutation lineage, not a replacement for writer authority.
