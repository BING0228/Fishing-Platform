#!/usr/bin/env bash
# Deploy the latest Spearhook control plane to a Linux host over SSH.
#
# Usage:
#   ./scripts/deploy-remote.sh <host> [user] [ssh_port]
#
# Examples:
#   ./scripts/deploy-remote.sh 203.0.113.10 root 22
#   SSH_PASSWORD=... PORT=8001 REMOTE_DIR=/opt/spearhook-test SERVICE_NAME=spearhook-test ./scripts/deploy-remote.sh example.com ubuntu 22
#
# Requirements:
#   - SSH access as root (or sudo-capable user); key (BatchMode) or SSH_PASSWORD.
#   - release/spearhook-linux-{amd64,arm64} built (build-all.sh or:
#     GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags='-s -w' -o release/spearhook-linux-amd64 .)
#   - The remote host has systemd and openssl.
#
# The script is idempotent: it replaces the binary/service but never overwrites
# an existing config.yaml or data directory.
set -euo pipefail

HOST=${1:?usage: $0 <host> [user] [ssh_port]}
REMOTE_USER=${2:-root}
SSH_PORT=${3:-22}
REMOTE_DIR=${REMOTE_DIR:-/opt/spearhook}
PORT=${PORT:-8000}
SERVICE_NAME=${SERVICE_NAME:-spearhook}
PUBLIC_BASE_URL=${PUBLIC_BASE_URL:-http://$HOST:$PORT}
if [ -n "${SSH_PASSWORD:-}" ]; then
  ASKPASS_FILE=$(mktemp)
  printf '#!/bin/sh\necho %q\n' "$SSH_PASSWORD" >"$ASKPASS_FILE"
  chmod 700 "$ASKPASS_FILE"
  export SSH_ASKPASS="$ASKPASS_FILE" SSH_ASKPASS_REQUIRE=force DISPLAY="${DISPLAY:-:0}"
  trap 'rm -f "$ASKPASS_FILE"' EXIT
  SSH_OPTS=(-o PreferredAuthentications=password -o PubkeyAuthentication=no -o ConnectTimeout=10 -p "$SSH_PORT")
  SCP_OPTS=(-o PreferredAuthentications=password -o PubkeyAuthentication=no -o ConnectTimeout=10 -P "$SSH_PORT")
else
  SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -p "$SSH_PORT")
  SCP_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -P "$SSH_PORT")
fi

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)

echo "==> checking SSH access to $REMOTE_USER@$HOST:$SSH_PORT"
REMOTE_ARCH=$(ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" 'uname -m')
case "$REMOTE_ARCH" in
  x86_64|amd64) BIN=spearhook-linux-amd64 ;;
  aarch64|arm64) BIN=spearhook-linux-arm64 ;;
  *) echo "unsupported remote arch: $REMOTE_ARCH" >&2; exit 1 ;;
esac
echo "==> remote arch $REMOTE_ARCH -> release/$BIN"
if [ ! -f "$ROOT_DIR/release/$BIN" ]; then
  echo "missing $ROOT_DIR/release/$BIN; build it first" >&2
  exit 1
fi
if [ ! -f "$ROOT_DIR/config.yaml.example" ]; then
  echo "missing config.yaml.example" >&2
  exit 1
fi

echo "==> uploading binary"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" "mkdir -p '$REMOTE_DIR/data'"
scp "${SCP_OPTS[@]}" "$ROOT_DIR/release/$BIN" "$REMOTE_USER@$HOST:$REMOTE_DIR/spearhook.new"
scp "${SCP_OPTS[@]}" "$ROOT_DIR/config.yaml.example" "$REMOTE_USER@$HOST:$REMOTE_DIR/config.yaml.example"

echo "==> installing config/service and starting"
ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" \
  "REMOTE_DIR='$REMOTE_DIR' SERVICE_NAME='$SERVICE_NAME' PUBLIC_BASE_URL='$PUBLIC_BASE_URL' PORT='$PORT' bash -s" <<'REMOTE_SCRIPT'
set -euo pipefail
cd "$REMOTE_DIR"
if [ ! -f config.yaml ]; then
  if ! command -v openssl >/dev/null 2>&1; then
    echo "openssl is required to generate secrets" >&2
    exit 1
  fi
  ENC=$(openssl rand -hex 32)
  JWT=$(openssl rand -hex 32)
  CONTAINER=$(openssl rand -hex 32)
  AGENT=$(openssl rand -hex 32)
  cat > config.yaml <<EOF
server:
  port: $PORT
  gin_mode: release

database:
  path: ./data/fishing.db

security:
  encryption_key: "$ENC"
  jwt_secret: "$JWT"
  container_secret: "$CONTAINER"
  agent_registration_token: "$AGENT"
  platform_basic_auth_enabled: false

container:
  ssl_cert_path: /app/certificates/cert.pem
  ssl_key_path: /app/certificates/key.pem

public_base_url: "$PUBLIC_BASE_URL"

mail_tracking:
  enabled: true
  secret: ""
  allow_redirect_hosts: []
EOF
  chmod 600 config.yaml
  echo "created config.yaml (public_base_url=$PUBLIC_BASE_URL)"
else
  echo "keeping existing config.yaml"
fi

chmod +x spearhook.new
if command -v ss >/dev/null 2>&1 && ss -ltn 2>/dev/null | grep -q ":$PORT "; then
  echo "WARNING: port $PORT is already in use; service may fail to bind" >&2
fi
mv -f spearhook.new spearhook

cat > "/etc/systemd/system/$SERVICE_NAME.service" <<'UNIT'
[Unit]
Description=Spearhook control plane
After=network-online.target docker.service
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/opt/spearhook
ExecStart=/opt/spearhook/spearhook
Restart=always
RestartSec=3
User=root
Environment=TERM=dumb
StandardInput=null

[Install]
WantedBy=multi-user.target
UNIT

sed -i "s|/opt/spearhook|$REMOTE_DIR|g" "/etc/systemd/system/$SERVICE_NAME.service"
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null
systemctl restart "$SERVICE_NAME"

if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q 'Status: active'; then
  ufw allow "$PORT/tcp" >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="$PORT/tcp" >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
fi

sleep 3
systemctl --no-pager --full status "$SERVICE_NAME" | head -20 || true
if [ -f "$REMOTE_DIR/data/admin_password.txt" ]; then
  echo "--- admin credentials ---"
  cat "$REMOTE_DIR/data/admin_password.txt"
fi
REMOTE_SCRIPT

echo "==> done: $PUBLIC_BASE_URL"
