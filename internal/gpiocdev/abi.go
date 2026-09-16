package gpiocdev

import "unsafe"

// Kernel GPIO character device uAPI v2, from include/uapi/linux/gpio.h
// (Linux 5.10+). The v1 ABI is deprecated and the /sys/class/gpio interface it
// replaced is scheduled for removal, so only v2 is implemented here.
//
// The kernel declares the 64-bit fields as __aligned_u64, which forces 8-byte
// alignment even on 32-bit targets. Go aligns uint64 to 4 bytes on 386 and arm,
// so every struct below carries the kernel's explicit padding and the layout is
// asserted at compile time rather than trusted to match.

const nameSize = 32

// maxLines and maxAttrs size the fixed arrays in the request structs. Only one
// line per request is used here, but the arrays are part of the ABI and the
// full size has to be sent.
const (
	maxLines = 64
	maxAttrs = 10
)

// Line flags (enum gpio_v2_line_flag).
const (
	flagUsed uint64 = 1 << iota
	flagActiveLow
	flagInput
	flagOutput
	flagEdgeRising
	flagEdgeFalling
	flagOpenDrain
	flagOpenSource
	flagBiasPullUp
	flagBiasPullDown
	flagBiasDisabled
	flagEventClockRealtime
	flagEventClockHTE
)

// Attribute ids (enum gpio_v2_line_attr_id).
const (
	attrIDFlags        uint32 = 1
	attrIDOutputValues uint32 = 2
	attrIDDebounce     uint32 = 3
)

// Event ids (enum gpio_v2_line_event_id).
const (
	eventRisingEdge  uint32 = 1
	eventFallingEdge uint32 = 2
)

type chipInfo struct {
	Name  [nameSize]byte
	Label [nameSize]byte
	Lines uint32
}

type lineValues struct {
	Bits uint64
	Mask uint64
}

type lineInfo struct {
	Name     [nameSize]byte
	Consumer [nameSize]byte
	Offset   uint32
	NumAttrs uint32
	Flags    uint64
	Attrs    [maxAttrs]lineAttribute
	Padding  [4]uint32
}

// lineAttribute holds the kernel's anonymous union of flags, output values, and
// debounce period. All three are read through Value; debounce only uses the low
// 32 bits.
type lineAttribute struct {
	ID      uint32
	Padding uint32
	Value   uint64
}

type lineConfigAttribute struct {
	Attr lineAttribute
	Mask uint64
}

type lineConfig struct {
	Flags    uint64
	NumAttrs uint32
	Padding  [5]uint32
	Attrs    [maxAttrs]lineConfigAttribute
}

type lineRequest struct {
	Offsets         [maxLines]uint32
	Consumer        [nameSize]byte
	Config          lineConfig
	NumLines        uint32
	EventBufferSize uint32
	Padding         [5]uint32
	Fd              int32
}

type lineEvent struct {
	TimestampNS uint64
	ID          uint32
	Offset      uint32
	Seqno       uint32
	LineSeqno   uint32
	Padding     [6]uint32
}

// Struct sizes are encoded in the ioctl request numbers, so they must match the
// kernel exactly or every ioctl fails with ENOTTY.
const (
	sizeofChipInfo    = 68
	sizeofLineValues  = 16
	sizeofLineInfo    = 256
	sizeofLineConfig  = 272
	sizeofLineRequest = 592
	sizeofLineEvent   = 48
)

// Compile-time layout assertions. If a struct does not match the kernel ABI on
// the target architecture, the array index below is out of range and the build
// fails instead of silently corrupting ioctl arguments at runtime.
var (
	_ = [1]struct{}{}[unsafe.Sizeof(chipInfo{})-sizeofChipInfo]
	_ = [1]struct{}{}[unsafe.Sizeof(lineValues{})-sizeofLineValues]
	_ = [1]struct{}{}[unsafe.Sizeof(lineInfo{})-sizeofLineInfo]
	_ = [1]struct{}{}[unsafe.Sizeof(lineConfig{})-sizeofLineConfig]
	_ = [1]struct{}{}[unsafe.Sizeof(lineRequest{})-sizeofLineRequest]
	_ = [1]struct{}{}[unsafe.Sizeof(lineEvent{})-sizeofLineEvent]

	_ = [1]struct{}{}[unsafe.Sizeof(lineAttribute{})-16]
	_ = [1]struct{}{}[unsafe.Sizeof(lineConfigAttribute{})-24]

	_ = [1]struct{}{}[unsafe.Offsetof(lineInfo{}.Flags)-72]
	_ = [1]struct{}{}[unsafe.Offsetof(lineInfo{}.Attrs)-80]
	_ = [1]struct{}{}[unsafe.Offsetof(lineConfig{}.Attrs)-32]
	_ = [1]struct{}{}[unsafe.Offsetof(lineRequest{}.Config)-288]
	_ = [1]struct{}{}[unsafe.Offsetof(lineRequest{}.NumLines)-560]
	_ = [1]struct{}{}[unsafe.Offsetof(lineRequest{}.Fd)-588]
	_ = [1]struct{}{}[unsafe.Offsetof(lineEvent{}.ID)-8]
)

// ioctl request encoding, from include/uapi/asm-generic/ioctl.h.
const (
	iocNRShift   = 0
	iocTypeShift = 8
	iocSizeShift = 16
	iocDirShift  = 30

	iocDirWrite = 1
	iocDirRead  = 2

	iocTypeGPIO = 0xB4
)

// ioctl numbers (nr) within the GPIO type.
const (
	nrChipInfo    = 0x01
	nrV2LineInfo  = 0x05
	nrV2GetLine   = 0x07
	nrV2SetConfig = 0x0D
	nrV2GetValues = 0x0E
	nrV2SetValues = 0x0F
)

func ioc(dir, nr, size uintptr) uintptr {
	return dir<<iocDirShift | size<<iocSizeShift | iocTypeGPIO<<iocTypeShift | nr<<iocNRShift
}

func ior(nr, size uintptr) uintptr  { return ioc(iocDirRead, nr, size) }
func iowr(nr, size uintptr) uintptr { return ioc(iocDirRead|iocDirWrite, nr, size) }

// The four ioctls this package issues.
var (
	ioctlChipInfo  = ior(nrChipInfo, sizeofChipInfo)
	ioctlLineInfo  = iowr(nrV2LineInfo, sizeofLineInfo)
	ioctlGetLine   = iowr(nrV2GetLine, sizeofLineRequest)
	ioctlSetConfig = iowr(nrV2SetConfig, sizeofLineConfig)
	ioctlGetValues = iowr(nrV2GetValues, sizeofLineValues)
	ioctlSetValues = iowr(nrV2SetValues, sizeofLineValues)
)

// cstring converts a NUL-padded kernel char array to a Go string.
func cstring(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
