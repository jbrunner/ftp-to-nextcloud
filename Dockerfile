FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ftp-to-nextcloud .

FROM scratch

COPY --from=builder /build/ftp-to-nextcloud /ftp-to-nextcloud

ENV FTP_PORT=2121 \
    FTP_TLS=false \
    DEBUG=false

USER 65534:65534

EXPOSE 2121

ENTRYPOINT ["/ftp-to-nextcloud"]
