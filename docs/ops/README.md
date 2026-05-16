# Operations Docs

This folder contains sanitized operational context for the private `nobody396/laoshiren-ai-gateway` repository.

Read these files before doing deployment, database backup, CDN work, or AI-assisted code review:

- `ENVIRONMENTS.md`: production environment inventory, service names, domains, CDN, GitHub/GHCR, database/Redis backup entry points, and common failure handling.
- `TEAM_WORKFLOW.md`: how human collaborators and AI agents should work together on branches, PRs, reviews, and releases.

Security rule: never commit real passwords, tokens, SSH private keys, JWT secrets, database passwords, Redis passwords, GitHub PATs, or user API keys. Use platform secrets, local keyrings, password managers, or owner-approved secure channels instead.

