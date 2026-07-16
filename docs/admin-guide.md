# MyGit Admin Guide

> Self-hosted Git platform (GitLab/Gitea alternative)  
> **Stack:** Python 3.12+ / Django 5 / DRF / Celery / PostgreSQL / Redis  
> **Frontend:** Vue 3 / TypeScript / TailwindCSS / Pinia / Vue Router  
> **Target OS:** Ubuntu 22.04+ / Debian 12+

---

## Table of Contents

1. [Installation](#1-installation)
2. [Configuration (.env)](#2-configuration-env)
3. [Backup & Restore](#3-backup--restore)
4. [CI/CD Setup](#4-cicd-setup)
5. [Troubleshooting](#5-troubleshooting)

---

## 1. Installation

### 1.1 Quick Install (Single Command)

```bash
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/setup.sh | sudo bash
```

To specify a custom domain and port:

```bash
sudo DOMAIN=git.company.com PORT=443 bash <(curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/setup.sh)
```

The installer automatically detects your IP, installs all dependencies (PostgreSQL, Redis, Nginx, Node.js 22+), creates a Python virtual environment, runs migrations, builds the frontend, and configures systemd services.

### 1.2 Prerequisites

| Requirement     | Minimum Version | Notes                           |
|-----------------|-----------------|----------------------------------|
| OS              | Ubuntu 22.04+   | Debian 12+ also supported        |
| Python          | 3.9+            | 3.12+ recommended                |
| PostgreSQL      | 14+             | SQLite fallback available         |
| Redis           | 6+              | Required for Celery & caching    |
| Nginx           | 1.22+           | Reverse proxy to Gunicorn        |
| Node.js         | 22+             | Only needed for frontend build   |
| Disk Space      | 10 GB+          | Depends on repository storage    |

### 1.3 Manual Installation

```bash
# 1. System dependencies
sudo apt update
sudo apt install -y git python3.12 python3.12-venv python3-pip \
    postgresql postgresql-contrib redis-server nginx rsync curl wget

# 2. Clone the repository
sudo mkdir -p /opt/mygit
sudo git clone --depth 1 https://github.com/ajjs1ajjs/MyGit.git /opt/mygit/backend

# 3. Create system user
sudo useradd -r -s /bin/bash -d /opt/mygit mygit

# 4. Set up Python virtual environment
python3.12 -m venv /opt/mygit/venv
/opt/mygit/venv/bin/pip install --upgrade pip
/opt/mygit/venv/bin/pip install -r /opt/mygit/backend/requirements.txt

# 5. Configure environment
cp /opt/mygit/backend/.env.example /opt/mygit/backend/.env
# Edit .env with your settings (see §2 below)

# 6. Database setup
sudo -u postgres psql -c "CREATE USER mygit WITH PASSWORD 'strong_password';"
sudo -u postgres psql -c "CREATE DATABASE mygit OWNER mygit;"

# 7. Run migrations & collect static files
cd /opt/mygit/backend
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py migrate --noinput
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py collectstatic --noinput

# 8. Create admin user
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py createsuperuser

# 9. Create required directories
mkdir -p /opt/mygit/repos /opt/mygit/logs /opt/mygit/static /opt/mygit/media
chown -R mygit:mygit /opt/mygit

# 10. Frontend build (if applicable)
cd /opt/mygit/backend/frontend
npm install && npm run build
cp -r dist/* /opt/mygit/static/frontend/
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py collectstatic --noinput -v0
```

### 1.4 Post-Installation Steps

```bash
# Start services manually
sudo systemctl start mygit-gunicorn mygit-celery mygit-celery-beat nginx
sudo systemctl enable mygit-gunicorn mygit-celery mygit-celery-beat nginx

# Check service status
sudo systemctl status mygit-gunicorn
sudo journalctl -u mygit-gunicorn -f
```

Open `http://YOUR_SERVER_IP:8060` and log in with the admin credentials.

### 1.5 Ansible Deployment

For automated provisioning, use the provided Ansible playbook:

```bash
# 1. Configure inventory
cat > inventory.ini <<EOF
[mygit]
git-server ansible_host=192.168.1.100 ansible_user=root
EOF

# 2. Set vault variables (create encrypted secrets)
ansible-vault create group_vars/all/vault.yml
# Add:
#   vault_postgres_password: "my-secret-db-password"
#   vault_django_secret_key: "my-django-secret-key-here"
#   vault_admin_password: "admin-initial-password"

# 3. Run the playbook
ansible-playbook -i inventory.ini ansible/deploy.yml --ask-vault-pass
```

### 1.6 Updating

```bash
# Quick update (one command)
curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/master/update.sh | sudo bash

# Or manually:
cd /opt/mygit/backend
sudo git pull
sudo /opt/mygit/venv/bin/pip install -r requirements.txt -q
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py migrate --noinput
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py collectstatic --noinput -v0
sudo systemctl restart mygit-gunicorn mygit-celery mygit-celery-beat
```

---

## 2. Configuration (.env)

### 2.1 Environment Variables Reference

All configuration is loaded from a `.env` file located at `/opt/mygit/backend/.env` (or the `MYGIT_HOME` directory).

| Variable                    | Required | Default                  | Description                                  |
|-----------------------------|----------|--------------------------|----------------------------------------------|
| **Django**                  |          |                          |                                              |
| `DJANGO_SECRET_KEY`         | ✅       | —                        | 64+ char random string. Generate with `openssl rand -base64 48` |
| `DJANGO_ALLOWED_HOSTS`      | ✅       | —                        | Comma-separated: `domain.com,IP,localhost`   |
| `DJANGO_SETTINGS_MODULE`    | ✅       | `config.settings.production` | Django settings module                  |
| **Database**                |          |                          |                                              |
| `DATABASE_URL`              | ✅       | `sqlite:///db.sqlite3`   | PostgreSQL: `postgres://user:pass@localhost:5432/mygit` |
| **Redis / Celery**          |          |                          |                                              |
| `REDIS_URL`                 | ✅       | `redis://localhost:6379/1` | Redis cache URL                            |
| `CELERY_BROKER_URL`         | ✅       | `redis://localhost:6379/0` | Celery message broker                     |
| `CELERY_RESULT_BACKEND`     | ✅       | `redis://localhost:6379/0` | Celery result backend                     |
| **Application**             |          |                          |                                              |
| `MYGIT_REPOS_ROOT`          | ✅       | `/opt/mygit/repos`       | Directory for bare Git repositories          |
| `MYGIT_SITE_NAME`           | —        | `MyGit`                  | Site title shown in UI                       |
| `MYGIT_INTERNAL_API_TOKEN`  | —        | —                        | Token for internal service communication     |
| `MYGIT_ADMIN_PASSWORD`      | —        | —                        | Auto-generated on first install              |
| `GIT_BINARY`                | —        | `git`                    | Path to the git binary                       |
| `ENCRYPTION_KEY`            | —        | —                        | Key for encrypting sensitive data at rest    |
| **CORS / CSRF**             |          |                          |                                              |
| `CORS_ALLOWED_ORIGINS`      | ✅       | —                        | Comma-separated origins for CORS             |
| `CSRF_TRUSTED_ORIGINS`      | ✅       | —                        | Comma-separated trusted origins              |
| **Security**                |          |                          |                                              |
| `SECURE_SSL_REDIRECT`       | —        | `True`                   | Redirect HTTP→HTTPS (set `False` behind reverse proxy) |
| `SESSION_COOKIE_SECURE`     | —        | `True`                   | Set `False` for HTTP-only dev                |
| `CSRF_COOKIE_SECURE`        | —        | `True`                   | Set `False` for HTTP-only dev                |
| **Email**                   |          |                          |                                              |
| `EMAIL_HOST`                | —        | `localhost`              | SMTP server                                  |
| `EMAIL_PORT`                | —        | `587`                    | SMTP port                                    |
| `EMAIL_USE_TLS`             | —        | `True`                   | Enable TLS for SMTP                          |
| `EMAIL_HOST_USER`           | —        | —                        | SMTP username                                |
| `EMAIL_HOST_PASSWORD`       | —        | —                        | SMTP password                                |
| `DEFAULT_FROM_EMAIL`        | —        | `noreply@mygit.local`    | Sender address for emails                    |
| **LDAP**                    |          |                          |                                              |
| `MYGIT_LDAP_ENABLED`        | —        | `False`                  | Enable LDAP/AD authentication                |
| `MYGIT_LDAP_SERVER_URI`     | —        | —                        | LDAP server URI (e.g. `ldap://dc01.company.local`) |
| `MYGIT_LDAP_BIND_DN`        | —        | —                        | Bind DN for LDAP search                      |
| `MYGIT_LDAP_BIND_PASSWORD`  | —        | —                        | Bind password                                |
| `MYGIT_LDAP_USER_SEARCH_BASE` | —      | —                        | Base DN for user searches                    |
| `MYGIT_LDAP_USER_SEARCH_FILTER` | —    | `(uid={username})`       | LDAP filter template                         |
| `MYGIT_LDAP_USERNAME_ATTR`  | —        | `uid`                    | Attribute mapping for username               |
| `MYGIT_LDAP_EMAIL_ATTR`     | —        | `mail`                   | Attribute mapping for email                  |
| `MYGIT_LDAP_FULL_NAME_ATTR` | —        | `cn`                     | Attribute mapping for display name           |
| **Monitoring**              |          |                          |                                              |
| `SENTRY_DSN`                | —        | —                        | Sentry DSN for error tracking                |

### 2.2 Security Best Practices

- **`DJANGO_SECRET_KEY`**: Generate with `openssl rand -base64 48`. Keep secret. Rotate on compromise.
- **`ENCRYPTION_KEY`**: Generate with `openssl rand -hex 32`. Used for encrypting tokens/secrets at rest.
- **Database password**: Use a 24+ character random hex string.
- **File permissions**: The `.env` file must be readable only by the `mygit` user: `chmod 600 /opt/mygit/backend/.env`.
- **HTTPS**: In production, terminate TLS at the Nginx reverse proxy and set `SECURE_SSL_REDIRECT=True`.

### 2.3 Example Production `.env`

```ini
DJANGO_SECRET_KEY=your-64-char-secret-here
DJANGO_ALLOWED_HOSTS=git.company.com,10.0.0.1,localhost
DATABASE_URL=postgres://mygit:StrongDBPass123@localhost:5432/mygit
REDIS_URL=redis://localhost:6379/1
CELERY_BROKER_URL=redis://localhost:6379/0
CELERY_RESULT_BACKEND=redis://localhost:6379/0

MYGIT_REPOS_ROOT=/opt/mygit/repos
MYGIT_SITE_NAME=MyGit
GIT_BINARY=/usr/bin/git

CORS_ALLOWED_ORIGINS=https://git.company.com
CSRF_TRUSTED_ORIGINS=https://git.company.com

SECURE_SSL_REDIRECT=True
SESSION_COOKIE_SECURE=True
CSRF_COOKIE_SECURE=True

EMAIL_HOST=smtp.company.com
EMAIL_PORT=587
EMAIL_USE_TLS=True
EMAIL_HOST_USER=mygit@company.com
EMAIL_HOST_PASSWORD=smtp-password
DEFAULT_FROM_EMAIL=noreply@git.company.com

MYGIT_LDAP_ENABLED=True
MYGIT_LDAP_SERVER_URI=ldap://dc01.company.com
MYGIT_LDAP_BIND_DN=CN=MyGit Service,OU=Service Accounts,DC=company,DC=com
MYGIT_LDAP_BIND_PASSWORD=ldap-password
MYGIT_LDAP_USER_SEARCH_BASE=DC=company,DC=com
MYGIT_LDAP_USER_SEARCH_FILTER=(sAMAccountName={username})
MYGIT_LDAP_USERNAME_ATTR=sAMAccountName
MYGIT_LDAP_EMAIL_ATTR=mail
MYGIT_LDAP_FULL_NAME_ATTR=displayName

SENTRY_DSN=https://key@sentry.io/project
```

---

## 3. Backup & Restore

### 3.1 Backup Tool

MyGit includes a built-in backup script at `/opt/mygit/backend/scripts/mygit-backup`.

```bash
# Basic local backup
sudo /opt/mygit/backend/scripts/mygit-backup create --output /opt/mygit/backups

# Encrypted backup with cloud upload
sudo /opt/mygit/backend/scripts/mygit-backup create --encrypt --upload

# Test-restore validation (safe, non-destructive)
sudo /opt/mygit/backend/scripts/mygit-backup test-restore /opt/mygit/backups/mygit-backup-20250101-120000.tar.gz

# Full restore
sudo /opt/mygit/backend/scripts/mygit-backup restore /opt/mygit/backups/mygit-backup-20250101-120000.tar.gz
```

**Backup contents:**

| Component      | Included | Notes                                    |
|----------------|----------|------------------------------------------|
| Database       | ✅       | SQLite or PostgreSQL dump                |
| Repositories   | ✅       | Bare Git repos from `MYGIT_REPOS_ROOT`  |
| Media files    | ✅       | Uploaded images, attachments             |
| `.env` file   | ✅       | Configuration with secrets               |
| SHA256 checksum| ✅       | Integrity verification                   |

**Storage backends:**

- **Local:** `--output /path/to/backups`
- **S3-compatible:** Set `MYGIT_BACKUP_CLOUD_S3_BUCKET` and `MYGIT_BACKUP_CLOUD_S3_ENDPOINT`
- **rclone:** Set `MYGIT_BACKUP_CLOUD_RCLONE_REMOTE` for any rclone-supported destination
- **Encryption:** AES-256-GCM (`--encrypt` flag)

### 3.2 Manual Backup Procedure

```bash
#!/bin/bash
# Manual backup for cron: /etc/cron.d/mygit-backup
# 0 2 * * * root /opt/mygit/backend/scripts/mygit-backup create --encrypt --upload

BACKUP_DIR=/opt/mygit/backups
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
mkdir -p "$BACKUP_DIR"

# Dump PostgreSQL
pg_dump -U mygit -h localhost mygit > /tmp/mygit-db-$TIMESTAMP.sql

# Archive everything
tar czf "$BACKUP_DIR/mygit-full-$TIMESTAMP.tar.gz" \
    -C /opt/mygit backend/.env \
    -C /opt/mygit repos \
    -C /opt/mygit media \
    --transform="s/.*/mygit-db-$TIMESTAMP.sql/" /tmp/mygit-db-$TIMESTAMP.sql

# Clean up temp files
rm /tmp/mygit-db-$TIMESTAMP.sql

# Keep only last 30 days
find "$BACKUP_DIR" -name "mygit-full-*.tar.gz" -mtime +30 -delete
```

### 3.3 Restore Procedure

```bash
sudo systemctl stop mygit-gunicorn mygit-celery mygit-celery-beat

# Restore from backup
sudo /opt/mygit/backend/scripts/mygit-backup restore /path/to/backup.tar.gz

# Or manually:
sudo tar xzf /path/to/backup.tar.gz -C /opt/mygit
sudo -u postgres psql -c "DROP DATABASE IF EXISTS mygit;"
sudo -u postgres psql -c "CREATE DATABASE mygit OWNER mygit;"
sudo -u postgres psql mygit < /tmp/mygit-db-*.sql

sudo chown -R mygit:mygit /opt/mygit
sudo systemctl start mygit-gunicorn mygit-celery mygit-celery-beat
```

### 3.4 Backup Automation

Add a cron job to `/etc/cron.d/mygit-backup`:

```
# Nightly backup at 2 AM
0 2 * * * root /opt/mygit/backend/scripts/mygit-backup create --encrypt --upload >/dev/null 2>&1

# Weekly test-restore validation (Sunday 3 AM)
0 3 * * 0 root /opt/mygit/backend/scripts/mygit-backup test-restore $(ls -t /opt/mygit/backups/*.tar.gz | head -1) >/dev/null 2>&1
```

---

## 4. CI/CD Setup

### 4.1 Overview

MyGit includes a built-in CI/CD system. Pipelines are defined in `.mygit-ci.yml` at the root of each repository.

**Architecture:** Runner agent → MyGit API → Jobs execute in isolated environments.

### 4.2 Pipeline Configuration (`.mygit-ci.yml`)

```yaml
stages:
  - test
  - build
  - deploy

variables:
  PYTHON_VERSION: "3.12"
  NODE_VERSION: "22"

test-backend:
  stage: test
  image: python:$PYTHON_VERSION
  script:
    - pip install -r requirements.txt
    - pytest
  only:
    - main
    - merge_requests

test-frontend:
  stage: test
  image: node:$NODE_VERSION
  script:
    - cd frontend
    - npm install
    - npm run test
  only:
    - main

build:
  stage: build
  script:
    - echo "Build complete"
  only:
    - tags

deploy:
  stage: deploy
  script:
    - ansible-playbook -i inventory.ini ansible/deploy.yml
  only:
    - main
  when: manual
```

### 4.3 Setting Up a Runner

```bash
# On a dedicated runner machine:
sudo apt install -y docker.io python3 python3-pip
sudo systemctl enable --now docker

# Register the runner with your MyGit instance
# (Assuming the runner executable is bundled with MyGit)
sudo /opt/mygit/backend/scripts/mygit-ci-runner register \
    --url https://git.company.com \
    --token YOUR_REGISTRATION_TOKEN \
    --name runner-01

# Start the runner
sudo systemctl start mygit-ci-runner
```

### 4.4 Environment Variables for Pipelines

| Variable               | Description                             |
|------------------------|-----------------------------------------|
| `CI_PROJECT_ID`        | Current project ID                      |
| `CI_COMMIT_SHA`        | Commit SHA being built                  |
| `CI_COMMIT_REF_NAME`   | Branch or tag name                      |
| `CI_PIPELINE_ID`       | Unique pipeline ID                      |
| `CI_JOB_ID`            | Unique job ID                           |
| `CI_REGISTRY_PASSWORD` | Temporary authentication token          |

### 4.5 Webhooks

Webhooks can be configured per project to trigger external services on push, MR, issue, or tag events. Each webhook payload includes an HMAC-SHA256 signature in the `X-MyGit-Signature` header for verification.

```bash
# Verify webhook signature (example)
echo -n "$payload" | openssl dgst -sha256 -hmac "$secret"
```

---

## 5. Troubleshooting

### 5.1 Service Health Checks

```bash
# Check all services
sudo systemctl status mygit-gunicorn mygit-celery mygit-celery-beat nginx postgresql redis-server

# View logs in real-time
sudo journalctl -u mygit-gunicorn -f --no-pager
sudo journalctl -u mygit-celery -f --no-pager
sudo journalctl -u mygit-celery-beat -f --no-pager
sudo tail -f /var/log/nginx/mygit-error.log

# Process health
ps aux | grep -E '(gunicorn|celery|nginx)'
```

### 5.2 Common Issues

---

#### Problem: 502 Bad Gateway from Nginx

**Causes:**
- Gunicorn is not running
- Gunicorn crashed or is binding to the wrong port
- Nginx cannot connect to the upstream socket

**Checks:**
```bash
sudo systemctl status mygit-gunicorn
sudo journalctl -u mygit-gunicorn -n 50 --no-pager
sudo nginx -t
curl -v http://127.0.0.1:8000/
```

**Fix:**
```bash
sudo systemctl restart mygit-gunicorn
sudo systemctl restart nginx
```

---

#### Problem: Database connection refused

**Causes:**
- PostgreSQL service is down
- Incorrect credentials in `.env`
- `pg_hba.conf` does not allow local connections

**Checks:**
```bash
sudo systemctl status postgresql
sudo -u postgres psql -c "\l"  # List databases
psql -U mygit -d mygit -h localhost -c "SELECT 1;"  # Test connection
```

**Fix:**
```bash
sudo systemctl restart postgresql
# Verify DATABASE_URL in .env
# Check pg_hba.conf for local auth method
```

---

#### Problem: Celery tasks not executing

**Causes:**
- Redis is not running
- Celery worker is not started
- Celery beat is not scheduling tasks

**Checks:**
```bash
sudo systemctl status redis-server
sudo systemctl status mygit-celery
sudo journalctl -u mygit-celery -n 30 --no-pager
redis-cli ping  # Should return PONG
```

**Fix:**
```bash
sudo systemctl restart redis-server
sudo systemctl restart mygit-celery mygit-celery-beat
# Check Celery can reach Redis
/opt/mygit/venv/bin/celery -A config inspect ping
```

---

#### Problem: Cannot push/pull via HTTP

**Causes:**
- Git HTTP backend (port 8001) is down
- Nginx Git location block is misconfigured
- Repository path in URL is incorrect
- Authentication issues

**Checks:**
```bash
# Test Git HTTP backend directly
curl -v http://127.0.0.1:8001/namespace/repo.git/info/refs?service=git-upload-pack

# Test Nginx Git proxy
curl -v http://localhost:8060/namespace/repo.git/info/refs?service=git-upload-pack
```

**Fix:**
```bash
sudo systemctl restart mygit-gunicorn
# Verify Nginx config has the Git location block
```

---

#### Problem: Static files not loading (blank UI)

**Causes:**
- `collectstatic` was not run
- Nginx `static/` alias path is incorrect
- Frontend was not built

**Checks:**
```bash
ls -la /opt/mygit/static/
ls -la /opt/mygit/backend/frontend/dist/
# Check Nginx error log for 404 on static files
```

**Fix:**
```bash
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py collectstatic --noinput
sudo systemctl reload nginx
```

---

#### Problem: "Invalid HTTP_HOST header" in logs

**Cause:** The `Host` header from the request is not in `DJANGO_ALLOWED_HOSTS`.

**Fix:** Add the hostname or IP to the `DJANGO_ALLOWED_HOSTS` in `.env`:
```ini
DJANGO_ALLOWED_HOSTS=git.company.com,10.0.0.1,192.168.1.100,localhost
```

---

#### Problem: Registration fails with 500 error

**Causes:**
- Email backend is misconfigured (trying to send verification email)
- Database migration not run

**Checks:**
```bash
sudo journalctl -u mygit-gunicorn -n 20 --no-pager | grep ERROR
```

**Fix:**
```bash
# Run migrations
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py migrate --noinput
# Disable email verification if email is not configured
# Set EMAIL_HOST and EMAIL_PORT to working SMTP or use console backend temporarily
```

---

#### Problem: High memory / CPU usage

**Diagnostics:**
```bash
top -b -n 1 | head -20
free -h
df -h /opt/mygit/repos

# Check Gunicorn worker count — adjust in systemd service file
# Recommended: (2 × CPU cores) + 1
```

**Tuning:**
```bash
# Adjust Gunicorn workers
sudo sed -i 's/--workers 9/--workers 5/' /etc/systemd/system/mygit-gunicorn.service
sudo systemctl daemon-reload
sudo systemctl restart mygit-gunicorn

# Adjust Celery concurrency
sudo sed -i 's/--concurrency=8/--concurrency=4/' /etc/systemd/system/mygit-celery.service
sudo systemctl daemon-reload
sudo systemctl restart mygit-celery
```

---

### 5.3 Log Locations

| Service           | Log Location / Command                          |
|-------------------|-------------------------------------------------|
| Gunicorn          | `journalctl -u mygit-gunicorn -f`               |
| Nginx access      | `/var/log/nginx/mygit-access.log`               |
| Nginx error       | `/var/log/nginx/mygit-error.log`                |
| Celery worker     | `journalctl -u mygit-celery -f`                 |
| Celery beat       | `journalctl -u mygit-celery-beat -f`            |
| Django            | `/opt/mygit/logs/mygit.log`                     |
| PostgreSQL        | `journalctl -u postgresql -f`                   |
| Redis             | `journalctl -u redis-server -f`                 |

### 5.4 Useful Commands

```bash
# Reload configuration without downtime
sudo systemctl reload nginx
sudo systemctl reload mygit-gunicorn

# Full restart sequence
sudo systemctl restart postgresql redis-server
sudo systemctl restart mygit-gunicorn mygit-celery mygit-celery-beat
sudo systemctl restart nginx

# Test Nginx configuration
sudo nginx -t

# Django shell (production settings)
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py shell

# Reset admin password
DJANGO_SETTINGS_MODULE=config.settings.production /opt/mygit/venv/bin/python manage.py shell -c "
from django.contrib.auth import get_user_model;
User = get_user_model();
u = User.objects.get(username='admin');
u.set_password('new-password');
u.save()
"

# Check disk usage of repositories
du -sh /opt/mygit/repos/*

# Repair Git repositories
find /opt/mygit/repos -type d -name "*.git" -exec git --git-dir={} fsck \; 2>/dev/null
```
