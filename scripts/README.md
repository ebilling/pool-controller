# Install private HTTPS certificates

This directory contains `generate-tls-certs.sh`, which creates:

- a private P-256 EC certificate authority (CA);
- a P-256 EC certificate and key for the pool controller; and
- DNS and IP Subject Alternative Names (SANs) used by browsers to validate the
  controller.

The CA certificate must be installed on every client device. Apple does not
sync manually trusted CAs through iCloud Keychain.

## 1. Install OpenSSL

The script requires an OpenSSL version that supports `req -addext`. On macOS,
install OpenSSL 3 with Homebrew:

```sh
brew install openssl@3
export PATH="$(brew --prefix openssl@3)/bin:$PATH"
openssl version
```

Run the remaining generation commands from the repository root.

## 2. Choose names and addresses

Include every hostname and IP address that people will type into a browser.
For example:

- `pool.example.net`
- `192.168.1.25`
- `fd00::25`

The certificate does not provide name resolution. A hostname must still exist
in DNS or in each client's hosts file. In particular, this project's deployment
instructions disable Avahi, so a `.local` hostname may not resolve even when it
is present in the certificate. Including the controller's stable LAN IP is
recommended.

## 3. Generate the CA

Generate the CA once on a trusted administration computer:

```sh
bash scripts/generate-tls-certs.sh ca
```

This creates:

```text
tls/ca.key   encrypted CA private key
tls/ca.crt   public CA certificate
```

Choose and record the CA-key password when prompted. Back up `ca.key` and its
password securely. Never copy `ca.key` to the pool controller or a client
device.

For unattended test environments only, `--no-encrypt-ca-key` creates an
unencrypted CA key.

## 4. Generate the controller certificate

Repeat `--dns` and `--ip` as needed:

```sh
bash scripts/generate-tls-certs.sh host \
  --dns pool.example.net \
  --ip 192.168.1.25 \
  --ip fd00::25
```

Enter the CA-key password when prompted. The command creates:

```text
tls/pool-controller.key   unencrypted server private key
tls/pool-controller.crt   server certificate
tls/pool-controller.csr   certificate signing request
```

The server key is intentionally unencrypted because the system service must
start without an interactive password prompt.

Inspect and verify the result:

```sh
openssl x509 -in tls/pool-controller.crt -noout -subject -issuer -dates \
  -ext subjectAltName
openssl verify -CAfile tls/ca.crt tls/pool-controller.crt
```

## 5. Install the certificate on the controller

Copy only the server certificate and server key to the Raspberry Pi:

```sh
scp tls/pool-controller.crt tls/pool-controller.key pi@192.168.1.25:/tmp/
ssh pi@192.168.1.25
```

On the Pi:

```sh
sudo install -d -m 755 /etc/pool-controller/tls
sudo install -m 644 /tmp/pool-controller.crt /etc/pool-controller/tls/
sudo install -m 600 /tmp/pool-controller.key /etc/pool-controller/tls/
rm /tmp/pool-controller.crt /tmp/pool-controller.key
```

Add the certificate paths to `/etc/default/pool-controller`, retaining any
other flags already required by the installation:

```sh
EXTRA_ARGS="-ssl_cert=/etc/pool-controller/tls/pool-controller.crt -ssl_key=/etc/pool-controller/tls/pool-controller.key"
```

Restart the service and inspect its status:

```sh
sudo systemctl restart pool-controller
sudo systemctl status pool-controller
```

Changing `/etc/default/pool-controller` does not require
`systemctl daemon-reload`.

From the administration computer, test the certificate before changing trust:

```sh
openssl s_client \
  -connect 192.168.1.25:443 \
  -servername pool.example.net \
  -CAfile tls/ca.crt </dev/null
```

The output should end with `Verify return code: 0 (ok)`.

## 6. Trust the CA on macOS

Install the public CA certificate—not the server certificate or either private
key—on each Mac:

```sh
sudo security add-trusted-cert \
  -d -r trustRoot \
  -k /Library/Keychains/System.keychain \
  tls/ca.crt
```

Alternatively, open Keychain Access, import `ca.crt` into the **System**
keychain, open the imported certificate, expand **Trust**, and select
**Always Trust**.

Quit and reopen the browser, then visit the controller using a hostname or IP
included in the certificate.

## 7. Trust the CA on iPhone and iPad

Repeat these steps on every iPhone and iPad:

1. Transfer only `tls/ca.crt` to the device, for example with AirDrop.
2. Open the certificate and approve the downloaded profile.
3. Open **Settings → General → VPN & Device Management** and install the
   profile.
4. Open **Settings → General → About → Certificate Trust Settings**.
5. Enable full trust for **Pool Controller CA** and confirm.

Opening the controller by an IP address or hostname not listed in the
certificate will still produce a warning.

## 8. Renew or change names

Keep the existing CA so client devices do not need a new trust profile. Issue a
new server certificate with the complete current SAN list:

```sh
bash scripts/generate-tls-certs.sh host --force \
  --dns pool.example.net \
  --ip 192.168.1.25
```

Install the new `.crt` and `.key` on the controller and restart the service.
Omitting an old name or address removes it from the replacement certificate.

## Security notes

- Keep `ca.key` offline or in encrypted backup storage.
- Never install or share either private key with client devices.
- The CA key is capable of issuing certificates trusted by every device on
  which `ca.crt` is installed.
- Use a stable DHCP reservation or local DNS name to reduce certificate
  replacements caused by address changes.
- Revoke trust by deleting **Pool Controller CA** from Keychain Access or the
  installed profile from iOS/iPadOS.
