#!/bin/bash
# MyGit (Go) - one-line installer/updater (Linux)
# The SAME command installs on first run and safely UPDATES on subsequent runs.
# Usage: curl -sSL https://raw.githubusercontent.com/ajjs1ajjs/MyGit/main/install.sh | sudo bash

set -e

INSTALL_DIR="/opt/mygit"
DATA_DIR="/var/lib/mygit"
REPOS_DIR="/var/lib/mygit/repos"
SERVICE_NAME="mygit"
VERSION="${MYGIT_VERSION:-latest}"
REPO="ajjs1ajjs/MyGit"
# Port to bind. If the default 8080 is already taken (e.g. by another service),
# set MYGIT_PORT to a free port.
PORT="${MYGIT_PORT:-8080}"

if [ "$(id -u)" -ne 0 ]; then
    echo "Please run as root (sudo ./install.sh)"
    exit 1
fi

IS_UPDATE=0
if [ -f "/etc/systemd/system/$SERVICE_NAME.service" ] || [ -x "$INSTALL_DIR/mygit" ] || [ -f "$DATA_DIR/mygit.db" ]; then
    IS_UPDATE=1
fi
if [ "$IS_UPDATE" = "1" ]; then MODE="Оновлення (update)"; else MODE="Встановлення (install)"; fi

echo "=============================================="
echo "   MyGit - $MODE"
echo "=============================================="
echo ""

OLD_VERSION=""
if [ -x "$INSTALL_DIR/mygit" ]; then
    OLD_VERSION="$($INSTALL_DIR/mygit --version 2>/dev/null || echo "?")"
fi

case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "ERROR: unsupported architecture"; exit 1 ;;
esac
BINARY_NAME="mygit-linux-${ARCH}"

if [ "$VERSION" = "latest" ]; then
    DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
else
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
fi

echo "[1/4] Downloading MyGit ${VERSION} (${BINARY_NAME})..."
TMP_BIN="$(mktemp)"
if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_BIN" || { echo "ERROR: download failed"; rm -f "$TMP_BIN"; exit 1; }
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$TMP_BIN" "$DOWNLOAD_URL" || { echo "ERROR: download failed"; rm -f "$TMP_BIN"; exit 1; }
else
    echo "ERROR: curl or wget required"; rm -f "$TMP_BIN"; exit 1
fi
chmod +x "$TMP_BIN"
"$TMP_BIN" --version >/dev/null 2>&1 || { echo "ERROR: not a valid binary"; rm -f "$TMP_BIN"; exit 1; }

echo "[2/4] Installing binary..."
mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$REPOS_DIR"
if [ -d "$INSTALL_DIR/mygit" ]; then rm -rf "$INSTALL_DIR/mygit"; fi
if [ "$IS_UPDATE" = "1" ] && [ -f "$INSTALL_DIR/mygit" ]; then
    cp -f "$INSTALL_DIR/mygit" "$INSTALL_DIR/mygit.old" 2>/dev/null || true
fi
install -m 0755 "$TMP_BIN" "$INSTALL_DIR/mygit"
rm -f "$TMP_BIN"

echo "[3/4] Configuring system user and service..."
if ! id mygit >/dev/null 2>&1; then
    useradd -r -s /bin/false -d "$INSTALL_DIR" mygit
fi
chown -R mygit:mygit "$INSTALL_DIR" "$DATA_DIR"
systemctl stop $SERVICE_NAME 2>/dev/null || true

cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
[Unit]
Description=MyGit - self-hosted Git platform
After=network.target

[Service]
User=mygit
Group=mygit
ExecStart=$INSTALL_DIR/mygit -port $PORT
Restart=always
RestartSec=5
Environment=MYGIT_BASE_DIR=$DATA_DIR
Environment=MYGIT_REPOS_ROOT=$REPOS_DIR
Environment=MYGIT_DB_PATH=$DATA_DIR/mygit.db

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable $SERVICE_NAME
systemctl restart $SERVICE_NAME

# Wait for MYGIT's own health endpoint. Checking / on the port is NOT enough —
# any other service (e.g. uptime-monitor) would answer and give a false OK.
echo -n "Waiting for MyGit on port $PORT to become healthy..."
for i in $(seq 1 15); do
    if curl -fsS "http://localhost:$PORT/api/v1/health" >/dev/null 2>&1; then echo " OK"; break; fi
    if [ "$i" = "15" ]; then echo " FAILED — is port $PORT free? (another service may be using it)"; exit 1; else echo -n "."; sleep 1; fi
done

echo "[4/4] Done."
echo ""
if [ "$IS_UPDATE" = "1" ]; then
    echo "MyGit updated: ${OLD_VERSION} -> ${NEW_VERSION:-$VERSION}"
    echo "Config, repositories and users preserved."
else
    echo "MyGit installed. Version: ${VERSION}"
fi
echo ""
echo "Dashboard: http://localhost:$PORT/"
echo "Зареєструйте ПЕРШИЙ обліковий запис — він стане власником (superuser)."
echo ""
if [ -f "$INSTALL_DIR/mygit.old" ]; then echo "Previous binary kept at: $INSTALL_DIR/mygit.old"; fi
echo "Installed version: $("$INSTALL_DIR/mygit" --version)"
