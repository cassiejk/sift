# Migration assessment: Bash `fa` to Go `sift`

## Status and evidence boundary

Phase 0 assessment, 30 October 2026, followed by Phase 1 implementation below.
The assessment sections describe the Bash reference and future migration plan;
help, version, `customer whoami`, and local `diff` are currently implemented in Go.
Unsupported commands continue to fail. No Bash delegation is introduced.

The requested reference `cassiejk/ngwaf-helper` currently redirects through the
GitHub API to `cassiejk/Fastly-Security-Audit`, matching the read-only checkout at
`/Users/cassandra.keller/src/ngwaf-helper`. Source baseline:
`7ce9e25e56858ad5e37b89445f30a5f9e86a441a`, launcher version `24.2.3`.
That checkout already has an uncommitted change in
`docs/roadmap-platform-metrics.md`; it is not a committed compatibility contract.
This assessment uses current launcher, dispatch, help, implementation and test
source, plus `docs/cli-reference.md`, `docs/ngwaf-api-sources.md` and
`docs/known-limitations.md`. Source wins over comments and historical docs.
No authenticated Fastly requests or Bash behavior tests were run. No customer
extension implementation, customer identifiers, local configuration, exports,
credentials, or raw production responses are copied into this repository.

Read both repositories' `AGENTS.md` before subsequent work. Changes to Bash,
tags and releases are outside this phase. Phase 0 documentation publication
was separately authorized. Phase 1 publication was subsequently authorized.
Endpoint descriptions in the assessment document existing Bash behavior; the
Phase 1 status section identifies the implemented Go surface.

## Existing entry points and command inventory

`fa` is an interactive alias for `fastly-audit`, not a distinct implementation.
`fastly-audit` enables `set -euo pipefail`, checks Bash 5.2+, establishes globals,
eagerly sources domain modules, help and dispatch, then calls `dispatch_cli`.
`ngwaf` is a compatibility launcher that executes the same entry point with
`FASTLY_AUDIT_INVOKED_AS=ngwaf`. Invocation name affects customer/snapshot
routing and usage text. Go should not inherit eager loading or shell runtime
requirements. The existing Go help/version are intentionally scaffold behavior,
not Bash byte-for-byte parity.

All paths below are relative to the Bash reference. Unless otherwise noted,
commands map to the matching `cmd_*` function; aliases share implementations.
This is the complete dispatch inventory, grouped by implementation dependency.

| Namespace / routes | Implementations | Principal dependency / output |
| --- | --- | --- |
| `help`, `-h`, `--help`, `help legacy`; namespace help | `lib/help.sh`, `lib/dispatch.sh` | Local text; namespace and legacy syntax differ |
| `version`; `customer whoami`, `customer use`, `customer show` | `lib/account.sh`: `cmd_version`, `cmd_whoami`, `cmd_use`, `cmd_customer` | Version local; token metadata JSON; validated local selection write; selected identity JSON |
| `setup`, `customer setup`; `doctor` | `lib/setup.sh`, `lib/doctor.sh` | Environment/config/install probes; doctor defaults to API probes, only `--local` is zero-network; JSON optional |
| `service list`, `service products`, `service inventory`; top-level `inventory` | `lib/services.sh`: `cmd_services`, `cmd_products`, `cmd_inventory` | Service search/fallback, product inventory, workspace inventory; JSON optional |
| `service domains`, `service vcl-size`, `customer vcl-size` | `lib/services.sh`: `cmd_service_domains`, `cmd_service_vcl_size`, `cmd_customer_vcl_size` | Service resolution, active version, domain/generated VCL retrieval; JSON optional |
| `service ngwaf-architecture` | `lib/ngwaf_architecture.sh`, `lib/ngwaf_architecture_vcl.pl` | Exact service-first resolution, active version fallback, VCL parser, separate product probe; JSON optional |
| `security summary`, `security legacy-map`, `customer security` | `lib/security.sh`: `cmd_security_summary`, `cmd_security_legacy_map`, `cmd_customer_security` | Multi-source inventory and conditional associations; legacy-map human/JSON/CSV; other JSON optional |
| `ngwaf workspaces`, `workspace`, `rule`, `rules`, `account-rules`, `thresholds`, `lists`, `posture` | `lib/ngwaf.sh`: corresponding `cmd_*` | Customer/workspace scope; paginated configuration lists; workspace/rule/posture JSON; workspaces human only |
| `ngwaf workspace-services` | `lib/ngwaf.sh`: `cmd_workspace_services`, `cmd_workspace_services_service_id`; `lib/security.sh` | Product/service/internal-site mapping; optional domains; exact-service path differs from workspace/all; JSON optional |
| `ngwaf link`, `ddos link` | `lib/links.sh`: `cmd_ngwaf_link`, `cmd_ddos_link` | Strict URL validation plus verified ownership; exactly one URL on stdout, no browser |
| `ngwaf agent-diagnostic` | `lib/agent_diagnostic.sh`: `cmd_ngwaf_agent_diagnostic` | Inventoried exact workspace, detail and Agents; JSON in both modes |
| `ngwaf platform-metrics-diagnostic` | `lib/platform_metrics_diagnostic.sh`: `cmd_platform_metrics_diagnostic` | Exact customer-owned detail UID then bounded Metrics cursor pages; diagnostic JSON optional, deliberate nonzero P0 status |
| `ngwaf overview`, `timeseries`, `metrics` | `lib/metrics.sh`: `cmd_overview`, `cmd_timeseries`, `cmd_metrics` | Time windows, account/workspace schema normalization and integrity; JSON optional |
| `ngwaf attack-profile`, `attack-origins` | `lib/attack_analysis.sh`: `cmd_attack_profile`, `cmd_attack_origins` | Metrics plus workspace inventory, signal report or per-workspace top-attacks fan-out; JSON optional |
| `ngwaf count`, `requests`, `signal-count` | `lib/request_search.sh`: `cmd_count`, `cmd_requests`, `cmd_signal_count` | Rolling buffered query, bounded search; count/signal-count JSON; requests auto/JSON/CSV/human rendering |
| `ngwaf rule-audit`, `template-audit` | `lib/rule_audit.sh`, `lib/template_audit.sh` | Rule/configuration reads plus request-search scope and per-target counts; JSON optional |
| `bot rules`, `ngwaf bot-rules`; `bot stats`, `bot workspaces` | `lib/bots.sh`: `cmd_bot_rules`, `cmd_bot_stats`, `cmd_bot_workspaces` | NGWAF rules; exact-service historical stats; Bot workspace cursor inventory or exact-service mode; JSON optional |
| `bot intl-stats`, `bot intl-types` | Customer extension loaded by launcher | Customer-specific, excluded from generic Go migration; do not copy constants or fixtures |
| `ddos export-event` | `lib/ddos.sh`: `cmd_ddos_export_event` | Validated event, cursor-paginated rule list, rule detail/traffic fan-out; protected file or stdout evidence JSON |
| `snapshot ngwaf`, `ngwaf snapshot`; `diff` | `lib/snapshots.sh`: `cmd_snapshot`, `cmd_diff` | Snapshot collects config and writes protected files; diff is entirely local; JSON optional |

Flat compatibility routes in `dispatch_flat`: `version`, `doctor`, `setup`,
`whoami`, `use`, `customer`, `services`, `products`, `inventory`, `workspaces`,
`link`, `workspace-services`, `overview`, `timeseries`, `metrics`, `attack-profile`,
`attack-origins`, `bot-stats`, `intl-bot-stats`, `intl-bot-types`, `account-rules`,
`workspace`, `posture`, `rule`, `rules`, `thresholds`, `lists`, `bot-rules`,
`count`, `requests`, `rule-audit`, `template-audit`, `signal-count`, `snapshot`,
`diff`, and help forms. Top-level `snapshot` has additional namespace handling.
Do not assume every namespaced command has a flat alias. Flags and argument
ordering must be characterized from each parser before porting; the Bash
reference's CLI table is an index, not a parser specification.

## Dependency map

```text
entry point -> runtime/globals -> eager module sourcing -> help/dispatch
 dispatch -> argument checks -> dependency/auth preflight -> customer context
 commands -> core.get / core.http_get_to_file -> curl -> public Fastly API
          -> core.get_all_pages -> workspace selection -> NGWAF config
          -> services.get_customer_services -> service/product discovery
              -> security aggregates / mapping / VCL / customer inventories
          -> request_search scope + window + fetch -> rule/template audits
          -> metrics normalization + windows -> attack analysis
          -> snapshot collection -> protected files -> local diff
 security legacy mapping -> separate internal-session transport/site identity
 customer security -> optional separate legacy SigSci Cloud WAF evidence
```

| Shared surface | Current contract and coupling | Go replacement direction |
| --- | --- | --- |
| Preflight (`lib/dispatch.sh`) | Help/version/doctor/setup bypass general dependency/auth checks; diff only requires jq. Other flat commands require curl/jq/token; whoami/use skip selected-customer confirmation. Bot workspaces owns its preflight. Some help/argument checks run before preflight, others after it. | Parse/validate locally first; explicitly document any ordering change rather than claiming exact failure parity |
| Selection (`core.sh`: `current_cid`, `load_customer_context`, `confirm_customer_scope`) | Nonempty `FASTLY_CUSTOMER_ID` precedes `$XDG_CONFIG_HOME/fastly-ngwaf/customer_id` (HOME fallback); validates `/customer/{id}` exact returned ID and nonempty name; caches `NGWAF_CUSTOMER_CONTEXT_JSON`; scope banner stderr | Immutable validated customer context passed explicitly; separate selection source from verified identity; isolated config writes |
| Public HTTP (`get`, `http_get_to_file`) | `API_BASE`, `FASTLY_API_TOKEN`, `CURL_*`; defaults connect 5s/transfer 60s/retries 1/retry-window 90s. `get` requires 2xx and jq-valid truthy JSON, exits 1 on failure. Probe returns HTTP status separately; transport failure yields `000`, which callers must interpret. | Injected HTTP client, context and typed transport/status/decode errors; preserve required-versus-optional probe semantics without shell sentinel values |
| NGWAF page traversal (`get_all_pages`) | limit/page 100, at most 1000 pages; combines `{meta:{limit,total,pages_fetched},data}`. Rejects premature empty/short pages versus advertised total; absent total uses short-page termination; changing total warns and uses latest. | Endpoint-specific bounded page collector; completeness explicit; no stdout rows before required traversal succeeds |
| Workspace resolution (`resolve_workspace_json`) | Full selected-customer workspace inventory even for an exact ID; exact ID wins, otherwise case-insensitive exact name; missing/duplicate/incomplete identity fails; scope stderr | Verified resolver returning ID/name/input/resolution metadata; never silently replace with direct lookup if claiming request parity |
| Services (`services.sh`) | `/search/services` cursor traversal; safe non-auth failure may use token-visible `/service`, exact customer filtering and enrichment. Authentication failure must not trigger scope-broadening fallback. Product probes add fan-out; selection behavior varies by command. | Service-specific discovery and exact-resource validation; token-visible provenance retained; separate reusable active-version resolver |
| Authentication domains | Public `api.fastly.com`: `FASTLY_API_TOKEN`; legacy dashboard SigSci: `SIGSCI_EMAIL`, `SIGSCI_TOKEN`, `SIGSCI_CORP`; internal console: expiring `FASTLY_DSTOKEN`, `FASTLY_VOUCH_COOKIE` session credentials. Modes/IDs across these domains are different. | Separate client/config types and allowed hosts; no credential fallback or automatic cross-domain joins |
| Request-search mutable state | `REQUEST_SEARCH_*` customer/workspace/input/query/window fields; defaults limit 25/page size 100/max 1000; normalizes `rule:` to `ruleid:` with stderr notice; 300-second ingestion buffer; audits reuse prepare/execute functions and globals | Explicit SearchScope, Query, Window, Result values and injected clock; bounded retrieval separate from count semantics |
| Date/parser/UI globals | `CORE_METRICS`, `BOT_HIST_FIELDS`, date-window fields, `TIMESERIES_*`, diagnostic parser state, `USAGE_PROGRAM`, `PROGRAM_NAME`, progress job state/traps, `NGWAF_*` controls; command substitutions can change cache lifetime | Per-invocation options/context, clock and progress sink; avoid process-global state and hidden dynamic scope |
| Files and tools | jq, curl and shell/date utilities; optional table formatter, SHA tools for snapshots; Perl VCL parser; export/snapshot temp files and permissions | Standard library JSON/HTTP/time/URL/crypto/files; VCL parsing is a separate design task; no external Go dependencies |

Request costs below count logical requests, exclude retry attempts, and assume
one customer-context load in a normal invocation. Do not treat them as measured
runtime traces. Most scoped commands add the initial customer GET; optional
probes and service discovery can dominate costs. Exact service mapping can
still fan out across internal site Inspection Service references. Required
inventory success is separate from unavailable optional Agents/Cloud evidence.

## Output, status and diagnostic contracts

There is no universal Bash JSON envelope or JSON error schema. Preserve each
command's projection, flags and evidence semantics separately; do not rename
fields globally. JSON key order/indentation is secondary to field presence,
types, array ordering and values, except where a textual contract is explicit.

| Contract | Observed source behavior to capture before migration |
| --- | --- |
| `customer whoami` | Default JSON `{name, scope, expires_at}` from `/tokens/self`; missing projected keys become null. No customer selection or scope banner required. Not a customer ownership assertion. |
| `customer show` | Default `{id,name,source}` from validated context; source is `environment` or `selected_file`. `customer use` instead prints a selection message and writes the selected ID only after GET success. |
| `ngwaf workspace` | Default keys `workspace`, `workspace_id`, `workspace_uid`, `workspace_input`, `mode`, `immediate_blocking`, `default_blocking_response_code`, `provenance`; UID absent/null becomes null, empty remains empty. Provenance labels a customer-scoped configuration snapshot, not enforcement. |
| `ngwaf rule` | Returned rule object augmented/overwritten with `context` and `provenance`; exact returned rule ID required. Preserve unknown API fields deliberately if adopting this passthrough contract. |
| Rules/lists/thresholds | Often JSON summaries by default; `rules --json` emits normalized full pagination plus context/provenance. `rules --list` appends a human table after summary JSON, so it is not a single JSON document. Workspaces has no `--json` mode. |
| Count/requests | Count includes customer/workspace/input, days/query, count, normalization status, rolling window and provenance; count is `meta.total` of retained searchable records, not raw traffic. Requests includes nullable total_matches, rows_returned, meta, bounded pagination and per-row projected fields plus raw. CSV/human/file modes are distinct contracts. |
| Timeseries/attack reports | Preserve account `dimensions` plus `values[]` versus direct workspace rows; reject malformed/non-numeric/duplicate/out-of-window data. Missing buckets stay unknown, not zero. Returned attack signals are not proven blocked/malicious traffic. Signal coverage and request totals have separate completeness. |
| Security/Bot/Agents | Preserve explicit true/false/null/missing distinctions, per-source status and provenance; unavailable optional evidence is not absence. Agents returned counts and reported totals are separate from deployment/architecture claims. No observed mapping is not undeployment. |
| Snapshot/diff | Snapshot schema `1.0`; customer/workspace identity and configuration sections, list-entry redaction/counts and safety metadata. Diff emits command/schema/customer/workspace/before/after/list_entries/changes/provenance/changed; compares five identity-indexed collections plus workspace config. Reject unsupported schema, duplicate or missing stable identities, cross-customer and cross-workspace comparison. Stable identity precedence is id/reference_id/name/description. |
| URL/export output | Link stdout is one translated URL, never browser side effects. Snapshot/export output files require protected creation and overwrite safeguards; never replay real exports as fixtures. |

Success normally returns 0. `die` and dispatch argument failures return 1;
`set -euo pipefail` also exposes shell/tool failures, so there is no exhaustive
normalized failure-code taxonomy. Do not promise all failures are exactly 1.
`diff --check` emits the report then returns 1 when changes exist; without
`--check`, changes alone are success. Architecture inspection can emit JSON then
return nonzero for required VCL/version failures. The Platform Metrics P0
probe deliberately returns 1 even after successful data retrieval while
`p0_validated=false`; HTTP 200 is not command success. Progress wrappers handle
interrupts (INT 130, TERM 143); `get` transport errors are normalized to exit 1,
so not every interruption path has the same exit code. Go scaffold invalid
commands/arguments currently return 2: preserve that behavior until an explicit
compatibility decision, rather than silently replacing it with Bash exit 1.

Machine-readable reports and URL results use stdout; customer/workspace scope,
query normalization notices, warnings, progress/heartbeat and errors use stderr.
Human report modes intentionally use stdout. Timing, TTY/color, dates and
program names are nondeterministic inputs that golden tests must control.
HTTP 401/403 response bodies are omitted for credential safety; other failures
may print bodies and debug mode may preview malformed JSON. Do not blindly
carry raw-body diagnostics into Go: define sanitized error categories and test
redaction. Stronger redaction is an intentional documented compatibility delta.
Broad inspections have TTY confirmation semantics; redirected stdin warns and
continues, cancellation is 1 and Ctrl-C 130. No browser launch is permitted.

## Recommended package boundaries (proposed, not created)

Keep `cmd/sift` limited to process exit/signal wiring and `internal/cli` to
parsing, help, stream selection and error-to-exit mapping. Domain code returns
values/errors instead of printing or exiting. Introduce packages when a command
needs them, not as an empty framework.

| Package | Responsibility and dependency direction |
| --- | --- |
| `internal/output` | Per-command JSON DTO/renderers, human/CSV rendering, sanitized diagnostics and progress sinks; no API calls |
| `internal/config` | Injected environment/filesystem selection and source; separate explicit writes; no automatic Bash config migration |
| `internal/fastly` | Public-only HTTP client, bounded body decoding, errors and endpoint-specific pagination; injected `http.Client`/transport and context |
| `internal/scope` | Selected-customer validation and exact ownership assertions; consumes public client interfaces; no guessed resource joins |
| `internal/ngwaf` | Workspace resolver/config projections and rules; consumes scope and public client; ID and UID separate |
| `internal/services` | Service discovery, ownership/version/domain/product queries; scope dependency; expose token-visible completeness |
| `internal/snapshot` | Offline schema validation and semantic diff first; collection orchestration later; no transport dependency for diff |
| Later `internal/search`, `internal/metrics` | Explicit query/window/result and clock; endpoint-specific evidence/completeness, not a generic sum-everything helper |
| Later `internal/ddos`, `internal/bot`, `internal/security` | Domain orchestration using narrow interfaces above; aggregators do not own transport/auth |
| Deferred legacy/internal clients | Separate credential types, host restrictions, schema and validated join interfaces; never reuse public token client |

Avoid a Go `core` package mirroring all Bash globals. Keep small command-owned
interfaces; share transport mechanisms, not assumptions about pagination,
identity or metric semantics. Use `json.Number`/explicit numeric decoding where
counts or dynamic JSON would otherwise pass through float64. Preserve missing,
null and false explicitly; use raw JSON only at documented passthrough edges.

## First three migration candidates

Ranking is by end-to-end complexity and regression risk, not just function size.
Help/version already exist and do not count as new migrations. Rank 1 is now implemented in Phase 1 with synthetic transport tests. Rank 2 is implemented in Phase 2 below. Rank 3 remains future work; no
authenticated Fastly validation has occurred.

| Rank | Command | Complexity / risk and prerequisites | Acceptance criteria / request budget |
| --- | --- | --- | --- |
| 1 | `customer whoami` | Low / low: one public GET and three-field projection; needs public transport/auth injection and CLI status decision, no customer resolver/pagination/config write. | One GET `/tokens/self`, no customer or inventory calls; exact three JSON keys including null expiry; credential missing/401/403/transport/malformed-response tests; diagnostics sanitized, no credential output. Characterize current jq permissiveness before choosing stricter schema validation. |
| 2 | `diff` | Moderate / low operational risk: entirely local and independent of shared API infrastructure; semantic comparison is larger than whoami, but no customer network access or file writes. Can be developed independently before rank 1's HTTP client. | Zero requests/token requirements; schema 1.0 and identity checks, unchanged/added/removed/modified entries, deterministic arrays and changed_fields; preserve JSON report and `--check` status; reject cross-scope/duplicate identities; fixture inputs unchanged. |
| 3 | `ngwaf workspace` | Moderate / medium: small useful config projection, but needs validated selection, complete list pagination and exact ID/name resolver before detail. No service inventory/internal API/search dependencies. | Logical budget `1 + P + 1`: customer GET, P workspace list pages, detail GET (plus retries counted separately). Verify request query/identity, exact ID precedence and case-insensitive exact name, ambiguity/incomplete-page failure, UID missing/null/empty fidelity and stdout/stderr isolation. |

The existing workspace projection trusts the detail body after scoped inventory
resolution; it does not independently compare returned detail ID/customer fields.
Go must fail closed on conflicting identity and decide how absent identity can
be established from an authoritative scoped response. Record that strengthening
as a deliberate delta, not already-existing validation. `customer use` likewise
fetches a customer before writing but lacks `load_customer_context`'s exact-ID
validation; defer it until validated, atomic configuration writes are designed.

Next incremental config candidate: `ngwaf rule`, reusing the workspace foundation
and exact returned-rule-ID validation. `ngwaf link` is not offline despite its
small URL transformation. `ddos link` adds event ownership and optional service
fallback; service domains/VCL size often invoke broad discovery even with an ID.
Do not choose these as infrastructure-free first ports. `doctor --local` can be
independent, but Bash-install checks need a Go-specific product decision rather
than a literal port. Setup, broad inventories, mapping, metrics, search/audits,
exports and aggregate security commands need shared infrastructure first.

## Synthetic testing approach

Reuse scenario design and assertions, not whole Bash implementations or fixture
directories. Review each selected body for synthetic provenance; some mocked
tests still contain real-looking customer names/IDs. Replace them with new
customer/service/workspace-agnostic values before adding Go testdata. No fixtures
are copied in Phase 0. Never import production captures or local config.

| Bash source to consult | Reusable synthetic scenarios |
| --- | --- |
| `tests/authentication.sh`, `tests/json-transport.sh` | Missing token zero-call behavior, HTTP categories, malformed/sensitive bodies redaction, large JSON through streams rather than process arguments |
| `tests/snapshots.sh` | Generated test-customer snapshots; before/after enabled-state and added-rule changes, schema/scope failures, list redaction, protected creation; take only diff scenarios for rank 2 |
| `tests/ngwaf-workspace.sh` | Synthetic workspace detail projection and one-detail-call assertion, UID value/empty/missing cases; resolver is stubbed, so this alone does not cover ownership/pagination |
| `tests/namespaces.sh`, `tests/smoke.sh`, `tests/count.sh`, `tests/rules.sh` | Route/argument rejection, stdout/status behavior, workspace selection and list pagination; extract narrowly relevant cases |
| `tests/service-search.sh`, `tests/service-inventory.sh`, `tests/mapping-ownership.sh` | Cursor loops/fallback safety, exact customer ownership and failure-before-output; later service foundation |
| `tests/requests.sh`, `tests/rule-audit.sh`, `tests/template-audit.sh` | Bounded page loops, normalized queries, missing totals, explicit windows and audit request counts; later search foundation |
| `tests/metrics.sh`, `tests/fixtures/timeseries/*.json` | Account/workspace valid/empty/missing/null/malformed shapes and synthetic signals report; inspect/sanitize selected files, preserve unknown intervals and reject duplicate values |
| `tests/boolean-evidence.sh`, `tests/bot-workspaces.sh`, `tests/agent-diagnostic.sh`, `tests/ngwaf-architecture.sh` | false/null/missing fidelity, optional evidence unavailable, count limits, active-version/VCL separation; later domains |
| `tests/link.sh`, `tests/ddos-link.sh`, `tests/ddos-export.sh` | URL rejection, conflicting event/service ownership, export bounds and protected output; sanitize identities rather than copying examples wholesale |

Use standard library `testing`, injected RoundTripper (or loopback httptest),
fixed clock, temp directories and captured stdout/stderr. Unhandled requests
must fail the test; never fall through to the network or read ambient tokens.
Assert method, path, query, credential header destination, page/call count and
absence of downstream requests on failure. Golden JSON compares structure,
including absent keys/null/false/numeric fidelity; preserve semantically ordered
arrays. Human/CSV/URL outputs need separate text assertions. Negative security
cases take priority over exhaustive formatting permutations. Add cancellation,
bounded retries/body size, repeated cursors and incomplete page tests when the
transport/collector is implemented. No authenticated live acceptance is implied
by passing mock tests.

Bash tests are reference material, not scripts to execute from this task. Its
AGENTS validation policy defaults documentation work to diff review and
`git diff --check`, with no behavior tests. Future executable changes start with
one focused relevant script/test target, stay within that repository's cumulative
60-second Tier 1/2 budget where applicable, and require explicit task permission
for its full regression suite. Do not run a broad Bash suite merely because a
shared helper is referenced. Go implementation tasks should run focused package
tests and the task's explicitly required checks, reporting coverage limits.

## Implementation phases and acceptance gates

1. **Phase 0 — assessment (this change).** Inventory routes, dependency map,
   output/status caveats, fixture sources, first candidates and open decisions
   recorded. Only this document changes; Go behavior preserved; Bash unchanged;
   no API functionality, dependency additions, credential/customer data copying,
   or Git publication. Diff reviewed and whitespace check passes.
2. **Phase 1 — explicit contracts and minimum public foundation.** Approve CLI
   namespace/exit/config decisions for the first command. Add standard-library
   public transport, injected credentials/clock/context, sanitized typed errors
   and output mapping only as needed for `customer whoami`. Accept only with
   single-request success and zero-call/malformed/auth/cancellation tests; no
   hidden customer lookup or alternate authentication domain. No live tests.
3. **Phase 2 — independent local comparison.** Implement `diff` without any
   API dependency or configuration requirement. Accept schema/identity safety,
   deterministic semantic deltas and report-before-`--check` failure. Do not
   implement snapshot collection as a prerequisite; use new synthetic files.
4. **Phase 3 — workspace foundation and first scoped command.** Add read-only
   selection, validated customer context, complete NGWAF pagination, resolver
   and workspace detail projection. Accept rank 3's request budget, negative
   ownership/ambiguity/page tests and explicit compatibility deltas. Port rule,
   rules, thresholds and lists incrementally after their contracts are pinned;
   posture and snapshot collection follow those collectors.
5. **Phase 4 — services and product configuration.** Establish search cursors,
   safe fallback, token-visible provenance, exact service ownership and active
   version selection; then list/domains/products/Bot workspace paths. Accept
   failure-before-output and per-command fan-out budgets. Treat VCL parsing and
   architecture/product association as separate evidence. Customer aggregates
   follow proven domain collectors, not the reverse.
6. **Phase 5 — observations and derived audits.** Introduce explicit time/query
   models and bounded search, then count/requests/signal-count and rule/template
   audits. Legacy metrics require both response shapes, window integrity and
   missing-data semantics before overview/timeseries/metrics or attack reports.
   Accept clocks, exact page/call budgets, partial coverage/provenance and
   nonnumeric/duplicate/missing interval cases. Metrics v2 stays a separate
   diagnostic/contract effort; no assumed legacy equivalence or zero filling.
7. **Phase 6 — complex joins and exports.** DDoS evidence bundles, Agents,
   security aggregates and optional legacy/internal integrations need approved
   identity contracts, separate auth clients, protected files and explicit
   required/optional failure policy. No name-only cross-domain joins; preserve
   unresolved mappings. Customer-specific extensions remain excluded unless
   redesigned as separately approved generic capabilities.
8. **Distribution — separately authorized.** Compatibility launchers, installer,
   CI build matrix, OS/architecture artifacts, checksums and release workflow
   require a product/release plan. No release/tag or Bash replacement is implied
   by any earlier gate. Full parity is claimed only for individually accepted
   commands, never for the entire CLI after a handful of ports.

## Remaining architecture decisions

- **CLI compatibility:** retain namespaced forms only initially or also flat
  aliases? Keep Go usage status 2 while preserving `diff --check` 1? Define a
  per-command compatibility matrix including intentional safety improvements.
- **Config ownership:** separate sift selection path versus opt-in read of Bash
  selection; environment precedence and source reporting; atomic protected
  writes and overwrite policy. Never automatically migrate credentials/config.
- **Strict JSON:** choose endpoint schema requirements, unknown-field passthrough,
  integer handling, duplicate-key rejection, null/missing representation and
  maximum response size. jq permissiveness is not automatically a Go guarantee.
- **Identity evidence:** define minimum customer-owned detail/list evidence and
  exact returned-ID checks per endpoint; UID remains distinct from workspace
  ID. Legacy/internal joins remain blocked without independently verified keys.
- **HTTP policy:** approve retryable conditions/backoff/Retry-After, cancellation,
  timeout scopes and total request budget. Do not equate curl retry count with
  logical API request count or reuse one pagination algorithm across APIs.
- **Partial success:** decide which optional evidence failures produce a valid
  report/nonzero status; retain source-level status rather than invent absence.
  Snapshot diff and P0 diagnostic exit semantics require explicit treatment.
- **Rendering/privacy:** specify stable JSON contracts/versioning, sanitized
  stderr/debug rules, human TTY/color/progress and confirmation behavior; avoid
  raw request records in diagnostics. Explicitly review output-file permissions.
- **Observation semantics:** approve clock/window boundaries, missing buckets,
  sampling and retained counts; v2 metrics semantics and completeness remain
  unresolved regardless of a working identity or HTTP success.
- **Platform scope:** define portable replacements for Bash installation doctor
  and Perl VCL parsing; no extra dependency or whole-shell invocation by default.


## Phase 1 implementation — customer whoami

Implemented syntax: `sift customer whoami`, with no operands or flags. JSON is
always emitted on stdout; no `--json` option or flat `whoami` alias is provided.
Help and version remain available. `diff` was subsequently implemented in Phase 2 below; other migration candidates
remain unimplemented. README and CLI help describe only the implemented surface.

Source contract checked against Bash `lib/account.sh:cmd_whoami`,
`lib/dispatch.sh:dispatch_preflight/dispatch_flat/dispatch_customer_namespace`,
`lib/core.sh:get`, help and `tests/namespaces.sh` argument rejection. Existing
setup/doctor tests mock `/tokens/self`, but there is no dedicated Bash whoami
success suite. Go adds focused offline command tests rather than claiming that
Bash's endpoint behavior has been live-validated.

`internal/fastly.Client.Whoami` sends GET to the fixed public URL
`https://api.fastly.com/tokens/self`, with `Fastly-Key` from `FASTLY_API_TOKEN`
and `Accept: application/json`. No query, customer lookup, discovery or other
API client is used. There is one HTTP attempt, or zero for invalid syntax,
missing credentials or a context already canceled/expired. Redirects are not
followed and no application retries are made. The production default transport
may perform its standard connection recovery; the single-attempt budget is at
the HTTP client invocation level, not a claim about TCP connections.

Output is exactly the three keys `name`, `scope`, `expires_at`. Missing fields
become null; explicit values preserve their JSON types, following Bash's jq
projection without inventing required fields or expiry parsing. Extra fields
are discarded. A top-level JSON object, including an empty object, is accepted.
No selected-customer or ownership claim is made by token metadata.

Intentional differences from Bash:

- Usage/unsupported arguments return 2, consistent with the Go scaffold;
  Bash rejects them with 1. Success is 0; credentials, API/transport/decode and
  output-write failures return 1. Cancellation/timeout returns 1 with a sanitized
  diagnostic rather than a shell/signal-dependent exit. The executable wires
  SIGINT/SIGTERM to request context cancellation.
- Only namespaced syntax is ported; Bash's flat alias and broad `whoami -h`
  help behavior are not ported. Use `sift help` for the available syntax.
- A 60-second context deadline bounds the entire operation (or an earlier
  caller deadline). No Bash `NGWAF_CURL_*` options or retry configuration is read.
- Redirects are rejected to keep credentials at the intended destination;
  non-2xx status is an explicit error. Response bodies, arbitrary transport
  errors, and argument values are never echoed in diagnostics. HTTP 401 and 403
  distinguish authentication from authorization; other statuses report the code.
  This is stronger redaction than Bash's non-auth error-body/debug previews.
- Responses are bounded to 1 MiB and must contain exactly one JSON object.
  Bash's jq pipeline can accept a stream of objects; Go rejects streams and
  top-level non-objects before writing stdout. Field types remain unmodified.
- Help text includes the new command. Unknown-command diagnostics no longer
  echo supplied values, which could contain credentials. No color/progress,
  curl/jq dependency, shell invocation or configuration access is added.

Tests inject an in-memory HTTP transport and environment reader, checking exact
method/URL/headers, one-or-zero calls, output projection, missing fields,
credentials, 401/403/503, redirect rejection, malformed/multiple JSON values,
unexpected shape, response-size bound, transport-error redaction, invalid
arguments before credential access, active cancellation and expired deadlines.
No test reads real credentials or makes a Fastly request.

Phase 1 command implementation is complete and reviewed for publication. Remaining broader
architecture decisions (selection, pagination, retries for future commands,
legacy clients, snapshot diff and workspace inspection) stay deferred. No live
API acceptance, release or tag has been performed. Phase 1 commit and normal
push to main were separately authorized.


### Review and Phase 2 boundary

The Phase 1 review confirmed that `sift customer whoami` follows the namespaced
Bash `fa customer whoami` syntax for a normal invocation. The explicitly listed
alias/help and failure-status differences above remain intentional. This is one
command, not migration of the customer command family.

The validation policy is now recorded in `AGENTS.md`. This documentation/policy
update reuses the earlier passing focused package tests for unchanged Go code;
no new test matrix or infrastructure is required. Future behavior changes need
only directly relevant tests and immediate dependencies, with broader checks
justified by an actual integration risk.

Phase 1 source and documented compatibility differences have been reviewed for
publication. Phase 2 remains the independent local
snapshot `diff` command: establish its schema/identity and `--check` contracts
with small synthetic files, without adding network or snapshot-collection
functionality. The Phase 2 implementation is recorded below. No live Fastly validation is
required as a prerequisite for offline development.


## Phase 2 implementation — local snapshot diff

Implemented syntax matches the normal Bash invocation:

```text
sift diff <before.json> <after.json> [--json] [--check]
```

The two files come first. Either flag order and repeated recognized flags are
accepted as in Bash. Other trailing operands/options are rejected before files
are opened. Default output is human-readable; `--json` emits a single report.
No `--output`, stdin, collection, discovery or pagination functionality is added.
The diff branch does not call the environment reader or public API client;
request count is zero, including failure cases. Inputs are read, never rewritten.

### Source contract and snapshot format

Reviewed Bash `lib/snapshots.sh` (`snapshot_validate_file`, `snapshot_diff_json`,
`snapshot_render_diff_human`, `cmd_diff`), `lib/dispatch.sh` preflight,
`lib/help.sh`, README, CLI reference and `tests/snapshots.sh`. Current source is
authoritative. Tests use newly written small synthetic files modeled on those
scenarios; no customer exports or Bash implementation files are copied.

Only the string snapshot schema version `"1.0"` is supported. Root customer and
workspace objects require nonempty string `id` values. Their IDs must match
across files; this is local scope consistency, not authenticated ownership.
`configuration` contains workspace settings plus these required arrays:

- `rules.workspace`
- `rules.account_applying`
- `thresholds`
- `lists.workspace`
- `lists.account`

Each array element is an object with a unique stable identity within its own
array. Precedence is `id`, `reference_id`, `name`, `description`; null/false
values fall through as with jq `//`. The chosen value must be a nonempty string:
an empty or numeric first candidate is invalid rather than falling through.
Duplicate identities, missing arrays, null in place of an array, malformed JSON,
unreadable files and unsupported versions fail without emitting a report.
Capture time, tool version and names are descriptive metadata, not required
identity. Missing capture fields are projected as null. Original customer and
workspace metadata in the report come from the before snapshot.

### Comparison and output

Collections are indexed by identity. Added, removed and modified arrays are
sorted by identity; changed field names are sorted. Empty deltas remain JSON
arrays, not null. Object key order and collection entry order do not affect
comparison; nested arrays retain order and are compared as values. Nothing is
trimmed, case-folded or normalized out of configuration. Numeric comparison is
exact (no float64 conversion): equivalent decimal spellings compare equal,
while original JSON numbers, booleans, strings, missing keys and nulls remain
intact in output. In particular, integers above 2^53 are not rounded.

JSON follows the Bash envelope: `command`, `snapshot_schema_version`, `customer`,
`workspace`, `before`, `after`, `list_entries`, `changes`, `provenance`, `changed`.
`changes.workspace` contains changed/changed_fields/before/after; each of the five
collection deltas has added/removed/modified/counts. Modified entries include
id/changed_fields/before/after; added and removed entries include id/value.
`provenance.data_class` is `local_snapshot_comparison`, `API_requests_made` is
false, and scope_validation states that customer and workspace IDs must match.
Capture metadata alone does not set `changed`. No list members are reconstructed
from redacted snapshots; metadata and reported counts are still compared. The
human report retains the Bash heading, status labels, collection counts,
modified-workspace-rule table and redaction note.

### Exit contract and intentional differences

Valid comparisons return 0, including differences unless `--check` is present.
With `--check`, configuration differences return 1 **after** emitting the full
human or JSON report; unchanged comparisons return 0. Input/scope/read/write
errors return 1 on stderr. Unsupported syntax returns 2 (the existing Go usage
convention), rather than Bash's 1. Use `sift help` for usage; no additional diff
help flag has been invented.

Safety/precision differences from Bash are explicit:

- Go accepts exactly one JSON object per input and rejects duplicate keys.
  Bash's jq validation can process a stream, while its slurped comparison uses
  only the first snapshot; Go rejects that ambiguity.
- Go requires `configuration.workspace` to be an object and an optional non-null
  `safety.list_entries_included` to be boolean. Bash does not validate those
  shapes up front. Absent/null inclusion flags default to false.
- Missing-to-null changes remain distinct and are included in `changed_fields`.
  Bash's object equality detects such changes, but its field accessor treats a
  missing key like null and can omit that key from `changed_fields`.
- Human output is plain text with a tab-separated modified-rule table, independent
  of Bash color controls and optional `column` alignment. JSON key formatting is
  deterministic but is not promised byte-for-byte identical to jq.
- Diagnostics identify before/after input and failure category without echoing
  file contents, supplied paths or raw parser errors. No shell/jq dependency is
  introduced. Whole reports intentionally contain supplied configuration values;
  handle them with the same care as their input snapshots.

### Focused validation and remaining limits

The snapshot package tests cover unchanged comparisons, meaningful deltas,
missing/null preservation, exact large numbers, numeric spelling equivalence,
collection ordering, deterministic output, human rendering, schema/identity
failures, malformed/multi-document JSON, duplicate keys and cross-scope rejection.
CLI tests cover both output modes, report-before-check failure, unchanged check,
unsupported arguments and missing files; injected environment/transport functions
fail the test if the local branch attempts credentials or HTTP. No Bash behavior
suite or authenticated Fastly request is required or performed.

Phase 2 has been reviewed for the separately authorized commit and normal push
to main. Snapshot collection and the rest
of the customer/NGWAF command families remain unavailable. Next work is the
separately scoped workspace/customer validation foundation from Phase 3; local
snapshots are not proof of current deployment or enforcement. Inputs are decoded
in memory; this phase does not add streaming diff, file-size limits or cross-version
conversion. No full Go suite, broad vet run or security scan is an acceptance gate.


## Architecture checkpoint before Phase 3

See [architecture-roadmap.md](architecture-roadmap.md) for the P0 implementation gates: verified resource identity, explicit retrieval completeness, bounded API cost, and evidence/provenance boundaries. Implement these incrementally for `ngwaf workspace`, rather than building generic frameworks. The roadmap documents future work only; it does not mean Phase 3 has started.
