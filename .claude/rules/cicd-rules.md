# CI/CD Workflows Rules (PART 28)

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO

- Use unpinned (tag-only) third-party GitHub Actions
- Grant workflows more than least-privilege permissions
- Expose secrets/write tokens to fork PRs

## CRITICAL - ALWAYS DO

- Pin third-party actions to a full commit SHA
- Keep secret-scan/vuln-scan/workflow-policy jobs blocking in ci.yml
- Match workflow platform (.github/.gitea/.gitlab-ci.yml) to the actual remote

## Key Rules Summary

- PART 28 defines the full CI/CD workflow structure and hardening checklist

For complete details, see AI.md PART 28
