# Web Frontend & Admin Panel Rules (PART 16, 17)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Ship desktop-only layouts (mobile-first is mandatory)
- Hide any server.yml setting from the admin panel
- Use inline CSS or JavaScript alerts
- Add JavaScript for anything HTML5+CSS already does (forms, validation, show/hide, dialogs, tabs) — JS is a LAST RESORT; every `<script>` must name a capability impossible without it; default answer to "add JS?" is NO

## CRITICAL - ALWAYS DO

- Build mobile-first, responsive UI for every page
- Give admins tooltips/help text for every setting
- Support dark/light/auto theme via CSS custom properties
- Layer in order: HTML5 → CSS → (only then) a thin JS layer that enhances a path already fully working without it

## Key Rules Summary

- PART 16 defines the full web frontend architecture and theming
- PART 17 defines the mandatory admin panel (auth, MFA suggestions, settings coverage)
- JS necessity gate (PART 16): native mechanisms required for form submit, field validation, show/hide, modals, tabs — never JS for these; legitimate JS names a concrete capability with no HTML/CSS equivalent (live search, canvas charts, drag-and-drop, WebSocket streams, clipboard copy)

For complete details, see AI.md PART 16, 17
