#!/usr/bin/env bash
# Minimal MyGit CI runner: polls for jobs, clones the repo at the job SHA,
# runs .mygit-ci.yml as a shell script, reports back. No dependencies but
# bash, curl, jq and git.
set -euo pipefail

: "${MYGIT_URL:?set MYGIT_URL, e.g. http://127.0.0.1:8060}"
: "${MYGIT_RUNNER_TOKEN:?set MYGIT_RUNNER_TOKEN (mygit_run_...)}"
WORKDIR="${RUNNER_WORKDIR:-/tmp/mygit-runner}"

auth=(-H "Authorization: Bearer $MYGIT_RUNNER_TOKEN")

while true; do
  JOB=$(curl -sf -X POST "${auth[@]}" "$MYGIT_URL/api/v1/runners/jobs/next/") || { sleep 10; continue; }
  ID=$(echo "$JOB" | jq -r '.job.id // empty')
  [ -z "$ID" ] && sleep 10 && continue
  REPO=$(echo "$JOB" | jq -r '.job.repo')
  SHA=$(echo "$JOB" | jq -r '.job.sha')
  REF=$(echo "$JOB" | jq -r '.job.ref')
  echo "== job $ID $REPO $REF $SHA"

  D="$WORKDIR/$ID"
  rm -rf "$D"; mkdir -p "$D"
  LOG="$D/log.txt"
  STATUS="success"
  {
    echo "\$ git clone $REPO@$SHA"
    git clone -q "$MYGIT_URL/$REPO.git" "$D/src" 2>&1 || { echo "clone failed"; STATUS="failed"; }
    if [ "$STATUS" = "success" ]; then
      cd "$D/src" && git checkout -q "$SHA" 2>&1
      if [ -f .mygit-ci.yml ]; then
        echo "\$ sh .mygit-ci.yml"
        sh .mygit-ci.yml 2>&1 || STATUS="failed"
      else
        echo "no .mygit-ci.yml at $SHA"
        STATUS="failed"
      fi
    fi
    echo "status=$STATUS"
  } >"$LOG" 2>&1 || STATUS="failed"

  # cap log at 1MB (server enforces the same bound)
  head -c 1048576 "$LOG" > "$LOG.cap" && mv "$LOG.cap" "$LOG"
  jq -n --arg s "$STATUS" --rawfile log "$LOG" '{status:$s,log:$log}' | \
    curl -sf -X POST "${auth[@]}" -H Content-Type:application/json --data-binary @- \
      "$MYGIT_URL/api/v1/runners/jobs/$ID/finish/" >/dev/null
  echo "== job $ID finished: $STATUS"
  rm -rf "$D"
done
