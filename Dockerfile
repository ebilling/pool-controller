# Linux userspace for build/test/sim. Not the Pi 3 kernel or Broadcom GPIO.
# Debian 11 (bullseye) would match Raspbian 11 more closely, but current
# debian-security packages 404; bookworm is the runnable stand-in.
FROM golang:1.27-bookworm AS toolchain

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		pkg-config \
		librrd-dev \
	&& rm -rf /var/lib/apt/lists/*

ENV CGO_ENABLED=1
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM toolchain AS test
RUN go test -mod=readonly -count=1 -short ./...

FROM toolchain AS build
ARG VERSION=unknown
RUN go build -mod=readonly -ldflags "-X main.version=${VERSION}" -o /out/pool-controller .

FROM debian:bookworm-slim AS runtime
RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
		ca-certificates \
		librrd8 \
		openssl \
	&& rm -rf /var/lib/apt/lists/* \
	&& mkdir -p /var/cache/homekit /etc/ssl/certs /etc/ssl/private

COPY --from=build /out/pool-controller /usr/local/bin/pool-controller
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENV DATA_DIR=/var/cache/homekit
EXPOSE 443
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["-simulate", "-sim-pump-temp=24", "-sim-roof-temp=45"]
