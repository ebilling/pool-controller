module github.com/ebilling/pool-controller

go 1.27

require (
	github.com/brutella/hc v1.2.5
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/stretchr/testify v1.12.1
	github.com/ziutek/rrd v0.0.4
	golang.org/x/crypto v0.57.0
	golang.org/x/sys v0.48.0
	// Only used by -gpio-driver periph, the fallback path. Pinned because host
	// v3.8.5 routes bcm283x.WaitForEdge through its gpioioctl driver, whose
	// SetReadDeadline fails with "file type does not support deadline" on the
	// Pi 3 (Raspbian 11, kernel 6.1), so every thermistor read times out.
	// v3.8.2 still waits on edges with raw epoll via sysfs, which works.
	// internal/gpiocdev is the replacement; once it has run in production long
	// enough, drop both of these.
	periph.io/x/conn/v3 v3.7.0
	periph.io/x/host/v3 v3.8.2
)

require (
	github.com/brutella/dnssd v1.2.14 // indirect
	github.com/miekg/dns v1.1.73 // indirect
	github.com/tadglines/go-pkgs v0.0.0-20210623144937-b983b20f54f9 // indirect
	github.com/vishvananda/netlink v1.2.1-beta.2 // indirect
	github.com/vishvananda/netns v0.0.0-20200728191858-db3c7e526aae // indirect
	github.com/xiam/to v0.0.0-20200126224905-d60d31e03561 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.59.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)
