# Raccoon Lab - AppWeb: el frontend Vue ya compilado, servido por Nginx
# Se construye desde la raíz del repo:
#   podman build -t localhost/raccoon-web:latest -f opt/container-environment/container-images/raccoon-web/build/Dockerfile .

# 1) Compilar el frontend con el Yarn 4.14.1 que viene en el repo
FROM docker.io/library/node:22-alpine3.22 AS build
WORKDIR /app

# Redes lentas: Node 22 prueba IPv6 e IPv4 a la vez ("happy eyeballs") con solo
# 250 ms por intento y Yarn termina en ETIMEDOUT aunque la red funcione.
# Se prefiere IPv4 y se da más tiempo a cada intento de conexión.
ENV NODE_OPTIONS="--dns-result-order=ipv4first --network-family-autoselection-attempt-timeout=2000"

# Primero solo las dependencias (se quedan en caché si no cambia yarn.lock)
COPY frontend/package.json frontend/yarn.lock frontend/.yarnrc.yml ./
COPY frontend/.yarn/releases ./.yarn/releases
RUN node .yarn/releases/yarn-4.14.1.cjs install --immutable

COPY frontend ./
RUN node .yarn/releases/yarn-4.14.1.cjs build

# 2) Imagen final: Nginx con los archivos de dist/
FROM docker.io/library/nginx:1.28-alpine
COPY opt/container-environment/container-images/raccoon-web/build/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
