# Imagen de la app Raccoon Lab (Go)
# El binario "raccoon" lo genera build_local.sh (CGO_ENABLED=0, estático)
FROM docker.io/library/alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

ENV TZ=America/Mexico_City
WORKDIR /app

COPY raccoon /app/raccoon
RUN chmod +x /app/raccoon

EXPOSE 8080
ENTRYPOINT ["/app/raccoon"]
