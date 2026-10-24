#!/usr/bin/env bash
# =====================================================================
#  Raccoon Lab v2 - instalación y despliegue en Rocky Linux 9
#
#  Uso (como root):   bash raccoon.sh
#
#  Hace todo: instala paquetes, configura SELinux y firewalld, clona o
#  actualiza el repo, genera el .env, construye las imágenes y levanta
#  los servicios. Se puede correr las veces que sea: no borra datos ni
#  cambia las contraseñas que ya se generaron.
# =====================================================================
set -euo pipefail

REPO_URL="https://github.com/zerobtq087/RacconLabV2.git"
REPO_DIR_DEFAULT="/opt/raccoon-lab"         # dónde se clona si el script viene solo
ESTRUCTURA="/opt/container-environment"     # estructura del servidor (enlace al repo)

# Rutas dentro del repo
PROD_REL="opt/container-environment/production-system/raccoon.lab"
IMAGES_REL="opt/container-environment/container-images"
PROYECTO="raccoon-lab"                      # nombre del proyecto en podman-compose
IMAGEN_C7200="c7200-adventerprisek9-mz.124-24.T5.bin"

LOG="/var/log/raccoon-lab-install.log"
IMAGEN_UBUNTU="ubuntu-gns3.tar"
PODMAN_RED="10.89.0.0/16"                   # redes que crea podman-compose (10.89.X.0/24)

# Lo que el usuario todavía tiene que hacer (se muestra en el resumen final)
PENDIENTES=()

# ---------------------------------------------------------------------
#  Mensajes
# ---------------------------------------------------------------------
paso()  { echo -e "\n\033[1;36m==> $*\033[0m"; }
info()  { echo "    $*"; }
aviso() { echo -e "\033[1;33m[AVISO]\033[0m $*"; }
error() { echo -e "\033[1;31m[ERROR]\033[0m $*" >&2; exit 1; }

# Texto aleatorio en hexadecimal (n caracteres) para contraseñas
secreto() {
    local s
    s=$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')
    echo "${s:0:$1}"
}

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
#  3. Paquetes
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
    dnf -y install git podman podman-compose cockpit firewalld iptables-nft \
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
        sed -i '/^root$/d' /etc/cockpit/disallowed-users
        info "root ya puede entrar a Cockpit"
    else
        info "root ya tenía permiso en Cockpit"
    fi
}

# ---------------------------------------------------------------------
#  4. SELinux en enforcing
#     - Firebird, Redis y Nginx montan sus archivos con :Z (etiqueta propia)
#     - Solo App1/App2 usan label=disable para poder abrir el socket de Docker
# ---------------------------------------------------------------------
configurar_selinux() {
    paso "SELinux"
    sed -i 's/^SELINUX=.*/SELINUX=enforcing/' /etc/selinux/config

    case "$(getenforce)" in
        Enforcing)
            info "SELinux ya está en enforcing" ;;
        Permissive)
            setenforce 1
            info "SELinux cambiado de permissive a enforcing" ;;
        *)
            touch /.autorelabel
            aviso "SELinux estaba deshabilitado. Ya quedó en enforcing para el próximo arranque"
            PENDIENTES+=("Reinicia el servidor (reboot) para activar SELinux y vuelve a correr: bash $REPO_DIR/raccoon.sh") ;;
    esac
}

# ---------------------------------------------------------------------
#  5. Carpetas, red y .env
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
#  6. firewalld: 22, 80, 9090 y los puertos de los labs SOLO para la red local
#
#  - Zona "raccoon": todo lo que venga de la red local puede usar ssh,
#    http, cockpit y el rango de los laboratorios. La zona por defecto
#    (public) ya no los permite.
#  - Los puertos publicados por contenedores (80 de Podman y el rango de
#    Docker) no pasan por las zonas (el tráfico se redirige con DNAT antes),
#    así que una regla directa en la tabla raw los bloquea desde fuera de la LAN.
# ---------------------------------------------------------------------
configurar_firewall() {
    paso "Firewall (red local $LAN)"
    local zona_def puertos p
    zona_def=$(firewall-cmd --get-default-zone)

    if ! firewall-cmd --permanent --get-zones | grep -qw raccoon; then
        firewall-cmd --permanent --new-zone=raccoon
    fi

    # La red local puede cambiar: se dejan solo las fuentes y puertos actuales
    for p in $(firewall-cmd --permanent --zone=raccoon --list-sources); do
        firewall-cmd --permanent --zone=raccoon --remove-source="$p" >/dev/null
    done
    for p in $(firewall-cmd --permanent --zone=raccoon --list-ports); do
        firewall-cmd --permanent --zone=raccoon --remove-port="$p" >/dev/null
    done

    firewall-cmd --permanent --zone=raccoon --add-source="$LAN" >/dev/null
    for p in ssh http cockpit; do
        firewall-cmd --permanent --zone=raccoon --add-service="$p" >/dev/null
    done
    firewall-cmd --permanent --zone=raccoon --add-port="${GNS3_BASE_PORT}-${LAB_FIN}/tcp" >/dev/null

    # Fuera de la red local ya no se permite ssh/http/cockpit
    for p in ssh http cockpit; do
        firewall-cmd --permanent --zone="$zona_def" --remove-service="$p" >/dev/null 2>&1 || true
    done

    # Puertos de contenedores: se tiran los que lleguen por $IFACE desde fuera de la LAN
    puertos="80,${GNS3_BASE_PORT}:${LAB_FIN}"
    firewall-cmd --permanent --direct --remove-rules ipv4 raw PREROUTING || true
    firewall-cmd --permanent --direct --remove-rules ipv6 raw PREROUTING || true
    firewall-cmd --permanent --direct --add-rule ipv4 raw PREROUTING 0 \
        -i "$IFACE" -p tcp -m multiport --dports "$puertos" ! -s "$LAN" -j DROP >/dev/null
    firewall-cmd --permanent --direct --add-rule ipv6 raw PREROUTING 0 \
        -i "$IFACE" -p tcp -m multiport --dports "$puertos" -j DROP >/dev/null

    firewall-cmd --reload >/dev/null
    info "Permitido desde $LAN: 22, 80, 9090 y ${GNS3_BASE_PORT}-${LAB_FIN}/tcp"
}

# ---------------------------------------------------------------------
#  Docker pone la política FORWARD en DROP y eso descarta el tráfico hacia
#  la red de Podman (Nginx, App1, App2...): http://IP no cargaba.
#  Se permite SOLO lo que entra o sale de las redes de Podman ($PODMAN_RED).
#  Los laboratorios están en la red de Docker, así que siguen limitados a la LAN.
#
#  La cadena DOCKER-USER la crea Docker al arrancar, por eso las reglas van en
#  un "drop-in" de systemd que se ejecuta cada vez que Docker arranca
#  (también al reiniciar la VM). Con -C se revisa antes de agregar: no se duplican.
# ---------------------------------------------------------------------
permitir_podman_en_docker() {
    paso "Tráfico entre Docker y Podman"
    local regla_s="iptables -C DOCKER-USER -s $PODMAN_RED -j ACCEPT 2>/dev/null || iptables -I DOCKER-USER -s $PODMAN_RED -j ACCEPT"
    local regla_d="iptables -C DOCKER-USER -d $PODMAN_RED -j ACCEPT 2>/dev/null || iptables -I DOCKER-USER -d $PODMAN_RED -j ACCEPT"

    mkdir -p /etc/systemd/system/docker.service.d
    cat > /etc/systemd/system/docker.service.d/raccoon-podman.conf <<EOF
# Generado por raccoon.sh: deja pasar el tráfico de las redes de Podman ($PODMAN_RED)
[Service]
ExecStartPost=/bin/sh -c '$regla_s'
ExecStartPost=/bin/sh -c '$regla_d'
EOF
    systemctl daemon-reload

    # Para la sesión actual (sin reiniciar Docker ni los laboratorios)
    sh -c "$regla_s"
    sh -c "$regla_d"
    info "Permitido el tráfico de $PODMAN_RED (se aplica cada vez que arranca Docker)"
}

# ---------------------------------------------------------------------
#  7. Imágenes
# ---------------------------------------------------------------------
construir_imagenes() {
    paso "Imagen de los laboratorios ($GNS3_WORKSPACE_IMAGE, Docker)"
    docker build -t "$GNS3_WORKSPACE_IMAGE" "$REPO_DIR/$IMAGES_REL/gns3-workspace/build"

    paso "Imagen de la API (Podman)"
    podman build -t localhost/raccoon-api:latest \
        -f "$REPO_DIR/$IMAGES_REL/raccoon-api/build/Dockerfile" "$REPO_DIR"

    paso "Imagen de AppWeb (Podman)"
    podman build -t localhost/raccoon-web:latest \
        -f "$REPO_DIR/$IMAGES_REL/raccoon-web/build/Dockerfile" "$REPO_DIR"

    # Quita las capas viejas que dejan las reconstrucciones
    podman image prune -f >/dev/null || true
}

# ---------------------------------------------------------------------
#  8. Servicios: Firebird, Redis, App1, App2, AppWeb y Nginx
# ---------------------------------------------------------------------
levantar_servicios() {
    paso "Servicios (podman-compose)"
    cd "$PROD_DIR"

    # down + up: los contenedores se recrean con las imágenes nuevas.
    # Los datos (volúmenes) y los laboratorios de Docker no se tocan.
    podman-compose -p "$PROYECTO" down >/dev/null 2>&1 || true
    podman-compose -p "$PROYECTO" up -d

    info "Esperando a la API (la primera vez Firebird crea la base)..."
    local codigo=""
    for _ in $(seq 90); do
        codigo=$(curl -s -o /dev/null -w '%{http_code}' "http://$IP:${HTTP_PORT}/api/current-user-acc" || true)
        [[ "$codigo" == "401" ]] && break
        sleep 2
    done
    if [[ "$codigo" == "401" ]]; then
        info "API respondiendo a través de Nginx"
    else
        aviso "La API todavía no responde (HTTP $codigo)"
        PENDIENTES+=("La página no respondió a tiempo. Revisa los errores con: podman logs raccoon-app1")
    fi
}

# ---------------------------------------------------------------------
#  9. Imágenes de routers (no están en git)
# ---------------------------------------------------------------------
revisar_imagenes() {
    paso "Imágenes de routers y hosts"
    local ios="$VISTA/gns3/volumes/gns3_images/IOS"
    local docker_dir="$VISTA/gns3/volumes/gns3_images/Docker"

    # Router principal (Dynamips)
    if [[ -f "$GNS3_VOLUMES/gns3_images/IOS/$IMAGEN_C7200" ]]; then
        info "Router c7200: listo"
    else
        if [[ -z "$(ls -A "$GNS3_VOLUMES/gns3_images/IOS")" ]]; then
            aviso "La carpeta de routers IOS está VACÍA: $ios"
        else
            aviso "Falta el router principal $IMAGEN_C7200 en: $ios"
        fi
        PENDIENTES+=("Copia los routers desde tu computadora (en la carpeta donde los tengas):
       scp $IMAGEN_C7200 c3725-adventerprisek9-mz.124-15.T14.image root@$IP:$ios/")
    fi

    # Host Ubuntu (Docker): si falta, los laboratorios usan VPCS
    if [[ -f "$GNS3_VOLUMES/gns3_images/Docker/$IMAGEN_UBUNTU" ]]; then
        info "Host Ubuntu ($IMAGEN_UBUNTU): listo"
    else
        if [[ -z "$(ls -A "$GNS3_VOLUMES/gns3_images/Docker")" ]]; then
            aviso "La carpeta de hosts Docker está VACÍA: $docker_dir"
        else
            aviso "Falta $IMAGEN_UBUNTU en: $docker_dir"
        fi
        PENDIENTES+=("Copia el host Ubuntu (mientras no esté, los laboratorios usan VPCS):
       scp $IMAGEN_UBUNTU root@$IP:$docker_dir/")
    fi
}

# ---------------------------------------------------------------------
#  10. Resumen
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
  Cockpit         : https://$IP:9090
  Admin           : $ADMIN_USER
  Contraseña      : $ADMIN_PASSWORD
  Laboratorios    : puertos ${GNS3_BASE_PORT}-${LAB_FIN} (GNS3 2.2.59 de escritorio)

  Configuración   : $VISTA/.env
  Bitácora        : $LOG
  Ver servicios   : podman ps   |   Labs: docker ps
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

    instalar_paquetes
    configurar_cockpit
    configurar_selinux
    preparar_carpetas
    detectar_red
    generar_env
    configurar_firewall
    permitir_podman_en_docker
    construir_imagenes
    levantar_servicios
    revisar_imagenes
    resumen
}

# Todo está dentro de funciones: bash lee el archivo completo antes de
# ejecutar, así un git pull que cambie este archivo no rompe la ejecución
main "$@"
