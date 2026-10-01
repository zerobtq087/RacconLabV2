#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab - Dependencias del servidor (Ubuntu Server 24.04 LTS)
#  Equivale a la sección 2 del manual + Podman
#  Docker  -> workspaces GNS3 y registry (igual que antes)
#  Podman  -> plataforma (app Go, Firebird, Redis)
#
#  Uso:  sudo ./instalar_dependencias.sh
#  Se puede ejecutar varias veces sin problema.
# =====================================================================
set -euo pipefail

[ "$EUID" -eq 0 ] || { echo "Ejecuta con sudo: sudo $0"; exit 1; }
USUARIO="${SUDO_USER:-$USER}"

. /etc/os-release
[ "$ID" = "ubuntu" ] || { echo "Este script es para Ubuntu (detectado: $ID)"; exit 1; }

echo ">> [1/4] Paquetes base (manual 2a)"
apt-get update
apt-get install -y ca-certificates curl gnupg git unzip zip tree openssl \
  iputils-ping iproute2 net-tools dnsutils nano

echo ">> [2/4] Docker (manual 2b-2e)"
if ! command -v docker &>/dev/null; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${UBUNTU_CODENAME:-$VERSION_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
else
  echo "   ya instalado: $(docker --version)"
fi
systemctl enable --now docker

echo ">> [3/4] Podman"
if ! command -v podman &>/dev/null; then
  apt-get install -y podman podman-compose
else
  echo "   ya instalado: $(podman --version)"
fi

echo ">> [4/4] Permisos para $USUARIO (manual 2f)"
getent group docker >/dev/null || groupadd docker
usermod -aG docker "$USUARIO"
# Los contenedores de Podman siguen vivos aunque se cierre la sesión SSH
loginctl enable-linger "$USUARIO"

echo ""
echo "=== Listo. Cierra sesión y vuelve a entrar (para el grupo docker)."
echo "    Luego, SIN sudo:  ./deploy_server.sh test"
