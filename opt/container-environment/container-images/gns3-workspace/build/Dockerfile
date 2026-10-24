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
