# CI/CD Rules (PART 28)

Read: AI.md PART 28

⚠️ **These rules are NON-NEGOTIABLE. Violations are bugs.** ⚠️

## CRITICAL - NEVER DO
- Use Makefile in CI/CD — explicit commands only
- Install tools inline in CI (apk add, go install, etc.) — use :build image
- Float third-party actions on @main or @v1 tags — pin to full SHA
- Use pull_request_target for untrusted code builds
- Cross-cancel different release refs

## CRITICAL - ALWAYS DO
- ensure-build-image job first (gates all downstream jobs)
- Pin all actions to full commit SHA
- CGO_ENABLED=0 in all CI build steps
- Build all 8 platforms in release
- truffleHog secret scan on all public repos
- govulncheck when go.sum present
- Renovate with pinDigests: true for GitHub Actions

## WORKFLOW FILES
| Provider | Location |
|----------|----------|
| GitHub | `.github/workflows/` |
| Gitea | `.gitea/workflows/` |
| Forgejo | `.forgejo/workflows/` |
| GitLab | `.gitlab-ci.yml` |
| Jenkins | `Jenkinsfile` |

## REQUIRED WORKFLOWS
- `ci.yml` — lint, test, build, security (all providers)
- `release.yml` — tag-triggered release (all providers)
- `build-toolchain.yml` — monthly :build image rebuild
- `docker.yml` — Docker image build on push

---
For complete details, see AI.md PART 28
