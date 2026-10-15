# Exact source contract (Smoke security-cell qualification)

Huram owns credential acquisition, exact candidate selection, source preparation and promotion. Smoke owns immutable composition/workspace identity and behavioral qualification. Agni owns disposable placement.

The qualification entrypoint `run.sh` consumes five **already materialized, clean Git worktrees**:

| Environment | Source |
| --- | --- |
| `MARAI_DIR`, `MARAI_REF` | xd-dash/marai |
| `PRAJAPATI_DIR`, `PRAJAPATI_REF` | xd-dash/prajapati |
| `LOGMA_DIR`, `LOGMA_REF` | xd-dash/logma |
| `RATELIMITER_DIR`, `RATELIMITER_REF` | dash-xd/ratelimiter |
| `AGNI_DIR`, `AGNI_REF` | dash-xd/agni |

Every `*_REF` must be a full 40-character commit SHA matching the corresponding worktree HEAD. Dirty or missing worktrees fail closed. The caller should record Smoke's own exact SHA, `smoke inspect` composition digest and `smoke env inspect` workspace digest alongside candidate SHAs and Binding digest.

For a Huram Actions source-preparation job, use its existing credential authority and `smoke ghxd bootstrap`, then `smoke ghxd worktree seed --repository <owner/repo> --sha <exact-sha> --destination <dir>`. Alternately consume the exact private-source artifact produced by Huram. No new token secret, direct private checkout workflow, refresh-token logic, or source resolver belongs in this Smoke recipe.

**Existing tests:** the port preserves the original Prajapati/Marai authorization positives and negatives, frozen Binding digest check, real Redis ACL DRYRUN publisher allow/subscriber/admin deny, and Marai lifecycle boundaries. These tests are *not* yet updated to Fatline v2 requirements; the original `tenant:world-17:test` fixture remains for historical parity. Do not treat this as multi-world, Skymill, or retained GCP qualification.

**Next gate:** reconcile Logma v2 graph/transport provider requirements and Skymill stream requirements, then add two-world isolation and 40 MiB noeviction before escalating to Agni CoreOS/GCP. Do not delete the original divergent branch until exact behavioral equivalence and CI are proven.
