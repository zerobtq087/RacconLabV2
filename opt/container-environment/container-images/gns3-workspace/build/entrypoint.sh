#!/bin/bash
# Raccoon Lab - arranque del laboratorio GNS3
# Las variables las manda la API de Go al crear el contenedor
set -e

# Docker interno (para nodos Docker dentro de GNS3)
echo "Iniciando Docker interno..."
rm -f /var/run/docker.pid 2>/dev/null || true
dockerd --host=unix:///var/run/docker.sock --storage-driver=vfs --log-level=error &
until docker info &>/dev/null; do
    echo "Esperando Docker..."
    sleep 2
done

# Host Ubuntu para las prácticas: se carga desde gns3_images/Docker/ubuntu-gns3.tar
# (no está en git) y se le pone un nombre fijo, sin importar el tag que traiga el .tar.
# La plantilla de GNS3 y la API de Go usan ese nombre fijo.
UBUNTU_TAR=/root/GNS3/images/Docker/ubuntu-gns3.tar
UBUNTU_IMAGE=${UBUNTU_IMAGE:-raccoon/ubuntu-host:latest}
if [ -f "$UBUNTU_TAR" ] && ! docker image inspect "$UBUNTU_IMAGE" &>/dev/null; then
    echo "Cargando la imagen Ubuntu desde $UBUNTU_TAR..."
    if salida=$(docker load -i "$UBUNTU_TAR"); then
        # docker load responde "Loaded image: nombre:tag" o "Loaded image ID: sha256:..."
        cargada=$(echo "$salida" | sed -n 's/^Loaded image\( ID\)\?: //p' | head -n 1)
        docker tag "$cargada" "$UBUNTU_IMAGE" && echo "Imagen Ubuntu lista: $UBUNTU_IMAGE (era $cargada)"
    else
        echo "AVISO: no se pudo cargar $UBUNTU_TAR; el laboratorio sigue sin el host Ubuntu"
    fi
fi

PORT=${GNS3_PORT:-3080}
CONSOLE_START=${GNS3_CONSOLE_START:-5000}
CONSOLE_END=${GNS3_CONSOLE_END:-5039}
GNS3_USER=${GNS3_USER:-raccoon}
GNS3_PASSWORD=${GNS3_PASSWORD:?Falta GNS3_PASSWORD}

mkdir -p /root/.config/GNS3/2.2

# host = 0.0.0.0: el GNS3 server y las consolas escuchan en todas las interfaces;
# Docker publica el mismo bloque de puertos hacia el servidor
cat <<CONF > /root/.config/GNS3/2.2/gns3_server.conf
[Server]
host = 0.0.0.0
port = ${PORT}
auth = True
user = ${GNS3_USER}
password = ${GNS3_PASSWORD}
console_start_port_range = ${CONSOLE_START}
console_end_port_range = ${CONSOLE_END}
ubridge_path = /usr/bin/ubridge
auto_close_time = 0
auto_close_projects = False

[Controller]
auto_close_projects = False
CONF

echo "GNS3 server en el puerto ${PORT} (consolas ${CONSOLE_START}-${CONSOLE_END})"
exec gns3server --config /root/.config/GNS3/2.2/gns3_server.conf
