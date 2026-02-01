FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ftp-to-nextcloud .

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/ftp-to-nextcloud /ftp-to-nextcloud

ENV FTP_PORT=2121 \
    PASV_MIN_PORT=30000 \
    PASV_MAX_PORT=30100 \
    FTP_TLS=false \
    INSECURE_SKIP_VERIFY=false \
    DEBUG=false

USER 65534:65534

EXPOSE 2121 30000-30100

ENTRYPOINT ["/ftp-to-nextcloud"]
