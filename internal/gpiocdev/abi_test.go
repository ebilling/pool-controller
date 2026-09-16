package gpiocdev

import (
	"testing"
	"unsafe"
)

// The request numbers encode the struct sizes, so these literals are the
// strongest available check that the Go layouts match include/uapi/linux/gpio.h.
// A wrong size produces a different number and the kernel answers ENOTTY.
func TestIoctlNumbersMatchKernel(t *testing.T) {
	tests := []struct {
		name string
		got  uintptr
		want uint64
	}{
		{"GPIO_GET_CHIPINFO_IOCTL", ioctlChipInfo, 0x8044b401},
		{"GPIO_V2_GET_LINEINFO_IOCTL", ioctlLineInfo, 0xc100b405},
		{"GPIO_V2_GET_LINE_IOCTL", ioctlGetLine, 0xc250b407},
		{"GPIO_V2_LINE_SET_CONFIG_IOCTL", ioctlSetConfig, 0xc110b40d},
		{"GPIO_V2_LINE_GET_VALUES_IOCTL", ioctlGetValues, 0xc010b40e},
		{"GPIO_V2_LINE_SET_VALUES_IOCTL", ioctlSetValues, 0xc010b40f},
	}
	for _, tc := range tests {
		if uint64(tc.got) != tc.want {
			t.Errorf("%s = %#x, want %#x", tc.name, tc.got, tc.want)
		}
	}
}

// Sizes are also asserted at compile time in abi.go; this reports which struct
// is wrong instead of only failing the build with an out-of-range index.
func TestStructSizes(t *testing.T) {
	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"gpiochip_info", unsafe.Sizeof(chipInfo{}), sizeofChipInfo},
		{"gpio_v2_line_values", unsafe.Sizeof(lineValues{}), sizeofLineValues},
		{"gpio_v2_line_info", unsafe.Sizeof(lineInfo{}), sizeofLineInfo},
		{"gpio_v2_line_attribute", unsafe.Sizeof(lineAttribute{}), 16},
		{"gpio_v2_line_config_attribute", unsafe.Sizeof(lineConfigAttribute{}), 24},
		{"gpio_v2_line_config", unsafe.Sizeof(lineConfig{}), sizeofLineConfig},
		{"gpio_v2_line_request", unsafe.Sizeof(lineRequest{}), sizeofLineRequest},
		{"gpio_v2_line_event", unsafe.Sizeof(lineEvent{}), sizeofLineEvent},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("sizeof(%s) = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestFieldOffsets(t *testing.T) {
	tests := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"line_config.attrs", unsafe.Offsetof(lineConfig{}.Attrs), 32},
		{"line_request.config", unsafe.Offsetof(lineRequest{}.Config), 288},
		{"line_request.num_lines", unsafe.Offsetof(lineRequest{}.NumLines), 560},
		{"line_request.fd", unsafe.Offsetof(lineRequest{}.Fd), 588},
		{"line_event.id", unsafe.Offsetof(lineEvent{}.ID), 8},
		{"line_event.line_seqno", unsafe.Offsetof(lineEvent{}.LineSeqno), 20},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("offsetof(%s) = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// Flag values are a bit-shifted enum; a misplaced iota would silently request
// the wrong line mode.
func TestLineFlags(t *testing.T) {
	tests := []struct {
		name string
		got  uint64
		want uint64
	}{
		{"INPUT", flagInput, 4},
		{"OUTPUT", flagOutput, 8},
		{"EDGE_RISING", flagEdgeRising, 16},
		{"EDGE_FALLING", flagEdgeFalling, 32},
		{"BIAS_PULL_UP", flagBiasPullUp, 256},
		{"BIAS_PULL_DOWN", flagBiasPullDown, 512},
		{"BIAS_DISABLED", flagBiasDisabled, 1024},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("GPIO_V2_LINE_FLAG_%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestCstring(t *testing.T) {
	tests := []struct {
		in   []byte
		want string
	}{
		{[]byte("gpiochip0\x00\x00\x00"), "gpiochip0"},
		{[]byte("pinctrl-bcm2835\x00"), "pinctrl-bcm2835"},
		{[]byte("\x00padding"), ""},
		{[]byte("nonul"), "nonul"},
	}
	for _, tc := range tests {
		if got := cstring(tc.in); got != tc.want {
			t.Errorf("cstring(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
