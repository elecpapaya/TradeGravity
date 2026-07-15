# Synthetic sample dataset

These files follow TradeGravity schema version 1.0 and contain three fictionalized reporter summaries for offline development. Values are synthetic and must not be used for research, policy, financial, or historical claims.

The sample is intentionally small, deterministic, and network-independent. CI validates it with the same `cmd/validator` used before production deployment.

To preview it, copy `meta.json` and `latest.json` into the ignored `site/data/` directory and start a static server as described in the root README. Copying the sample replaces any locally generated output in that directory.
