# sift

`sift` is an early-stage Go rewrite of the read-only `fa` Fastly security inspection CLI.

**Status:** Help, version, `sift customer whoami`, and local `sift diff` are implemented. The whoami command reads `FASTLY_API_TOKEN`, makes one public `GET /tokens/self` request, and prints JSON token metadata (`name`, `scope`, `expires_at`). It accepts no flags or operands and requires no selected customer. Tests are offline; no live Fastly validation or release has been performed. The existing Bash `fa` project is separate and unchanged.

## Local development

Requires Go 1.23+.

```sh
go test ./internal/snapshot ./internal/cli
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
