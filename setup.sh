#!/usr/bin/env bash
set -euo pipefail

echo "============================================"
echo "  MyGit - Self-hosted Git Platform Installer"
echo "============================================"
echo ""

INSTALL_DIR="${INSTALL_DIR:-/opt/mygit}"
PORT="${PORT:-8060}"
DETECTED_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || ip -4 addr show scope global 2>/dev/null | grep -oP 'inet \K[\d.]+' | head -1 || echo "127.0.0.1")
DETECTED_IP="${DETECTED_IP:-127.0.0.1}"
DOMAIN="${DOMAIN:-$DETECTED_IP}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-$(openssl rand -base64 12 2>/dev/null || python3 -c "import secrets;print(secrets.token_urlsafe(12))")}"
DB_PASSWORD="$(openssl rand -base64 24 2>/dev/null || python3 -c "import secrets;print(secrets.token_urlsafe(24))")"
REPO_URL="${REPO_URL:-https://github.com/ajjs1ajjs/MyGit.git}"

echo "  Server IP:  ${DETECTED_IP}"
echo "  Port:       ${PORT}"
echo "  Host:       ${DOMAIN}"
echo ""

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo bash setup.sh"
    exit 1
fi

# -------------------------------------------------------------------
# Detect Python
# -------------------------------------------------------------------
detect_python() {
    for ver in 3.12 3.11 3.10 3.9 3; do
        if command -v "python${ver}" &>/dev/null; then
            PYTHON_BIN="python${ver}"
            PYTHON_VER="$ver"
            return
        fi
    done
    echo "ERROR: No Python 3 found. Install python3 first." >&2
    exit 1
}
detect_python
echo "  Python: ${PYTHON_BIN}"

# -------------------------------------------------------------------
# [1/7] System packages
# -------------------------------------------------------------------
echo ""
echo "[1/7] Installing system packages..."
if command -v apt-get &>/dev/null; then
    apt-get update -qq
    PKGS="git ${PYTHON_BIN} ${PYTHON_BIN}-venv python3-pip postgresql postgresql-contrib redis-server nginx rsync curl wget ca-certificates"
    if [ "$PYTHON_VER" = "3" ]; then
        PKGS="git python3 python3-venv python3-pip postgresql postgresql-contrib redis-server nginx rsync curl wget ca-certificates"
    fi
    apt-get install -y -qq $PKGS 2>/dev/null || apt-get install -y $PKGS
elif command -v dnf &>/dev/null; then
    dnf install -y git "${PYTHON_BIN}" python3-pip postgresql-server redis nginx rsync curl wget ca-certificates
elif command -v yum &>/dev/null; then
    yum install -y git "${PYTHON_BIN}" python3-pip postgresql-server redis nginx rsync curl wget ca-certificates
fi

# -------------------------------------------------------------------
# Clone repo
# -------------------------------------------------------------------
echo ""
echo "[*] Cloning repository..."
if [ -d "${INSTALL_DIR}/backend/.git" ]; then
    echo "  Already cloned, pulling..."
    cd "${INSTALL_DIR}/backend"; git pull --ff-only 2>/dev/null || true
else
    mkdir -p "${INSTALL_DIR}"
    git clone --depth 1 "$REPO_URL" "${INSTALL_DIR}/backend"
fi

# -------------------------------------------------------------------
# [2/7] Database
# -------------------------------------------------------------------
echo ""
echo "[2/7] Setting up database..."
if command -v pg_isready &>/dev/null; then
    systemctl start postgresql 2>/dev/null || pg_ctlcluster auto start 2>/dev/null || true
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='mygit'\" 2>/dev/null | grep -q 1 || psql -c \"CREATE USER mygit WITH PASSWORD '${DB_PASSWORD}';\"" 2>/dev/null || true
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='mygit'\" 2>/dev/null | grep -q 1 || psql -c \"CREATE DATABASE mygit OWNER mygit;\"" 2>/dev/null || true
    DB_URL="postgres://mygit:${DB_PASSWORD}@localhost:5432/mygit"
    echo "  PostgreSQL configured."
else
    DB_URL="sqlite:///${INSTALL_DIR}/backend/db.sqlite3"
    echo "  PostgreSQL not found. Using SQLite."
fi

# -------------------------------------------------------------------
# [3/7] Python backend
# -------------------------------------------------------------------
echo ""
echo "[3/7] Installing Python backend..."

"${PYTHON_BIN}" -m venv "${INSTALL_DIR}/venv"
"${INSTALL_DIR}/venv/bin/pip" install --upgrade pip -q
"${INSTALL_DIR}/venv/bin/pip" install -r "${INSTALL_DIR}/backend/requirements.txt" -q

if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD="$(openssl rand -base64 12)"
fi

DJANGO_SECRET_KEY=$(openssl rand -base64 48 2>/dev/null || python3 -c "import secrets;print(secrets.token_urlsafe(48))")

cat > "${INSTALL_DIR}/backend/.env" <<EOF
DJANGO_SECRET_KEY=${DJANGO_SECRET_KEY}
DJANGO_ALLOWED_HOSTS=${DOMAIN},${DETECTED_IP},localhost,127.0.0.1
DATABASE_URL=${DB_URL}
REDIS_URL=redis://localhost:6379/1
CELERY_BROKER_URL=redis://localhost:6379/0
CELERY_RESULT_BACKEND=redis://localhost:6379/0
CORS_ALLOWED_ORIGINS=http://${DOMAIN}:${PORT},http://${DETECTED_IP}:${PORT}
MYGIT_REPOS_ROOT=${INSTALL_DIR}/repos
MYGIT_SITE_NAME=MyGit
GIT_BINARY=git
EMAIL_HOST=localhost
EMAIL_PORT=25
SECURE_SSL_REDIRECT=False
SESSION_COOKIE_SECURE=False
CSRF_COOKIE_SECURE=False
CSRF_TRUSTED_ORIGINS=http://${DETECTED_IP}:${PORT},http://localhost:${PORT}
EOF

mkdir -p "${INSTALL_DIR}/repos" "${INSTALL_DIR}/backend/logs" "${INSTALL_DIR}/static" "${INSTALL_DIR}/media"

cd "${INSTALL_DIR}/backend"
DJANGO_SETTINGS_MODULE=config.settings.production "${INSTALL_DIR}/venv/bin/python" manage.py migrate --noinput
DJANGO_SETTINGS_MODULE=config.settings.production "${INSTALL_DIR}/venv/bin/python" manage.py collectstatic --noinput

echo "from django.contrib.auth import get_user_model; User = get_user_model(); user, _ = User.objects.get_or_create(email='${ADMIN_EMAIL}', defaults={'username':'${ADMIN_USERNAME}'}); user.set_password('${ADMIN_PASSWORD}'); user.is_superuser=True; user.is_staff=True; user.must_change_password=True; user.save()" | DJANGO_SETTINGS_MODULE=config.settings.production "${INSTALL_DIR}/venv/bin/python" manage.py shell 2>/dev/null || true

VENV_BIN="${INSTALL_DIR}/venv/bin"

# -------------------------------------------------------------------
# [4/7] Node.js frontend
# -------------------------------------------------------------------
echo ""
echo "[4/7] Installing Node.js frontend..."
NODE_BIN=$(command -v node 2>/dev/null || command -v nodejs 2>/dev/null || echo "")
if [ -z "$NODE_BIN" ]; then
    if command -v apt-get &>/dev/null; then
        curl -fsSL https://deb.nodesource.com/setup_22.x | bash - 2>/dev/null || true
        apt-get install -y nodejs 2>/dev/null || true
    elif command -v dnf &>/dev/null; then
        dnf module install -y nodejs:22 2>/dev/null || dnf install -y nodejs 2>/dev/null || true
    fi
fi
cd "${INSTALL_DIR}/backend/frontend"
npm install --silent 2>/dev/null || npm install
npm run build 2>/dev/null || npx vite build
mkdir -p "${INSTALL_DIR}/static/frontend"
cp -r dist/* "${INSTALL_DIR}/static/frontend/"
cd "${INSTALL_DIR}/backend"
DJANGO_SETTINGS_MODULE=config.settings.production "${INSTALL_DIR}/venv/bin/python" manage.py collectstatic --noinput -v0

# -------------------------------------------------------------------
# [5/7] Systemd services
# -------------------------------------------------------------------
echo ""
echo "[5/7] Setting up systemd services..."
cat > "/etc/systemd/system/mygit-api.service" <<SVCEND
[Unit]
Description=MyGit API
After=network.target

[Service]
User=mygit
WorkingDirectory=${INSTALL_DIR}/backend
Environment="DJANGO_SETTINGS_MODULE=config.settings.production"
EnvironmentFile=${INSTALL_DIR}/backend/.env
ExecStart=${VENV_BIN}/uvicorn config.asgi:application --host 127.0.0.1 --port 8000
Restart=always
SVCEND

cat > "/etc/systemd/system/mygit-git-http.service" <<SVCEND
[Unit]
Description=MyGit Git HTTP
After=network.target

[Service]
User=mygit
WorkingDirectory=${INSTALL_DIR}/backend
Environment="DJANGO_SETTINGS_MODULE=config.settings.production"
EnvironmentFile=${INSTALL_DIR}/backend/.env
ExecStart=${VENV_BIN}/gunicorn config.wsgi:application --bind 127.0.0.1:8001 --workers 4
Restart=always
SVCEND

cat > "/etc/systemd/system/mygit-celery.service" <<SVCEND
[Unit]
Description=MyGit Celery
After=network.target

[Service]
User=mygit
WorkingDirectory=${INSTALL_DIR}/backend
Environment="DJANGO_SETTINGS_MODULE=config.settings.production"
EnvironmentFile=${INSTALL_DIR}/backend/.env
ExecStart=${VENV_BIN}/celery -A config worker -l info
Restart=always
SVCEND

systemctl daemon-reload
systemctl enable mygit-api mygit-git-http mygit-celery 2>/dev/null || true
systemctl restart mygit-api mygit-git-http mygit-celery 2>/dev/null || true

# -------------------------------------------------------------------
# [6/7] Nginx
# -------------------------------------------------------------------
echo ""
echo "[6/7] Setting up Nginx..."
cat > "/etc/nginx/sites-available/mygit" <<NGINXEOF
server {
    listen ${PORT} default_server;
    server_name ${DOMAIN} ${DETECTED_IP} localhost;

    location / {
        root ${INSTALL_DIR}/static/frontend;
        try_files \$uri /index.html;
    }
    location /api/ {
        proxy_pass http://127.0.0.1:8000;
        proxy_set_header Host \$host;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
    }
    location /django-admin/ {
        proxy_pass http://127.0.0.1:8000/django-admin/;
        proxy_set_header Host \$host;
    }
    location ~ ^/(.+/.+)\\.git/ {
        proxy_pass http://127.0.0.1:8001;
        proxy_set_header Host \$host;
        client_max_body_size 0;
        proxy_request_buffering off;
    }
    location /static/ { alias ${INSTALL_DIR}/static/; }
    location /media/ { alias ${INSTALL_DIR}/media/; }
    location /metrics/ { proxy_pass http://127.0.0.1:8000; }
}
NGINXEOF

if [ -d /etc/nginx/sites-enabled ]; then
    ln -sf "/etc/nginx/sites-available/mygit" "/etc/nginx/sites-enabled/mygit"
    rm -f /etc/nginx/sites-enabled/default
elif [ -d /etc/nginx/conf.d ]; then
    cp "/etc/nginx/sites-available/mygit" "/etc/nginx/conf.d/mygit.conf"
fi
nginx -t 2>/dev/null && systemctl restart nginx 2>/dev/null || true

# -------------------------------------------------------------------
# [7/7] SSH
# -------------------------------------------------------------------
echo ""
echo "[7/7] Setting up SSH access..."
grep -q "mygit-authorized-keys" /etc/ssh/sshd_config 2>/dev/null || cat >> /etc/ssh/sshd_config <<EOF

# MyGit SSH
AuthorizedKeysCommand ${INSTALL_DIR}/backend/scripts/mygit-authorized-keys
AuthorizedKeysCommandUser root
EOF
systemctl restart sshd 2>/dev/null || systemctl restart ssh 2>/dev/null || true

# -------------------------------------------------------------------
# Done
# -------------------------------------------------------------------
echo ""
echo "============================================"
echo "  MyGit installed successfully!"
echo "============================================"
echo ""
echo "  URL:      http://${DETECTED_IP}:${PORT}"
echo "  Admin:    ${ADMIN_EMAIL}"
echo "  Password: (set via ADMIN_PASSWORD env var)"
echo ""
echo "  Logs:     journalctl -u mygit-api -f"
echo "  Backup:   ${INSTALL_DIR}/backend/scripts/mygit-backup /backup"

