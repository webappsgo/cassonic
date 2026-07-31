# Binary, CLI & Client Rules (PART 7, 8, 33)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Add CLI flags or commands not defined in PART 8
- Ship without --help/--version output matching spec format
- Skip the CLI client (src/client/ is required for every project)

## CRITICAL - ALWAYS DO

- Match the standard CLI flag set exactly (--help, --version, --config, --data, --log, --pid, --debug)
- Provide a companion CLI binary ({project_name}-cli)
- Keep binary naming and release artifact naming per PART 7

## Key Rules Summary

- PART 7 defines binary naming/build requirements
- PART 8 defines the full server binary CLI surface
- PART 33 requires src/client/; src/agent/ only for monitoring/remote-management projects

For complete details, see AI.md PART 7, 8, 33
