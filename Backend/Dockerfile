# Imagem do backend para o deploy (deploy/compose.yaml). O desenvolvimento
# continua com `go run` no host -- ver Makefile.
#
# Leva dois binarios: `api` (servidor e `api -migrate`) e `admin`, que cadastra
# usuario, talhao, no e calibracao direto na VM:
#   docker compose exec api admin usuario -email=... -nome=...
#
# Nomes totalmente qualificados: o Podman nao assume docker.io para nome curto.
FROM docker.io/library/golang:1.26.10-alpine3.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# CGO desligado: binario estatico, sem depender da libc da imagem final. A PWA
# e as migrations ja vao dentro dele por //go:embed.
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/ ./cmd/api ./cmd/admin

FROM docker.io/library/alpine:3.24.4
# Alpine e nao distroless: o healthcheck do compose usa o wget do busybox.
RUN adduser -D -H -u 10001 tcc
COPY --from=build /out/api /out/admin /usr/local/bin/
USER tcc
EXPOSE 8080
CMD ["api"]
