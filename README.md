# sift

`sift` is an early-stage Go rewrite of the read-only `fa` Fastly security inspection CLI.

**Status:** Help, version, and `sift customer whoami` are implemented. The whoami command reads `FASTLY_API_TOKEN`, makes one public `GET /tokens/self` request, and prints JSON token metadata (`name`, `scope`, `expires_at`). It accepts no flags or operands and requires no selected customer. Tests are offline; no live Fastly validation or release has been performed. The existing Bash `fa` project is separate and unchanged.

## Local development

Requires Go 1.23+.

```sh
go test ./...
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
