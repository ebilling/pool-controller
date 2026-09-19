#!/usr/bin/env bash
# macOS still ships Bash 3.2, where nounset treats an empty indexed array as
# unbound. Keep this script compatible with the system shell.
set -eo pipefail

umask 077
TEMP_EXTENSION_FILE=
trap 'rm -f "${TEMP_EXTENSION_FILE:-}"' EXIT

usage() {
	cat <<'EOF'
Generate an EC certificate authority and server certificates.

Usage:
  generate-tls-certs.sh ca [options]
  generate-tls-certs.sh host --dns HOSTNAME [--dns HOSTNAME ...] [--ip ADDRESS ...] [options]

CA options:
  --out-dir DIR             Output directory (default: tls)
  --common-name NAME        CA common name (default: Pool Controller CA)
  --days DAYS               CA lifetime (default: 3650)
  --no-encrypt-ca-key       Do not password-protect the CA private key
  --force                   Replace files that already exist

Host options:
  --out-dir DIR             Output directory (default: tls)
  --name NAME               Output filename prefix (default: pool-controller)
  --ca-cert FILE            CA certificate (default: OUT_DIR/ca.crt)
  --ca-key FILE             CA private key (default: OUT_DIR/ca.key)
  --dns HOSTNAME            Add a DNS SAN; may be repeated
  --hostname HOSTNAME       Alias for --dns
  --ip ADDRESS              Add an IPv4 or IPv6 SAN; may be repeated
  --days DAYS               Certificate lifetime (default: 825)
  --force                   Replace files that already exist

Examples:
  scripts/generate-tls-certs.sh ca
  scripts/generate-tls-certs.sh host \
    --dns pool-controller.local --dns pool.example.net \
    --ip 192.168.1.25 --ip fd00::25
EOF
}

die() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

require_value() {
	[[ $# -ge 2 ]] || die "$1 requires a value"
}

valid_days() {
	[[ $1 =~ ^[1-9][0-9]*$ ]]
}

valid_dns_name() {
	local name=$1 label
	[[ ${#name} -le 253 && $name != *..* ]] || return 1
	if [[ $name == \*.* ]]; then
		name=${name#*.}
	fi
	[[ $name =~ ^[A-Za-z0-9.-]+$ ]] || return 1
	IFS=. read -r -a labels <<<"$name"
	for label in "${labels[@]}"; do
		[[ -n $label && ${#label} -le 63 ]] || return 1
		[[ $label =~ ^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$ ]] || return 1
	done
}

valid_ip_text() {
	# OpenSSL performs the full address parse. This check prevents config-file
	# delimiters and newlines from reaching the generated extension file.
	[[ $1 =~ ^[0-9A-Fa-f:.]+$ ]]
}

ensure_outputs_available() {
	local force=$1
	shift
	if [[ $force == true ]]; then
		rm -f -- "$@"
		return
	fi
	local file
	for file in "$@"; do
		[[ ! -e $file ]] || die "$file already exists (use --force to replace it)"
	done
}

generate_ca() {
	local out_dir=tls common_name="Pool Controller CA" days=3650
	local encrypt=true force=false

	while (($#)); do
		case $1 in
		--out-dir)
			require_value "$@"; out_dir=$2; shift 2 ;;
		--common-name)
			require_value "$@"; common_name=$2; shift 2 ;;
		--days)
			require_value "$@"; days=$2; shift 2 ;;
		--no-encrypt-ca-key)
			encrypt=false; shift ;;
		--force)
			force=true; shift ;;
		-h|--help)
			usage; exit 0 ;;
		*)
			die "unknown CA option: $1" ;;
		esac
	done

	valid_days "$days" || die "--days must be a positive integer"
	[[ -n $common_name && $common_name != *$'\n'* && $common_name != */* ]] ||
		die "--common-name must not contain a slash or newline"

	mkdir -p "$out_dir"
	local key="$out_dir/ca.key" cert="$out_dir/ca.crt"
	ensure_outputs_available "$force" "$key" "$cert"

	local -a key_args=(genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256)
	if [[ $encrypt == true ]]; then
		key_args+=(-aes-256-cbc)
	fi
	openssl "${key_args[@]}" -out "$key"
	chmod 600 "$key"

	openssl req -new -x509 -sha256 -days "$days" \
		-key "$key" -out "$cert" \
		-subj "/CN=$common_name" \
		-addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
		-addext "keyUsage=critical,keyCertSign,cRLSign" \
		-addext "subjectKeyIdentifier=hash"
	chmod 644 "$cert"

	printf 'Created EC certificate authority:\n  key:  %s\n  cert: %s\n' "$key" "$cert"
}

generate_host() {
	local out_dir=tls name=pool-controller ca_cert= ca_key= days=825
	local force=false
	local -a dns_names=() ip_addresses=()

	while (($#)); do
		case $1 in
		--out-dir)
			require_value "$@"; out_dir=$2; shift 2 ;;
		--name)
			require_value "$@"; name=$2; shift 2 ;;
		--ca-cert)
			require_value "$@"; ca_cert=$2; shift 2 ;;
		--ca-key)
			require_value "$@"; ca_key=$2; shift 2 ;;
		--dns|--hostname)
			require_value "$@"; dns_names+=("$2"); shift 2 ;;
		--ip)
			require_value "$@"; ip_addresses+=("$2"); shift 2 ;;
		--days)
			require_value "$@"; days=$2; shift 2 ;;
		--force)
			force=true; shift ;;
		-h|--help)
			usage; exit 0 ;;
		*)
			die "unknown host option: $1" ;;
		esac
	done

	valid_days "$days" || die "--days must be a positive integer"
	[[ $name =~ ^[A-Za-z0-9._-]+$ ]] || die "--name contains unsupported characters"
	((${#dns_names[@]} + ${#ip_addresses[@]} > 0)) ||
		die "provide at least one --dns/--hostname or --ip value"

	local value
	for value in "${dns_names[@]}"; do
		valid_dns_name "$value" || die "invalid DNS hostname: $value"
	done
	for value in "${ip_addresses[@]}"; do
		valid_ip_text "$value" || die "invalid IP address: $value"
	done

	[[ -n $ca_cert ]] || ca_cert="$out_dir/ca.crt"
	[[ -n $ca_key ]] || ca_key="$out_dir/ca.key"
	[[ -f $ca_cert ]] || die "CA certificate not found: $ca_cert"
	[[ -f $ca_key ]] || die "CA private key not found: $ca_key"

	mkdir -p "$out_dir"
	local key="$out_dir/$name.key" csr="$out_dir/$name.csr"
	local cert="$out_dir/$name.crt" extensions
	extensions=$(mktemp "${TMPDIR:-/tmp}/pool-controller-cert.XXXXXX")
	TEMP_EXTENSION_FILE=$extensions
	ensure_outputs_available "$force" "$key" "$csr" "$cert"

	local -a sans=()
	for value in "${dns_names[@]}"; do sans+=("DNS:$value"); done
	for value in "${ip_addresses[@]}"; do sans+=("IP:$value"); done
	local san
	(IFS=,; san="${sans[*]}"; cat >"$extensions" <<EOF
basicConstraints=critical,CA:FALSE
keyUsage=critical,digitalSignature
extendedKeyUsage=serverAuth
subjectKeyIdentifier=hash
authorityKeyIdentifier=keyid,issuer
subjectAltName=$san
EOF
	)

	local common_name=${dns_names[0]:-${ip_addresses[0]}}
	openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:P-256 -out "$key"
	chmod 600 "$key"
	openssl req -new -sha256 -key "$key" -out "$csr" -subj "/CN=$common_name"
	openssl x509 -req -sha256 -days "$days" \
		-in "$csr" -CA "$ca_cert" -CAkey "$ca_key" \
		-set_serial "0x$(openssl rand -hex 16)" \
		-extfile "$extensions" -out "$cert"
	chmod 644 "$csr" "$cert"

	openssl verify -CAfile "$ca_cert" "$cert"
	printf 'Created EC server certificate:\n  key:  %s\n  cert: %s\n  csr:  %s\n' \
		"$key" "$cert" "$csr"
	rm -f "$extensions"
	TEMP_EXTENSION_FILE=
}

command=${1:-}
case $command in
ca)
	shift
	generate_ca "$@"
	;;
host)
	shift
	generate_host "$@"
	;;
-h|--help|"")
	usage
	;;
*)
	die "unknown command: $command"
	;;
esac
