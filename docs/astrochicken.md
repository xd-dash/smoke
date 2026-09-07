# Agni / Astrochicken composition

Astrochicken is Smoke domain vocabulary. Agni does not implement an Astrochicken environment or Terraform root.

The composed command is:

```text
smoke agni astrochicken deploy
```

The dependency boundary is:

```text
Smoke cmd/agni
    |
    +-- Smoke Astrochicken recipe
    |      - chooses a two-node representative topology
    |      - owns gateway/world test meaning
    |      - owns execution/egress service-address meaning
    |      - owns outside-vs-environment lifecycle policy
    |      - writes the transient Terraform root
    |
    v
Smoke agni.Provider contract
    |
    v
compiled dash-xd/agni provider
    |      - materializes only requested reusable Terraform modules
    |      - invokes Terraform unchanged
    v
Agni generic modules
    regional-network
    regional-internal-addresses
    coreos-node
    regional-cell
    cloud-function-v1-http
    cloud-function-v2-http
```

Agni module names and implementations are generic. They do not contain `astrochicken`, `farcaster`, `world`, `fatline`, or Logma topology policy. A full regional deployment can use a `/28` and twelve node slots, while Astrochicken deliberately chooses the same generic modules with a `/29` and only two nodes.

## Astrochicken network

The Smoke recipe requires an IPv4 `/29`. Google Cloud reserves four addresses, leaving four usable addresses. Astrochicken gives all four an explicit Smoke-owned role:

```text
host offset 2  gateway node primary address
host offset 3  world node primary address
host offset 4  execution service address -> gateway alias /32
host offset 5  egress service address    -> gateway alias /32
```

The execution and egress addresses are reserved as static internal GCE addresses before the gateway VM is created. They are then attached to the gateway network interface as alias `/32`s. Smoke names the intended service roles: the execution address is available for an Nginx-facing execution frontend and the egress address is available for a Squid-facing egress frontend. Agni only knows that the addresses are reserved and attached as aliases.

The guest OS/workload must still configure the alias addresses locally before binding a process to the literal IP. That workload configuration is deliberately separate from the Terraform address-allocation primitive.

The regional subnet is dual-stack and enables Private Google Access. These service addresses remain Farcaster-owned regional identities; serverless functions do not consume them.

## Shadow serverless functions

The Astrochicken root can optionally deploy maps of 1st-gen and 2nd-gen HTTP functions from caller-supplied Cloud Storage source objects. Both use `ALLOW_INTERNAL_ONLY` ingress. The probe VM service account is automatically included as an invoker when `service_account_email` is set; additional IAM principals can be supplied through `function_invoker_members`.

This gives the intended request boundary:

```text
public caller
    -> Cloudflare
    -> gateway/world VM
       -> Nginx execution frontend
          -> local gospace / pyspace / simple-router-builder execution
          -> or authenticated internal invocation of GCF Gen1 / Gen2
    <- function/local response
    <- VM
    <- Cloudflare
    <- public caller
```

The execution service address is not the Cloud Function address. It is the stable Farcaster-side frontend that may choose local execution or invoke a serverless shadow. Likewise, the egress service address is a Farcaster-owned identity suitable for Squid policy. The Cloud Functions remain behind Google's serverless frontend and are invoked over the VPC/private Google path.

The function response returning through the VM does not require the function itself to accept public ingress. For 2nd-gen functions, invocation IAM is `roles/run.invoker`; 1st-gen uses `roles/cloudfunctions.invoker`.

The function maps default empty, so `smoke agni astrochicken deploy` can deploy only the `/29`, service-address reservations, and two CoreOS nodes. Supplying `gen1_functions` and/or `gen2_functions` through normal Terraform variables adds the shadow functions without changing the Smoke command or Agni module boundary.

## Runtime placement boundary

Smoke owns qualification vocabulary for the logical execution provider, but the workload should preserve one HTTP-shaped contract across placements:

```text
logical handler
    -> local Go executor
    -> local Python executor
    -> Gen1 executor
    -> Gen2 executor
```

A gateway may therefore send a cold request to a serverless shadow while warming the local implementation, then serve later requests locally. It may also choose serverless because of load, policy, or a serverless-only marker. The caller remains unaware of placement.

Application authentication terminates at the Farcaster layer. Farcaster uses its VM service account and Google IAM authentication for the internal function hop; public clients do not need Google credentials.

## Scope

Outside a named Smoke environment, Astrochicken may run Terraform lifecycle operations (`deploy`, `plan`, `destroy`, `output`). Inside `smoke env run`, the initial contract is deliberately constrained to `output`; mutation stays outside the environment boundary. Later qualification-only operations can be added without granting environment-scoped infrastructure mutation.

## Terraform state

Astrochicken does not replace Terraform state semantics. Its persistent transient root defaults to `~/.smoke/agni/astrochicken`, or `SMOKE_ASTROCHICKEN_WORKSPACE` when explicitly set. Terraform therefore owns state and lifecycle within that root. A future remote backend can be supplied by the Smoke recipe without changing Agni's provider contract.

## Composition

Smoke core never imports Agni. A client composes the Agni provider package in the normal Go-import manner. That package may blank-import `github.com/xd-dash/smoke/cmd/agni` so adding the provider also adds the `smoke agni` command.

The provider receives a selection of Agni module names rather than an Agni environment name. This is intentional: Agni can contain other reusable modules and Terraform roots while each Smoke recipe materializes only the implementation it needs.
