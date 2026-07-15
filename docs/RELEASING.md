# Releasing TradeGravity

Tagged releases provide stable citation and change-history points. Create releases only from a reviewed commit on `main` after the public site and scheduled pipeline are healthy.

## 1. Prepare

1. Choose a Semantic Versioning tag. The first public release is expected to be `v0.1.0`.
2. Move relevant entries in `CHANGELOG.md` from **Unreleased** into a dated version section.
3. Add the matching `version` and `date-released` values to `CITATION.cff`.
4. Confirm that documentation, schema compatibility notes, and generated-data interpretation are current.

## 2. Verify

Run from a clean checkout:

```bash
gofmt -l ./cmd ./internal
go build ./...
go test -race ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
node --check site/app.js
node --check site/security.js
node --check site/data-tools.js
node --check site/explorer-tools.js
node --check site/intelligence-tools.js
node --test site/security.test.cjs site/data-tools.test.cjs site/explorer-tools.test.cjs site/intelligence-tools.test.cjs site/structure.test.cjs
python -m pip install --disable-pip-version-check cffconvert==2.0.0
cffconvert --validate
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7
go run ./cmd/validator -dir examples/sample-data -min-reporters 3
```

Confirm that all required pull-request checks pass. Then run the collector, publisher, and validator against the intended provider or verify the latest successful scheduled run:

```bash
go run ./cmd/context
go run ./cmd/collector run -history-years 9
go run ./cmd/collector products -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector strategic -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector matrix -provider comtrade -primary-provider wits -year auto
go run ./cmd/collector tariffs -provider trains -year auto -data-type aveestimated
go run ./cmd/publisher build -db tradegravity.db -out site/data -series-years 10
go run ./cmd/explainer -dir site/data
go run ./cmd/validator -dir site/data -min-reporters 40
```

Preview the site locally and confirm same-period/all modes, exact-period selection, regional/group filters, normalizations, share URL restoration, both exports, trends, HS2/strategic HS6 products, tariff source labels, multi-partner fallback behavior, scenario assumptions, quality signals, explanations, accessible table, and small-screen layout.

## 3. Tag and publish

After the release-preparation pull request is merged:

```bash
git switch main
git pull --ff-only
git tag -a v0.1.0 -m "TradeGravity v0.1.0"
git push origin v0.1.0
gh release create v0.1.0 --verify-tag --generate-notes --title "TradeGravity v0.1.0"
```

Replace `v0.1.0` with the prepared version. Do not move an existing public tag; publish a new patch version for corrections.

## 4. Post-release checks

- Verify the release page, source archive, citation metadata, and changelog link.
- Verify the public GitHub Pages site and its `meta.json` and `latest.json` endpoints.
- Confirm the main-branch site deployment reused the latest published dataset without running provider collectors.
- Confirm the next scheduled collection and deployment succeeds.
- Record any release regression in a public issue and fix it through the normal pull-request process.
