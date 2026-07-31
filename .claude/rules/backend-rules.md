# Backend, Data & Security Rules (PART 9, 10, 11, 32)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use string concatenation for SQL queries
- Use SELECT * in application code
- Log raw passwords, tokens, or secrets
- Skip input validation on any user-controlled input

## CRITICAL - ALWAYS DO

- Use parameterized queries with explicit column lists
- Apply defense-in-depth: validate, sanitize, rate-limit, fail secure
- Handle errors with context via %w wrapping
- Support Tor hidden service per PART 32 if enabled

## Key Rules Summary

- PART 9 defines error handling and caching patterns
- PART 10 defines database and cluster behavior
- PART 11 is the full security/logging baseline (rate limits, error message audiences, attack prevention)
- PART 32 covers Tor hidden service integration

For complete details, see AI.md PART 9, 10, 11, 32
