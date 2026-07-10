# Maintenance Window Runbook

This runbook is the executable U3-U5 release contract for the Beijing-time
maintenance window **2026-07-11 01:00-05:00 (UTC+8)**. It is subordinate to
`docs/plans/2026-07-10-001-refactor-production-architecture-hardening-plan.md`.
The window is for releasing a pre-built, pre-tested candidate, not for coding or
redesigning the plan.

## Hard boundaries

- A failed gate is a **No-Go**. Keep the current app and route online; an
  announced outage never justifies an unsafe deployment.
- Release and rollback inputs are exact `image@sha256:...` references. Never use
  `:main`, a short commit, an unverified tag, or `docker service rollback` as the
  source of truth.
- Activate maintenance traffic only after the independent maintenance task is
  healthy. Stop the app only after public HTML and API maintenance responses
  are proven.
- Keep the production PostgreSQL and Redis services running. Scale only the app
  service to zero.
- Prefer app-only rollback to the recorded prior digest. A database restore is
  forbidden after traffic has reopened or any post-backup write may exist.
- Never print, copy, or save the full Traefik route file, full Docker service
  spec, container environment, DSN, credential, token, API key, or database
  contents. The route backup contains protected origin-routing data and stays
  root-only on the server.
- Paid smoke tests use only the approved owned identity described in the plan.
  If it cannot be proven safely, record `action-required` and do not borrow a
  customer identity.

## Fixed production identifiers

These names are identifiers, not mutable release inputs:

```bash
CTX=laoshirenai-hostinger
APP_SERVICE=laoshirenai-app-tazu5m
POSTGRES_SERVICE=laoshirenai-postgres-xc1pnj
REDIS_SERVICE=laoshirenai-redis-yhmnps
MAINT_SERVICE=laoshirenai-maintenance
NETWORK=dokploy-network
ROUTE_FILE=/etc/dokploy/traefik/dynamic/laoshirenai-app-tazu5m.yml
ROUTE_SWITCH_REMOTE=/root/laoshirenai-maintenance-route-switch.py
WINDOW_ID=20260711-0100-cst
```

Before use, re-read the live service/network names. If the topology differs,
stop rather than editing these commands during the window.

## Independent maintenance response contract

### Artifact boundary

- The service is built only from `deploy/maintenance/Dockerfile`,
  `deploy/maintenance/nginx.conf`, and `deploy/maintenance/index.html`.
- Its public Nginx base is pinned by registry digest. The built image also needs
  its own exact release digest from the same successful CI run as the app.
- It runs as the unprivileged `nginx` user on port `8080` and has no app,
  PostgreSQL, Redis, credential, or internal-address dependency.
- Container-local health is `GET /__maintenance_health` and returns `204` only
  to loopback. Public `/health`, `/livez`, and `/readyz` deliberately return
  maintenance JSON with HTTP `503`.
- Access logging is disabled so client paths and query strings cannot place
  credentials in maintenance-container logs.

### Public response contract

| Request class | Paths | Required response |
| --- | --- | --- |
| Browser | All paths not classified below | Chinese maintenance HTML, HTTP `503`, explicit `2026-07-11 01:00-05:00` Beijing time |
| Product API | `/api` and `/api/*` | Parseable JSON, HTTP `503` |
| Model API | `/v1`, `/v1/*`, `/v1beta`, `/v1beta/*`, `/antigravity`, `/antigravity/*`, `/gpt-image`, `/gpt-image/*`, `/responses`, `/responses/*`, `/chat/*`, `/images/*` | Parseable JSON, HTTP `503` |
| Setup API | `/setup` and `/setup/*` | Parseable JSON, HTTP `503` |
| Public health aliases | `/health`, `/livez`, `/readyz` | The same JSON `503`; never false green |

Every public `503` includes `Retry-After: 300` and
`Cache-Control: no-store, no-cache, must-revalidate`. Responses contain no
secret, token, internal address, or upstream implementation detail.

### Local verification

```bash
./deploy/maintenance/test.sh --static
./deploy/maintenance/test.sh --docker
```

`test.sh` accepts only a local Docker context. It refuses `ssh://` and `tcp://`
contexts, so this test cannot accidentally start a remote production service.
Both commands must pass before the candidate is frozen.

## Before 00:00 - candidate and restore drill

### 1. Freeze exact CI artifacts

The successful caller CI run produces an `immutable-image-<40SHA>` artifact
whose `immutable-image.env` contains only commit/digest metadata. Download it to
a private temporary directory; do **not** `source` it as shell code. Parse the
five expected keys, reject duplicate/unknown keys, and set:

```bash
TARGET_COMMIT=<full-40-character-commit-from-artifact>
TARGET_IMAGE_REF=<app-image@sha256-from-artifact>
MAINT_IMAGE_REF=<maintenance-image@sha256-from-artifact>
```

Validate without printing any credential:

```bash
REPO_ROOT=$(git rev-parse --show-toplevel)
CONTRACT="$REPO_ROOT/tools/release/release_contract.py"

test "$(git rev-parse HEAD)" = "$TARGET_COMMIT"
TARGET_IMAGE_REF=$(python3 "$CONTRACT" validate-deploy-ref "$TARGET_IMAGE_REF")
MAINT_IMAGE_REF=$(python3 "$CONTRACT" validate-deploy-ref "$MAINT_IMAGE_REF")
```

The CI run, artifact commit, local commit, app OCI revision, and maintenance OCI
revision must all be the same full commit. Any mismatch is a No-Go.

### 2. Record the prior digest without saving secrets

Docker may report the live service as `repository:tag@digest`; normalize it to
`repository@digest` and keep it in the root-only operation record:

```bash
OLD_SERVICE_REF=$(docker --context "$CTX" service inspect "$APP_SERVICE" \
  --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}')
OLD_IMAGE_REF=$(python3 "$CONTRACT" canonicalize-deployed-ref "$OLD_SERVICE_REF")
```

Record only the prior/target commit and digest plus selected non-sensitive
fields such as replica count, stop grace, and update order. Do not save raw
`docker service inspect` JSON because it can contain production environment
values.

### 3. Create and validate a secure snapshot

Run under Bash with a private directory and no command tracing:

```bash
set +x
umask 077
BACKUP_ROOT=/Users/fujunhao/laoshirenai/backups/postgres
mkdir -p -m 700 "$BACKUP_ROOT"
BACKUP_FILE="$BACKUP_ROOT/sub2api-${WINDOW_ID}-pre.dump"

PG_CONTAINER=$(docker --context "$CTX" ps \
  --filter "label=com.docker.swarm.service.name=$POSTGRES_SERVICE" \
  --format '{{.ID}}')
test "$(printf '%s\n' "$PG_CONTAINER" | sed '/^$/d' | wc -l | tr -d ' ')" = 1

docker --context "$CTX" exec "$PG_CONTAINER" sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom --no-owner --no-acl' \
  > "$BACKUP_FILE"
chmod 600 "$BACKUP_FILE"
test -s "$BACKUP_FILE"
if command -v pg_restore >/dev/null 2>&1; then
  pg_restore --list "$BACKUP_FILE" >/dev/null
else
  docker run --rm -i postgres:18.1-alpine3.23 \
    pg_restore --list < "$BACKUP_FILE" >/dev/null
fi
shasum -a 256 "$BACKUP_FILE" > "$BACKUP_FILE.sha256"
chmod 600 "$BACKUP_FILE.sha256"
```

Check local free space before dumping. A broken stream, empty file, failed
catalog read, weak permissions, or insufficient space is a No-Go.

### 4. Restore drill against isolated PostgreSQL 18

The restore drill is local-only and uses no production credential:

1. Start a disposable PostgreSQL 18 container and Redis container on a new
   isolated Docker network without published ports.
2. Restore with `pg_restore --exit-on-error --no-owner --no-acl` into a fresh
   database and verify the catalog.
3. In a disposable volume, create a root-only test config and installation lock
   that point explicitly at the restored database. Generate its temporary
   fixture keys inside a local helper container without printing them. Do not
   use first-run/auto-setup mode for this compatibility proof because it can
   select or initialize the wrong empty database instead of the restored one.
4. Start the **target digest** with that disposable config against the restore;
   wait for `/livez` and `/readyz`.
5. Stop it cleanly, start the target digest a second time, and prove migrations
   are idempotent.
6. Verify migration `142` is recorded once, the repaired table/index and safe
   compatibility attributes exist, and no conflict/destructive DDL occurred.
7. Start the **prior digest** against the migrated restore and prove its legacy
   health endpoint succeeds. This proves app-only rollback compatibility.
8. Destroy only the disposable containers/network/volume; preserve the secured dump
   and a redacted evidence record.

Never mount the production volume into this drill. If either exact image cannot
start against the restored/migrated database, the release is a No-Go.

## 00:30-00:50 - final preflight without public traffic

### 1. Revalidate topology and service health

- App, PostgreSQL, and Redis each have exactly one healthy desired task.
- The shared network exists and is attachable to the maintenance service.
- The live app image normalizes to the recorded prior digest.
- The live route is a regular root-owned file. Do not display its contents.
- Target and maintenance image references still pass the digest contract.
- CI, restore drill, backup capacity, operator connectivity, and approved smoke
  identity are all available.
- Owned identity id `2` still matches its approved owner metadata and has an
  active key. Record its live role (pre-window it was `agent`); do not change
  the role or use that smoke as proof of ordinary-user authorization semantics.

### 2. Copy and prove the route switcher without changing the live route

```bash
scp deploy/maintenance/route_switch.py \
  "laoshirenai-hostinger:$ROUTE_SWITCH_REMOTE"
LOCAL_SWITCH_SHA=$(shasum -a 256 deploy/maintenance/route_switch.py | awk '{print $1}')
REMOTE_SWITCH_SHA=$(ssh laoshirenai-hostinger \
  "chmod 700 '$ROUTE_SWITCH_REMOTE' && sha256sum '$ROUTE_SWITCH_REMOTE'" | awk '{print $1}')
test "$LOCAL_SWITCH_SHA" = "$REMOTE_SWITCH_SHA"
```

On the server, copy the protected route to a root-only rehearsal file in the
same directory, run `switch` and `restore` against the rehearsal copy, compare
its final hash with the untouched live route, then securely remove rehearsal
files. Pass both service names on restore so a tampered/wrong backup is refused.
Never `cat`, `grep`, or otherwise render either route file.

```bash
ssh laoshirenai-hostinger bash -s -- \
  "$ROUTE_FILE" "$ROUTE_SWITCH_REMOTE" "$APP_SERVICE" "$MAINT_SERVICE" <<'REMOTE'
set -euo pipefail
umask 077
route_file=$1
switcher=$2
app_service=$3
maintenance_service=$4
rehearsal="${route_file}.rehearsal"
rehearsal_backup="${rehearsal}.backup"
trap 'rm -f "$rehearsal" "$rehearsal_backup"' EXIT HUP INT TERM
install -m 600 "$route_file" "$rehearsal"
python3 "$switcher" switch \
  --config "$rehearsal" --backup "$rehearsal_backup" \
  --from-service "$app_service" --to-service "$maintenance_service" \
  --expected-count 3
python3 "$switcher" restore \
  --config "$rehearsal" --backup "$rehearsal_backup" \
  --current-service "$maintenance_service" --restore-service "$app_service" \
  --expected-count 3
cmp -s "$route_file" "$rehearsal"
REMOTE
```

### 3. Stage the maintenance service off-route

Create it with the exact maintenance digest on `dokploy-network`, one replica,
no published port, and no Traefik label/router. For example:

```bash
docker --context "$CTX" service create \
  --name "$MAINT_SERVICE" \
  --network "$NETWORK" \
  --replicas 1 \
  --restart-condition on-failure \
  --stop-grace-period 10s \
  --with-registry-auth \
  "$MAINT_IMAGE_REF"
```

Wait for exactly one running task and container health `healthy`. From an
existing trusted container on the overlay network, request
`http://$MAINT_SERVICE:8080/__maintenance_health` and require `204`. Confirm the
service spec contains the exact digest. A start/pull/health mismatch is a No-Go;
remove only the maintenance service and leave the app route untouched.

## 01:00 - activate maintenance first

Set the actual root-only backup path:

```bash
ROUTE_BACKUP="${ROUTE_FILE}.pre-maintenance-${WINDOW_ID}"
```

On the production server:

```bash
python3 "$ROUTE_SWITCH_REMOTE" switch \
  --config "$ROUTE_FILE" \
  --backup "$ROUTE_BACKUP" \
  --from-service "$APP_SERVICE" \
  --to-service "$MAINT_SERVICE" \
  --expected-count 3
```

The command changes only the three expected backend URLs and emits only status,
replacement count, and a hash. It never renders protected route content.

Through the public CDN, require all of the following before stopping the app:

- `https://laoshirenai.com/...` and `https://www.laoshirenai.com/...`: HTML 503,
  Beijing window text, `Retry-After: 300`, and no-store.
- `https://api.laoshirenai.com/v1/models`, `/api/...`, and `/readyz`: parseable
  JSON 503 with code `maintenance`, `Retry-After: 300`, and no-store.
- No response contains an internal address, credential-like value, or stale 200.

If public verification fails, immediately run the restore command below, verify
the old app is public again, remove the maintenance service, and declare No-Go.
Do not scale down the app.

## 01:00-01:15 - stop only app writers

After maintenance responses are publicly proven, give in-flight connections a
short quiet interval, then set the app stop grace and scale it to zero:

```bash
docker --context "$CTX" service update \
  --stop-grace-period 45s \
  --replicas 0 \
  "$APP_SERVICE"
```

Require zero app tasks and confirm PostgreSQL and Redis remain healthy. Do not
restart, rebuild, scale down, or clear either data service.

Create the fresh window snapshot now using the secure snapshot procedure above,
even if a pre-window rehearsal snapshot exists. Validate its catalog and
permissions before continuing.

## 01:35-02:30 - apply target digest behind maintenance

With public traffic still on the maintenance service:

```bash
docker --context "$CTX" service update \
  --image "$TARGET_IMAGE_REF" \
  --stop-grace-period 45s \
  --replicas 1 \
  --with-registry-auth \
  "$APP_SERVICE"
```

Do not use `--force`. Verify, in order:

1. The selected app task is running and its service image normalizes exactly to
   `TARGET_IMAGE_REF`.
2. Internal `/livez` returns 200.
3. Internal `/readyz` returns 200 only after DB, Redis, migrations, and draining
   state are healthy.
4. Migration 142 is recorded once and its additive schema contract is present.
5. A second normal restart does not reapply or loop migrations.
6. Logs contain no migration error, panic, readiness false-green, or
   secret-bearing error output.

A failure keeps the public maintenance route closed and triggers app-only
rollback before 03:15.

## 02:30-03:15 - closed-route smoke and hard decision

Run free boundary probes first, then the minimum approved owned-user smoke:

- login/refresh/session boundary;
- admin read boundary;
- one minimal streaming request;
- exactly one matching usage record and billing result;
- no duplicate charge and no unexplained balance delta.

Do not use an arbitrary customer key. At 03:15, any critical mismatch is a
rollback; do not spend the remaining window debugging an unplanned redesign.

## App-only rollback while maintenance stays closed

```bash
docker --context "$CTX" service update \
  --image "$OLD_IMAGE_REF" \
  --stop-grace-period 45s \
  --replicas 1 \
  --with-registry-auth \
  "$APP_SERVICE"
```

Require exact prior digest, an internally healthy old endpoint, and compatibility
with the additive migrated schema. Leave migration 142 in place. A whole-
database restore is allowed only if the target has never accepted public writes,
maintenance is still closed, and the operator has proved that no post-backup
write exists. Otherwise it is prohibited.

## Restore public traffic

For either the healthy target or healthy prior digest, restore the route on the
server:

```bash
python3 "$ROUTE_SWITCH_REMOTE" restore \
  --config "$ROUTE_FILE" \
  --backup "$ROUTE_BACKUP" \
  --current-service "$MAINT_SERVICE" \
  --restore-service "$APP_SERVICE" \
  --expected-count 3
```

Then require through the public CDN:

- site and static assets are normal and do not contain the maintenance page;
- `/livez` and `/readyz` return 200;
- unauthenticated model routes return their normal auth boundary, not 503;
- login/session/admin/owned smoke succeeds;
- repeated cache-busting requests do not return stale maintenance HTML/JSON.

If this public smoke fails, the retained verified backup allows `switch` to
reactivate maintenance safely. Do that first, then diagnose or roll back behind
the closed route.

Observe the chosen app for an uninterrupted 30 minutes. Only after the stable
interval and CDN checks pass:

```bash
docker --context "$CTX" service rm "$MAINT_SERVICE"
```

Keep the root-only route backup through the rollback observation period, then
remove it without displaying it. Remove the remote route-switch script after
closure.

## Completion record

Record only:

- target and prior commit/digest;
- CI run and immutable artifact identity;
- backup identifier, SHA-256, permissions, and restore-drill result;
- applied migration and readiness evidence;
- maintenance activation/restoration hashes;
- approved smoke identity ID (never its credential);
- usage/billing reconciliation;
- 30-minute observation interval;
- Beijing-time completion and rollback state.

Never record secret values, tokens, API keys, DSNs, raw service specs, private
route content, or backup contents. Publish either a success notice or an
explicit postponement/rollback notice by 05:00 Beijing time.
