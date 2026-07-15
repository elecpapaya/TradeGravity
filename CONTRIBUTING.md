# Contributing to TradeGravity

Thanks for helping improve TradeGravity. Contributions to code, tests, data-source documentation, accessibility, and reproducible bug reports are welcome.

## Before opening an issue

- Search existing issues for the same behavior.
- Include the command, provider, reporter/partner, and period involved.
- Remove API keys, tokens, and personal information from logs.
- For data discrepancies, link to the upstream source and record the reporting period.

Feature requests should explain the user problem and how the change preserves data provenance or makes the project easier to reuse.

## Local development

Requirements:

- Go 1.25.12+
- Python 3 or another local static server for the viewer

Run the checks before submitting a pull request:

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
node --check site/app.js
node --check site/security.js
node --test site/security.test.cjs
go run ./cmd/validator -dir cmd/validator/testdata/valid -min-reporters 1
```

For a local end-to-end run:

```bash
go run ./cmd/collector run
go run ./cmd/publisher build -out site/data
cd site
python -m http.server 8080
```

Generated files under `site/data/` and `tradegravity.db` should not be committed.

## Pull requests

- Keep the change focused and explain the behavior it changes.
- Add or update tests for parsing, calculation, or persistence changes.
- Update README or design documentation when commands, output, or data semantics change.
- Preserve provider attribution and never silently combine observations from different sources.
- Escape third-party text before HTML rendering and allowlist protocols for external URLs.
- Confirm that `go test ./...` and `go vet ./...` pass.

Small pull requests are easier to review. Maintainers may ask to split unrelated changes.

## Project direction

See [ROADMAP.md](ROADMAP.md) for current priorities. A pull request does not need to be listed there, but substantial changes should start with an issue so scope and data semantics can be agreed on first.
