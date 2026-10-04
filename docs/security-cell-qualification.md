# Security-cell qualification

Smoke qualifies behavior across exact component worktrees; it does not own production authority, recovery policy, durable backup state, organization authorization policy, or deployment lifecycle.

The current Fatline security-cell composition is:

```text
Huram
  exact refs + deployment intent + evidence
        |
        v
Smoke
  immutable composition + behavioral recipe
        |
        v
Agni
  disposable local/CoreOS/GCP placement mechanics
        |
        +-- Logma
        |     durable Fatline binding/events
        |     local Redis execution identity
        |
        +-- Prajapati
        |     semantic identity/authz
        |
        `-- Marai
              isolated crypto authority
```

The ownership rule is strict:

```text
Prajapati
  principal + audience + action + resource authorization

ratelimiter
  rate/lifecycle machine policy
  reusable local Redis ACL enforcement mechanics

Logma
  durable Binding/digest/artifact/alias/lifecycle attachment/events

Marai
  cryptographic keys + lifecycle authority
```

Redis ACL usernames are local execution identities, never portable Fatline principals. Prajapati does not replace ratelimiter's Redis ACL compiler. Logma does not promote Redis credentials into distributed identity.

## Exact local qualification

`qualification/security-cell/run.sh` composes exact checked-out candidates through Agni's local security-cell backend. The test generates temporary Ed25519 identities only inside the disposable qualification workspace:

```text
ed25519:logma/world-17
ed25519:callback/axiom/world-17
ed25519:gateway/world-17
```

The same compiled authorization artifacts are used by both control and runtime layers:

```text
Prajapati authz.Policy
        |
        +-- canonical policy digest
        |
        v
Logma Binding
        |
        +-- immutable Binding digest
        |
        `-- redis_acl_profile
                 |
                 v
        ratelimiter/redisacl
          local Redis rules
```

The qualification must prove all of the following in one run:

```text
exact binding digest                         ✓
Prajapati principal authentication           ✓
exact audience/action/resource authorization ✓
callback encrypt positive path                ✓
Logma secret decrypt positive path            ✓
wrong action denied                           ✓
wrong resource denied                         ✓
wrong binding digest denied                   ✓
real Redis publisher ACL succeeds             ✓
Redis SUBSCRIBE denied to publisher           ✓
Redis ACL administration denied               ✓
marai-app cannot QUIESCE                      ✓
Prajapati has no lifecycle HTTP surface       ✓
marai-admin lifecycle path succeeds           ✓
QUIESCED Marai makes Prajapati unready        ✓
ZEROIZE reaches DEAD                          ✓
```

The Logma secret path uses a durable ciphertext artifact carrying the exact frozen binding digest. Logma presents a short-lived Prajapati credential to request decrypt; it never receives a Marai master key or Marai Redis credential. Plaintext is materialized only into the disposable runtime path with mode `0600`.

## Marai lifecycle qualification

For Marai, the minimum lifecycle proof remains:

```text
BOOTSTRAP
    -> KMS.CREATE
    -> ACTIVE
    -> application crypto
    -> QUIESCE
    -> application crypto denied
    -> optional terminal MRS1 EXPORT
    -> ZEROIZE
    -> DEAD
```

Checkpoint-capable qualification additionally crosses a real process boundary:

```text
Marai A
  ACTIVE -> QUIESCED -> MRS1 export -> DEAD

fresh Marai B
  BOOTSTRAP
    -> tampered MRS1 rejected without mutation
    -> authorized bootstrap-only import
    -> ACTIVE in a new authority era
```

Recovery must prove `instance(A) != instance(B)` and a new authority era. `KMS.IMPORT` is not part of steady-state `marai-admin`.

## Local capability boundaries

Marai keeps its own deliberately static Redis ACL split:

```text
marai-app
  app crypto FCALLs / exact helpers
  KMS.STATUS
  PING

marai-admin
  KMS.CREATE
  KMS.ROTATE
  KMS.STATUS
  KMS.QUIESCE
  KMS.EXPORT
  KMS.ZEROIZE
  PING
```

Prajapati receives only the Marai application directory/socket/password. The admin credential is physically separate and is used only by an explicit Agni/Huram lifecycle executor.

Logma's Redis ACLs are a different domain. A compiled Fatline binding may select local profiles such as publisher/subscriber/tenant/tenant-functions; those profiles constrain the Redis connection that executes already-authorized work.

## Relationship to Smoke environments

Security-cell qualification follows the normal Smoke environment model. Exact component worktrees are composed into the existing workspace/session model; there is no second dependency graph for security cells.

Evidence is the combination of:

```text
Smoke composition identity
+ workspace snapshot identity
+ exact candidate SHAs
+ frozen Fatline Binding digest
+ qualification result
```

Smoke may create temporary keys, passwords, ciphertext artifacts, and runtime files inside one disposable workspace. They are not canonical state and are destroyed with the session unless the caller explicitly collects non-secret evidence.

## Target escalation

Agni's local backend is the first placement target. Once the same behavior is green locally, Huram may apply the exact recipe to a disposable CoreOS/GCP target using Agni's target mechanics.

Preferred escalation:

```text
repository tests
    -> production image tests
    -> Smoke + Agni local security cell
    -> Huram exact-SHA disposable CoreOS/GCP target
    -> retained deployment qualification
```

Target creation, credentials, recovery authority, and exact-identity destruction remain Huram/Agni responsibilities. Cleanup must use the exact deployment identity created by the run; discovery-based deletion is forbidden.

## NQC remains later

Logma/NQC may later distribute checkpoint or secret-revision hints, but Pub/Sub is not secret storage, checkpoint authority, consensus, or active-active Marai mutation. Convergence work begins only after the local authority/authz/secret contract is green.
