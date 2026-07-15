## Summary

Describe the user-visible or maintainer-visible change and why it is needed.

## Data and compatibility impact

- Does this change a provider request, data meaning, JSON field, or generated value?
- Does it preserve source attribution and period information?
- Is the change backward-compatible for static-data consumers?

## Validation

- [ ] `go test ./...`
- [ ] `go vet ./...`
- [ ] `node --test site/security.test.cjs`
- [ ] Local viewer checked when UI behavior changed
- [ ] Documentation updated when commands, schema, or data semantics changed

## Security

- [ ] No credentials, tokens, local databases, or generated secrets are included
- [ ] New external data is rendered safely and its URLs are validated
