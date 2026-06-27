# Project SPEC

**Project:** cassonic | **Org:** webappsgo | **Internal name:** cassonic

## FIRST TURN - READ THIS

On EVERY new conversation or after context compaction:
1. Read `.claude/rules/ai-rules.md` first
2. Load only the rule files relevant to the current task
3. NEVER assume — always verify against spec before implementing

## Before ANY Code Change

Ask yourself:
1. Have I read the relevant PART in AI.md?
2. Does this follow the spec EXACTLY?
3. Am I guessing or do I KNOW from the spec?
4. Would this pass the compliance checklist?

If unsure: READ THE SPEC. Do not guess.

## Quick Reference

- **AI.md** — complete implementation specification (source of truth, READ-ONLY)
- **IDEA.md** — project description, variables, and business logic (editable)
- **`.claude/rules/`** — pre-extracted rule files (load by task; see table below)

## Rule Files

| File | When to load |
|------|-------------|
| `.claude/rules/ai-rules.md` | Always (AI assistant behavior, spec compliance) |
| `.claude/rules/project-rules.md` | Project structure, paths, license |
| `.claude/rules/config-rules.md` | Config, modes, server settings |
| `.claude/rules/binary-rules.md` | Binary, CLI flags, entrypoints |
| `.claude/rules/backend-rules.md` | DB, errors, caching, security, Tor |
| `.claude/rules/api-rules.md` | Health, versioning, API, SSL/TLS |
| `.claude/rules/frontend-rules.md` | WebUI, admin panel |
| `.claude/rules/features-rules.md` | Email, scheduler, GeoIP, metrics, backup |
| `.claude/rules/service-rules.md` | Privilege escalation, service support |
| `.claude/rules/makefile-rules.md` | Makefile (local dev only) |
| `.claude/rules/docker-rules.md` | Docker, containers |
| `.claude/rules/cicd-rules.md` | CI/CD workflows |
| `.claude/rules/testing-rules.md` | Testing, docs, i18n |
| `.claude/rules/optional-rules.md` | Multi-user, orgs, custom domains |

## Session Start (MANDATORY)

1. Read `.claude/rules/ai-rules.md` first
2. Load only the rule files relevant to the current task
3. Never guess — read the spec section, then implement

## Build Rules (CRITICAL)

- **NEVER** run Go binaries on the host — always use `docker run --rm -v $PWD:/build -w /build -e CGO_ENABLED=0 casjaysdev/go:latest sh -c "..."`
- **CGO_ENABLED=0** always
- **No `docker/Dockerfile.build`** — Go uses `casjaysdev/go:latest` directly
- **Coverage threshold**: 60% minimum — CI fails below this
- **All Actions**: pinned to full commit SHA, never tags
