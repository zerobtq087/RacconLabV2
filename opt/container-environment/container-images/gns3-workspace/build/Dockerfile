#Raccoon LAB DockerfileWS
# Base fija: Docker Hub no publica la etiqueta 2.2.59, así que se parte de 2.2.61
# (ya trae dynamips, vpcs, ubridge y qemu) y se baja el servidor a 2.2.59 con pip,
# la misma versión que el GNS3 de escritorio de los alumnos
FROM gns3/gns3-server:2.2.61

USER root

# GNS3 2.2.59 + Ansible (versiones fijas para que la imagen sea replicable)
RUN pip uninstall -y gns3-server --break-system-packages && \
    pip install --no-cache-dir --break-system-packages \
        gns3-server==2.2.59 \
        ansible==14.5.0 \
        netaddr==1.3.0 \
        netmiko==4.8.0

# Arreglo de GNS3 2.2.59: al "Recargar" (Reload) un nodo Docker solo hace
# "docker restart": el contenedor arranca en una red nueva sin eth0, el init de
# GNS3 se queda esperando la interfaz y la consola no responde (pasa al aplicar
# Edit config). Se cambia por detener + arrancar, que sí conecta la red y la
# consola. Si el código de GNS3 cambia, la construcción falla aquí.
RUN F=$(cd / && python3 -c 'import gns3server.compute.docker.docker_vm as m; print(m.__file__)') && \
    grep -q 'await self.manager.query("POST", "containers/{}/restart".format(self._cid))' "$F" && \
    sed -i 's|^\(\s*\)await self.manager.query("POST", "containers/{}/restart".format(self._cid))$|\1await self.stop()\n\1await self.start()|' "$F" && \
    ! grep -q 'containers/{}/restart' "$F" && \
    grep -A8 'async def restart' "$F" | grep -q 'await self.stop()'

RUN chmod +x /usr/bin/ubridge && chmod +s /usr/bin/ubridge

# Install dependences
RUN apt-get update --allow-releaseinfo-change && \
    apt-get install -y --no-install-recommends \
        docker.io \
        busybox-static \
        iproute2 \
        iputils-ping \
        libcap2-bin \
        curl \
        openssh-client \
        && \
    apt-get clean \
    && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* \
    && ln -sf /usr/bin/docker /usr/local/bin/docker

# Si falta alguna herramienta de GNS3 la construcción falla aquí
RUN which dynamips vpcs ubridge qemu-system-x86_64 && \
    test "$(gns3server --version)" = "2.2.59"

# La configuración de GNS3 (puerto, consolas, usuario y token) la escribe entrypoint.sh al arrancar
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Enviroment variables
ENV DOCKER_HOST=unix:///var/run/docker.sock

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# Avoid garbage arguments
CMD []
