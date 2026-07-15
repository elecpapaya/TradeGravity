# Synthetic sample dataset

These files follow TradeGravity schema version 2.0 and contain three fictionalized reporter summaries with five annual periods, HS2 chapters, seven customs-visible semiconductor-stage proxies, 12 synthetic monthly periods, bilateral counterpart reports for mirror diagnostics, country context, quality signals, and evidence-grounded explanations. Values are synthetic and must not be used for research, policy, financial, or historical claims. The design/EDA stage remains context-only because services and intangible flows are not represented by the HS6 fixture.

The sample is intentionally small, deterministic, and network-independent. CI validates it with the same `cmd/validator` used before production deployment.

To preview it, copy the entire directory contents—including `products/`, `strategic-hs6/`, `semiconductors/`, `bilateral-matrix/`, `mirror/`, and `explanations/`—into the ignored `site/data/` directory and start a static server as described in the root README.

Regenerate the sample without network access:

```bash
go run ./cmd/sampledata -db sample-fixture.db -context examples/sample-data/context.json
go run ./cmd/publisher build -db sample-fixture.db -out examples/sample-data -context examples/sample-data/context.json
go run ./cmd/explainer -dir examples/sample-data
go run ./cmd/validator -dir examples/sample-data -min-reporters 3
```

`cmd/sampledata` refuses to overwrite an existing fixture DB. Remove that disposable file yourself after inspection.
