## Summary

Describe the user-visible or maintainer-visible change and why it is needed.

## Iteration evidence

- Linked issue or [`docs/ITERATION_LOG.md`](https://github.com/elecpapaya/TradeGravity/blob/main/docs/ITERATION_LOG.md) entry:
- Observation and hypothesis:
- Keep, revise, or stop signal:
- Evidence available now (label automated, local, production, or external user):
- Result or next evidence-gathering step:

Use `Not measured yet` for an evidence gap. Do not present automated checks or simulated sessions as external-user validation.

## Data and compatibility impact

- Does this change a provider request, data meaning, JSON field, or generated value?
- Does it preserve source attribution and period information?
- Is the change backward-compatible for static-data consumers?

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `node --test site/*.test.cjs`
- [ ] Local viewer checked when UI behavior changed
- [ ] Documentation updated when commands, schema, or data semantics changed

## Security

- [ ] No credentials, tokens, local databases, or generated secrets are included
- [ ] New external data is rendered safely and its URLs are validated
