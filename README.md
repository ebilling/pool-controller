# pool-controller

Raspberry Pi based controller for a solar heated pool written in go.

Requires Go 1.27+.

A `go build` from this git tree stamps the commit automatically. `pool-controller`
logs `pool-controller <sha> go=<version>` on startup, and `-version` prints the
same line. If you are building without `.git` (Docker copies omit it), pass the
SHA at link time:

```sh
go build -ldflags "-X main.version=$(git rev-parse --short=12 HEAD)"
VERSION=$(git rev-parse --short=12 HEAD) docker compose build
```

## Operating model

The controller drives a main circulation pump, a sweep/cleaner pump, and the
solar valve. Its automatic states are:

- `PUMP` — circulation only
- `SWEEP` — circulation plus the sweep pump
- `SOLAR` — circulation through the roof panels
- `MIXING` — solar plus the sweep pump

The water probe is beside the equipment, not in the pool. While circulation is
off, the displayed pool temperature is deliberately held at the last reading
taken with water moving. A pump-only reading immediately after startup can
describe water that was standing in the pipes—or the very hot skin under the
pool cover—rather than the bulk pool.

Heating starts when the held pool temperature is below the target minus the
tolerance and the roof is at least `Min delta` hotter. Cooling is the inverse:
the pool must be above target plus tolerance and hotter than the roof by the
configured delta. Outside the pre-swim period, an automatic solar request first
circulates without solar for three minutes and then re-evaluates the fresh
reading.

### House-PV and swim schedule

The schedule is designed for house solar production from 11:00–13:00 and a
usual swim period from 14:00–17:00:

- **11:00–14:00:** useful pool solar runs as `MIXING`. The sweep pushes the hot
  surface layer down before swimming, and the resulting temperature is a
  better measurement of the whole pool.
- **14:00–17:00:** automatic control never runs the sweep. At 14:00 it changes
  `MIXING` to `SOLAR` when solar is still useful, or stops when demand has
  ended. Manual HomeKit and button requests remain immediate.
- A new sweep is not started so close to 14:00 that its five-minute minimum run
  would overlap the swim window.

### Cleaning cadence

Cleaning is based on actual continuous sweep-motor work, not on the main
pump's last stop:

- `Required continuous sweep runtime` defaults to two hours. `SWEEP` and
  `MIXING` both count, including transitions between them while the sweep motor
  never stops. Shorter runs do not qualify.
- The instant that interval completes is persisted as
  `LastCleaningCompleted`.
- `Cleaning frequency` is measured in days from that completion. Solar-only
  activity neither satisfies nor postpones cleaning.
- A due cleaning is preferentially started after 11:00, early enough to finish
  before the 14:00 swim window. 04:00–06:00 remains the fallback when no
  qualifying sweep occurred during the PV/pre-swim period.

These settings are under **Cleaning / Sweep** on the configuration page. A
legacy config without `LastCleaningCompleted` is due for a qualifying sweep;
its existing frequency and runtime values are retained. Manual control has a
separate six-hour timeout, so changing the cleaning runtime no longer changes
when automatic control resumes.

### Motor and sensor safeguards

Automatic stops give each motor at least five minutes of runtime, and an
automatic cold start waits until the pumps have rested for fifteen minutes.
Starting the sweep while circulation is already running is allowed immediately
because it starts a different motor. Safety disable and manual requests bypass
these timing gates.

Thermometer values include the timestamp and error status of their last sample.
Stale readings cannot initiate temperature-driven activity. If a probe fails
while equipment is already running, the controller holds that relay state until
it gets a trustworthy measurement instead of treating the cached value as a
reason to stop. Clock-driven cleaning can still start with stale probes.

## Web interface

The status page updates its cards and graph images in place; it no longer
reloads the entire page on every refresh. Temperature graphs contain only
values this controller actually collects, while old RRD data-source names are
retained for file compatibility.

Configuration uses one persisted thermostat mode (`Off`, `Heat only`, `Cool
only`, or `Auto`). Older `HeatDisabled`/`CoolDisabled` files are migrated when
read. Mutating routes require HTTP Basic authentication, reject unsupported
methods and oversized forms, and the HTTPS server has defensive read/write
timeouts and a TLS 1.2 minimum.

### HTTPS certificates

`scripts/generate-tls-certs.sh` creates a P-256 EC certificate authority and
server certificate. See [`scripts/README.md`](scripts/README.md) for complete
controller, macOS, iPhone, and iPad installation instructions. Generate the CA
once, then repeat `--dns` and `--ip` for every name or address used to open the
controller:

```sh
bash scripts/generate-tls-certs.sh ca
bash scripts/generate-tls-certs.sh host \
  --dns pool-controller.local --dns pool.example.net \
  --ip 192.168.1.25 --ip fd00::25
```

The CA key is encrypted by default and prompts for its password when creating
or signing certificates. The host key is unencrypted so the service can start
unattended. Install only `tls/ca.crt` as a trusted root on client devices; keep
`tls/ca.key` private. Configure the daemon with:

```sh
-ssl_cert=/path/to/tls/pool-controller.crt -ssl_key=/path/to/tls/pool-controller.key
```

Run the `host` command again with `--force` when its SAN names or addresses
change. Use `--ca-cert` and `--ca-key` if the CA files are stored elsewhere.

## GPIO

GPIO uses `internal/gpiocdev`, a small pure-Go driver for the Linux GPIO
character device (uAPI v2). It waits for edges with a raw `poll(2)` and takes
each thermistor charge time from the **kernel's** edge timestamp, recorded in
the GPIO interrupt handler. That keeps this process's scheduling delay out of
the measurement, which matters because a full charge through the 100 nF
capacitor is only about 1 ms. No fd is wrapped in an `os.File`, so the Go
runtime poller is never involved.

On startup the controller unexports leftover `/sys/class/gpio` claims on the
pins it uses (LED, thermistors, button, relays). Older binaries left those
exports behind after stop, and chardev will not share them.

## Capture timings on the Pi (100 nF)

Stop `pool-controller` first. Cross-compile from this Mac (Pi 3 is 32-bit ARM):

```sh
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o therm-capture ./cmd/therm-capture
# copy therm-capture to the Pi, then:
sudo ./therm-capture -seconds 30 -adjust 1.75 > capture.jsonl
```

Default GPIOs are pump 15 and roof 14. `-adjust 1.75` matches deployed `real.conf`. Output is JSON lines on stdout and a duration summary on stderr.

Each sample also carries `config_ns`: how long `SET_CONFIG` took (typically
~300 µs on a Pi 3). That is a diagnostic, not an error bar. The charge duration
is clocked from *before* that ioctl, because on this SoC the pinmux happens at
the start of the call.

## HomeKit pairing

The accessories are published as one bridge through `brutella/hap`. The pairing
page renders the `X-HM://` setup payload as a scannable code with the setup
digits under it, the way Apple prints a setup label; the bare setup code is not
a payload the Home app recognizes. The payload is built from the setup code, the
bridge category and the setup id, and is logged at startup as
`HomeKit setup payload`.

An accessory that holds a pairing advertises `sf=0` ("already paired", not
discoverable). Discoverability is not a mode that can be switched on: it is
derived from the pairings stored in `<data_dir>/*.pairing`. Removing the
accessory in the Home app deletes Apple's side of the pairing, and `hap` deletes
its own side and re-announces itself — but only if it handles that request. If
this process is stopped or unreachable at the time, the pairing stays on disk
and the accessory never offers itself again, so the Home app either will not
list it or accepts the setup code and spins forever.

The pairing page reports this state, and while a pairing is present it offers a
**Forget paired controllers** button. That deletes the stored pairings and
restarts the daemon, which is what makes the new state reach the network: `hap`
decides discoverability while announcing itself and cannot be asked to
re-evaluate it. The restart briefly stops the pumps.

`-reset-homekit-pairings` does the same from the command line, for when the web
interface is not reachable. It is maintenance rather than a way to start: it
forgets the pairings and exits without becoming the daemon, so stop the service
first and start it again afterwards.

```sh
sudo service pool-controller stop
sudo pool-controller -data_dir=/var/cache/homekit -reset-homekit-pairings
sudo service pool-controller start
```

The daemon is a native systemd unit (`/etc/systemd/system/pool-controller.service`),
not the SysV script in `init/pool-controller`. Flags that the daemon should run
with go in `/etc/default/pool-controller`. After changing the unit file itself,
`systemctl daemon-reload` is required; changing only `EXTRA_ARGS` needs a restart.

```sh
echo 'EXTRA_ARGS="-debug"' | sudo tee /etc/default/pool-controller
sudo systemctl restart pool-controller
```

The startup log prints the arguments it was given, so `Args:` confirms what
took effect. The reset flags do not belong in this file: they exit instead of
starting, so `Restart=always` would loop.

Either path keeps the accessory's own identity (`uuid`) and key pair, so only
the iOS pairings are dropped. Startup also logs whether any pairing remains.

`-reset-homekit-identity` goes further and discards `uuid`, `keypair` and hap's
bookkeeping as well, so the accessory returns as a device HomeKit has never
seen. Reach for it when a controller or home hub still shows the old accessory:
the id and the first key it was given are remembered together, so an accessory
that keeps its id while its keys change can go on being displayed, with its
last known values, by a controller that can no longer talk to it. The
configuration and the recorded history in the same directory are kept. Remove
the accessory in the Home app first, then:

```sh
sudo service pool-controller stop
sudo pool-controller -data_dir=/var/cache/homekit -reset-homekit-identity
sudo service pool-controller start
```

The Home app reports every failure as "unable to add accessory", so the reason
comes from `hap`'s own log, which goes to syslog alongside everything else.
`pairing is not allowed` means a pairing is still on disk and the setup code
will not be accepted until it is forgotten. `-debug`, or **Debug** on the
configuration page, adds a line per pairing request, including the verdict on
the controller's signature.

HomeKit serves on `-homekit_port` (51826 by default) and announces that port
over mDNS with hap's own responder. Raspberry Pi OS also runs `avahi-daemon` on
UDP 5353. Both processes bound to that port is why the Home app accepts the
setup code and then spins with **no** pairing lines in the log: the phone never
finds `_hap._tcp`. Stop and disable avahi so only `pool-controller` owns 5353:

```sh
sudo ss -ulnp | grep 5353
sudo systemctl stop avahi-daemon
sudo systemctl disable avahi-daemon
```

That drops `hostname.local` resolution; it does not affect HomeKit. If 5353 is
clean and the log still has no `pair-setup` during an attempt, the phone cannot
reach the port — test from the same Wi-Fi with `nc -z <pi> 51826`. On a host
with more than one interface, `-homekit_iface` limits the announcement to one
of them.

On the first start after upgrading from `brutella/hc`, `hap` copies the
accessory's keys and any paired controllers out of the `*.entity` files that
library wrote, and those files are then deleted. A controller paired before the
upgrade stays paired, so the accessory keeps its identity in the Home app.

## Docker (simulation)

This is a Linux userspace stand-in for laptop/CI work. It does **not** reproduce Pi GPIO, capacitor timing, or OS-upgrade accuracy. The image is Debian 12 (bookworm); the working pool host is Raspbian 11 on a Pi 3. Debian 11 was skipped because its current security-mirror packages 404 from Docker.

```sh
# Unit tests (Debian 12 + pkg-config + librrd)
docker build --target test .

# Simulated controller: HTTPS on localhost:8443
docker compose up --build
```

Useful flags (passed after the image entrypoint):

- `-simulate` — skip `host.Init()`, use in-memory pins, script thermometer readings, log relay outputs
- `-sim-pump-temp` / `-sim-roof-temp` — Celsius values for the scripted sensors
- `-http_port` — HTTPS listen port inside the container (default 443)
- `-homekit_port` / `-homekit_iface` — where HomeKit listens and which interface it announces on
- `-debug` — debug logging, including each HomeKit pairing request

HomeKit pairing from an iPhone needs multicast. `docker compose` publishes only TCP 8443 by default; use host networking if you are testing discovery.
