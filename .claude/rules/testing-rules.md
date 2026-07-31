# Testing, Docs & I18N Rules (PART 29, 30, 31)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Commit code with failing tests
- Hardcode English user-facing strings instead of translation keys
- Let ReadTheDocs/mkdocs nav drift from actual docs/ content

## CRITICAL - ALWAYS DO

- Run tests in Docker/Incus per PART 29, never on host
- Translate every user-facing string added or modified per PART 31
- Keep docs/ and mkdocs.yml nav in sync with the app per PART 30

## Key Rules Summary

- make test must pass before every commit
- Every language file (en, es, fr, de, zh, ar, ja) needs matching keys

For complete details, see AI.md PART 29, 30, 31
