# API, Health & SSL Rules (PART 13, 14, 15)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Invent a custom error response envelope
- Skip the /server/healthz and /api/{api_version}/server/healthz pair
- Hardcode SSL cert paths instead of following PART 15

## CRITICAL - ALWAYS DO

- Use the canonical health/versioning format from PART 13
- Follow noun-based, versioned REST conventions from PART 14 with RFC 7807-style errors
- Support Let's Encrypt / SSL config per PART 15

## Key Rules Summary

- Every web route needs a matching JSON API route and vice versa
- API responses always end with a single trailing newline
- SSL/TLS defaults to secure; manual cert paths are an explicit opt-in

For complete details, see AI.md PART 13, 14, 15
