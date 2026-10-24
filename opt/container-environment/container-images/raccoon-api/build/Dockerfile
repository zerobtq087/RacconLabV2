# Raccoon Lab - API en Go (App1 y App2 usan esta misma imagen)
# Se construye desde la raíz del repo:
#   podman build -t localhost/raccoon-api:latest -f opt/container-environment/container-images/raccoon-api/build/Dockerfile .

# 1) Compilar: el driver de Firebird es Go puro, así que sale un binario estático
FROM docker.io/library/golang:1.27.2-alpine3.23 AS build
WORKDIR /src

# Primero solo las dependencias (se quedan en caché si no cambia go.mod)
COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /raccoon-api ./backend/cmd

# 2) Imagen final: solo el binario (sin compilador ni código fuente)
FROM docker.io/library/alpine:3.23
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /raccoon-api /usr/local/bin/raccoon-api

# La configuración llega por variables de entorno (podman-compose lee el .env)
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/raccoon-api"]
