# Dockerfile para nó validador EQUA
FROM golang:1.24-alpine AS builder

# Instalar dependências
RUN apk add --no-cache gcc musl-dev linux-headers git make

# Copiar código fonte
WORKDIR /build
COPY . .

# Compilar geth
RUN go mod download
RUN go run build/ci.go install ./cmd/geth

# Imagem final
FROM alpine:latest

RUN apk add --no-cache ca-certificates bash curl jq openssl

# Copiar binário
COPY --from=builder /build/build/bin/geth /usr/local/bin/

# Criar diretórios
RUN mkdir -p /data /keys

# Copiar entrypoint scripts
COPY docker/entrypoint.sh /usr/local/bin/
COPY docker/validator-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/validator-entrypoint.sh

# Expor portas
# 8545: HTTP RPC
# 8546: WebSocket
# 30303: P2P
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

