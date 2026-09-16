# pool-controller

Raspberry Pi based controller for a solar heated pool written in go.

Requires Go 1.27+.

## GPIO drivers

`-gpio-driver` selects how the process talks to the GPIO pins.

- `cdev` (default) — `internal/gpiocdev`, a small pure-Go driver for the Linux
  GPIO character device (uAPI v2). It waits for edges with a raw `poll(2)` and
  takes each charge time from the **kernel's** edge timestamp, recorded in the
  GPIO interrupt handler. That keeps this process's scheduling delay out of the
  measurement, which matters because a full charge through the 100 nF capacitor
  is only about 1 ms.
- `periph` — the older `periph.io` path, timing charges from a userspace clock
  read after the process wakes up. Kept as a fallback.

`periph.io/x/host` is pinned to v3.8.2 for that fallback. From v3.8.3 on,
`bcm283x.WaitForEdge` delegates to periph's `gpioioctl` driver, which hands the
line descriptor to `os.NewFile` and then calls `SetReadDeadline`. When the Go
runtime cannot register that descriptor with epoll, `os.NewFile` discards the
registration error, so every later `SetReadDeadline` fails with:

```
GPIOLine.WaitForEdge() setReadDeadline() returned: file type does not support deadline
```

`WaitForEdge` then returns false immediately and no thermometer reading ever
succeeds. `internal/gpiocdev` avoids this by never wrapping the descriptor in an
`os.File`.

On startup, `cdev` unexports leftover `/sys/class/gpio` claims on the pins this
process uses (LED, thermistors, button, relays). The previous periph binary
leaves those exports behind after stop, and chardev will not share them.

## Capture timings on the Pi (100 nF)

Stop `pool-controller` first. Cross-compile from this Mac (Pi 3 is 32-bit ARM):

```sh
GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -o therm-capture ./cmd/therm-capture
# copy therm-capture to the Pi, then:
sudo ./therm-capture -seconds 30 -adjust 1.75 > capture.jsonl
```

Default GPIOs are pump 15 and roof 14. `-adjust 1.75` matches deployed `real.conf`. Output is JSON lines on stdout and a duration summary on stderr.

`-driver cdev|periph` picks the backend, so the two can be compared on the same
hardware. Run them one at a time, since each claims the lines exclusively:

```sh
sudo ./therm-capture -n 200 -driver cdev   > cdev.jsonl
sudo ./therm-capture -n 200 -driver periph > periph.jsonl
```

Each sample also carries `config_ns`: for `cdev` it is how long `SET_CONFIG`
took (typically ~300 µs on a Pi 3). That is a diagnostic, not an error bar.
The charge duration is clocked from *before* that ioctl, because on this SoC
the pinmux happens at the start of the call. `periph` reports 0.

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

HomeKit pairing from an iPhone needs multicast. `docker compose` publishes only TCP 8443 by default; use host networking if you are testing discovery.
