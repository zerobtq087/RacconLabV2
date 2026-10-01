#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab v2 - Despliegue en el servidor (Ubuntu Server 24.04 LTS)
#  Copia opt/container-environment/ del repo a /opt/container-environment/,
#  genera el .env (solo la primera vez), construye imágenes y levanta todo.
#
#  Uso (SIN sudo; pedirá contraseña solo si hace falta):
#     ./deploy_server.sh [test|production] [dominio]
#  Requiere: haber corrido antes  sudo ./instalar_dependencias.sh
# =====================================================================
set -euo pipefail

SISTEMA="${1:-test}"
DOMINIO="${2:-raccoon.lab}"
REPO="$(cd "$(dirname "$0")" && pwd)"
SRC="$REPO/opt/container-environment"
BASE="/opt/container-environment"
RAIZ="$BASE/${SISTEMA}-system/$DOMINIO"

case "$SISTEMA" in
  test)       PUERTO=8081 ;;
  production) PUERTO=8080 ;;
  *) echo "Uso: $0 [test|production] [dominio]"; exit 1 ;;
esac

[ "$EUID" -ne 0 ] || { echo "Ejecútalo SIN sudo (Podman corre con tu usuario)"; exit 1; }

paso() { echo ""; echo ">> $*"; }
aleatorio() { openssl rand -base64 48 | tr -dc 'A-Za-z0-9' | head -c "$1"; }

echo "=== Raccoon Lab v2 deploy | sistema: $SISTEMA | dominio: $DOMINIO ==="

# ---------- 0. Verificaciones ----------
for cmd in docker podman podman-compose openssl; do
  command -v "$cmd" &>/dev/null || { echo "Falta '$cmd'. Corre: sudo ./instalar_dependencias.sh"; exit 1; }
done
docker info &>/dev/null || { echo "No tienes acceso a Docker. Cierra sesión y vuelve a entrar (grupo docker)."; exit 1; }
[ -d "$SRC/${SISTEMA}-system/$DOMINIO" ] || { echo "No existe $SRC/${SISTEMA}-system/$DOMINIO en el repo"; exit 1; }
[ -f "$SRC/${SISTEMA}-system/$DOMINIO/web-server/volumes/internal/web/dist/index.html" ] || \
  { echo "Falta el frontend compilado. En tu laptop: ./build_local.sh $SISTEMA"; exit 1; }
[ -x "$SRC/container-images/raccoon/build/raccoon" ] || \
  { echo "Falta el binario Go. En tu laptop: ./build_local.sh $SISTEMA"; exit 1; }

# ---------- 1. /opt con permisos para tu usuario ----------
paso "[1/6] Preparando $BASE"
if [ ! -d "$BASE" ] || [ ! -w "$BASE" ]; then
  sudo mkdir -p "$BASE"
  sudo chown "$USER:$USER" "$BASE"
fi

# ---------- 2. Copiar plantillas y artefactos del repo ----------
paso "[2/6] Copiando archivos del repo"
mkdir -p "$BASE/container-images" "$RAIZ"
cp -a "$SRC/container-images/." "$BASE/container-images/"

# El frontend se reemplaza completo (los nombres de assets cambian en cada build)
rm -rf "$RAIZ/web-server/volumes/internal/web/dist"
# Copia todo lo del sistema; NO toca .env ni datos que no vienen en el repo
cp -a "$SRC/${SISTEMA}-system/$DOMINIO/." "$RAIZ/"

# Carpetas de datos (no vienen en el repo)
mkdir -p "$RAIZ/web-server/volumes/logs"
mkdir -p "$RAIZ/db-engine/volumes/firebird" "$RAIZ/db-engine/volumes/redis"
mkdir -p "$RAIZ/gns3/volumes/gns3_projects"
mkdir -p "$RAIZ/gns3/volumes/gns3_images/IOS" "$RAIZ/gns3/volumes/gns3_images/QEMU"
mkdir -p "$RAIZ/backup/volumes/config" "$RAIZ/backup/volumes/destino"

# ---------- 3. .env (solo la primera vez) ----------
paso "[3/6] Variables de entorno"
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
  echo "   .env generado en $RAIZ/.env"
else
  echo "   .env ya existe (no se modifica)"
fi
chmod 600 "$RAIZ/.env"
# El puerto real es el del .env (por si ya existía)
PUERTO="$(grep -E '^PORT=' "$RAIZ/.env" | cut -d= -f2 || echo "$PUERTO")"
echo "   Puerto de la app: $PUERTO"

# ---------- 4. Docker: registry + imágenes GNS3 ----------
paso "[4/6] Docker (registry e imágenes GNS3)"
if ! docker ps -a --format '{{.Names}}' | grep -qx registro-local; then
  docker run -d --name registro-local --restart always \
    -p 5000:5000 -v registro_data:/var/lib/registry registry:2
else
  docker start registro-local >/dev/null
  echo "   registry ya existe"
fi

IMG_GNS3="gns3-workspace:2.2.59"
if ! docker image inspect "$IMG_GNS3" &>/dev/null || [ "${RECONSTRUIR:-0}" = "1" ]; then
  echo "   Construyendo $IMG_GNS3 (puede tardar varios minutos)..."
  docker build -t "$IMG_GNS3" "$BASE/container-images/gns3-workspace/build"
else
  echo "   $IMG_GNS3 ya existe (RECONSTRUIR=1 para forzar)"
fi

IMG_HOST="127.0.0.1:5000/mi-ubuntu-gns3:1.0"
if ! docker image inspect "$IMG_HOST" &>/dev/null || [ "${RECONSTRUIR:-0}" = "1" ]; then
  echo "   Construyendo imagen Ubuntu para nodos..."
  docker build -t "$IMG_HOST" "$BASE/container-images/ubuntu-host/build"
  docker push "$IMG_HOST"
else
  echo "   $IMG_HOST ya existe"
fi

# ---------- 5. Podman: imagen de la app ----------
paso "[5/6] Podman: imagen localhost/raccoon:$SISTEMA"
podman build -t "localhost/raccoon:$SISTEMA" "$BASE/container-images/raccoon/build"

# ---------- 6. Levantar servicios ----------
paso "[6/6] Levantando web-server"
cd "$RAIZ/web-server"
podman-compose down 2>/dev/null || true
podman-compose up -d

# Arranque automático tras reiniciar el servidor
systemctl --user enable --now podman-restart.service &>/dev/null || true

# ---------- Resultado ----------
IP="$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src"){print $(i+1); exit}}')"
sleep 2
echo ""
if curl -fsS "http://127.0.0.1:$PUERTO/api/salud" >/dev/null 2>&1; then
  echo "=== Raccoon Lab ($SISTEMA) en línea: http://${IP:-<IP>}:$PUERTO"
else
  echo "=== La app no respondió todavía. Revisa:  podman logs raccoon$([ "$SISTEMA" = test ] && echo -test)-app"
fi
echo "    Admin inicial: ADMIN / $(grep '^ADMIN_PASSWORD=' "$RAIZ/.env" | cut -d= -f2)"
