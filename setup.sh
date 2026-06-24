#!/usr/bin/env bash
set -euo pipefail

echo "============================================"
echo "  MyGit - Self-hosted Git Platform Installer"
echo "============================================"
echo ""

INSTALL_DIR="${INSTALL_DIR:-/opt/mygit}"
DOMAIN="${DOMAIN:-git.example.com}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
USE_DOCKER="${USE_DOCKER:-no}"

if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo bash setup.sh"
    exit 1
fi

echo "[1/7] Installing system packages..."
if command -v apt-get &>/dev/null; then
    apt-get update -qq
    apt-get install -y -qq git python3.12 python3.12-venv python3-pip \
        postgresql postgresql-contrib redis-server nginx certbot python3-certbot-nginx \
        rsync curl
elif command -v dnf &>/dev/null; then
    dnf install -y git python3.12 python3-pip postgresql-server redis nginx certbot
elif command -v yum &>/dev/null; then
    yum install -y git python3.12 python3-pip postgresql-server redis nginx certbot
fi

echo ""
echo "[2/7] Setting up database..."
if command -v pg_isready &>/dev/null; then
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_roles WHERE rolname='mygit'\" | grep -q 1 || psql -c \"CREATE USER mygit WITH PASSWORD 'mygit_password';\"" 2>/dev/null || true
    su - postgres -c "psql -tc \"SELECT 1 FROM pg_database WHERE datname='mygit'\" | grep -q 1 || psql -c \"CREATE DATABASE mygit OWNER mygit;\"" 2>/dev/null || true
    DB_URL="postgres://mygit:mygit_password@localhost:5432/mygit"
else
    DB_URL="sqlite:///${INSTALL_DIR}/backend/db.sqlite3"
    echo "  PostgreSQL not found. Using SQLite."
fi

echo ""
echo "[3/7] Installing Python backend..."
mkdir -p "${INSTALL_DIR}"
cp -r . "${INSTALL_DIR}/backend/"

python3.12 -m venv "${INSTALL_DIR}/venv"
"${INSTALL_DIR}/venv/bin/pip" install --upgrade pip -q
"${INSTALL_DIR}/venv/bin/pip" install -r "${INSTALL_DIR}/backend/requirements.txt" -q

if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD=$(openssl rand -base64 16)
    echo "  Generated admin password: $ADMIN_PASSWORD"
fi

DJANGO_SECRET_KEY=$(openssl rand -base64 48)

cat > "${INSTALL_DIR}/.env" <<EOF
DJANGO_SECRET_KEY=${DJANGO_SECRET_KEY}
DJANGO_ALLOWED_HOSTS=${DOMAIN},localhost,127.0.0.1
DATABASE_URL=${DB_URL}
REDIS_URL=redis://localhost:6379/1
CELERY_BROKER_URL=redis://localhost:6379/0
CELERY_RESULT_BACKEND=redis://localhost:6379/0
CORS_ALLOWED_ORIGINS=https://${DOMAIN}
MYGIT_REPOS_ROOT=${INSTALL_DIR}/repos
MYGIT_SITE_NAME=MyGit
GIT_BINARY=git
EMAIL_HOST=localhost
EMAIL_PORT=25
EOF

mkdir -p "${INSTALL_DIR}/repos" "${INSTALL_DIR}/logs" "${INSTALL_DIR}/static" "${INSTALL_DIR}/media"

cd "${INSTALL_DIR}/backend"
"${INSTALL_DIR}/venv/bin/python" manage.py migrate --noinput
"${INSTALL_DIR}/venv/bin/python" manage.py collectstatic --noinput

echo "from django.contrib.auth import get_user_model; User = get_user_model(); User.objects.create_superuser('${ADMIN_EMAIL}', '${ADMIN_USERNAME}', '${ADMIN_PASSWORD}') if not User.objects.filter(email='${ADMIN_EMAIL}').exists() else None" | "${INSTALL_DIR}/venv/bin/python" manage.py shell 2>/dev/null || true

echo ""
echo "[4/7] Installing Node.js frontend..."
if ! command -v node &>/dev/null; then
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
    apt-get install -y nodejs 2>/dev/null || true
fi
cd "${INSTALL_DIR}/backend/frontend"
npm install --silent
npm run build
cp -r dist/* "${INSTALL_DIR}/static/"

echo ""
echo "[5/7] Setting up systemd services..."
for svc in mygit-api mygit-git-http mygit-celery; do
    cat > "/etc/systemd/system/${svc}.service" <<SVCEND
[Unit]
Description=MyGit - ${svc}
After=network.target

[Service]
User=root
WorkingDirectory=${INSTALL_DIR}/backend
Environment="DJANGO_SETTINGS_MODULE=config.settings.production"
EnvironmentFile=${INSTALL_DIR}/.env
SVCEND
done

cat >> /etc/systemd/system/mygit-api.service <<'EOF'
ExecStart=/opt/mygit/venv/bin/uvicorn config.asgi:application --host 127.0.0.1 --port 8000
Restart=always
EOF

cat >> /etc/systemd/system/mygit-git-http.service <<'EOF'
ExecStart=/opt/mygit/venv/bin/gunicorn config.wsgi:application --bind 127.0.0.1:8001 --workers 4
Restart=always
EOF

cat >> /etc/systemd/system/mygit-celery.service <<'EOF'
ExecStart=/opt/mygit/venv/bin/celery -A config worker -l info
Restart=always
EOF

systemctl daemon-reload
systemctl enable mygit-api mygit-git-http mygit-celery
systemctl restart mygit-api mygit-git-http mygit-celery 2>/dev/null || true

echo ""
echo "[6/7] Setting up Nginx..."
cat > "/etc/nginx/sites-available/${DOMAIN}" <<NGINXEOF
server {
    listen 80;
    server_name ${DOMAIN};

    location / {
        root ${INSTALL_DIR}/static;
        try_files \$uri /index.html;
    }

    location /api/ { proxy_pass http://127.0.0.1:8000; proxy_set_header Host \$host; }
    location /admin/ { proxy_pass http://127.0.0.1:8000; }

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

ln -sf "/etc/nginx/sites-available/${DOMAIN}" "/etc/nginx/sites-enabled/${DOMAIN}" 2>/dev/null || cp "/etc/nginx/sites-available/${DOMAIN}" "/etc/nginx/conf.d/${DOMAIN}.conf"
rm -f /etc/nginx/sites-enabled/default 2>/dev/null || true
systemctl restart nginx 2>/dev/null || true

echo ""
echo "[7/7] Setting up SSH access..."
cat >> /etc/ssh/sshd_config <<EOF

# MyGit SSH
AuthorizedKeysCommand ${INSTALL_DIR}/backend/scripts/mygit-authorized-keys
AuthorizedKeysCommandUser root
EOF
systemctl restart sshd 2>/dev/null || true

echo ""
echo "============================================"
echo "  MyGit installed successfully!"
echo "============================================"
echo ""
echo "  URL:      https://${DOMAIN}"
echo "  Admin:    ${ADMIN_EMAIL}"
echo "  Password: ${ADMIN_PASSWORD}"
echo ""
echo "  Logs:     journalctl -u mygit-api -f"
echo "  Backup:   ${INSTALL_DIR}/backend/scripts/mygit-backup /backup"
echo ""
echo "  (Optional) run certbot for HTTPS:"
echo "  certbot --nginx -d ${DOMAIN}"
