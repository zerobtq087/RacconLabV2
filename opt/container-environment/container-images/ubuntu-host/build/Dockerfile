FROM ubuntu:latest

# Evita preguntas interactivas durante la instalación de paquetes
ENV DEBIAN_FRONTEND=noninteractive

# Actualización e instalación de todas las herramientas requeridas
RUN apt-get update && apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        net-tools \
        openssh-client \
        openssh-server \
        fish \
        nano \
        nmap \
        curl \
        inetutils-ping \
        traceroute \
        isc-dhcp-server \
        apache2 \
        bind9 \
        vsftpd \
        fastfetch \
        speedtest-cli && \
    apt-get autoremove -y && \
    apt-get autoclean && \
    rm -rf /var/lib/apt/lists/*

# Configuración de soporte SSH extendido en /etc/ssh/ssh_config
RUN echo "\nHost *" >> /etc/ssh/ssh_config && \
    echo "    Ciphers chacha20-poly1305@openssh.com,aes128-ctr,aes192-ctr,aes256-ctr,aes128-gcm@openssh.com,aes256-gcm@openssh.com,3des-cbc,aes128-cbc,aes192-cbc,aes256-cbc" >> /etc/ssh/ssh_config && \
    echo "    KexAlgorithms sntrup761x25519-sha512@openssh.com,curve25519-sha256,curve25519-sha256@libssh.org,ecdh-sha2-nistp256,ecdh-sha2-nistp384,ecdh-sha2-nistp521,diffie-hellman-group-exchange-sha256,diffie-hellman-group16-sha512,diffie-hellman-group18-sha512,diffie-hellman-group14-sha256,diffie-hellman-group-exchange-sha1,diffie-hellman-group1-sha1,diffie-hellman-group14-sha1" >> /etc/ssh/ssh_config && \
    echo "    HostKeyAlgorithms ssh-ed25519-cert-v01@openssh.com,ecdsa-sha2-nistp256-cert-v01@openssh.com,ecdsa-sha2-nistp384-cert-v01@openssh.com,ecdsa-sha2-nistp521-cert-v01@openssh.com,sk-ssh-ed25519-cert-v01@openssh.com,sk-ecdsa-sha2-nistp256-cert-v01@openssh.com,rsa-sha2-512-cert-v01@openssh.com,rsa-sha2-256-cert-v01@openssh.com,ssh-ed25519,ecdsa-sha2-nistp256,ecdsa-sha2-nistp384,ecdsa-sha2-nistp521,sk-ssh-ed25519@openssh.com,sk-ecdsa-sha2-nistp256@openssh.com,rsa-sha2-512,rsa-sha2-256,ssh-dss,ssh-dss-cert-v01@openssh.com,ssh-rsa,ssh-rsa-cert-v01@openssh.com,webauthn-sk-ecdsa-sha2-nistp256@openssh.com" >> /etc/ssh/ssh_config && \
    echo "    MACs umac-64-etm@openssh.com,umac-128-etm@openssh.com,hmac-sha2-256-etm@openssh.com,hmac-sha2-512-etm@openssh.com,hmac-sha1-etm@openssh.com,umac-64@openssh.com,umac-128@openssh.com,hmac-sha2-256,hmac-sha2-512,hmac-sha1,hmac-md5,hmac-md5-96,hmac-md5-96-etm@openssh.com,hmac-md5-etm@openssh.com,hmac-sha1-96,hmac-sha1-96-etm@openssh.com" >> /etc/ssh/ssh_config && \
    echo "    PubkeyAcceptedAlgorithms ssh-ed25519-cert-v01@openssh.com,ecdsa-sha2-nistp256-cert-v01@openssh.com,ecdsa-sha2-nistp384-cert-v01@openssh.com,ecdsa-sha2-nistp521-cert-v01@openssh.com,sk-ssh-ed25519-cert-v01@openssh.com,sk-ecdsa-sha2-nistp256-cert-v01@openssh.com,rsa-sha2-512-cert-v01@openssh.com,rsa-sha2-256-cert-v01@openssh.com,ssh-ed25519,ecdsa-sha2-nistp256,ecdsa-sha2-nistp384,ecdsa-sha2-nistp521,sk-ssh-ed25519@openssh.com,sk-ecdsa-sha2-nistp256@openssh.com,rsa-sha2-512,rsa-sha2-256,ssh-dss,ssh-dss-cert-v01@openssh.com,ssh-rsa,ssh-rsa-cert-v01@openssh.com,webauthn-sk-ecdsa-sha2-nistp256@openssh.com" >> /etc/ssh/ssh_config

# Precargar logo ascii en el directorio de fastfetch
RUN mkdir -p /root/.config/fastfetch
COPY <<-"EOF" /root/.config/fastfetch/my_logo.txt
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣀⣀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣀⣴⣶⣶⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣾⠟⠉⠉⠙⠻⣶⣄⠀⠀⠀⠀⠀⠀⠀⠀⢀⡀⠀⠀⠀⠀⠀⢀⣴⠟⠋⠀⠀⠀⠙⣷⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣾⠃⠀⣠⡤⢄⡀⠀⠙⢷⣄⢀⣀⣀⣀⣾⢦⡿⣡⡶⣂⣀⣀⣴⠟⠁⠀⣠⠖⢋⠶⡀⢹⡇⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣇⠀⢰⠇⣽⣀⡻⣦⠀⠈⠛⠛⠋⠉⠉⠀⠀⠀⠀⠀⠈⠉⠉⠁⠀⠀⢼⣭⣞⣇⣴⡇⢸⡇⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⡇⠀⢸⡃⢠⢷⡱⠟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠙⠻⢽⣴⡇⢸⡇⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣇⠀⠘⢶⠟⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠀⣾⠃⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣌⡿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠠⡆⠀⠈⠓⣄⠀⠀⠀⠀⡴⠁⠀⠀⠖⠒⠒⠢⢤⡀⠀⠙⣿⡛⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢨⣿⠇⠀⠀⠀⢀⡠⠎⠀⠀⠀⢀⣀⣀⣀⡀⠀⠘⣶⠾⠿⣶⡇⠀⣀⣤⣤⣄⣀⡀⠀⠙⠲⡄⢹⣟⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣰⡟⠀⠀⠀⡤⠋⠀⠀⣠⡴⠚⠏⠉⠉⠙⠉⠢⣤⡇⠀⠀⠈⢧⡾⠋⠁⠀⡀⠀⠋⢷⣄⠀⠘⢦⢻⣆⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⡿⠁⢀⡤⠊⠀⢀⣴⠾⠋⠀⠀⢀⣤⣤⣄⠀⠀⠉⠀⠀⠀⠀⠀⠀⠀⣠⣶⣶⡄⠀⠀⠛⢿⣄⠀⠑⢽⣧⣀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣶⠟⠔⠒⠉⠀⠀⣰⠟⠁⠀⠀⠀⠀⣾⣿⣿⣿⡆⣀⣤⣤⡄⠀⡀⣤⣤⣶⣿⣿⣿⣿⠀⠀⠀⠀⠻⣧⠀⠀⢨⣍⠁⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣾⠃⠀⠀⠀⠀⣰⠃⠀⠀⠀⠀⠀⠀⠙⠿⠟⣁⠞⠉⠀⠀⣿⣿⣿⣿⣆⡀⠙⣿⠋⠁⠀⠀⠀⠀⠀⢻⡇⠀⠀⣽⠃⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢰⡄⠀⠀⠀⠀⢻⣇⠀⠀⠀⠀⠀⠀⠀⢀⡼⠋⠀⠀⠀⠀⠉⠛⣿⣟⣿⡷⣀⠈⠻⢄⡀⠀⠀⢀⡰⠏⠀⠀⢠⡿⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠘⣷⡀⠀⠀⠀⠀⠉⠳⠤⢤⣤⡤⠤⠚⠁⠀⠀⣸⣷⣦⣤⣴⢾⡿⢷⠶⠾⠛⠷⣦⡀⠀⠉⠉⠁⠀⠀⠀⣠⡿⠁⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠻⣦⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣼⠟⠁⣠⣄⡴⡿⢑⣷⡍⣉⣣⣤⡌⠻⣧⡀⠀⠀⢀⣠⡾⠟⠁⠀⠀⠀⠀⠀
⠀⠀⣤⣴⣾⠿⠒⠳⢶⣤⡀⠀⠀⢨⣿⠛⠦⠤⠀⠀⠀⠀⠀⠀⠀⠀⣾⠳⠒⠚⢿⡿⠁⢳⣺⡀⢣⠀⠈⠛⠧⣄⣸⣧⣀⠰⠛⣿⠗⠀⠀⠀⠀⠀⠀⠀
⠀⣴⡟⠉⠀⠀⠀⢀⠀⠉⢿⣄⠀⠐⣿⠂⠀⠀⠀⠀⠀⣀⣀⣀⣤⡾⠟⣛⣻⣿⣌⠀⠀⡞⢿⡇⢸⣠⣄⠀⢠⡿⠿⡯⠻⢷⣄⣙⢷⡄⠀⠀⠀⠀⠀⠀
⢸⣏⠃⠀⠀⠀⣠⠖⠶⠞⠋⣿⣀⣾⠏⠀⠀⠀⠀⠀⠈⠉⠉⢻⡋⠀⠀⠈⢩⣴⣿⣄⣴⣧⠀⡇⠀⠟⢟⠀⣿⡿⣶⠀⠀⠀⡿⠉⠈⢻⡆⠀⠀⠀⠀⠀
⣿⡯⠴⠋⠉⠉⠁⠀⠀⢀⣀⢸⡿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠸⡆⠀⠀⠀⠀⣠⡿⠋⡟⠋⠀⢷⠀⢀⣈⠓⡟⣷⡔⠀⠀⣠⠟⠀⠀⢸⣿⠀⠀⠀⠀⠀
⣿⡇⠀⠀⡀⣠⠖⠒⠖⠋⢩⣿⠃⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⣆⡀⠀⣼⡟⠀⢸⠀⠀⣦⣼⠀⠿⠿⠀⢹⣾⣷⡶⠿⠋⠀⠀⠀⣸⡟⠀⠀⠀⠀⠀
⣿⣇⣴⠋⠉⠉⠀⠀⠄⠀⣼⠇⠀⠀⠀⠀⢠⣄⠀⠀⠀⠀⠀⠀⠀⢀⣨⣿⠋⠛⢷⣮⣤⣀⣀⣸⣆⣠⣤⡿⠛⠀⠘⠻⣦⣤⣀⣤⣴⠿⣷⣄⠀⠀⠀⠀
⣿⣷⠀⠀⠀⢀⣀⡰⠛⠙⣿⠀⠀⠀⠀⠀⠈⠙⠷⠶⢶⣤⣶⣶⠾⠟⠉⠀⠀⠀⠀⠀⠉⠉⠉⠙⠉⠉⠁⠀⠀⠀⠀⠀⠀⠹⠉⠉⠀⠀⠀⠻⣧⠀⠀⠀
⢸⣇⠀⣀⣴⠉⠁⠀⠀⢸⣟⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠙⢷⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠓⠀⠠⡇⠀⠀⠀⠀⠀⢹⡇⠀⠀
⠈⢿⡏⠀⠀⠀⠀⣀⣾⡙⣯⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠹⣧⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⠁⠀⠀⠀⠀⠀⢸⡇⠀⠀
⠀⠈⢻⣆⠀⠀⡞⠃⠀⠀⣿⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⠎⠀⠀⢀⣠⣤⣤⣿⣁⠀⠀
⠀⠀⠀⠙⢷⣴⣇⠀⠀⢀⡟⣷⡀⠀⠀⠀⠀⠀⠀⠀⠀⢀⣤⡶⠶⠛⠿⢿⣄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡴⠋⢀⣴⠿⠋⠁⢴⣗⢻⣿⣷⡀
⠀⠀⠀⠀⠀⠉⠻⠶⣶⣿⣀⣈⣿⣦⡀⠀⠀⠀⠀⢀⡴⠋⠁⠀⢰⣤⡘⣦⣹⡆⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣠⣶⣯⣄⣠⣿ me ⠀⠀⠀⠀⢙⣷⣿⡿⠁
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠉⠉⠉⠛⠷⣦⣤⣄⣸⣃⠀⠀⠀⠀⣸⣧⣼⣿⠟⠓⠶⠶⠶⠾⠛⠛⠛⠛⠉⠉⠀⠀⠉⠙⠛⠛⠛⠛⠛⠛⠛⠛⠉⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠉⠉⠙⠛⠛⠛⠉⠉⠉⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
       
            ╦═╗┌─┐┌─┐┌─┐┌─┐┌─┐┌┐┌  ╦  ╔═╗╔╗ 
            ╠╦╝├─┤│  │  │ ││ ││││  ║  ╠═╣╠╩╗
            ╩╚═┴ ┴└─┘└─┘└─┘└─┘┘└┘  ╩═╝╩ ╩╚═╝

==========================================================================
                 Universidad Tecnologica de Tula-Tepeji
  Cuerpo Colegiado Ecosistema de Aplicación Tecnológica para la Sociedad
            Emulador de Redes para Media Superior y Superior
==========================================================================
EOF

# Configuración básica de fastfetch
RUN echo '{\n\
  "$schema": "https://github.com/fastfetch-cli/fastfetch/raw/dev/doc/json_schema.json",\n\
  "logo": {\n\
    "type": "file",\n\
    "source": "/root/.config/fastfetch/my_logo.txt"\n\
  },\n\
  "modules": [\n\
    {\n\
      "type": "os",\n\
      "key": "Sistema Operativo"\n\
    },\n\
    {\n\
      "type": "kernel",\n\
      "key": "Kernel"\n\
    },\n\
    {\n\
      "type": "uptime",\n\
      "key": "Tiempo Encendido"\n\
    },\n\
    {\n\
      "type": "localip",\n\
      "key": "Dirección IP Local"\n\
    },\n\
    {\n\
      "type": "memory",\n\
      "key": "Uso de Memoria RAM"\n\
    }\n\
  ]\n\
}' > /root/.config/fastfetch/config.jsonc

# Ejecutar fastfetch con fish
RUN mkdir -p /root/.config/fish && \
    echo 'fastfetch' >> /root/.config/fish/config.fish

# Establecer Shell por defecto
SHELL ["/usr/bin/fish", "-c"]

CMD ["fish"]