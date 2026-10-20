#!/usr/bin/env bash
# One-command Spearhook deployment over SSH.
#
#   ./scripts/deploy.sh binary <host> [user] [ssh_port]
#   ./scripts/deploy.sh docker <host> [user] [ssh_port]
#
# Common env:
#   PORT=8000                 application port (binary + container)
#   PUBLIC_BASE_URL=...       external URL stored in config
#   REMOTE_DIR=/opt/spearhook remote install directory
#   SERVICE_NAME=spearhook    systemd unit / docker container name
#   SSH_PASSWORD=...          password auth (otherwise SSH key/BatchMode)
#   INSTALL_CHROMIUM=1        binary mode: install Debian chromium for SPA clone
#   INSTALL_PYTHON=1          binary mode: install Python/Flask for local projects
#   INSTALL_DOCKER=0          docker mode: apt install docker.io if missing
#   SKIP_BUILD=0              reuse existing binary/image
#   BUILD_FRONTEND=0          binary mode: force npm build
#   IMAGE=spearhook:latest    docker image tag
#   DRY_RUN=0                 print actions without executing
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  ./scripts/deploy.sh binary <host> [user] [ssh_port]
  ./scripts/deploy.sh docker <host> [user] [ssh_port]

Modes:
  binary   cross-compile a Linux binary, install systemd service + Chromium
  docker   build a Linux image (includes Chromium), load it, run a container

Examples:
  SSH_PASSWORD=... ./scripts/deploy.sh binary 203.0.113.10 root 22
  SSH_PASSWORD=... INSTALL_DOCKER=1 ./scripts/deploy.sh docker 203.0.113.10 root 22

See docs/deployment.md for details.
EOF
}

MODE="${1:-}"
case "$MODE" in
  binary|docker) ;;
  -h|--help|help|"") usage; exit 0 ;;
  *) echo "unsupported mode: $MODE" >&2; usage >&2; exit 2 ;;
esac
HOST="${2:-}"
if [ -z "$HOST" ]; then
  echo "host is required" >&2
  usage >&2
  exit 2
fi
REMOTE_USER="${3:-root}"
SSH_PORT="${4:-22}"
PORT="${PORT:-8000}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-http://$HOST:$PORT}"
REMOTE_DIR="${REMOTE_DIR:-/opt/spearhook}"
SERVICE_NAME="${SERVICE_NAME:-spearhook}"
IMAGE="${IMAGE:-spearhook:latest}"
INSTALL_CHROMIUM="${INSTALL_CHROMIUM:-1}"
INSTALL_PYTHON="${INSTALL_PYTHON:-1}"
INSTALL_DOCKER="${INSTALL_DOCKER:-0}"
SKIP_BUILD="${SKIP_BUILD:-0}"
BUILD_FRONTEND="${BUILD_FRONTEND:-0}"
DRY_RUN="${DRY_RUN:-0}"

if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
  echo "invalid PORT: $PORT" >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [ -n "${SSH_PASSWORD:-}" ]; then
  ASKPASS_FILE="$(mktemp)"
  printf '#!/bin/sh\necho %q\n' "$SSH_PASSWORD" >"$ASKPASS_FILE"
  chmod 700 "$ASKPASS_FILE"
  export SSH_ASKPASS="$ASKPASS_FILE" SSH_ASKPASS_REQUIRE=force DISPLAY="${DISPLAY:-:0}"
  trap 'rm -f "$ASKPASS_FILE"' EXIT
  SSH_OPTS=(-o PreferredAuthentications=password -o PubkeyAuthentication=no -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p "$SSH_PORT")
  SCP_OPTS=(-o PreferredAuthentications=password -o PubkeyAuthentication=no -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -P "$SSH_PORT")
else
  SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p "$SSH_PORT")
  SCP_OPTS=(-o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -P "$SSH_PORT")
fi

log() { printf '\n==> %s\n' "$*"; }
die() { echo "ERROR: $*" >&2; exit 1; }
run() {
  if [ "$DRY_RUN" = 1 ]; then
    printf '[dry-run] '; printf '%q ' "$@"; echo
    return 0
  fi
  "$@"
}
ssh_run() {
  if [ "$DRY_RUN" = 1 ]; then
    printf '[dry-run] ssh %s@%s ' "$REMOTE_USER" "$HOST"
    printf '%q ' "$@"; echo
    return 0
  fi
  ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" "$@"
}
scp_to() {
  local src="$1" dst="$2"
  if [ "$DRY_RUN" = 1 ]; then
    echo "[dry-run] scp $src $REMOTE_USER@$HOST:$dst"
    return 0
  fi
  scp "${SCP_OPTS[@]}" "$src" "$REMOTE_USER@$HOST:$dst"
}
remote_script() {
  local envs="$1"
  if [ "$DRY_RUN" = 1 ]; then
    echo "[dry-run] remote script: $envs"
    sed 's/^/    /'
    return 0
  fi
  ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" "$envs bash -s"
}

log "target $REMOTE_USER@$HOST:$SSH_PORT  mode=$MODE  port=$PORT"
if [ "$DRY_RUN" = 1 ]; then
  ARCH=amd64
  echo "[dry-run] assume remote arch amd64"
else
  REMOTE_ARCH="$(ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" 'uname -m')"
  case "$REMOTE_ARCH" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) die "unsupported remote architecture: $REMOTE_ARCH" ;;
  esac
  echo "remote arch: $REMOTE_ARCH -> linux/$ARCH"
fi

# ---------------------------------------------------------------------------
# Shared: ensure config.yaml + data directory (never overwrite existing)
# ---------------------------------------------------------------------------
ensure_remote_config() {
  log "ensuring remote config and data directory"
  remote_script "REMOTE_DIR='$REMOTE_DIR' PORT='$PORT' PUBLIC_BASE_URL='$PUBLIC_BASE_URL'" <<'REMOTE'
set -euo pipefail
mkdir -p "$REMOTE_DIR/data"
cd "$REMOTE_DIR"
if [ -f config.yaml ]; then
  echo "keeping existing config.yaml"
  exit 0
fi
command -v openssl >/dev/null 2>&1 || { echo "openssl is required" >&2; exit 1; }
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
REMOTE
}

ensure_remote_firewall() {
  log "opening port $PORT when a host firewall is active"
  remote_script "PORT='$PORT'" <<'REMOTE'
set -euo pipefail
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q 'Status: active'; then
  ufw allow "$PORT/tcp" >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="$PORT/tcp" >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
fi
REMOTE
}

print_admin_credentials() {
  log "initial admin credentials"
  ssh_run "sleep 2; cat '$REMOTE_DIR/data/admin_password.txt' 2>/dev/null || echo 'admin_password.txt not found yet; check service logs'"
}

ensure_remote_config

# ---------------------------------------------------------------------------
# Binary mode
# ---------------------------------------------------------------------------
deploy_binary() {
  local bin="$ROOT_DIR/release/spearhook-linux-$ARCH"
  if [ "$SKIP_BUILD" != 1 ]; then
    if [ "$BUILD_FRONTEND" = 1 ] || [ ! -f "$ROOT_DIR/frontend/dist/index.html" ]; then
      log "building frontend"
      (cd "$ROOT_DIR/frontend" && run npm ci --no-audit --no-fund && run npm run build)
    fi
    log "building Linux/$ARCH binary"
    mkdir -p "$ROOT_DIR/release"
    (cd "$ROOT_DIR" && run env GOOS=linux GOARCH="$ARCH" CGO_ENABLED=0 go build -ldflags='-s -w' -o "$bin" .)
  fi
  [ -f "$bin" ] || die "missing $bin (run without SKIP_BUILD=1 or build it first)"

  if [ "$INSTALL_CHROMIUM" != 0 ]; then
    log "ensuring Chromium for headless QR clone"
    ssh_run 'if ! command -v chromium >/dev/null 2>&1; then apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq chromium >/dev/null; fi'
  else
    echo "WARNING: INSTALL_CHROMIUM=0; SPA/QR headless clone will fail unless Chromium is already installed"
  fi
  if [ "$INSTALL_PYTHON" != 0 ]; then
    log "ensuring Python/Flask for local project runtime"
    ssh_run 'if ! python3 -c "import flask, flask_cors, requests" >/dev/null 2>&1; then apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq python3 python3-flask python3-flask-cors python3-requests >/dev/null; fi'
  fi

  log "uploading binary"
  ssh_run "mkdir -p '$REMOTE_DIR/data'"
  scp_to "$bin" "$REMOTE_DIR/spearhook.new"

  log "installing systemd service"
  remote_script "REMOTE_DIR='$REMOTE_DIR' SERVICE_NAME='$SERVICE_NAME'" <<'REMOTE'
set -euo pipefail
cd "$REMOTE_DIR"
chmod +x spearhook.new
mv -f spearhook.new spearhook
cat > "/etc/systemd/system/$SERVICE_NAME.service" <<UNIT
[Unit]
Description=Spearhook control plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$REMOTE_DIR
ExecStart=$REMOTE_DIR/spearhook
Restart=always
RestartSec=3
User=root
Environment=TERM=dumb
Environment=CHROME_PATH=/usr/bin/chromium
StandardInput=null

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable "$SERVICE_NAME" >/dev/null
systemctl restart "$SERVICE_NAME"
REMOTE

  ensure_remote_firewall
  log "service status"
  ssh_run "systemctl --no-pager --full status '$SERVICE_NAME' | head -20 || true"
  print_admin_credentials
  echo
  echo "Binary deployment complete: http://$HOST:$PORT"
  echo "Manage: systemctl status $SERVICE_NAME; journalctl -u $SERVICE_NAME -f"
}

# ---------------------------------------------------------------------------
# Docker mode
# ---------------------------------------------------------------------------
deploy_docker() {
  command -v docker >/dev/null 2>&1 || die "local Docker is required to build the image"
  run docker version >/dev/null

  if [ "$SKIP_BUILD" != 1 ]; then
    log "building image $IMAGE for linux/$ARCH"
    run docker build --platform "linux/$ARCH" -t "$IMAGE" -f "$ROOT_DIR/Dockerfile" "$ROOT_DIR"
  fi

  log "checking remote Docker"
  if ! ssh_run 'command -v docker >/dev/null 2>&1'; then
    if [ "$INSTALL_DOCKER" = 1 ]; then
      log "installing Docker on target"
      ssh_run 'apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq docker.io >/dev/null && systemctl enable --now docker'
    else
      die "remote Docker is not installed; rerun with INSTALL_DOCKER=1 or install Docker manually"
    fi
  fi
  ssh_run 'docker info >/dev/null 2>&1 || { echo "remote Docker daemon is not running" >&2; exit 1; }'

  log "loading image on target (this can take a while)"
  if [ "$DRY_RUN" = 1 ]; then
    echo "[dry-run] docker save $IMAGE | ssh $REMOTE_USER@$HOST 'docker load'"
  else
    docker save "$IMAGE" | ssh "${SSH_OPTS[@]}" "$REMOTE_USER@$HOST" 'docker load'
  fi

  log "starting container"
  # Host network: local Flask projects bind host ports. Docker socket: sibling
  # project containers are created on the host daemon.
  ssh_run "docker rm -f '$SERVICE_NAME' >/dev/null 2>&1 || true; docker run -d --name '$SERVICE_NAME' --restart unless-stopped --network host -v '$REMOTE_DIR/config.yaml:/app/config.yaml:ro' -v '$REMOTE_DIR/data:/app/data' -v '/var/run/docker.sock:/var/run/docker.sock' -e CHROME_PATH=/usr/bin/chromium -e DOCKER_HOST=unix:///var/run/docker.sock -e TZ=Asia/Shanghai '$IMAGE'"

  ensure_remote_firewall
  log "container status"
  ssh_run "docker ps --filter 'name=^/$SERVICE_NAME$' --format '{{.Status}}' || true"
  print_admin_credentials
  echo
  echo "Docker deployment complete: http://$HOST:$PORT"
  echo "Manage: docker logs -f $SERVICE_NAME; docker restart $SERVICE_NAME"
}

case "$MODE" in
  binary) deploy_binary ;;
  docker) deploy_docker ;;
esac

echo
echo "Done."
