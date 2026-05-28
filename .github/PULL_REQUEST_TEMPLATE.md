## Summary

<!-- Describe what this PR changes and why -->

## Motivation

<!-- Why does this change need to exist? What problem does it solve? -->

## Test Evidence

<!-- Paste `make test` output or describe how you verified the change -->

```
make test output here
```

## Documentation and Config Updates

- [ ] `docs/` updated for any user/admin/operator-facing changes
- [ ] `docs/configuration.md` updated if any config keys were added/changed/removed
- [ ] `docs/api.md` updated if any API routes or responses changed

## Breaking Changes

<!-- Does this change break existing behavior, config, or API contracts? -->
- [ ] No breaking changes
- [ ] Breaking change — describe impact and migration path:

## Security and Privacy Impact

<!-- Does this change affect authentication, authorization, data handling, or input validation? -->
- [ ] No security/privacy impact
- [ ] Security/privacy impact — describe:

## Checklist

- [ ] Code is `gofmt`-formatted
- [ ] No `TODO`, `FIXME`, or stub behavior in committed code
- [ ] No hardcoded machine-specific values (hostname, IP, CPU count, memory)
- [ ] `config.ParseBool()` used for all boolean parsing (not `strconv.ParseBool()`)
- [ ] Parameterized queries only — no string-interpolated SQL
- [ ] New/changed package logic has corresponding `*_test.go` unit tests
- [ ] New/changed endpoints have corresponding `./tests/*.sh` integration tests
- [ ] CI passes
