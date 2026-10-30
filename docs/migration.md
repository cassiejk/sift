# Migration plan: Bash `fa` → Go `sift`

## Goal and non-goals

Build a new private Go CLI named `sift`, leaving Bash `fa` untouched. Preserve equivalent behavior for commands that are intentionally ported; do not claim parity for unimplemented commands. Initially, `sift` supports only local `help` and `version` scaffolding.

## Source of existing behavior

The existing Bash repository is separate (`cassiejk/ngwaf-helper`, if currently accessible). Consult its `AGENTS.md`, dispatcher, help, command implementations, CLI reference, known limitations, and synthetic tests. Do not assume GitHub includes uncommitted Codex changes.

## Phases

1. **Inventory:** map current commands to source modules, HTTP calls, auth, customer scope, selectors, JSON, exit status, request budgets, and tests. No changes to Bash.
2. **Go foundation:** design HTTP transport, distinct API clients, configuration, exact resource/ownership validation, errors, pagination, and output. Do not add network functionality until contracts are characterized.
3. **First port:** select one simple read-only exact-resource command; use injected synthetic HTTP responses and compare documented CLI behavior with Bash fixtures.
4. **Expand:** port command families deliberately, preserving distinct API identity and failure semantics; retain unsupported command visibility.
5. **Distribution:** add OS/architecture builds, checksums, installer and CI only after the command surface and release plan are approved.

## P0 compatibility hazards

- Resource `id` versus `workspace_uid` differs. Proven for one legacy workspace only; do not generalize.
- Sparse Metrics v2 fields and absent buckets are not necessarily zero.
- Page traversal does not establish metric population completeness or ingestion finality.
- Legacy NGWAF request totals are not established as equivalent to Metrics v2 totals.
- Legacy `ngwaf` launcher and Bash `fa` have historical routing/installation behavior; compatibility must be deliberate, not assumed.

## First acceptance gate

- `sift help` and `sift version` work offline.
- Tests pass without network access.
- Unsupported commands fail clearly, never silently delegate or mutate.
- The first functional port has an explicit parity matrix for inputs, output, errors, API request count and scoped ownership.
- No credential or customer data is included in this repository.
