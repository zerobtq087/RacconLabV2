#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab v2 - instalación y despliegue en Rocky Linux 9
#
#  Uso (como root):   bash raccoon.sh
#
#  Hace todo: instala paquetes, configura SELinux y el firewall, clona o
#  actualiza el repo, genera el .env, construye las imágenes, levanta los
#  servicios y al final se autoprueba. Se puede correr las veces que sea:
#  no borra datos ni cambia las contraseñas que ya se generaron.
#
#  Errores: si falla algo crítico (red, paquetes, construir imágenes,
#  levantar servicios) el script se detiene y dice qué comando falló.
#  Si falla algo no crítico, avisa, sigue y lo anota en "FALTA HACER".
# =====================================================================
set -Eeuo pipefail

REPO_URL="https://github.com/zerobtq087/RacconLabV2.git"
REPO_DIR_DEFAULT="/opt/raccoon-lab"         # dónde se clona si el script viene solo
ESTRUCTURA="/opt/container-environment"     # estructura del servidor (enlace al repo)

# Rutas dentro del repo
PROD_REL="opt/container-environment/production-system/raccoon.lab"
IMAGES_REL="opt/container-environment/container-images"
PROYECTO="raccoon-lab"                      # nombre del proyecto en podman-compose
IMAGEN_C7200="c7200-adventerprisek9-mz.124-24.T5.bin"
IMAGEN_UBUNTU="ubuntu-gns3.tar"

LOG="/var/log/raccoon-lab-install.log"
PODMAN_RED="10.89.0.0/16"                   # redes que crea podman-compose (10.89.X.0/24)

# Archivos que este script instala en el servidor
REPARAR_RED="/usr/local/sbin/raccoon-red"   # repara la red (se puede correr a mano)
NFT_ARCHIVO="/etc/raccoon-lab/solo-lan.nft" # filtro "solo LAN" de los puertos de contenedores
DOCKER_DROPIN="/etc/systemd/system/docker.service.d/raccoon-podman.conf"
UNIDAD_RED="/etc/systemd/system/raccoon-red.service"

# Lo que el usuario todavía tiene que hacer (se muestra en el resumen final)
PENDIENTES=()
# IDs de las imágenes construidas en esta ejecución (para la autoprueba)
ID_GNS3="" ID_API="" ID_WEB=""

# ---------------------------------------------------------------------
#  Mensajes
# ---------------------------------------------------------------------
paso()  { echo -e "\n\033[1;36m==> $*\033[0m"; }
info()  { echo "    $*"; }
bien()  { echo -e "    \033[1;32m✔\033[0m $*"; }
aviso() { echo -e "\033[1;33m[AVISO]\033[0m $*"; }
error() { echo -e "\033[1;31m[ERROR]\033[0m $*" >&2; exit 1; }
pendiente() { PENDIENTES+=("$1"); }

# Si un comando crítico falla, se dice cuál fue antes de salir
trap 'echo -e "\033[1;31m[ERROR]\033[0m Falló (línea $LINENO): $BASH_COMMAND" >&2' ERR

# no_critico FUNCIÓN "qué hacer si falla": corre el paso y, si falla, avisa y sigue.
# (Dentro de esas funciones cada comando revisa su propio error con || ...)
no_critico() {
    if ! "$1"; then
        aviso "El paso '$1' no terminó bien; se continúa con lo demás"
        pendiente "$2"
    fi
}

# Texto aleatorio en hexadecimal (n caracteres) para contraseñas
secreto() {
    local s
    s=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
    echo "${s:0:$1}"
}

# ID de imagen sin el prefijo "sha256:" (Docker lo pone, Podman no)
id_corto() { echo "${1#sha256:}"; }

# ---------------------------------------------------------------------
#  1. Sistema
# ---------------------------------------------------------------------
verificar_sistema() {
    [[ $EUID -eq 0 ]] || error "Corre el script como root: sudo bash raccoon.sh"

    # shellcheck disable=SC1091
    . /etc/os-release
    if [[ "${ID:-}" != "rocky" || "${VERSION_ID%%.*}" != "9" ]]; then
        aviso "El script está hecho para Rocky Linux 9; este sistema es: ${PRETTY_NAME:-desconocido}"
    fi
}

# ---------------------------------------------------------------------
#  2. Código: si el script viene solo, clona el repo (o hace git pull)
#     y se vuelve a ejecutar desde el repo con la versión más nueva
# ---------------------------------------------------------------------
obtener_repo() {
    local aqui
    aqui="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

    if [[ -d "$aqui/.git" && -f "$aqui/$PROD_REL/compose.yml" ]]; then
        REPO_DIR="$aqui"
    else
        REPO_DIR="$REPO_DIR_DEFAULT"
    fi

    # Ya se actualizó en la ejecución anterior (antes del exec)
    [[ -n "${RACCOON_ACTUALIZADO:-}" ]] && return

    paso "Código del proyecto"
    dnf -y install git

    if [[ -d "$REPO_DIR/.git" ]]; then
        info "Actualizando $REPO_DIR (git pull)"
        git -C "$REPO_DIR" pull --ff-only \
            || aviso "No se pudo hacer git pull (¿cambios locales?); se usa el código que ya está"
    else
        info "Clonando $REPO_URL en $REPO_DIR"
        git clone "$REPO_URL" "$REPO_DIR"
    fi

    # Se ejecuta el raccoon.sh del repo, por si cambió con el pull
    export RACCOON_ACTUALIZADO=1
    exec bash "$REPO_DIR/raccoon.sh" "$@"
}

# ---------------------------------------------------------------------
#  3. Paquetes (crítico)
# ---------------------------------------------------------------------
instalar_paquetes() {
    paso "Paquetes (dnf)"

    # podman-compose viene de EPEL (que necesita CRB)
    dnf -y install dnf-plugins-core epel-release
    dnf config-manager --set-enabled crb

    # Docker CE (solo para los laboratorios GNS3)
    if [[ ! -f /etc/yum.repos.d/docker-ce.repo ]]; then
        dnf config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
    fi

    # (curl ya viene en Rocky como curl-minimal; instalar "curl" choca con él)
    dnf -y install git podman podman-compose cockpit firewalld iptables-nft nftables \
        policycoreutils-python-utils
    # containerd.io reemplaza a runc; podman sigue funcionando con crun
    dnf -y install --allowerasing docker-ce docker-ce-cli containerd.io

    systemctl enable --now firewalld
    systemctl enable --now cockpit.socket
    systemctl enable --now docker
    # Al reiniciar el servidor, arranca los contenedores con restart: always
    systemctl enable podman-restart.service
}

# Cockpit no deja entrar a root por defecto, y en el servidor solo existe root
configurar_cockpit() {
    paso "Cockpit"
    if [[ -f /etc/cockpit/disallowed-users ]] && grep -qx root /etc/cockpit/disallowed-users; then
        sed -i '/^root$/d' /etc/cockpit/disallowed-users || return 1
        bien "root ya puede entrar a Cockpit"
    else
        bien "root tiene permiso en Cockpit"
    fi
}

# ---------------------------------------------------------------------
#  4. SELinux en enforcing
#     - Firebird, Redis y Nginx montan sus archivos con :Z (etiqueta propia)
#     - Solo App1/App2 usan label=disable para poder abrir el socket de Docker
# ---------------------------------------------------------------------
configurar_selinux() {
    paso "SELinux"
    sed -i 's/^SELINUX=.*/SELINUX=enforcing/' /etc/selinux/config || return 1

    case "$(getenforce)" in
        Enforcing)
            bien "SELinux en enforcing" ;;
        Permissive)
            setenforce 1 || return 1
            bien "SELinux cambiado de permissive a enforcing" ;;
        *)
            touch /.autorelabel
            aviso "SELinux estaba deshabilitado. Ya quedó en enforcing para el próximo arranque"
            pendiente "Reinicia el servidor (reboot) para activar SELinux y vuelve a correr: bash $REPO_DIR/raccoon.sh" ;;
    esac
}

# ---------------------------------------------------------------------
#  5. Carpetas, red y .env (crítico)
# ---------------------------------------------------------------------
preparar_carpetas() {
    paso "Carpetas"
    PROD_DIR="$REPO_DIR/$PROD_REL"
    GNS3_VOLUMES="$PROD_DIR/gns3/volumes"

    # /opt/container-environment apunta al repo (estructura que pide el proyecto)
    if [[ ! -e "$ESTRUCTURA" ]]; then
        ln -s "$REPO_DIR/opt/container-environment" "$ESTRUCTURA"
    fi
    if [[ "$(readlink -f "$ESTRUCTURA")" == "$(readlink -f "$REPO_DIR/opt/container-environment")" ]]; then
        VISTA="$ESTRUCTURA/production-system/raccoon.lab"
    else
        aviso "$ESTRUCTURA ya existe y no apunta al repo; se usan las rutas del repo"
        VISTA="$PROD_DIR"
    fi

    mkdir -p "$GNS3_VOLUMES/gns3_images/IOS" "$GNS3_VOLUMES/gns3_images/QEMU" \
        "$GNS3_VOLUMES/gns3_images/Docker" "$GNS3_VOLUMES/gns3_projects"
    info "Volúmenes GNS3: $VISTA/gns3/volumes"
}

detectar_red() {
    IFACE=$(ip -4 route show default | awk 'NR==1 {print $5}')
    [[ -n "$IFACE" ]] || error "No hay ruta por defecto: revisa la red del servidor"
    IP=$(ip -4 -o addr show dev "$IFACE" | awk 'NR==1 {split($4, a, "/"); print a[1]}')
    LAN=$(ip -4 route show dev "$IFACE" scope link proto kernel | awk 'NR==1 {print $1}')
    [[ -n "$IP" && -n "$LAN" ]] || error "No se pudo detectar la IP o la red local de $IFACE"
    info "Interfaz $IFACE, IP $IP, red local $LAN"
}

generar_env() {
    paso "Archivo .env"
    ENV_FILE="$PROD_DIR/.env"

    if [[ -f "$ENV_FILE" ]]; then
        info "Ya existe $VISTA/.env: se conservan las contraseñas"
    else
        # Si la base ya existe con otra contraseña, un .env nuevo la dejaría inaccesible
        if podman volume exists "${PROYECTO}_firebird_data" 2>/dev/null; then
            error "Falta el .env pero la base de datos ya existe. Restaura el .env o borra la base con: podman volume rm ${PROYECTO}_firebird_data"
        fi

        (
            umask 077
            cat > "$ENV_FILE" <<EOF
# Raccoon Lab - generado por raccoon.sh el $(date '+%F %T')
# NO se sube a git. Si lo pierdes, la base de datos queda inaccesible.
TZ=America/Mexico_City
HTTP_PORT=80

FIREBIRD_ROOT_PASSWORD=$(secreto 32)
REDIS_PASSWORD=$(secreto 32)

# Admin general (no se puede editar ni borrar desde la página)
ADMIN_USER=admin
ADMIN_PASSWORD=$(secreto 12)

# Laboratorios GNS3 (Docker)
GNS3_VOLUMES=$GNS3_VOLUMES
GNS3_WORKSPACE_IMAGE=gns3-workspace:2.2.59
GNS3_BASE_PORT=30000
LAB_PUERTOS=40
LABS_MAX=60
EOF
        )
        info "Creado $VISTA/.env con contraseñas aleatorias"
    fi

    set -a
    # shellcheck disable=SC1090
    . "$ENV_FILE"
    set +a
    LAB_FIN=$((GNS3_BASE_PORT + LAB_PUERTOS * LABS_MAX - 1))
}

# ---------------------------------------------------------------------
#  6. firewalld: ssh, http y cockpit SOLO desde la red local
#
#  Cada cambio se aplica en runtime (ya) y en --permanent (al reiniciar),
#  y solo si falta. NO se hace "firewall-cmd --reload": un reload puede
#  borrar las reglas de Docker y de Podman (era lo que rompía la página).
# ---------------------------------------------------------------------

# fw_asegurar ZONA TIPO VALOR: agrega (service, rich-rule...) solo si falta
fw_asegurar() {
    local z=$1 t=$2 v=$3 r=0
    firewall-cmd -q --zone="$z" --query-"$t"="$v" 2>/dev/null \
        || firewall-cmd -q --zone="$z" --add-"$t"="$v" || r=1
    firewall-cmd -q --permanent --zone="$z" --query-"$t"="$v" 2>/dev/null \
        || firewall-cmd -q --permanent --zone="$z" --add-"$t"="$v" || r=1
    return $r
}

# fw_quitar ZONA TIPO VALOR: quita solo si está
fw_quitar() {
    local z=$1 t=$2 v=$3 r=0
    if firewall-cmd -q --zone="$z" --query-"$t"="$v" 2>/dev/null; then
        firewall-cmd -q --zone="$z" --remove-"$t"="$v" || r=1
    fi
    if firewall-cmd -q --permanent --zone="$z" --query-"$t"="$v" 2>/dev/null; then
        firewall-cmd -q --permanent --zone="$z" --remove-"$t"="$v" || r=1
    fi
    return $r
}

configurar_firewall() {
    paso "Firewall (solo la red local $LAN)"
    local z r=0 s regla patron
    z=$(firewall-cmd --get-default-zone) || return 1

    # 1) Desde la LAN: ssh, http y cockpit (reglas "rich" con la red de origen)
    for s in ssh http cockpit; do
        fw_asegurar "$z" rich-rule "rule family=\"ipv4\" source address=\"$LAN\" service name=\"$s\" accept" || r=1
    done

    # Si la red local cambió, se quitan las reglas de la red anterior
    patron='source address="([^"]+)" service name="(ssh|http|cockpit)" accept'
    while IFS= read -r regla; do
        [[ $regla =~ $patron ]] || continue
        [[ "${BASH_REMATCH[1]}" == "$LAN" ]] && continue
        fw_quitar "$z" rich-rule "$regla" || r=1
    done < <(firewall-cmd --permanent --zone="$z" --list-rich-rules 2>/dev/null)

    # 2) Desde fuera de la LAN: ya no
    for s in ssh http cockpit; do
        fw_quitar "$z" service "$s" || r=1
    done

    # 3) Limpieza de la versión anterior de este script:
    #    - la zona "raccoon" (se reemplazó por las reglas rich de arriba)
    #    - la regla "direct" de iptables: mientras exista, cada reload de firewalld
    #      vacía TODO iptables (cadenas de Docker y de netavark incluidas)
    if firewall-cmd --permanent --get-zones | grep -qw raccoon; then
        for s in $(firewall-cmd --zone=raccoon --list-sources 2>/dev/null); do
            firewall-cmd -q --zone=raccoon --remove-source="$s" || r=1
        done
        firewall-cmd -q --permanent --delete-zone=raccoon || r=1
        info "Se quitó la zona 'raccoon' de la versión anterior"
    fi
    if [[ -n "$(firewall-cmd --permanent --direct --get-all-rules 2>/dev/null)" ]]; then
        firewall-cmd -q --permanent --direct --remove-rules ipv4 raw PREROUTING || r=1
        firewall-cmd -q --permanent --direct --remove-rules ipv6 raw PREROUTING || r=1
        info "Se quitó la regla 'direct' de la versión anterior (permanente)"
    fi
    if [[ -n "$(firewall-cmd --direct --get-all-rules 2>/dev/null)" ]]; then
        firewall-cmd -q --direct --remove-rules ipv4 raw PREROUTING || r=1
        firewall-cmd -q --direct --remove-rules ipv6 raw PREROUTING || r=1
        info "Se quitó la regla 'direct' de la versión anterior (runtime)"
    fi

    # 4) Si alguien recarga firewalld a mano, netavark (Podman) vuelve a poner sus reglas solo
    if systemctl cat netavark-firewalld-reload.service >/dev/null 2>&1; then
        systemctl enable -q --now netavark-firewalld-reload.service || r=1
    fi

    [[ $r -eq 0 ]] && bien "ssh, http y cockpit permitidos solo desde $LAN (sin recargar firewalld)"
    return $r
}

# ---------------------------------------------------------------------
#  7. Red de los contenedores: comando raccoon-red
#
#  - Filtro "solo LAN" (tabla nftables propia "inet raccoon"): los puertos
#    publicados por contenedores (80 de Podman y ${GNS3_BASE_PORT}-${LAB_FIN} de los labs)
#    se redirigen con DNAT ANTES de llegar a las zonas de firewalld, así
#    que se filtran en prerouting con prioridad "raw" (antes del DNAT).
#    Es una tabla aparte: ni firewalld ni Docker ni netavark la tocan.
#  - DOCKER-USER: Docker pone la política FORWARD en DROP y eso descartaba el
#    tráfico hacia la red de Podman (http://IP no cargaba). Se permite SOLO lo
#    que entra o sale de $PODMAN_RED. Se borran y vuelven a poner (sin duplicados).
#  - podman network reload: vuelve a poner las reglas de netavark (DNAT del 80).
#
#  raccoon-red se ejecuta: aquí, cada vez que arranca Docker (drop-in) y al
#  arrancar el servidor (raccoon-red.service). También se puede correr a mano.
# ---------------------------------------------------------------------
instalar_reparar_red() {
    paso "Red de los contenedores (raccoon-red)"
    local r=0

    mkdir -p "$(dirname "$NFT_ARCHIVO")" "$(dirname "$DOCKER_DROPIN")" || return 1

    cat > "$NFT_ARCHIVO" <<EOF || r=1
# Generado por raccoon.sh: puertos publicados por contenedores solo desde la LAN.
# Prioridad "raw" (-300): se revisa antes del DNAT de Docker y Podman (-100).
table inet raccoon
delete table inet raccoon
table inet raccoon {
    chain solo_lan {
        type filter hook prerouting priority raw; policy accept;
        iifname "$IFACE" ip saddr != $LAN tcp dport { 80, ${GNS3_BASE_PORT}-${LAB_FIN} } drop
        iifname "$IFACE" meta nfproto ipv6 tcp dport { 80, ${GNS3_BASE_PORT}-${LAB_FIN} } drop
    }
}
EOF

    cat > "$REPARAR_RED" <<EOF || r=1
#!/bin/bash
# Generado por raccoon.sh: repara la red de Raccoon Lab. Se puede correr cuando sea.
#   raccoon-red          todo: filtro solo-LAN, reglas DOCKER-USER y redes de Podman
#   raccoon-red docker   solo las reglas DOCKER-USER (lo usa Docker al arrancar)
PODMAN_RED="$PODMAN_RED"
NFT_ARCHIVO="$NFT_ARCHIVO"
ESPERA=\${RACCOON_ESPERA:-60}   # segundos máximos esperando a que Docker cree DOCKER-USER

reglas_docker() {
    local i
    for ((i = 0; i < ESPERA; i++)); do
        iptables -n -L DOCKER-USER >/dev/null 2>&1 && break
        sleep 1
    done
    if ! iptables -n -L DOCKER-USER >/dev/null 2>&1; then
        echo "raccoon-red: no existe la cadena DOCKER-USER (¿Docker apagado?)" >&2
        return 1
    fi
    # Se quitan todas las copias y se pone una sola (así nunca hay duplicados)
    for dir in -s -d; do
        while iptables -D DOCKER-USER "\$dir" "\$PODMAN_RED" -j ACCEPT 2>/dev/null; do :; done
        iptables -I DOCKER-USER "\$dir" "\$PODMAN_RED" -j ACCEPT || return 1
    done
}

case "\${1:-todo}" in
    docker)
        reglas_docker ;;
    *)
        r=0
        nft -f "\$NFT_ARCHIVO" || { echo "raccoon-red: no se cargó el filtro solo-LAN" >&2; r=1; }
        reglas_docker || r=1
        podman network reload --all >/dev/null || { echo "raccoon-red: falló podman network reload" >&2; r=1; }
        exit \$r ;;
esac
EOF
    chmod 755 "$REPARAR_RED" || r=1

    # Cada vez que Docker arranca vuelve a crear DOCKER-USER: se agregan las reglas.
    # El "-" hace que un fallo aquí no impida que Docker arranque.
    cat > "$DOCKER_DROPIN" <<EOF || r=1
# Generado por raccoon.sh: reglas DOCKER-USER para las redes de Podman ($PODMAN_RED)
[Service]
ExecStartPost=-$REPARAR_RED docker
EOF

    # Al arrancar el servidor: filtro solo-LAN + reglas + redes de Podman
    cat > "$UNIDAD_RED" <<EOF || r=1
# Generado por raccoon.sh: red de Raccoon Lab al arrancar el servidor
[Unit]
Description=Raccoon Lab: filtro solo LAN y reglas de red de los contenedores
After=network-online.target firewalld.service docker.service
Wants=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=$REPARAR_RED

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload || r=1
    systemctl enable -q raccoon-red.service || r=1

    # Aplicarlo ya (sin reiniciar Docker ni los laboratorios)
    if "$REPARAR_RED"; then
        bien "Filtro solo-LAN y reglas DOCKER-USER aplicados (comando: raccoon-red)"
    else
        r=1
    fi
    return $r
}

# ---------------------------------------------------------------------
#  8. Imágenes (crítico)
# ---------------------------------------------------------------------
construir_imagenes() {
    local tmp
    tmp=$(mktemp -d)

    paso "Imagen de los laboratorios ($GNS3_WORKSPACE_IMAGE, Docker)"
    docker build --iidfile "$tmp/gns3" -t "$GNS3_WORKSPACE_IMAGE" "$REPO_DIR/$IMAGES_REL/gns3-workspace/build"
    ID_GNS3=$(id_corto "$(cat "$tmp/gns3")")

    paso "Imagen de la API (Podman)"
    podman build --iidfile "$tmp/api" -t localhost/raccoon-api:latest \
        -f "$REPO_DIR/$IMAGES_REL/raccoon-api/build/Dockerfile" "$REPO_DIR"
    ID_API=$(id_corto "$(cat "$tmp/api")")

    paso "Imagen de AppWeb (Podman)"
    podman build --iidfile "$tmp/web" -t localhost/raccoon-web:latest \
        -f "$REPO_DIR/$IMAGES_REL/raccoon-web/build/Dockerfile" "$REPO_DIR"
    ID_WEB=$(id_corto "$(cat "$tmp/web")")

    rm -rf "$tmp"
    # Quita las capas viejas que dejan las reconstrucciones
    podman image prune -f >/dev/null || true
}

# Labs que siguen corriendo con una imagen anterior de gns3-workspace
avisar_labs_viejos() {
    local viejos=() nombre imagen
    while read -r nombre imagen; do
        [[ -z "$nombre" ]] && continue
        [[ "$(id_corto "$imagen")" == "$ID_GNS3" ]] || viejos+=("${nombre#/}")
    done < <(docker ps -q --filter label=raccoon.lab | xargs -r docker inspect -f '{{.Name}} {{.Image}}')

    if [[ ${#viejos[@]} -eq 0 ]]; then
        bien "Ningún laboratorio usa una imagen anterior"
        return 0
    fi
    aviso "Laboratorios con la imagen ANTERIOR (sin las mejoras nuevas): ${viejos[*]}"
    pendiente "Estos laboratorios siguen con la imagen anterior: ${viejos[*]}
       Que su dueño pulse \"Limpiar\" en Mi laboratorio y lo cree de nuevo
       (o ciérralos desde el Monitor del admin)."
}

# ---------------------------------------------------------------------
#  9. Servicios: Firebird, Redis, App1, App2, AppWeb y Nginx (crítico)
# ---------------------------------------------------------------------
levantar_servicios() {
    paso "Servicios (podman-compose)"
    cd "$PROD_DIR"

    # down + up: los contenedores se recrean con las imágenes nuevas.
    # Los datos (volúmenes) y los laboratorios de Docker no se tocan.
    podman-compose -p "$PROYECTO" down >/dev/null 2>&1 || true
    podman-compose -p "$PROYECTO" up -d
}

# ---------------------------------------------------------------------
#  10. Imágenes de routers y hosts (no están en git)
# ---------------------------------------------------------------------
revisar_imagenes() {
    paso "Imágenes de routers y hosts"
    local ios="$VISTA/gns3/volumes/gns3_images/IOS"
    local docker_dir="$VISTA/gns3/volumes/gns3_images/Docker"

    # Router principal (Dynamips)
    if [[ -f "$GNS3_VOLUMES/gns3_images/IOS/$IMAGEN_C7200" ]]; then
        bien "Router c7200: listo"
    else
        if [[ -z "$(ls -A "$GNS3_VOLUMES/gns3_images/IOS")" ]]; then
            aviso "La carpeta de routers IOS está VACÍA: $ios"
        else
            aviso "Falta el router principal $IMAGEN_C7200 en: $ios"
        fi
        pendiente "Copia los routers desde tu computadora (en la carpeta donde los tengas):
       scp $IMAGEN_C7200 c3725-adventerprisek9-mz.124-15.T14.image root@$IP:$ios/"
    fi

    # Host Ubuntu (Docker): si falta, los laboratorios usan VPCS
    if [[ -f "$GNS3_VOLUMES/gns3_images/Docker/$IMAGEN_UBUNTU" ]]; then
        bien "Host Ubuntu ($IMAGEN_UBUNTU): listo"
    else
        if [[ -z "$(ls -A "$GNS3_VOLUMES/gns3_images/Docker")" ]]; then
            aviso "La carpeta de hosts Docker está VACÍA: $docker_dir"
        else
            aviso "Falta $IMAGEN_UBUNTU en: $docker_dir"
        fi
        pendiente "Copia el host Ubuntu (mientras no esté, los laboratorios usan VPCS):
       scp $IMAGEN_UBUNTU root@$IP:$docker_dir/"
    fi
}

# ---------------------------------------------------------------------
#  11. Autoprueba: revisa que todo quedó funcionando
# ---------------------------------------------------------------------
autoprueba() {
    paso "Autoprueba"
    local url="http://$IP:${HTTP_PORT}" codigo="" i n_s n_d usado fecha par cont id img

    # La API tarda en arrancar la primera vez (Firebird crea la base)
    info "Esperando a la API..."
    for ((i = 0; i < 90; i++)); do
        codigo=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$url/api/current-user-acc" || true)
        [[ "$codigo" == "401" ]] && break
        sleep 2
    done

    # 1) La API responde a través de Nginx (401 = responde y pide sesión)
    if [[ "$codigo" == "401" ]]; then
        bien "La API responde a través de Nginx"
    else
        aviso "La API no responde a través de Nginx (HTTP $codigo)"
        pendiente "La API no responde. Revisa los errores con: podman logs raccoon-app1"
    fi

    # 2) La página responde en la IP de la LAN
    codigo=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$url/" || true)
    if [[ "$codigo" == "200" ]]; then
        bien "La página responde en $url"
    else
        aviso "La página no responde en $url (HTTP $codigo)"
        pendiente "La página no responde en $url. Repara la red con: raccoon-red"
    fi

    # 3) La página también responde desde otra red (un contenedor de Docker):
    #    así se prueba el DNAT de Podman como lo usa el tráfico que viene de fuera
    codigo=$(docker run --rm --entrypoint curl "$GNS3_WORKSPACE_IMAGE" \
        -s -o /dev/null -w '%{http_code}' --max-time 5 "$url/" 2>/dev/null || true)
    if [[ "$codigo" == "200" ]]; then
        bien "La página responde desde otra red (DNAT de Podman correcto)"
    else
        aviso "La página no responde desde otra red (HTTP $codigo)"
        pendiente "La página no responde desde la red. Corre: raccoon-red  (si sigue igual: systemctl restart docker && raccoon-red)"
    fi

    # 4) Los contenedores usan las imágenes construidas en esta ejecución
    for par in "raccoon-appweb:$ID_WEB:raccoon-web" "raccoon-app1:$ID_API:raccoon-api" "raccoon-app2:$ID_API:raccoon-api"; do
        IFS=: read -r cont id img <<< "$par"
        usado=$(id_corto "$(podman inspect -f '{{.Image}}' "$cont" 2>/dev/null || true)")
        fecha=$(podman image inspect -f '{{.Created.Format "2006-01-02 15:04"}}' "localhost/$img:latest" 2>/dev/null || echo "?")
        if [[ -n "$id" && "$usado" == "$id" ]]; then
            bien "$cont usa la imagen $img de esta ejecución (creada $fecha)"
        else
            aviso "$cont NO usa la imagen $img recién construida"
            pendiente "$cont usa una imagen vieja. Corre: cd $PROD_DIR && podman-compose -p $PROYECTO up -d --force-recreate"
        fi
    done
    # Con caché, una imagen sin cambios conserva su fecha original: por eso se compara
    # el ID que dejó el build de ESTA ejecución, no la fecha
    usado=$(id_corto "$(docker image inspect -f '{{.Id}}' "$GNS3_WORKSPACE_IMAGE" 2>/dev/null || true)")
    fecha=$(docker image inspect -f '{{.Created}}' "$GNS3_WORKSPACE_IMAGE" 2>/dev/null | cut -c1-16 | tr T ' ')
    if [[ -n "$ID_GNS3" && "$usado" == "$ID_GNS3" ]]; then
        bien "$GNS3_WORKSPACE_IMAGE es la de esta ejecución (creada $fecha); los labs nuevos la usan"
    else
        aviso "$GNS3_WORKSPACE_IMAGE no coincide con la recién construida"
        pendiente "Reconstruye la imagen de los labs: vuelve a correr bash $REPO_DIR/raccoon.sh"
    fi

    # 5) Reglas DOCKER-USER: una sola de cada una
    n_s=$(iptables -S DOCKER-USER 2>/dev/null | grep -c -- "-s $PODMAN_RED -j ACCEPT" || true)
    n_d=$(iptables -S DOCKER-USER 2>/dev/null | grep -c -- "-d $PODMAN_RED -j ACCEPT" || true)
    if [[ "$n_s" == "1" && "$n_d" == "1" ]]; then
        bien "Reglas DOCKER-USER correctas (sin duplicados)"
    else
        aviso "Reglas DOCKER-USER incorrectas (-s: $n_s, -d: $n_d; deben ser 1 y 1)"
        pendiente "Reglas DOCKER-USER incorrectas. Corre: raccoon-red"
    fi

    # 6) Filtro solo-LAN cargado
    if nft list table inet raccoon >/dev/null 2>&1; then
        bien "Filtro solo-LAN activo (80 y ${GNS3_BASE_PORT}-${LAB_FIN} solo desde $LAN)"
    else
        aviso "El filtro solo-LAN no está cargado"
        pendiente "El filtro solo-LAN no está cargado. Corre: raccoon-red"
    fi
}

# ---------------------------------------------------------------------
#  12. Resumen
# ---------------------------------------------------------------------
resumen() {
    local url="http://$IP"
    [[ "$HTTP_PORT" != "80" ]] && url="$url:$HTTP_PORT"
    cat <<EOF

=====================================================================
  Raccoon Lab instalado
=====================================================================
  IP del servidor : $IP
  Página          : $url
  Cockpit         : https://$IP:9090   (usuario root)
  Admin           : $ADMIN_USER
  Contraseña      : $ADMIN_PASSWORD
  Laboratorios    : puertos ${GNS3_BASE_PORT}-${LAB_FIN} (GNS3 2.2.59 de escritorio)

  Configuración   : $VISTA/.env
  Bitácora        : $LOG
  Ver servicios   : podman ps   |   Labs: docker ps
  Si la página deja de cargar (p. ej. tras "firewall-cmd --reload"): raccoon-red
=====================================================================
EOF

    if [[ ${#PENDIENTES[@]} -eq 0 ]]; then
        echo -e "\n  \033[1;32mTodo listo.\033[0m Entra a $url con el usuario admin.\n"
        return
    fi

    echo -e "\n  \033[1;33mFALTA HACER:\033[0m"
    local i=1 p
    for p in "${PENDIENTES[@]}"; do
        echo "   $i. $p"
        i=$((i + 1))
    done
    cat <<EOF

  Las imágenes se usan en los laboratorios que se creen DESPUÉS de copiarlas
  (no hay que reiniciar nada; un lab ya abierto se limpia y se vuelve a crear).
  Opcionales (QEMU, necesitan KVM): copia los .qcow2 a $VISTA/gns3/volumes/gns3_images/QEMU/

EOF
}

main() {
    verificar_sistema
    obtener_repo "$@"

    # A partir de aquí ya corre la versión del repo: todo queda en la bitácora
    exec > >(tee -a "$LOG") 2>&1
    echo "---- $(date '+%F %T') raccoon.sh desde $REPO_DIR ----"

    # Críticos: si fallan, el script se detiene
    instalar_paquetes
    preparar_carpetas
    detectar_red
    generar_env

    # No críticos: si fallan, avisan y se anotan en "FALTA HACER"
    no_critico configurar_cockpit "Habilita root en Cockpit: quita la línea 'root' de /etc/cockpit/disallowed-users"
    no_critico configurar_selinux "Revisa SELinux: getenforce debe decir Enforcing (setenforce 1)"
    no_critico configurar_firewall "Revisa el firewall: firewall-cmd --list-all (y vuelve a correr raccoon.sh)"
    no_critico instalar_reparar_red "Repara la red de los contenedores con: raccoon-red"

    # Críticos
    construir_imagenes
    no_critico avisar_labs_viejos "Revisa qué laboratorios usan la imagen anterior: docker ps"
    levantar_servicios

    # Revisión final
    revisar_imagenes
    no_critico autoprueba "La autoprueba no terminó: revisa la bitácora $LOG"
    resumen
}

# Todo está dentro de funciones: bash lee el archivo completo antes de
# ejecutar, así un git pull que cambie este archivo no rompe la ejecución.
# (Si se carga con "source", solo define las funciones: así se prueban por separado.)
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
