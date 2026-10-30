# Engineering guidance for sift

## Scope

- This repository is a new Go implementation, not a rename or in-place conversion of the existing Bash `fa` repository.
- Keep the existing Bash repository untouched. Do not copy its customer-specific fixtures, secrets, tokens, cookies, or local configuration.
- Implement commands only after reviewing Bash behavior, help, and documentation in the existing repository. Never invent CLI flags or API behavior.

## Safety and identity

- Read-only Fastly security inspections only. Do not mutate customer resources.
- Check exact selected-customer ownership and resource identity before labeling results customer-scoped; unknown/conflicting ownership fails closed.
- Treat public Fastly, legacy SigSci, and internal SigSci as separate authentication/identity domains.
- Report missing observations and incomplete pagination as unknown, not zero or complete.
- Keep stdout machine-readable and stderr for diagnostics/progress. Avoid credentials in logs, errors, fixtures, and docs.
- Preserve cancellation, timeouts, bounded pagination, and explicit request budgets.

## Implementation

- Use Go's standard library by default. Prefer explicit types, `context.Context`, injected HTTP clients, and synthetic fixtures.
- Implement one command at a time with parity tests for inputs, stdout JSON, stderr, exit codes, ownership failures, and request counts.
- Keep modules and tests customer/service/workspace agnostic.
- Do not use authenticated customer API calls for development tests.

## Validation and Git

- Start with narrowly focused tests on changed packages; avoid broad tests unless justified by the changes.
- Preserve uncommitted changes. Do not push, release, or tag without explicit approval.
- Record discrepancies between Bash behavior and documentation; current Bash source is authoritative until a Go replacement is explicitly accepted.
