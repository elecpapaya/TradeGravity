# Synthetic sample dataset

These files follow TradeGravity schema version 2.0 and contain three fictionalized reporter summaries with five annual periods, HS2 chapters, country context, quality signals, and evidence-grounded explanations. Values are synthetic and must not be used for research, policy, financial, or historical claims.

The sample is intentionally small, deterministic, and network-independent. CI validates it with the same `cmd/validator` used before production deployment.

To preview it, copy the entire directory contents—including `products/` and `explanations/`—into the ignored `site/data/` directory and start a static server as described in the root README.

Regenerate the sample without network access:

```bash
go run ./cmd/sampledata -db sample-fixture.db -context examples/sample-data/context.json
go run ./cmd/publisher build -db sample-fixture.db -out examples/sample-data -context examples/sample-data/context.json
go run ./cmd/explainer -dir examples/sample-data
go run ./cmd/validator -dir examples/sample-data -min-reporters 3
```

`cmd/sampledata` refuses to overwrite an existing fixture DB. Remove that disposable file yourself after inspection.
