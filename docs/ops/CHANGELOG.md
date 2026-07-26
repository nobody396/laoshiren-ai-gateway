# Build in Public Changelog

The changelog records **what we have already built** and **why we chose to build it**.
It is a public product history, not a second announcement center and not a rendering of Git history.

## Content boundary

| Surface | Use it for | Do not use it for |
| --- | --- | --- |
| Changelog | Shipped features, model/config changes, experience improvements, meaningful fixes | User action requests, incidents in progress, raw technical diffs |
| Announcements | Messages that truly need to interrupt or reach users now | Routine product progress |
| Git / PR | Engineering implementation and review history | Public-facing release copy |

Git traceability is optional:

- `commit_sha` and `pull_request_url` may be stored for internal lookup.
- They never appear in public API responses or public pages.
- A commit, merge, tag, or deployment never creates or publishes a changelog entry automatically.
- One changelog entry may summarize several commits; many commits may have no changelog entry.

## Editorial format

Every public entry contains:

1. **Title** — a concrete result that has shipped.
2. **One-line summary** — what changed, in plain language.
3. **Why we built it** — the product judgment or problem behind the work.
4. **Full update** — optional context, tradeoffs, and the final change in Markdown.
5. **Category** — feature, model/config, improvement, or fix.
6. **Related products** — optional public product or feature names.

Keep public copy focused on the capability users can see. Do not disclose upstream
supplier identities, account-routing details, credentials, internal incidents, or
other operationally sensitive data.

## Workflow

1. Create a draft in **Admin → Changelog**.
2. Preview the public presentation while writing.
3. Optionally add Git traceability under the collapsed internal section.
4. Publish immediately or choose a future publish date.
5. Copy the stable public link when a specific entry needs to be referenced.
6. Archive an outdated entry instead of deleting a published entry.

Drafts and archived entries are not returned by the public API. Entries with a
future `published_at` are treated as scheduled and stay private until that time.

## Public API

- `GET /api/v1/changelog`
- `GET /api/v1/changelog/latest`
- `GET /api/v1/changelog/:slug`

Public responses intentionally exclude workflow state, editor identity, commit SHA,
and pull request URL.
