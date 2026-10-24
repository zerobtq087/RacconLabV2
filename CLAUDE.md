# Raccoon Lab v2 — contexto para Claude Code

Plataforma escolar multiusuario de laboratorios de redes con GNS3 (UTTT).
Repo: https://github.com/zerobtq087/RacconLabV2 — debe ser replicable:
`git clone` + `./raccoon.sh` instala, construye y levanta todo.

## Cómo trabajar conmigo
- Respóndeme en español, conciso y paso a paso.
- PREGÚNTAME antes de cambiar archivos o configuraciones que ya existen.
- Explícame cada parte nueva (lo tengo que defender ante mi profe).
- Archivos completos, no fragmentos. Compila/prueba antes de decir que algo quedó.
- No inventes estructura: sigue el estilo del proyecto del profe (fiber3) descrito abajo.
- Commit y `git push` solo cuando yo lo pida.
- Nada de secretos ni IPs fijas en el repo (todo va en `.env`, que está en .gitignore).

## Arquitectura (la pide el profe)
- Servidor: Rocky Linux 9 (9.8), SELinux enforcing, firewalld, Cockpit (9090). Sin KVM (laptop).
- Casi todo en Podman + podman-compose. Docker SOLO para los laboratorios GNS3.
- Nginx (puerto 80) balancea a 2 réplicas de la API (App1, App2) y sirve el frontend (AppWeb aparte).
- Firebird 5 = todos los datos. Redis = SOLO sesiones de usuario.
- Estructura de servidor: /opt/container-environment/{production-system,container-images}/...

## Backend: Go + Fiber v3 + GORM (gorm-firebird), módulo `raccoon-lab`
Carpetas como fiber3: backend/{cmd,config,database,domain,repository,handler,middleware,router}
+ backend/service (lógica pesada: importador CSV/XLSX, Docker, laboratorios).
- Apuntadores, SIN variables globales: main.go crea todo y lo pasa por puntero
  (config.LoadConfig() *Env, database.ConnectDB(dns) *gorm.DB, NewXRepository(db), NewXHandler(...)).
- SQL escrito a mano con db.Raw(...).Scan / db.Exec, nombres en MAYÚSCULAS,
  IDs UUID con CHAR_TO_UUID / UUID_TO_CHAR (como el profe).
- El middleware (`middleware.Session`) es lo ÚNICO que toca la sesión de Redis
  (Validate, RequireRol, Create, Destroy, CloseAll). Los handlers leen c.Locals("userID"/"rol"...).
- Sesión: cookie `rl_sesion`, store de Fiber con storage/redis creado en cmd/main.go.
- Respuestas como fiber3: "success" o texto de error con su status; JSON para datos.
- Cualquier estado compartido va en Firebird (no en memoria) porque hay 2 réplicas:
  bloqueo de login (INTENTOS_LOGIN, 5 intentos → 15 min, HTTP 429 + Retry-After),
  avance de importación (IMPORTACIONES), laboratorios (LABORATORIOS).

## Base de datos (docs/esquema.sql)
- Normalizada (3FN), catálogos en vez de CHECK. Sin CREATE INDEX (solo PK/UNIQUE/FK).
- La aplica el contenedor de Firebird al crear la base (/docker-entrypoint-initdb.d/01-esquema.sql).
- Roles: ADMIN puede entrar como admin/maestro/alumno; PROFESOR como maestro/alumno; ALUMNO solo alumno.
- Nadie se registra: el admin importa CSV/Excel (alumnos: matricula,nombre,contrasenia,grupo,carrera;
  maestros/admins: clave,nombre,contrasenia). Roles por tipo en TIPO_ROLES.
- Admin general = ADMIN_USER del .env ("admin"), protegido: no se edita ni se borra.

## Frontend: Vue 3 + Vuetify 4 + TypeScript (yarn create vuetify), Yarn 4.14.1
- SIEMPRE `<script lang="ts" setup>` en todos los .vue.
- Estructura como fiber3: src/{components/AppFrame.vue, models/*.ts, pages/*.vue, plugins, router/index.ts, stores/auth.ts}.
- axios directo en cada página, SweetAlert2 para alertas, Pinia solo en stores/auth.ts.
- Rutas a mano en router/index.ts con meta.requiresAuth y meta.roles; menú por rol en AppFrame.vue.
- Dev: vite.config.mts manda /api a http://localhost:8080. Build: frontend/dist.

## Laboratorios GNS3 (así es el proyecto original)
- Los alumnos usan GNS3 DE ESCRITORIO 2.2.59 conectado al servidor (Preferences → Server → remoto).
- Un contenedor Docker `gns3-workspace:2.2.59` por laboratorio, red bridge.
- Cada laboratorio = bloque de 40 puertos publicados 1:1: el primero es el GNS3 server,
  los otros 39 son consolas telnet de los nodos. Máximo 60 laboratorios → rango 30000-32399.
- El GNS3 server pide usuario/contraseña: usuario = matrícula del dueño, contraseña = token de 8.
  Go NO abre puertos ni hace proxy.
- Modo equipo: código de 6 letras e invitaciones a compañeros del mismo grupo;
  todos usan el mismo puerto y token. El dueño "limpia" (borra contenedor); un miembro solo sale.
- Imagen: opt/container-environment/container-images/gns3-workspace/build (Dockerfile + entrypoint.sh).
  Base fija gns3/gns3-server:2.2.61 + pip gns3-server==2.2.59 (la construcción falla si no da 2.2.59).
  Parche en el Dockerfile: GNS3 2.2.59 "recarga" nodos Docker con docker restart (quedan sin eth0
  y la consola no responde); se cambia restart() por stop()+start().
- Docker interno de cada lab con overlay2 en el volumen `<contenedor>-docker` (/var/lib/docker),
  respaldo a vfs. Remove borra contenedor + volumen (Limpiar y Monitor). ~830 MB por lab.
- Ansible: opt/container-environment/production-system/raccoon.lab/gns3/volumes/ansible
  (deploy_templates.yml al crear el lab, deploy_topology.yml para plantillas), con basic auth
  (gns3_user = matrícula, gns3_password = token).
- Plantillas de GNS3: Cisco 7200 (principal, Dynamips, idlepc 0x606df838, 512 MB), Cisco 3725,
  QEMU opcionales solo si existe la imagen (IOSv, IOSvL2, CSR1000v, MikroTik) y
  "Ubuntu RaccoonLAB" (Docker) solo si la imagen está cargada; si no, los hosts son VPCS.
- Host Ubuntu: gns3_images/Docker/ubuntu-gns3.tar; entrypoint.sh lo carga y lo etiqueta
  raccoon/ubuntu-host:latest. Si existe gns3_images/Docker/Dockerfile, raccoon.sh construye la
  imagen y regenera el .tar cuando no coincide (como el original, que la construía en el servidor).
- Si falta el c7200 o la imagen Ubuntu el lab queda CORRIENDO con un aviso en MENSAJE.
- Volúmenes: gns3_images/{IOS,QEMU,Docker} y gns3_projects (no se suben a git).

## Entorno de desarrollo (mi laptop, EndeavourOS, shell fish)
- Firebird de prueba: `podman` contenedor rl-firebird en 127.0.0.1:3050 (pass dev12345).
- Redis de prueba: contenedor rl-redis en 127.0.0.1:6380.
- NO tocar los contenedores Docker del profe: redis-session (6379), firebird5-engine (3051), mysql-server (3306).
- Correr: `go run ./backend/cmd` (raíz) y `yarn dev` (frontend). El .env local está en ~/raccoon/.env.

## Despliegue en el servidor (raccoon.sh, un solo archivo)
- El usuario solo pasa raccoon.sh a la VM, lo ejecuta (root) y copia las imágenes. Idempotente.
- Si viene solo, clona el repo en /opt/raccoon-lab (o git pull) y se re-ejecuta desde ahí;
  /opt/container-environment es un enlace al opt/ del repo.
- dnf: EPEL+CRB, podman, podman-compose, docker-ce, cockpit (root habilitado), firewalld, nftables.
- .env de producción en opt/.../raccoon.lab/.env (secretos aleatorios la primera vez, no se toca después).
- compose.yml (podman-compose, proyecto raccoon-lab): Firebird 5, Redis, App1, App2 (label=disable
  por el socket de Docker; GNS3_VOLUMES montado en la misma ruta), AppWeb y Nginx balanceador (80).
  Imágenes: container-images/raccoon-api y raccoon-web (Dockerfiles, contexto = raíz del repo).
- Red (aprendido en la VM): NUNCA firewall-cmd --reload con reglas "direct": vacía todo iptables
  (Docker y netavark). Ahora: reglas rich solo-LAN (ssh/http/cockpit) en runtime+permanente sin
  reload; filtro solo-LAN de puertos de contenedores en tabla nftables propia `inet raccoon`
  (prerouting priority raw); reglas DOCKER-USER para 10.89.0.0/16 (Docker pone FORWARD en DROP).
  Todo eso lo aplica /usr/local/sbin/raccoon-red (drop-in de docker.service + raccoon-red.service).
- Pasos no críticos avisan y siguen ("FALTA HACER" al final); críticos se detienen.
- Autoprueba final: página por la IP de la LAN y desde otra red, API por Nginx, imágenes de esta
  ejecución, DOCKER-USER sin duplicados, filtro nft y consola real de un nodo Ubuntu (y tras recargar).

## Hecho y probado
- Login con roles y bloqueo, Mi cuenta, Usuarios, Grupos, Importar CSV/XLSX.
- Laboratorios reales (laptop y VM Rocky 9.8): crear, unirse, invitaciones, limpiar, monitor;
  GNS3 2.2.59 con auth; GNS3 de escritorio conectado por la red; Cisco 7200/3725 y Ubuntu
  RaccoonLAB arrancan; consolas funcionan (con mate-terminal; xterm en Wayland no deja escribir);
  Edit config pone la IP; ping entre router y Ubuntu.
- raccoon.sh probado en la VM Rocky 9.8 (4 vCPU, 8 GB, sin KVM): instala, levanta y se autoprueba.
- Laboratorio.vue: copiar con respaldo execCommand (http sin HTTPS); el botón cambia a "Copiado" 2 s.

## Pendiente (en este orden)
1. Backend de Plantillas: POST /api/plantillas/despliegue (solo PROFESOR, sobre SU laboratorio
   corriendo) que ejecute deploy_topology.yml dentro del contenedor, en segundo plano con avance
   en Firebird; Plantillas.vue muestra avance y resultado. Preparado para topologías de la IA.
2. Después (no ahora): calculadora de subredes en Go y la IA (Qwen en otra PC por LAN, documentos .md en Firebird).

## Proyectos de referencia (solo leer, NUNCA modificar ni copiar su estructura)
- ~/GoProjects/fiber3 → proyecto del profe. ES EL MODELO a seguir para backend
  (cmd/config/database/domain/repository/handler/middleware/router) y frontend.
- ~/referencias/raccoon_lab (https://github.com/verkhuntsmen-png/raccoon_lab) → versión
  ORIGINAL del proyecto. Úsala para entender QUÉ hacía, no CÓMO:
  - pkg/workspace/docker_manager.go: contenedor por alumno, puerto base + consolas, KVM.
  - pkg/workspace/ansible_manager.go y ansible/: carga de plantillas y topologías en GNS3.
  - pkg/handlers/laboratorio_handler.go: crear / unirse por código / limpiar / estado.
  - sql/ y la documentación: modelo original de datos y flujo del alumno.
  Lo que YA NO se usa de ahí: JWT (ahora sesión en Redis), net/http (ahora Fiber v3),
  NetworkMode host (ahora bridge con bloque de puertos), proxy en Go del puerto +10000
  (ahora el GNS3 server pide usuario y token), registro de usuarios (ahora importación CSV/Excel).
