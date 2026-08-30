#!/usr/bin/env bash
set -Eeuo pipefail

readonly REPO_URL="${OPORA_REPO_URL:-https://github.com/ssssargsian/opora.git}"
readonly DEPLOY_DIR="${OPORA_DEPLOY_DIR:-/opt/opora}"
readonly EXPECTED_IP="${OPORA_EXPECTED_IP:-155.212.223.107}"

if [[ ${EUID} -ne 0 ]]; then
  echo "Run this script as root." >&2
  exit 1
fi

. /etc/os-release
if [[ ${ID:-} != "ubuntu" ]]; then
  echo "Ubuntu is required." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl git openssl ufw

if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  arch="$(dpkg --print-architecture)"
  printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu %s stable\n' \
    "$arch" "${VERSION_CODENAME}" >/etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get install -y --no-install-recommends docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
systemctl enable --now docker

cpu_count="$(nproc)"
memory_kib="$(awk '/MemTotal/ {print $2}' /proc/meminfo)"
docker_free_kib="$(df --output=avail -k /var/lib/docker | tail -1 | tr -d ' ')"
if (( cpu_count < 2 || memory_kib < 3670016 || docker_free_kib < 41943040 )); then
  cat >&2 <<EOF
Server capacity is insufficient for the production Opora stack with ONLYOFFICE and ClamAV.
Required: at least 2 vCPU, 4 GiB RAM, and 40 GiB free for Docker data.
Detected: ${cpu_count} vCPU, $((memory_kib / 1024)) MiB RAM, $((docker_free_kib / 1024)) MiB free.
Resize the server and run this script again; no application data or secrets were changed.
EOF
  exit 1
fi

ssh_port="$(sshd -T 2>/dev/null | awk '$1 == "port" { print $2; exit }')"
ssh_port="${ssh_port:-22}"
ufw default deny incoming
ufw default allow outgoing
ufw allow "${ssh_port}/tcp"
ufw allow 80/tcp
ufw allow 443/tcp
ufw --force enable

# Swap protects the document services from short memory spikes; it does not replace the RAM preflight above.
if ! swapon --show=NAME --noheadings | grep -q .; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
fi

if [[ -d "${DEPLOY_DIR}/.git" ]]; then
  git -C "$DEPLOY_DIR" fetch --prune origin
  git -C "$DEPLOY_DIR" pull --ff-only
else
  if [[ -e "$DEPLOY_DIR" ]] && [[ -n "$(find "$DEPLOY_DIR" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
    echo "${DEPLOY_DIR} exists and is not an Opora checkout." >&2
    exit 1
  fi
  mkdir -p "$(dirname "$DEPLOY_DIR")"
  git clone "$REPO_URL" "$DEPLOY_DIR"
fi

cd "$DEPLOY_DIR"
env_file="${DEPLOY_DIR}/.env.production"
if [[ ! -f "$env_file" ]]; then
  umask 077
  postgres_password="$(openssl rand -hex 32)"
  minio_user="opora_$(openssl rand -hex 8)"
  minio_password="$(openssl rand -hex 32)"
  onlyoffice_secret="$(openssl rand -hex 48)"
  cat >"$env_file" <<EOF
APP_DOMAIN=opora.cyberlineup.ru
OFFICE_DOMAIN=office.opora.cyberlineup.ru
POSTGRES_DB=opora
POSTGRES_USER=opora
POSTGRES_PASSWORD=${postgres_password}
MINIO_ROOT_USER=${minio_user}
MINIO_ROOT_PASSWORD=${minio_password}
S3_REGION=us-east-1
S3_BUCKET=opora-documents
ONLYOFFICE_JWT_SECRET=${onlyoffice_secret}
EOF
fi
chmod 600 "$env_file"

for domain in opora.cyberlineup.ru office.opora.cyberlineup.ru; do
  resolved="$(getent ahostsv4 "$domain" 2>/dev/null | awk '{print $1; exit}' || true)"
  if [[ "$resolved" != "$EXPECTED_IP" ]]; then
    echo "WARNING: ${domain} resolves to ${resolved:-nothing}, expected ${EXPECTED_IP}; HTTPS validation will wait for DNS." >&2
  fi
done

exec "${DEPLOY_DIR}/deploy/deploy.sh"
