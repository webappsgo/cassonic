# Project SPEC

Project: CASSONIC
Role: Efficient loader for AI.md

⚠️ **THIS FILE IS AUTO-LOADED EVERY CONVERSATION. FOLLOW IT EXACTLY.** ⚠️

Purpose:
- This file is a short loader for the most important rules
- `AI.md` is the full source of truth
- For complete details, read the referenced PARTs in `AI.md`

## FIRST TURN - MANDATORY

On EVERY new conversation or after "context compacted" message:
1. **READ** the relevant `.claude/rules/*.md` for your current task
2. **NEVER** assume or guess - verify against AI.md before implementing

## Asking Questions

- **Default to continuing work** - do not stop just to ask whether you should continue
- **Never guess** - if the answer cannot be determined from `AI.md`, `IDEA.md`, or the codebase, ASK
- **Question mark = question** - when user ends with `?`, answer/clarify, don't execute
- **Use AskUserQuestion wizard** - one question at a time with options

**Ask only when at least one of these is true:**
1. A required business/product decision is missing
2. Two or more reasonable implementations would produce materially different behavior
3. The action is destructive, irreversible, or impacts production/user data
4. The spec explicitly says to ask or confirm

## Before ANY Code Change

1. Have I read the relevant PART in AI.md? (If no → read it)
2. Does this follow the spec EXACTLY? (If unsure → check spec)
3. Am I guessing or do I KNOW from the spec? (If guessing → read spec)
4. Would this pass the compliance checklist? (AI.md FINAL section)

**WHEN IN DOUBT: READ THE SPEC. DO NOT GUESS.**

## Binary Terminology
- **server** = `cassonic` (main binary, runs as service)
- **client** = `cassonic-cli` (REQUIRED companion, CLI/TUI/GUI)
- **agent** = `cassonic-agent` (optional, runs on remote machines)

## Key Placeholders
- `{project_name}` = cassonic
- `{project_org}` = local
- `{internal_name}` = cassonic
- `{admin_path}` = admin

## Account Types (CRITICAL)
- **Server Admin** = manages the app (NOT a privileged OS user)
- **Primary Admin** = first admin, cannot be deleted
- **Regular User** = end-user (PART 34, optional feature)
- Server Admins != Regular Users (separate DB tables)

## NEVER Do (Top 19) - VIOLATIONS ARE BUGS
1. Use bcrypt -> Use Argon2id
2. Put Dockerfile in root -> `docker/Dockerfile`
3. Use CGO -> CGO_ENABLED=0 always
4. Hardcode dev values -> Detect at runtime
5. Use external cron -> Internal scheduler (PART 19)
6. Store passwords plaintext -> Argon2id (tokens use SHA-256)
7. Create premium tiers -> All features free, no paywalls
8. Use Makefile in CI/CD -> Explicit commands only
9. Guess or assume values -> Run the command or read spec
10. Skip platforms -> Build all 8 (linux/darwin/windows x amd64/arm64)
11. Client-side rendering (React/Vue) -> Server-side Go templates
12. Require JavaScript for core features -> Progressive enhancement only
13. Let long strings break mobile -> Use word-break CSS
14. Skip validation -> Server validates EVERYTHING
15. Implement without reading spec -> Read relevant PART first
16. Modify AI.md content -> READ-ONLY
17. Edit internal_name -> FROZEN forever
18. Read image > 1000x1000 directly -> Resize first
19. Use non-conforming IDEA.md -> Migrate it first

## ALWAYS Do - NON-NEGOTIABLE
1. Read AI.md before implementing ANY feature
2. Server-side processing (server does the work, client displays)
3. Mobile-first responsive CSS
4. All features work without JavaScript
5. Tor hidden service support (auto-enabled if Tor found)
6. Built-in scheduler, GeoIP, metrics, email, backup, update
7. Full admin panel with ALL settings
8. Client binary for ALL projects
9. Commit often - small, focused commits

## File Locations
- Config: `/etc/local/cassonic/server.yml` (privileged) or `~/.config/local/cassonic/server.yml`
- Data: `/var/lib/local/cassonic/` (privileged)
- Logs: `/var/log/local/cassonic/` (privileged)
- Source: `src/`
- Docker: `docker/`

## Where to Find Details
- AI behavior: `.claude/rules/ai-rules.md` (PART 0, 1)
- Project structure: `.claude/rules/project-rules.md` (PART 2, 3, 4)
- Frontend/WebUI: `.claude/rules/frontend-rules.md` (PART 16, 17)
- Full spec: `AI.md` (source of truth)

## Current Project State
- Last updated: 2026-06-04
- Current task: Spec compliance fixes (post-audit)
- Status: Core server scaffolded — Subsonic v1.1.0–v1.16.1, Ampache v5+v6, native REST API, scheduler (17 jobs), GeoIP, backup (AES-256-GCM), Tor, i18n (7 locales), WebUI (server-side Go templates), Icecast relay, scrobbling (6 services), podcast, tag editor, MusicBrainz lookup all implemented. cassonic-agent is optional and not yet scaffolded.
- Relevant PARTs: all (0–36)
