#Raccoon LAB DockerfileWS
FROM gns3/gns3-server:latest

# Downgrade to 2.2.59 e instalar Ansible mediante pip
RUN pip uninstall -y gns3-server --break-system-packages && \
    pip install gns3-server==2.2.59 ansible netaddr netmiko --break-system-packages

RUN chmod +x /usr/bin/ubridge && chmod +s /usr/bin/ubridge

USER root

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

# Turn websocket always on 
# Make root directory and some configuration flags
RUN mkdir -p /root/.config/GNS3/2.2/ && \
    printf "[Server]\n\
auto_close_projects = False\n\
ubridge_path = /usr/bin/ubridge\n\
# Expose ports\n\
first_port = 5000\n\
last_port = 5199\n\
\n\
[Controller]\n\
auto_close_projects = False\n" > /root/.config/GNS3/2.2/gns3_server.conf && \
    printf "[Controller]\nauto_close_projects = False\n" > /root/.config/GNS3/2.2/gns3

# Entrypoint
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Enviroment variables
ENV DOCKER_HOST=unix:///var/run/docker.sock

EXPOSE 3080

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

# Avoid garbage arguments
CMD []
