# Sift architecture and improvement roadmap

Status: design requirements plus verified Phase 3 implementation status below; planned capabilities are not implemented merely by appearing here. This complements [migration.md](migration.md), which records Bash command contracts and actual Go implementation status. The Bash repository remains unchanged.

## Purpose

Build a maintainable, read-only Go CLI, not a Bash line-for-line port. Preserve externally useful command semantics, documented evidence boundaries, and important failure behavior. Simplify aliases, hidden state, broad fan-out, and duplicated implementation where defensible. Current Go support is help/version, `customer whoami`, local `diff`, and `ngwaf workspace`; do not describe planned commands as available.

## Non-negotiable distinctions

- Selected customer is an **input**; validated customer is an **API-verified identity**. No selected-customer claim before checking the authoritative returned ID. Missing/conflicting ownership fails closed.
- Identifiers are typed by **domain and purpose**: customer ID, public NGWAF workspace ID, opaque workspace UID, legacy site ID, service ID, Inspection Service ID, etc. Similar names do not prove joins; do not silently coerce or join across authentication domains.
- **Configuration**, **ownership**, **product attachment**, **deployment**, **observed traffic**, and **enforcement** are distinct evidence levels. A configured rule or enabled product does not alone prove requests traverse the rule or are blocked by it.
- **Complete**, **partial**, **unavailable**, and **unverified** have different meanings. A failed/empty/absent optional source is not proof of no deployment; missing metrics are not zero. Do not impose a universal JSON envelope on existing commands.
- Keep public Fastly, legacy SigSci, and internal SigSci transports and credentials separate. No implicit credential fallback or cross-domain ownership equivalence.
- Request counts refer to logical HTTP operations unless otherwise specified; distinguish retries and actual attempts. Unverified API semantics stay unverified, even if HTTP returns 200.

## P0: Foundations required before Phase 3 grows

These are acceptance gates for `sift ngwaf workspace <workspace>`, **not** a request to implement a general-purpose framework.

### P0-A — Explicit, verified identity and scope

- Define minimal explicit values for selected customer ID/source, API-confirmed customer ID/name, and resolved workspace identity.
- Verify exact customer ID from the customer detail response before describing data as customer-owned.
- Resolve workspace by exact ID first or unambiguous case-insensitive exact name, as Bash currently does; reject ambiguity and incomplete required inventory. Preserve public `workspace_id` versus `workspace_uid`.
- Report selection and verification distinctly; do not treat a token-visible resource or shared display name as ownership.
- **Done when:** a synthetic valid customer/workspace succeeds; mismatched, missing, duplicate, and cross-customer identities cannot produce a verified report.

### P0-B — Explicit retrieval completeness

- For each required paginated source define termination, page/request cap, observed vs advertised counts, and error behavior from the actual endpoint contract.
- Never emit a successful customer-scoped inventory or derived negative assertion from incomplete required pagination.
- Optional evidence can be reported separately as partial/unavailable only if the command's meaning permits it; define that policy per command.
- **Done when:** missing pages, inconsistent totals, truncated result sets, and optional-source failures have explicit, nonzero/non-complete outcomes, not silent empty arrays.

### P0-C — Bounded cost and observable progress

- Define a logical-request budget, page limit, timeout, and known fan-out for every network command before implementing it.
- Prefer narrow exact-resource reads **only if** API evidence is sufficient to prove the required scope; otherwise use the necessary bounded inventory and disclose the cost.
- Count requests actually issued; fail visibly at budget exhaustion. Separate estimates from observed counts.
- Add human-readable stderr progress/heartbeat for commands that can run for many seconds; no progress text in JSON stdout. Avoid excessive noise for a one-request command.
- **Done when:** a synthetic transport confirms calls stay within the documented bound and cancellation ends work.

### P0-D — Evidence provenance and conclusion limits

- Record the endpoint/source, requested scope, verified identifiers, and status relevant to the conclusion without exposing credentials or sensitive raw responses.
- Do not promote configuration to deployment, product inventory to complete attachment coverage, agents to architecture, or observed traffic to enforcement.
- Keep unresolved mapping/ownership as unresolved even if some constituent API calls succeed.
- **Done when:** Phase 3 output explicitly calls itself a verified customer-scoped *configuration* inspection, not deployment/enforcement validation.

## P1: Introduce when a command needs it

1. **HTTP consistency:** small endpoint-specific clients; explicit status/decode/transport failures, contextual timeouts, bounded bodies, safe redirects, and documented retry policy. No blanket retry/fallback on 401/403 or scope-broadening errors.
2. **Output contract:** argument validation before I/O; stdout result / stderr diagnostic; stable JSON field types, null-vs-absent meaning, status codes and human formatting. Preserve existing command outputs unless an intentional deviation is documented.
3. **Package boundaries:** CLI orchestration, domain operations, transport, validated scope, output, snapshots; pass explicit context and injected clients rather than globals. Avoid catch-all `utils` or speculative interfaces.
4. **Troubleshooting:** a bounded request-count/progress summary for costly commands, with safe diagnostics that do not expose token, request body, raw customer payload, or arbitrary transport error text.

## P2: Defer until there is demonstrated need

- Portable installation, signing, cross-platform releases and updater design.
- Broader telemetry, legacy/internal joins, historical mapping, VCL parsing, and Platform Metrics v2. Keep documented known limitations until independently verified.
- Generic reporting envelopes, shared caches, concurrency/fan-out scheduling, broad retry engines, compatibility aliases, automatic customer-config migration, and extensive CI/security suites.
- A global `--json`/exit-code overhaul: decide command by command to avoid surprising existing scripts.

## Per-command migration checklist

1. Inspect current Bash dispatcher, implementation, help, tests, and known limitations; source takes precedence over docs. Record the Bash baseline commit.
2. State *why* the command is useful, its exact syntax, authentication domain, required scope, API calls/page bounds and any expensive fan-out.
3. Classify the output: configuration, ownership, attachment, deployment, observation or enforcement. Name what it **cannot** prove.
4. Decide required versus optional sources, completeness/failure policy, JSON projection, exit behavior, and intentional deviations from Bash.
5. Implement only the minimal typed Go dependencies required for that command. No Bash subprocess fallback.
6. Validate with a few relevant synthetic tests: success plus material failure modes (scope, pagination, auth, redaction or request cap as applicable). No live authenticated tests or routine whole-suite gates.
7. Update CLI help and migration status; label unimplemented capabilities accurately. Record skipped validation and known risks.
8. Review before any commit/push; follow `AGENTS.md`.

## Near-term order / checkpoints

| Checkpoint | Scope | Status / exit gate |
| --- | --- | --- |
| 0 | Initial CLI and migration assessment | Completed |
| 1 | `customer whoami` | Implemented; offline tests passed, live API not validated |
| 2 | Local `diff` | Implemented; offline tests passed, no snapshot collection |
| 2.5 | This architecture/P0 design | Documentation approved before Phase 3 implementation |
| 3A | Customer selection + verified customer detail | Exact returned identity and explicit selection source |
| 3B | Workspace list + bounded complete pagination | Exhaustion and inconsistent totals fail explicitly |
| 3C | Exact workspace resolution + configuration report | No ambiguous identity; no deployment claim |
| 4+ | Remaining commands, one narrow dependency group at a time | Individual contracts/risks reviewed before implementation |

## Open decisions (do not silently choose)

- Customer selection in Go: environment-only first or opt-in read of Bash's existing selection file; define precedence and whether a Go-owned file is ever written. Never auto-migrate credentials.
- Whether a documented direct workspace detail API can supply sufficient authoritative customer ownership to avoid full inventory. Until verified, retain the Bash-verified inventory-based semantics.
- Endpoint-specific page-total consistency and optional-result policy; do not assume all Fastly APIs paginate alike.
- Output schema/versioning, status codes for partial results and local `diff --check`, and whether aliases should exist at all.
- Which old legacy mapping and Metrics v2 behaviors are worth porting, with independent evidence gates.

## Deliberate exclusions

Do not create generic frameworks or exhaustive tests solely to satisfy this roadmap. Do not copy production credentials, customer-specific extensions, or customer data. Do not change the Bash repository. Roadmap status is not evidence of live API behavior.


## Phase 3 verified implementation status

The remote architecture roadmap is the primary design reference. Its original
P0/P1/P2 requirements remain intact above. The following implementation evidence
was reconciled from the local Phase 3 review; the earlier missing-roadmap claim
applied only to the stale local checkout, not to the remote repository. No
executable changes were needed during this reconciliation.

Customer selection now explicitly follows Bash precedence: nonempty
FASTLY_CUSTOMER_ID, then read-only access to the existing Bash selection file
under XDG_CONFIG_HOME or HOME/.config. No Go-owned selection file or automatic
credential/config migration is introduced. This command-level compatibility
choice is documented rather than silently resolving the broader future config
design. Direct detail-only ownership remains unverified; full scoped inventory
is retained. Other open decisions and P1/P2 capabilities remain deferred.

Checkpoints 3A–3C are implemented and reviewed for the authorized Phase 3 commit
and normal push. Scope and pagination tests passed offline; no live API
acceptance is claimed. Completeness is against reported totals only.

### P0-A — Identity and scope

**Verified:** selection is read by `internal/config/customer.go` and validated
separately in `internal/ngwaf/workspace.go`. A selected customer ID is not trusted
until GET `/customer/{customer_id}` returns that exact ID and a nonempty name.
Complete customer-filtered inventory membership precedes workspace resolution;
exact ID wins over an ASCII case-insensitive exact name. Partial/ambiguous names
and duplicate IDs fail. Detail must return the resolved ID; explicit owner
metadata and supplied names cannot conflict. Workspace ID and detail UID remain
separate fields; missing/null UID is not fabricated.

**Gap fixed:** JSON now distinguishes `provenance.customer_identity.selected_id`
from `api_confirmed_id`, records the customer verification endpoint, and records
selection_source (`environment` or `selected_file` in CLI output; `caller` when
the inspection function is used without the CLI selection layer).
`workspace_identity` records resolution method, inventory membership, verified
detail ID, ownership basis and presence of explicit detail customer_id.

**Limit:** when an owner field is absent, ownership relies on the public
customer_id-filtered inventory contract, not an independent owner field. Scope
verification is not an assertion of universal token visibility or account-wide
completeness. A successful report contains confirmed identity only; failures
emit no partial verified report.

### P0-B — Completeness

**Verified, rejection logic unchanged:** limit=100, maximum 1,000 pages;
required data arrays and stable nonnegative integer meta.total; valid row
identities; no duplicate IDs; no excess rows; no short/empty page before total.
Malformed/duplicate JSON keys, inconsistent totals and repeated rows fail.
No selector is resolved from a partial prefix, even if already encountered.

**Gap fixed:** output now records inventory endpoint and customer filter,
pages_fetched, rows_fetched, reported_total, and the deliberately qualified
`complete_against_reported_total` status. These values are recorded from the
actual accepted traversal, not inferred from the selected detail response.

**Limit:** this proves retrieval consistency against advertised metadata, not
atomic consistency during concurrent edits or visibility beyond the token.
No successful report exists for failed/incomplete pagination. The existing
pagination rejection tests are reused; no new paginator was introduced.

### P0-C — Request costs, cancellation and progress

**Verified:** one customer GET, P workspace-list GETs, one detail GET. Success
cost is exactly `P + 2` logical HTTP client invocations (normally three), maximum
`1000 + 2 = 1002`. Detail failure can also consume the maximum. Page-cap failure
without completion costs at most `1 + 1000 = 1001` and never requests detail.
Earlier failure/cancellation reduces requests. The for-loop enforces the page
cap, and single fixed customer/detail call sites enforce the two additional
calls; the budget is not merely a documentation promise. No application retries,
redirects or other API branches run. Standard transport connection recovery is
not a separately counted logical request.

The inspection uses a 120-second context deadline; each HTTP request uses that
context and at most 60 seconds, with earlier caller/client deadlines preserved.
The public client checks context before each request, and the executable
propagates SIGINT/SIGTERM. Configuration selection occurs before the inspection
context and is a local file read; the deadline is not a filesystem watchdog.
An injected transport must honor context cancellation, as required by Go HTTP.

**Gaps fixed:** requests provenance reports actual successful logical count and
max_logical_requests=1002; a final context check prevents a verified report when
cancellation arrives during the final detail response. A lightweight stderr
start message and page milestones (first, every tenth, completion) provide
progress for potentially long pagination without a goroutine/ticker framework.
Progress reports accepted retrieval, never ownership confirmation before detail
validation. There is no idle heartbeat during a single blocked HTTP request;
its deadline bounds that wait.

### P0-D — Evidence and provenance

**Gap fixed:** provenance originally listed only the detail endpoint, data_class
and customer_scoped flag despite relying on three sources. It now additionally
reports customer_identity, workspace_identity, inventory and requests evidence
above. Existing top-level configuration fields and original provenance keys
remain; these are additive intentional differences from Bash output.

All endpoints are public api.fastly.com GETs in the fixed-host client. Report
source paths use endpoint templates plus validated identity fields; credentials,
headers and raw response bodies are not recorded. Configuration values still
come from detail, while workspace name/ID come from scoped inventory. Identity
selection source is local input, not an authenticated API observation.

**Verified:** data_class remains `configuration_snapshot`. No deployment,
product attachment, observed traffic, architecture classification, enforcement
or efficacy claim is emitted. UID is not a deployment classification or a
substitute for workspace ID. Absence of optional fields remains unknown/null.

### Validation and remaining work

New/revised focused tests only:
`go test ./internal/ngwaf ./internal/cli -run 'TestWorkspaceP0Evidence|TestWorkspaceCLI'`.
Both packages passed. Tests inspect serialized provenance, actual counts, progress,
selection source, and cancellation during detail. Existing passing selection,
resolution, pagination and HTTP tests are reused for unchanged behavior. The
page-cap proof is static against the bounded loop and fixed call sites, not an
exhaustive 100,000-row test. `git diff --check` passes.

No full suite, broad vet, security scan, authenticated Fastly call or Bash test
run. No changes to Bash, tags or releases. Phase 3 publication is now separately authorized. Remaining limitations
are the scoped API ownership contract, token visibility, concurrent inventory
changes, no idle heartbeat and lack of live acceptance; none is silently upgraded
to deployment or completeness evidence. The original P0/P1/P2 requirements above are retained from origin/main.
