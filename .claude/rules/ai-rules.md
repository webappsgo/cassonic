# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Guess or assume values a command can produce
- Skip verification before claiming a task is done
- Add unrequested features or 'improve' the spec
- Edit PARTS 0-36 of AI.md (read-only)
- Run `go`/`cargo`/toolchain commands directly on the host

## CRITICAL - ALWAYS DO

- Read the relevant PART before implementing, every time
- Ask when uncertain instead of guessing
- Verify with real tools (build, test, curl) before saying 'done'
- Keep IDEA.md in sync with the project as features change
- Use Docker for all toolchain operations

## Key Rules Summary

- AI.md (HOW) is read-only; IDEA.md (WHAT) is updated as the project evolves
- SPEC.md overrides AI.md for project-specific rules
- TODO.AI.md is required once 3+ tasks are pending
- Every full web app feature needs a browser route, API route, and CLI coverage

For complete details, see AI.md PART 0, 1
