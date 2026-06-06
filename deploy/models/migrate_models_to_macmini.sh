#!/usr/bin/env bash
set -euo pipefail

SSH_HOST="${WEKNORA_MACMINI_HOST:-192.168.8.139}"
SSH_USER="${WEKNORA_MACMINI_USER:-}"
REMOTE_DIR="${WEKNORA_MACMINI_MODELS_DIR:-~/weknora_models}"

EMBED_IMAGE="${WEKNORA_EMBED_IMAGE:-embedding-service-embedding-service:latest}"
RERANK_IMAGE="${WEKNORA_RERANK_IMAGE:-rerank-service-rerank-service:latest}"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE_LOCAL="${SCRIPT_DIR}/docker-compose.models.yml"

SSH_TARGET="$SSH_HOST"
if [[ -n "$SSH_USER" ]]; then
  SSH_TARGET="${SSH_USER}@${SSH_HOST}"
fi

REMOTE_HOME="$(ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" 'printf %s "$HOME"')"
case "$REMOTE_DIR" in
  "~")
    REMOTE_DIR="${REMOTE_HOME}"
    ;;
  "~/"*)
    REMOTE_DIR="${REMOTE_HOME}/${REMOTE_DIR:2}"
    ;;
esac

REMOTE_DOCKER_BIN="$(ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" '
  if command -v docker >/dev/null 2>&1; then
    command -v docker
  elif [ -x /Applications/Docker.app/Contents/Resources/bin/docker ]; then
    printf %s /Applications/Docker.app/Contents/Resources/bin/docker
  else
    exit 127
  fi
')"

echo "==> Verifying local images exist"
docker image inspect "$EMBED_IMAGE" >/dev/null
docker image inspect "$RERANK_IMAGE" >/dev/null

echo "==> Preparing remote directory: ${SSH_TARGET}:${REMOTE_DIR}"
ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" "mkdir -p \"$REMOTE_DIR\""

echo "==> Copying compose file"
scp -q "$COMPOSE_FILE_LOCAL" "${SSH_TARGET}:${REMOTE_DIR}/docker-compose.models.yml"

echo "==> Transferring images (docker save | ssh docker load)"
docker save "$EMBED_IMAGE" "$RERANK_IMAGE" | ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" "\"$REMOTE_DOCKER_BIN\" load"

echo "==> Starting services on Mac mini"
ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" "cd \"$REMOTE_DIR\" && \"$REMOTE_DOCKER_BIN\" compose -f docker-compose.models.yml up -d"

echo "==> Verifying services are up"
ssh -o BatchMode=yes -o ConnectTimeout=15 "$SSH_TARGET" "\"$REMOTE_DOCKER_BIN\" ps --format 'table {{.Names}}\\t{{.Image}}\\t{{.Status}}\\t{{.Ports}}' | (sed -n '1p'; rg -n 'embedding-service|rerank-service' -n || true)"

cat <<'TXT'

Next: update WeKnora model endpoints (on the Mac mini, in WeKnora model settings):
- Embedding base_url: http://host.docker.internal:8100   (if WeKnora runs in Docker and you keep host port mapping)
- Rerank base_url:    http://host.docker.internal:8101

If you instead add these services into the same docker-compose network as WeKnora,
prefer using container DNS names:
- Embedding base_url: http://embedding-service:8100
- Rerank base_url:    http://rerank-service:8101
TXT
