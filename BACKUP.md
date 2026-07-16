# MyGit Backup and Restore

MyGit includes a local and cloud-ready backup utility at `scripts/mygit-backup`.

The backup archive contains:

- database dump: SQLite copy or PostgreSQL `pg_dump`
- Git repositories from `MYGIT_REPOS_ROOT`
- uploaded media from `MYGIT_MEDIA_ROOT` or `media/`
- `.env`
- `manifest.json` with file sizes and SHA256 checksums

## Create a Local Backup

```bash
scripts/mygit-backup create --output /opt/mygit/backups
```

Legacy shorthand is still supported:

```bash
scripts/mygit-backup /opt/mygit/backups
```

Add a label:

```bash
scripts/mygit-backup create --label before-upgrade
```

List local backups:

```bash
scripts/mygit-backup list
```

## Cloud Backups

Two cloud modes are supported without Python dependencies:

- `rclone`: Google Drive, OneDrive, S3, SFTP, Backblaze, and many other remotes.
- `s3`: AWS CLI for S3-compatible storage.

Configure `.env` or the service environment:

```bash
MYGIT_BACKUP_CLOUD_PROVIDER=rclone
MYGIT_BACKUP_CLOUD_TARGET=myremote:mygit/backups
```

Create and upload in one command:

```bash
scripts/mygit-backup create --upload
```

Encrypt before storing or uploading:

```bash
MYGIT_BACKUP_ENCRYPTION_KEY='long-random-secret'
scripts/mygit-backup create --encrypt --upload
```

Encrypted archives use AES-256-GCM with PBKDF2-HMAC-SHA256 key derivation and the `.enc` suffix.

Upload an existing archive:

```bash
scripts/mygit-backup upload /opt/mygit/backups/mygit-backup-20260716-091500.tar.gz
```

List local and cloud backups:

```bash
scripts/mygit-backup list --cloud
```

Replicate repositories to the cloud target without creating a full archive:

```bash
scripts/mygit-backup replicate-repos
```

Use `--delete` only when the remote mirror must exactly match local repositories:

```bash
scripts/mygit-backup replicate-repos --delete
```

For S3-compatible storage:

```bash
MYGIT_BACKUP_CLOUD_PROVIDER=s3
MYGIT_BACKUP_CLOUD_TARGET=s3://my-company-backups/mygit
```

The server must already have `aws` configured with credentials.

## Restore

Stop MyGit services before restore so the database and repositories are not being written:

```bash
sudo systemctl stop mygit-api mygit-git-http mygit-celery
```

Restore from a local archive:

```bash
scripts/mygit-backup restore /opt/mygit/backups/mygit-backup-20260716-091500.tar.gz
```

Restore from cloud:

```bash
scripts/mygit-backup restore mygit-backup-20260716-091500.tar.gz --from-cloud
```

Encrypted archives are detected automatically during `restore`, `verify`, and `test-restore` when `MYGIT_BACKUP_ENCRYPTION_KEY` is set.

Use `--yes` for non-interactive disaster recovery automation:

```bash
scripts/mygit-backup restore /opt/mygit/backups/latest.tar.gz --yes
```

Start services after restore:

```bash
sudo systemctl start mygit-api mygit-git-http mygit-celery
```

## Retention

By default, the utility keeps the newest 14 local archives. Override it:

```bash
scripts/mygit-backup create --keep-local 30
```

or set:

```bash
MYGIT_BACKUP_KEEP_LOCAL=30
```

Cloud retention should be configured on the cloud provider or via `rclone` lifecycle tooling.

## Verification and Test Restore

Verify an archive without restoring it:

```bash
scripts/mygit-backup verify /opt/mygit/backups/mygit-backup-20260716-091500.tar.gz
```

Run a safe restore test in a temporary directory:

```bash
scripts/mygit-backup test-restore /opt/mygit/backups/mygit-backup-20260716-091500.tar.gz
```

For SQLite backups, `test-restore` also runs `PRAGMA integrity_check`. For PostgreSQL backups, checksum verification is always run; pass a throwaway `--database-url` to test a full PostgreSQL restore.

## Recommended Cron

Nightly local and cloud backup:

```cron
15 2 * * * cd /opt/mygit/backend && . .env && scripts/mygit-backup create --encrypt --upload >> logs/backup.log 2>&1
45 2 * * 0 cd /opt/mygit/backend && . .env && scripts/mygit-backup test-restore "$(ls -t backups/mygit-backup-*.tar.gz* | head -1)" >> logs/backup.log 2>&1
```
