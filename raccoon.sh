#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab v2 - Instalador ÚNICO
#  Instala dependencias, compila (frontend + Go) dentro de contenedores,
#  prepara /opt/container-environment y levanta todo.
#
#  Uso (como usuario normal, NO con sudo; pedirá tu contraseña):
#     ./raccoon.sh                 -> sistema de pruebas (test, puerto 8081)
#     ./raccoon.sh production      -> sistema de producción (puerto 8080)
#
#  Funciona en: Ubuntu Server 24.04 (apt)  y  Arch/EndeavourOS (pacman)
#  Se puede ejecutar las veces que quieras: no borra datos ni el .env
# =====================================================================
set -euo pipefail

SISTEMA="${1:-test}"
DOMINIO="${2:-raccoon.lab}"
REPO="$(cd "$(dirname "$0")" && pwd)"
SRC="$REPO/opt/container-environment"
BASE="/opt/container-environment"
RAIZ="$BASE/${SISTEMA}-system/$DOMINIO"

case "$SISTEMA" in
  test)       PUERTO=8081; PREFIJO="raccoon-test"; MODO_VITE="development" ;;
  production) PUERTO=8080; PREFIJO="raccoon";      MODO_VITE="production" ;;
  *) echo "Uso: $0 [test|production]"; exit 1 ;;
esac

[ "$EUID" -ne 0 ] || { echo "Ejecútalo SIN sudo:  ./raccoon.sh $SISTEMA"; exit 1; }

paso() { echo ""; echo "==> $*"; }
aleatorio() { openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | head -c "$1"; }

echo "================================================================"
echo "  Raccoon Lab v2 | sistema: $SISTEMA | dominio: $DOMINIO"
echo "================================================================"

# =====================================================================
# 1. DEPENDENCIAS (Docker para GNS3, Podman para la plataforma)
# =====================================================================
paso "[1/8] Dependencias del sistema"
. /etc/os-release
DISTRO="${ID}"; [[ "${ID_LIKE:-}" == *arch* ]] && DISTRO="arch"

if [ "$DISTRO" = "ubuntu" ] || [ "$DISTRO" = "debian" ]; then
  sudo apt-get update -qq
  sudo apt-get install -y -qq ca-certificates curl gnupg git openssl tree iproute2 uidmap
  if ! command -v docker &>/dev/null; then
    echo "   Instalando Docker (repositorio oficial)..."
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL "https://download.docker.com/linux/$ID/gpg" -o /etc/apt/keyrings/docker.asc
    sudo chmod a+r /etc/apt/keyrings/docker.asc
    sudo tee /etc/apt/sources.list.d/docker.sources >/dev/null <<EOF
Types: deb
URIs: https://download.docker.com/linux/$ID
Suites: ${UBUNTU_CODENAME:-$VERSION_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF
    sudo apt-get update -qq
    sudo apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
  command -v podman &>/dev/null || sudo apt-get install -y -qq podman podman-compose
elif [ "$DISTRO" = "arch" ]; then
  sudo pacman -S --needed --noconfirm git openssl curl tree iproute2 docker docker-buildx podman podman-compose
else
  echo "Distribución no soportada: $ID (usa Ubuntu Server 24.04 o Arch/EndeavourOS)"; exit 1
fi

sudo systemctl enable --now docker
getent group docker >/dev/null || sudo groupadd docker
sudo usermod -aG docker "$USER"
sudo loginctl enable-linger "$USER"

# Podman sin root necesita rangos de UID/GID para tu usuario
if ! grep -q "^$USER:" /etc/subuid 2>/dev/null; then
  sudo usermod --add-subuids 100000-165535 --add-subgids 100000-165535 "$USER"
  podman system migrate || true
fi

# Si el grupo docker aún no aplica en esta sesión, usamos sudo para docker
if docker info &>/dev/null; then DOCKER="docker"; else DOCKER="sudo docker"; fi
echo "   Docker: $(docker --version | cut -d, -f1) | Podman: $(podman --version)"

# =====================================================================
# 2. COMPILAR FRONTEND (Vue + yarn) dentro de un contenedor de Node
# =====================================================================
paso "[2/8] Compilando frontend (modo $MODO_VITE)"
DIST="$REPO/internal/web/dist"
mkdir -p "$DIST"
podman run --rm \
  -v "$REPO/frontend:/app" \
  -v "$DIST:/salida" \
  -w /app \
  docker.io/library/node:22-alpine \
  sh -c "yarn install --frozen-lockfile --network-timeout 600000 && yarn build --mode $MODO_VITE --outDir /salida --emptyOutDir"
[ -f "$DIST/index.html" ] || { echo "ERROR: el frontend no se generó"; exit 1; }

# =====================================================================
# 3. COMPILAR BACKEND (Go) dentro de un contenedor de Go
# =====================================================================
paso "[3/8] Compilando backend Go"
BIN_DIR="$REPO/internal/bin"
mkdir -p "$BIN_DIR"
VERSION="$(date +%Y.%m.%d-%H%M)"
podman run --rm \
  -v "$REPO:/src" \
  -v "$BIN_DIR:/salida" \
  -w /src \
  -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 \
  docker.io/library/golang:1.24-alpine \
  go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /salida/raccoon ./backend/cmd
[ -x "$BIN_DIR/raccoon" ] || { echo "ERROR: el binario Go no se generó"; exit 1; }

# =====================================================================
# 4. /opt/container-environment
# =====================================================================
paso "[4/8] Preparando $RAIZ"
if [ ! -d "$BASE" ] || [ ! -w "$BASE" ]; then
  sudo mkdir -p "$BASE"
  sudo chown "$USER:$USER" "$BASE"
fi
mkdir -p "$BASE/container-images" "$RAIZ"

# Plantillas del repo (compose, Dockerfiles, ansible). NO toca .env ni datos.
cp -a "$SRC/container-images/." "$BASE/container-images/"
cp -a "$SRC/${SISTEMA}-system/$DOMINIO/." "$RAIZ/"

# Artefactos recién compilados
rm -rf "$RAIZ/web-server/volumes/internal/web/dist"
mkdir -p "$RAIZ/web-server/volumes/internal/web/dist"
cp -a "$DIST/." "$RAIZ/web-server/volumes/internal/web/dist/"
cp "$BIN_DIR/raccoon" "$BASE/container-images/raccoon/build/raccoon"

# Carpetas de datos
mkdir -p "$RAIZ/web-server/volumes/logs"
mkdir -p "$RAIZ/db-engine/volumes/firebird" "$RAIZ/db-engine/volumes/redis" "$RAIZ/db-engine/volumes/init"
mkdir -p "$RAIZ/gns3/volumes/gns3_projects" "$RAIZ/gns3/volumes/gns3_images/IOS" "$RAIZ/gns3/volumes/gns3_images/QEMU"
mkdir -p "$RAIZ/backup/volumes/config" "$RAIZ/backup/volumes/destino"

# =====================================================================
# 5. .env (solo la primera vez; contraseñas aleatorias)
# =====================================================================
paso "[5/8] Variables de entorno"
if [ ! -s "$RAIZ/.env" ]; then
  cat > "$RAIZ/.env" <<EOF
# ===== App Go (web-server) =====
PORT=$PUERTO
ENV=$SISTEMA
GNS3_VOLUMES=$RAIZ/gns3/volumes
JWT_SECRET=$(aleatorio 64)

# Admin inicial (se crea solo en el primer arranque)
ADMIN_CLAVE=ADMIN
ADMIN_PASSWORD=$(aleatorio 16)

# ===== Firebird 5 (db-engine) =====
DB_USER=SYSDBA
DB_PASSWORD=$(aleatorio 24)
DB_HOST=127.0.0.1
DB_PORT=3050
DB_NAME=raccoon.fdb

# ===== Redis 7 (db-engine) =====
REDIS_ADDR=127.0.0.1:6379
REDIS_PASSWORD=$(aleatorio 32)

# ===== GNS3 (Docker) =====
GNS3_BASE_PORT=30000
GNS3_WORKSPACE_IMAGE=gns3-workspace:2.2.59
REGISTRY=127.0.0.1:5000

# ===== Duplicati (backup) =====
DUPLICATI_PASSWORD=$(aleatorio 16)
DUPLICATI_ENCRYPTION_KEY=$(aleatorio 32)

# ===== IA Qwen en la Machenike (Fase 4) =====
IA_URL=
IA_API_KEY=
EOF
  echo "   .env generado"
else
  echo "   .env ya existe (no se modifica)"
fi
chmod 600 "$RAIZ/.env"
PUERTO="$(grep -E '^PORT=' "$RAIZ/.env" | cut -d= -f2)"

# =====================================================================
# 6. DOCKER: registry + imágenes GNS3
# =====================================================================
paso "[6/8] Docker: registry e imágenes GNS3"
if ! $DOCKER ps -a --format '{{.Names}}' | grep -qx registro-local; then
  $DOCKER run -d --name registro-local --restart always \
    -p 5000:5000 -v registro_data:/var/lib/registry registry:2
else
  $DOCKER start registro-local >/dev/null
fi

IMG_GNS3="gns3-workspace:2.2.59"
if ! $DOCKER image inspect "$IMG_GNS3" &>/dev/null || [ "${RECONSTRUIR:-0}" = "1" ]; then
  echo "   Construyendo $IMG_GNS3 (la primera vez tarda varios minutos)..."
  $DOCKER build -t "$IMG_GNS3" "$BASE/container-images/gns3-workspace/build"
else
  echo "   $IMG_GNS3 ya existe"
fi

IMG_HOST="127.0.0.1:5000/mi-ubuntu-gns3:1.0"
if ! $DOCKER image inspect "$IMG_HOST" &>/dev/null || [ "${RECONSTRUIR:-0}" = "1" ]; then
  echo "   Construyendo imagen Ubuntu para nodos..."
  $DOCKER build -t "$IMG_HOST" "$BASE/container-images/ubuntu-host/build"
  $DOCKER push "$IMG_HOST"
else
  echo "   $IMG_HOST ya existe"
fi

# =====================================================================
# 7. PODMAN: bases de datos (Firebird 5 + Redis 7)
# =====================================================================
paso "[7/8] Podman: Firebird + Redis"
cd "$RAIZ/db-engine"
ln -sf ../.env .env          # podman-compose lee ${VARIABLES} de ./.env
podman-compose down 2>/dev/null || true
podman-compose up -d
for i in $(seq 1 30); do
  if (exec 3<>/dev/tcp/127.0.0.1/3050) 2>/dev/null; then echo "   Firebird listo"; break; fi
  sleep 2
  [ "$i" = 30 ] && echo "   AVISO: Firebird no respondió. Revisa: podman logs ${PREFIJO}-firebird"
done

# =====================================================================
# 8. PODMAN: app (Go + frontend)
# =====================================================================
paso "[8/8] Podman: app Raccoon Lab"
podman build -t "localhost/raccoon:$SISTEMA" "$BASE/container-images/raccoon/build"
cd "$RAIZ/web-server"
podman-compose down 2>/dev/null || true
podman-compose up -d

# Arranque automático al reiniciar el servidor
systemctl --user enable --now podman-restart.service &>/dev/null || true

# =====================================================================
# RESULTADO
# =====================================================================
IP="$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -v '^10\.0\.2\.' | grep -v ':' | grep -v '^$' | head -1)"
[ -n "$IP" ] || IP="$(ip -4 -o addr show scope global | awk '{print $4}' | cut -d/ -f1 | grep -v '^10\.0\.2\.' | head -1)"
sleep 3
echo ""
echo "================================================================"
if curl -fsS "http://127.0.0.1:$PUERTO/api/salud" >/dev/null 2>&1; then
  echo "  Raccoon Lab ($SISTEMA) EN LÍNEA:  http://${IP:-<IP>}:$PUERTO"
else
  echo "  La app no respondió. Revisa:  podman logs ${PREFIJO}-app"
fi
echo "  Admin inicial: ADMIN / $(grep '^ADMIN_PASSWORD=' "$RAIZ/.env" | cut -d= -f2)"
podman ps --format "  {{.Names}}  ->  {{.Status}}"
echo "================================================================"
if [ "$DOCKER" = "sudo docker" ]; then
  echo "  Nota: cierra sesión y vuelve a entrar para usar 'docker' sin sudo."
fi
