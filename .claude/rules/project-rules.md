# Project Structure & Licensing Rules (PART 2, 3, 4)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use a license other than MIT
- Invent directory layouts not defined in PART 3
- Hardcode OS-specific paths instead of using PART 4 path resolution

## CRITICAL - ALWAYS DO

- Keep LICENSE.md at repo root, unmodified MIT text (only copyright line changes)
- Follow the src/, docker/, docs/ layout defined in PART 3
- Resolve config/data/log paths per-OS as defined in PART 4

## Key Rules Summary

- src/ holds all Go source, organized by package per PART 3
- docker/ holds all Docker build/compose files
- OS-specific paths (XDG on Linux, AppData on Windows, Library on macOS) come from PART 4, never hardcoded

For complete details, see AI.md PART 2, 3, 4
