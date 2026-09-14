#!/bin/sh
set -eu

# Existing named volumes may have been created by the historical root-running
# image. Transfer ownership before dropping privileges so old backups and
# uploads remain readable after deployment. This changes ownership only; it
# never deletes, renames, or rewrites user data.
mkdir -p /app/backups /app/uploads
for data_dir in /app/backups /app/uploads; do
  # Avoid rescanning large volumes on every restart once ownership is already
  # correct. A legacy/root-owned volume gets one complete, non-destructive handoff.
  owner=$(stat -c '%u:%g' "$data_dir" 2>/dev/null || echo '')
  if [ "$owner" != "10001:10001" ]; then
    chown -R lms:lms "$data_dir"
  fi
done

exec su-exec lms "$@"
