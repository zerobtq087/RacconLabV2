#!/bin/bash
# Raccoon Lab - arranque del laboratorio GNS3
# Las variables las manda la API de Go al crear el contenedor
set -e

# Docker interno (para nodos Docker dentro de GNS3).
# overlay2 comparte las capas de la imagen entre nodos, pero necesita que
# /var/lib/docker sea un volumen (la API de Go lo crea). Si no se puede, vfs.
iniciar_dockerd() {
    rm -f /var/run/docker.pid
    dockerd --host=unix:///var/run/docker.sock --storage-driver="$1" --log-level=error &
    DOCKERD_PID=$!
    for _ in $(seq 30); do
        docker info &>/dev/null && return 0
        kill -0 "$DOCKERD_PID" 2>/dev/null || return 1   # dockerd terminó con error
        sleep 1
    done
    return 1
}

echo "Iniciando Docker interno (overlay2)..."
if ! iniciar_dockerd overlay2; then
    echo "AVISO: overlay2 no funcionó aquí; se usa vfs (ocupa más disco)"
    kill "$DOCKERD_PID" 2>/dev/null || true
    wait "$DOCKERD_PID" 2>/dev/null || true
    iniciar_dockerd vfs || { echo "ERROR: no arrancó el Docker interno"; exit 1; }
fi
echo "Docker interno listo con $(docker info --format '{{.Driver}}')"

# Host Ubuntu para las prácticas: se carga desde gns3_images/Docker/ubuntu-gns3.tar
# (no está en git) y se le pone un nombre fijo, sin importar el tag que traiga el .tar.
# La plantilla de GNS3 y la API de Go usan ese nombre fijo.
UBUNTU_TAR=/root/GNS3/images/Docker/ubuntu-gns3.tar
UBUNTU_IMAGE=${UBUNTU_IMAGE:-raccoon/ubuntu-host:latest}
if [ -f "$UBUNTU_TAR" ] && ! docker image inspect "$UBUNTU_IMAGE" &>/dev/null; then
    echo "Cargando la imagen Ubuntu desde $UBUNTU_TAR..."
    if salida=$(docker load -i "$UBUNTU_TAR"); then
        # docker load responde "Loaded image: nombre:tag" o "Loaded image ID: sha256:..."
        # (una línea por imagen). Si el .tar trae varias, se prefiere la que se llame
        # ubuntu-gns3; si no, la primera. Se avisa para que no haya sorpresas.
        imagenes=$(echo "$salida" | sed -n 's/^Loaded image\( ID\)\?: //p')
        cargada=$(echo "$imagenes" | grep -m1 'ubuntu-gns3' || echo "$imagenes" | head -n 1)
        if [ "$(echo "$imagenes" | wc -l)" -gt 1 ]; then
            echo "AVISO: el .tar trae varias imágenes: $(echo "$imagenes" | tr '\n' ' ')"
        fi
        # docker tag solo agrega un nombre: CMD y ENTRYPOINT quedan como en la imagen
        docker tag "$cargada" "$UBUNTU_IMAGE" && echo "Imagen Ubuntu lista: $UBUNTU_IMAGE (era $cargada)"
        echo "  arranca con: $(docker image inspect -f 'CMD={{json .Config.Cmd}} ENTRYPOINT={{json .Config.Entrypoint}}' "$UBUNTU_IMAGE")"
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
