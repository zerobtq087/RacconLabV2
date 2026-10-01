#!/bin/bash
set -e

# Start Docker Daemon interno
echo "Iniciando Docker Daemon..."
rm -f /var/run/docker.pid 2>/dev/null || true

mkdir -p /etc/docker
cat <<EOF > /etc/docker/daemon.json
{
  "insecure-registries": ["127.0.0.1:5000", "172.17.0.1:5000"]
}
EOF

dockerd --host=unix:///var/run/docker.sock --storage-driver=vfs --log-level=error &

until docker info &>/dev/null; do
    echo "Esperando Docker..."
    sleep 2
done

# Lectura limpia de variables enviadas desde el handler de Go
PORT_WEB=${GNS3_WEB_PORT:-3080}
CONSOLE_START=${GNS3_CONSOLE_START:-5000}
CONSOLE_END=${GNS3_CONSOLE_END:-5029}

mkdir -p /root/.config/GNS3/2.2

# Escribimos el archivo de configuración respetando el estándar exacto de GNS3 2.2.x
cat <<EOF > /root/.config/GNS3/2.2/gns3_server.conf
[Server]
host = 0.0.0.0
port = ${PORT_WEB}
auth = False
console_start_port_range = ${CONSOLE_START}
console_end_port_range = ${CONSOLE_END}
ubridge_path = /usr/bin/ubridge
auto_close_time = 0
auto_close_projects = False

[ServerSettings]
console_bind_address = 127.0.0.1

[Console]
host = 127.0.0.1

[Controller]
auto_close_projects = False
EOF

echo "Iniciando gns3server en 0.0.0.0:${PORT_WEB} (Consolas asignadas: ${CONSOLE_START}-${CONSOLE_END})..."
exec gns3server --config /root/.config/GNS3/2.2/gns3_server.conf
