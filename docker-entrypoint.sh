#!/bin/sh
set -eu

DATA_DIR="${DATA_DIR:-/var/cache/homekit}"
SSL_CERT="${SSL_CERT:-/etc/ssl/certs/pool-controller.crt}"
SSL_KEY="${SSL_KEY:-/etc/ssl/private/pool-controller.key}"

mkdir -p "$DATA_DIR" "$(dirname "$SSL_CERT")" "$(dirname "$SSL_KEY")"

if [ ! -f "$SSL_CERT" ] || [ ! -f "$SSL_KEY" ]; then
	openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes \
		-keyout "$SSL_KEY" -out "$SSL_CERT" \
		-subj "/CN=pool-controller"
fi

exec /usr/local/bin/pool-controller \
	-ssl_cert "$SSL_CERT" \
	-ssl_key "$SSL_KEY" \
	-data_dir "$DATA_DIR" \
	-pid "${DATA_DIR}/pool-controller.pid" \
	"$@"
