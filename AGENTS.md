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

- Prioritize fast, incremental development. Documentation-only changes require diff review and `git diff --check`, not behavior tests.
- For Go changes, run focused tests for changed behavior and immediate dependencies. New commands need tests for the command and its immediate dependencies; HTTP, authentication, and ownership changes need focused checks for the specific behavior changed.
- Do not automatically run `go test ./...`, `go vet ./...`, or the full suite. Broader validation requires an actual cross-package change or an identified integration risk; merely importing a shared package is insufficient.
- Do not repeat successful validation for unchanged code, including after documentation/comment edits, commits, or pushes. Git publication does not independently require additional tests.
- Do not create mandatory broad pre-commit/pre-push hooks or full-suite CI gates without explicit authorization.
- Use standard `testing` and small synthetic HTTP fixtures. Cover success and important failures, preserving critical authentication, credential-redaction, ownership, and incomplete-result checks where applicable. Do not construct speculative security-test matrices or tests for future functionality, or add security scanning tools without a specific need.
- Do not make authenticated Fastly requests during automated tests. Inject synthetic transports and isolate tests from ambient credentials.
- At task completion, concisely report tests actually run and their results, reused validation, intentionally skipped checks, and known risks. Do not run extra tests just to improve the report.
- Preserve uncommitted changes. Do not push, release, or tag without explicit approval.
- Record discrepancies between Bash behavior and documentation; current Bash source is authoritative until a Go replacement is explicitly accepted.
