# sift

`sift` is an early-stage Go rewrite of the read-only `fa` Fastly security inspection CLI.

**Status:** Help, version, `sift customer whoami`, local `sift diff`, and `sift ngwaf workspace <workspace>` are implemented. The whoami command reads `FASTLY_API_TOKEN`, makes one public `GET /tokens/self` request, and prints JSON token metadata (`name`, `scope`, `expires_at`). It accepts no flags or operands and requires no selected customer. Tests are offline; no live Fastly validation or release has been performed. The existing Bash `fa` project is separate and unchanged.

## Local development

Requires Go 1.23+.

```sh
go test ./internal/config ./internal/fastly ./internal/ngwaf ./internal/cli
go run ./cmd/sift --help
go run ./cmd/sift version
```

See [migration plan](docs/migration.md) and [engineering instructions](AGENTS.md).

## Boundaries

- Read-only inspection; no configuration mutations.
- No credentials, real customer data, or live API calls in tests.
- Treat selected-customer ownership as independently verified metadata, not an inferred join.
- Differentiate configuration, telemetry, deployment, and enforcement evidence.
- No compatibility guarantee with Bash `fa` until each command is explicitly ported and tested.


## Local snapshot comparison

```sh
sift diff before.json after.json [--json] [--check]
```

Both file operands precede the optional flags. Default output is a human report;
`--json` emits one structured report. The command reads no credentials and makes
zero network requests. Snapshot collection is not implemented.

Inputs must be schema `1.0` snapshots for the same customer and workspace, with
nonempty customer/workspace IDs, a workspace configuration object and all five
configuration collections: workspace rules, applicable account rules,
thresholds, workspace lists and account lists. Entries need unique nonempty
identities selected by `id`, `reference_id`, `name`, then `description`.
Missing required collections are errors, not empty inventories.

The report includes workspace changes and sorted added/removed/modified entries
with counts, original values and changed fields. Collection order and object key
order are ignored; nested array order and missing-versus-null values are
preserved. Capture timestamps and tool versions are reported but do not cause
drift. List contents are compared only as present in the supplied snapshots;
redacted entries cannot establish membership equality.

Exit status is 0 for valid comparisons, including changes. With `--check`,
changes produce status 1 **after** the report is written. Invalid snapshots,
incompatible identities and read/write failures return 1; unsupported syntax
returns 2. Errors go to stderr, with no report on invalid inputs.

Compared with Bash, Go rejects duplicate JSON keys, multiple JSON documents and
invalid workspace/safety shapes; missing-to-null fields are explicitly named in
`changed_fields`. Human output has fixed plain labels and a tab-separated rule
table, without Bash's terminal color or optional external column alignment.
See the migration roadmap for the complete compatibility notes.


## NGWAF workspace inspection

```sh
sift ngwaf workspace <workspace>
```

Use an exact workspace ID or an exact name (ASCII case-insensitive); quote names
containing spaces. No flags or extra operands are accepted. Output is always
JSON, matching the Bash command's normal syntax; there is no human report mode
or `--json` option for this command.

Requires `FASTLY_API_TOKEN` and a selected customer. A nonempty
`FASTLY_CUSTOMER_ID` takes precedence over the existing Bash selection file:
`$XDG_CONFIG_HOME/fastly-ngwaf/customer_id`, or
`$HOME/.config/fastly-ngwaf/customer_id` when XDG_CONFIG_HOME is empty. Reading
this file is the only compatibility configuration access; nothing is copied,
written or migrated. `customer use` and setup are not implemented in sift.

The command validates the selected customer's exact ID/name metadata, reads
**all** customer-filtered workspace pages, resolves exact ID before exact name,
and retrieves only that workspace's detail. It makes `P + 2` read-only requests:

- `GET /customer/{customer_id}`
- `GET /ngwaf/v1/workspaces?customer_id=...&limit=100&page=N` (P pages)
- `GET /ngwaf/v1/workspaces/{workspace_id}?customer_id=...`

The one-page case is three requests, even for an exact ID. The hard bound is
1,002 HTTP client attempts, with no application retries or redirects, a
120-second overall deadline and a 60-second deadline per request (earlier caller
or injected-client deadlines also apply). Failure stops subsequent requests.
No service, product, Agents or legacy API is queried.

JSON fields are `workspace`, `workspace_id`, `workspace_uid`, `workspace_input`,
`mode`, `immediate_blocking`, `default_blocking_response_code`, and `provenance`.
Missing optional configuration/UID fields are null, never zero; an empty UID
remains empty. Workspace ID and UID are distinct. The report describes a
customer-scoped configuration snapshot; it does not prove deployment,
architecture, live traffic or enforcement.

Ownership relies on validated customer identity plus membership in the complete
customer_id-filtered inventory and an exact matching detail ID. Explicit owner
metadata, when returned, must agree; empty/null/malformed explicit owner
values fail. If the API omits the owner field entirely, scoped membership is the
ownership evidence. Names alone never establish ownership. Conflicting detail
names and duplicate IDs fail. Pagination requires a stable nonnegative integer
`meta.total`, explicit `data` arrays and exact unique-row completion; missing or
inconsistent metadata fails rather than becoming an empty inventory.

Success returns 0. Argument errors return 2 before credentials or HTTP access;
selection, API, ownership, decoding and incomplete-pagination failures return 1
without a JSON report. Sanitized errors and a scope-verification summary go to
stderr. Go is intentionally stricter than Bash on detail identity, contradictory
ownership metadata and pagination totals. See the roadmap for exact differences.
