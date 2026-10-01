#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab v2 - Compilación local (en tu laptop de desarrollo)
#  Genera los artefactos DENTRO de opt/container-environment/ del repo,
#  para que el servidor solo tenga que descargar y ejecutar.
#
#  Uso:  ./build_local.sh [test|production] [dominio]
#    test       -> frontend en modo simulado (mock), para probar sin backend
#    production -> frontend contra la API real
#  Requiere: yarn, go
# =====================================================================
set -euo pipefail

SISTEMA="${1:-test}"
DOMINIO="${2:-raccoon.lab}"
REPO="$(cd "$(dirname "$0")" && pwd)"
OPT="$REPO/opt/container-environment"

case "$SISTEMA" in
  test)       MODO_VITE="development" ;;
  production) MODO_VITE="production" ;;
  *) echo "Uso: $0 [test|production] [dominio]"; exit 1 ;;
esac

for cmd in yarn go; do
  command -v "$cmd" &>/dev/null || { echo "Falta '$cmd' en esta máquina"; exit 1; }
done

VERSION="$(date +%Y.%m.%d-%H%M)"
DIST_TMP="$REPO/internal/web/dist"
DIST_DESTINO="$OPT/${SISTEMA}-system/$DOMINIO/web-server/volumes/internal/web/dist"
BINARIO="$OPT/container-images/raccoon/build/raccoon"

echo "=== Raccoon Lab v2 build | sistema: $SISTEMA | versión: $VERSION ==="

# ---------- 1. Frontend ----------
echo ">> [1/2] Compilando frontend (modo $MODO_VITE)..."
cd "$REPO/frontend"
yarn install --frozen-lockfile
yarn build --mode "$MODO_VITE" --outDir "$DIST_TMP" --emptyOutDir

mkdir -p "$DIST_DESTINO"
rm -rf "${DIST_DESTINO:?}"/*
cp -a "$DIST_TMP"/. "$DIST_DESTINO"/
echo "   -> $DIST_DESTINO"

# ---------- 2. Backend Go ----------
echo ">> [2/2] Compilando backend Go (linux/amd64, estático)..."
cd "$REPO"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
  -o "$BINARIO" ./backend/cmd
chmod +x "$BINARIO"
echo "   -> $BINARIO ($(du -h "$BINARIO" | cut -f1))"

echo ""
echo "=== Listo. Sube los cambios al repo (git add/commit/push) y en el servidor:"
echo "    ./deploy_server.sh $SISTEMA"
