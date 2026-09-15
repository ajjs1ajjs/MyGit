# CI Runners (MVP)

MyGit pipelines are deliberately boring: the server owns the queue,
external runners own execution (same split as GitHub Actions).

## How it works

1. Push a commit whose tree contains `.mygit-ci.yml` at the root.
2. `post-receive` enqueues one job per pushed ref (idempotent per repo+ref+sha).
3. A runner polls `POST /api/v1/runners/jobs/next/` with
   `Authorization: Bearer mygit_run_...`, runs the job (usually by cloning
   the repo and executing `.mygit-ci.yml` itself), then reports
   `POST /api/v1/runners/jobs/{id}/finish/` with `{status, log}`.
4. Users watch `GET /api/v1/projects/{id}/pipelines/`.

Runners never get user credentials: the runner token only claims/finishes
jobs. Registration and deletion are superuser-only
(`POST/DELETE /api/v1/admin/runners/`).

## Register a runner

```bash
curl -s -H "Authorization: Bearer $ADMIN_PAT" \
  -H Content-Type:application/json \
  -d '{"name":"builder-01"}' \
  http://host:8060/api/v1/admin/runners/
# -> {"id":1,"name":"builder-01","token":"mygit_run_..."}  (shown once)
```

## Minimal runner loop

```bash
TOKEN=mygit_run_... SRV=http://host:8060
while true; do
  JOB=$(curl -s -X POST -H "Authorization: Bearer $TOKEN" $SRV/api/v1/runners/jobs/next/)
  ID=$(echo "$JOB" | jq -r .job.id // empty)
  [ -z "$ID" ] && sleep 10 && continue
  # ... clone repo@$sha, run .mygit-ci.yml, capture log ...
  curl -s -X POST -H "Authorization: Bearer $TOKEN" -H Content-Type:application/json \
    -d '{"status":"success","log":"..."}' $SRV/api/v1/runners/jobs/$ID/finish/
done
```

See `scripts/example-runner.sh` for a runnable version.

## Hook wiring

`post-receive` needs the pushed ref lines on stdin (same format git uses:
`<old> <new> <ref>` per line) plus `?repo=owner/name`:

```bash
#!/bin/sh
read line; echo "$line" | curl -s -X POST \
  -H "Authorization: Bearer $MYGIT_INTERNAL_API_TOKEN" \
  --data-binary @- "$SRV/api/v1/internal/post-receive?repo=owner/name"
```
