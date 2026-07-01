# AI Assistant Rules (PART 0, 1)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Guess or assume values — use commands (`date`, `basename "$PWD"`, `git remote get-url origin`, etc.) or ask
- Say "done" without verifying — run tests, check output
- Skip reading the relevant PART before implementing
- Run `go` commands directly on the host machine
- Make up answers or pretend to know
- Add unrequested features or "improvements"
- Edit AI.md or TEMPLATE.md (both are READ-ONLY)
- Create report/analysis files (AUDIT.md, COMPLIANCE.md, SUMMARY.md) — fix directly instead
- Rely on memory between sessions — re-read the relevant PART

## CRITICAL - ALWAYS DO

- Read the relevant AI.md PART(s) before implementing each task
- Test and verify before claiming completion
- Ask when uncertain (asking ~100 tokens; wrong implementation ~5000+)
- Stop and ask when multiple interpretations are possible
- Use Docker `casjaysdev/go:latest` for ALL Go builds and tests — NEVER on host
- Update IDEA.md when features change
- Perform session initialization steps on every fresh session

## Project Facts

- project_name: cassonic
- project_org: webappsgo (GitHub)
- internal_name: cassonic
- Go build: `docker run --rm -v $PWD:/build -w /build -e CGO_ENABLED=0 casjaysdev/go:latest sh -c "..."`

## Session Initialization (Every Session)

1. Read existing `CLAUDE.md` and `.claude/CLAUDE.md`
2. Check if `.claude/rules/` directory exists and is current
3. If missing or outdated: CREATE/UPDATE all rule files
4. Read `TODO.AI.md` and check for needed updates

## Key Rules Summary

| Rule | Description |
|------|-------------|
| AI.md is source of truth | HOW to implement; read PART before each task |
| IDEA.md = WHAT | Business logic, features, project variables |
| NEVER guess | Use commands or ask |
| Container-only builds | No Go on host; use casjaysdev/go:latest |
| Correct over fast | Verified slow answer beats unverified fast answer |

## Red Flags — STOP IMMEDIATELY

- "This is probably what they meant..." → STOP - ASK
- "I'll just assume..." → STOP - ASK
- "This should work..." → STOP - TEST
- "Close enough..." → STOP - DO IT RIGHT
- "I think I remember..." → STOP - READ THE SPEC

For complete details, see AI.md PART 0, 1
