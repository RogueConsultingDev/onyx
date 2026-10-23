# AGENTS.md

Instructions for AI coding agents working in this repository.

**onyx** — The uncompromising Golang code formatter

- Go 1.27.3 application, module `github.com/RogueConsultingDev/onyx`

See also `.github/copilot-instructions.md` for pull request description and review guidelines. Where the two overlap, this file wins for code; that file wins for PR content.

## Definition of done

Review the code to remove redundancies and dead code (code that is only used by its own tests).

Run these in order before declaring any code change complete. Do not skip ahead; `lint` will fail on formatting that `format` would have fixed.

```shell
make format   # golangci-lint formatters (gci, golines)
make lint     # all pre-commit hooks, including golangci-lint --fix
make test     # full suite + coverage gate
```

`make test package=./path/to/pkg` tests a single package while iterating, producing a ".coverage" file which you should use to determine coverage. Run the full `make test` and ensure it passes before considering the work complete. To run a single test function, use `go test -run '^TestName$' ./path/to/pkg` for a quick check — it does not satisfy the coverage gate.

## Hard rules

1. **Do not edit template-managed files.** This repo is generated from a cookiecutter template and synced with `cruft`; local edits to these files are silently reverted by the next `cruft update` (see `.github/workflows/cruft-update.yml`):
   `AGENTS.md`, `Makefile`, `.makefiles/**`, `Dockerfile`, `.github/CODEOWNERS`, `.github/pull_request_template.md`, `.github/copilot-instructions.md`, `.github/workflows/**`, `.golangci.yml`, `.pre-commit-config.yaml`, `.mockery.yaml`.
   If one of them genuinely needs to change, say so and explain that the fix belongs in the template repository. `README.md` may be edited.
2. **Never read or print `*.env*` files** (`.env`, `.env.local`, `.env.production`, …). They contain secrets.
3. **Never commit secrets**, credentials, or GCP/registry tokens.
4. **Do not edit `*.gen.go` files.** They are generated; change the source interface and run `make mock.gen` or `make gen`.
5. **Do not silence linters to make a build pass.** `//nolint` requires a specific reason (`nolintlint` has `require-explanation: true` and `allow-unused: false`), so an unused or unexplained directive is itself a lint failure.

## Code conventions

### Patterns the linters reject

These are the rules agents break most often. The full configuration is in `.golangci.yml` (33 linters).

| Rule                                                      | Linter                          | Do this instead                                                                      |
| --------------------------------------------------------- | ------------------------------- | ------------------------------------------------------------------------------------ |
| No inline error assignment: `if err := f(); err != nil {` | `noinlineerr`                   | `err := f()` on its own line, then `if err != nil {`                                 |
| No `fmt.Print*`, `print`, `println`                       | `forbidigo`                     | Use the logger                                                                       |
| Do not import `github.com/rs/zerolog/log`                 | `depguard`                      | Import `github.com/rs/zerolog` and take the logger from context                      |
| No `init()` functions                                     | `gochecknoinits`                | Explicit constructors called from `main()`                                           |
| No shadowing `err` or `ctx` in a way that changes meaning | `govet` (shadow)                | Distinct names; re-declaring `err`/`ctx` in a nested scope is tolerated but avoid it |
| Unchecked errors                                          | `errcheck`                      | Handle or explicitly `_ =` with a reason                                             |
| HTTP/SQL calls without a context                          | `noctx`                         | Pass `ctx` through; `http.NewRequestWithContext`                                     |
| Unclosed `sql.Rows` / missing `rows.Err()`                | `sqlclosecheck`, `rowserrcheck` | `defer rows.Close()` and check `rows.Err()`                                          |
| Non-exhaustive `switch` over an enum                      | `exhaustive`                    | Cover every case or add a `default`                                                  |
| Copy-pasted blocks                                        | `dupl`                          | Extract a helper                                                                     |
| Misused testify assertions                                | `testifylint`                   | `require` for fatal preconditions, `assert` for checks                               |

Complexity and size limits: 50 statements per function (`funlen`), cyclomatic complexity 15 (`gocyclo`), line length enforced by `lll` and auto-wrapped by the `golines` formatter.

In `_test.go` files, `dupl`, `errcheck`, `funlen`, `gocyclo` and `gosec` are disabled — do not contort test code to satisfy them.

Imports are grouped by `gci` in this order: standard library, third party, `github.com/RogueConsultingDev/`, then this module. `make format` handles it.

### Code style

- Avoid single letter variables, unless the scope is very small.
- Do not create xPtr functions, where x is a type. Use go's `new()` instead.
- Use `lo.FromPtr` or `lo.FromPtrOr` for coalescing `nil`s to a zero-value or a default value.
- Avoid overly verbose comments on functions. Do not describe the business logic, stay factual about what it does.
- Add comments on statements explaining why something is done, when necessary.
- Do not comment about what the code used to be and how it was changed, only explain the current functionality in comments.

### Logging

Use slog with a console style output, not json.

### Errors

- Handle every error explicitly.
- **Either log the error or return it, never both** — double-logging an error as it unwinds is a review rejection.
- Wrap errors before returning: `fmt.Errorf("error fetching user %s: %w", id, err)`.

### Testing

Testify (`require` / `assert`) with standard `net/http/httptest` where relevant. Take the context from the test:

```go
func TestHandler(t *testing.T) {
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	newRouter().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
```

- Use random values wherever possible, with `github.com/jaswdr/faker/v2`. This ensures tests can run multiple times with the same databases.
- Prefer table-driven tests (using `t.Run()`) for multiple cases.
- Use `t.Parallel()` if the test doesn't mutates shared state.
- Do not write redundant checks like `require.Error` before `require.ErrorContains`.

**Coverage is enforced and effectively total.** `make test` runs tests through `courtney` and fails if _any_ statement block reports zero coverage; `*.gen.go` files are stripped from the report first. New code without tests will fail the build. Inspect gaps with `make cov.render`.

If your tests use testcontainers, `make test` will build the image from the `Dockerfile`; run `make docker.build` first to avoid a slow first run. The scaffold has no testcontainers dependency — this applies once you add one.

### Mocks

Mockery v3 generates mocks for every interface in `internal/` and `pkg/` into `internal/testing/mocks/` and `pkg/testing/mocks/`, mirroring the source package layout. Generated files are named `<InterfaceName>.gen.go` with struct `Mock<InterfaceName>`.

Add or change an interface, then run `make mock.gen` (it deletes and regenerates every `mocks` directory). Never hand-write mocks nor hand-edit a file in a `mocks` directory.

### Commits and pull requests

#### Commits

Create commits as you see fit, we don't enforce rules on those. They will all be squashed when merged to main anyway.
Do keep in consideration that every push of a commit runs the CI. This can be inhibited by prefixing the commit message with a `[skip ci] ` tag.

#### Pull requests

Large features should be split in several stacked PRs that each focus on a single area, e.g.: the api layer, the clients used in the service layer, the service layer, etc.

```
feat: add user authentication
fix: handle expired tokens correctly
docs: document the new make targets
```

- Use imperative mood for the title message: "Add login page" not "Added login page functionality".
- Follow the template in `.github/pull_request_template.md`. The "why" and "what" sections are mandatory. Remove the others if you don't need them.
- Do not mention unit tests tests in "QA Validation Steps".

## Layout

```
onyx/
├── cmd/                  # CLI and service entry points, one directory per binary
├── internal/             # private code — not importable by other modules
├── pkg/                  # public, reusable packages (treat as a stable API)
├── migrations/           # Atlas migrations (generated, do not hand-edit)
├── Makefile, .makefiles/ # development commands
├── Dockerfile            # multi-stage build
└── .golangci.yml, .mockery.yaml, .pre-commit-config.yaml
```

Most directories above are created as you need them. The scaffold ships `cmd/delete-me/` as a placeholder — rename or delete it when adding the first real binary; do not build on top of it.

Put code in `internal/` by default. Promote to `pkg/` only when another repository needs to import it.

## Commands

| Command                     | Description                                   |
| --------------------------- | --------------------------------------------- |
| `make help`                 | List every available target                   |
| `make format`               | Format code (golangci-lint formatters)        |
| `make lint`                 | Run all pre-commit hooks                      |
| `make test`                 | Run tests with the coverage gate              |
| `make test package=./path`  | Run tests for one package                     |
| `make cov.render`           | Open the coverage report in a browser         |
| `make gen`                  | Run `go generate ./...`                       |
| `make mock.gen`             | Regenerate all mocks                          |
| `make mod`                  | `go mod tidy`                                 |
| `make update`               | Update dependencies and tools                 |
| `make upgrade`              | `cruft update` + dependency update            |
| `make clean`                | `go clean`                                    |
| `make docker.build`         | Build the Docker image                        |
| `make docker.echo.tag`      | Print this project's image tag                |
| `make docker.clean`         | Remove this project's containers              |
| `make migration desc="..."` | Generate a migration from the schema diff     |
| `make migration.new`        | Create a blank migration                      |
| `make migration.recreate`   | Delete the latest migration and regenerate it |
| `make graphql.gen`          | Regenerate GraphQL models and resolvers       |

Targets are conditional on files being present: Docker targets require a `Dockerfile`, migration targets an `atlas.hcl`, GraphQL targets a `gqlgen.yaml`. Run `make help` if a target is missing.

Always create schema changes with `make migration` — never hand-write a file in `migrations/`, because `atlas.sum` must stay consistent and a pre-commit hook runs `atlas migrate validate`.
A migration can be manually edited after it was created, in which case, `atlas migrate hash` must be executed to regenerate `atlas.sum`.

## CI

`.github/workflows/ci-cd.yml` runs on pull requests and pushes to `main`: pre-commit linters, a Docker build, then `make test` with a JUnit report, plus migration validation on PRs, and deploys to dev from `main`. `promote.yml` promotes to higher environments manually. `cruft-update.yml` periodically opens a PR syncing this repo with the template; while that PR is open, `cruft-check.yml` fails on every other PR, so merge the cruft PR first and rebase on `main`.

If CI fails, reproduce it locally with `make lint` and `make test` rather than pushing speculative fixes.
