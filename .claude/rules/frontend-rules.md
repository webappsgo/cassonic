# Frontend Rules (PART 16, 17)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use server-side rendering frameworks (React, Vue, Angular SPA) — server-side templates only
- Hardcode colors — use CSS custom properties
- Default to light mode — dark mode is the default
- Build a display-only frontend — it MUST be fully functional (CRUD)
- Ship broken forms — all forms must submit to backend and handle responses
- Use bundlers unless required for templating — keep JS simple
- Skip mobile-responsive design — mobile-first from day one

## CRITICAL - ALWAYS DO

- Server-side templates only (Go `html/template` or similar)
- Dark mode as default; support dark/light/auto theme switching
- CSS custom properties for ALL colors (no hardcoded hex/rgb)
- Mobile-first responsive design
- Every form must work: submit, show backend errors, redirect on success
- Every user-facing feature needs both Web (HTML) and API (JSON) routes
- Accessibility: semantic HTML, ARIA labels, keyboard navigation
- PWA support: installable, offline-capable

## Theme Rules

| Requirement | Rule |
|-------------|------|
| Default | Dark mode |
| Switching | Dark / Light / Auto (system) toggle |
| Colors | CSS custom properties only — no hardcoded values |
| Framework | MkDocs Material (docs), custom CSS (app) |

## Frontend Functionality Requirements

| Requirement | Description |
|-------------|-------------|
| Forms work | Submit to backend, display errors, redirect on success |
| CRUD complete | Create, Read, Update, Delete all work from frontend |
| Error handling | Display backend errors appropriately |
| Validation | Client-side matches server-side rules |
| No partial features | If it shows, it works |

## Admin Panel (PART 17)

- Located at `/server/{admin_path}/` (configurable path, default `admin`)
- Admin auth bypassed ONLY in debug mode (`--debug`)
- First-run wizard creates initial admin account
- Admin panel mirrors every admin API endpoint

For complete details, see AI.md PART 16, 17
