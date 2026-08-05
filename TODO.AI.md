# TODO

- admin: the admin panel's route hierarchy
  (`src/server/handler/admin/admin.go` `Routes()`) is flat —
  `/`, `/system`, `/library`, `/scheduler`, `/config`, `/logs`, `/backup` —
  but AI.md PART 17 (ADMIN PANEL) specifies a much deeper nested hierarchy
  under `/server/{admin_path}/config/...`, including
  `/config/settings`, `/config/security/auth/{oidc,ldap,saml}`,
  `/config/users/`, `/config/orgs/`, and `/config/cluster/`. Discovered while
  implementing the `SaveConfig` form-persist feature; out of scope for that
  change since it only covers `server.yml` persistence, not route
  restructuring. Needs a full PART 17 re-read and a route-hierarchy rework
  (or a written decision that the flat structure is an intentional,
  documented deviation) before the admin panel can be called PART 17
  compliant.
