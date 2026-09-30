// Package smartctl runs smartctl and parses its text output.
//
// The text output is parsed rather than smartctl's JSON because the config names attributes and labels the way the
// text output does: "Temperature_Celsius", "Serial Number" or "Current Drive Temperature". Their JSON counterparts are
// named differently, and switching would break every existing config.
package smartctl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Exit status bits smartctl reports that mean it produced no output worth reading. The other bits report on the disk's
// health, e.g. an attribute past its threshold or errors in its log, and the output is complete when they are set.
const (
	exitCommandLine = 1 << 0
	exitDeviceOpen  = 1 << 1
)

// Runner runs smartctl with the given arguments and returns its standard output.
type Runner func(ctx context.Context, args ...string) (string, error)

// Exec runs the smartctl on the PATH.
func Exec(ctx context.Context, args ...string) (string, error) {
	//nolint:gosec // the arguments are fixed flags and the device paths smartctl's own scan returned.
	out, err := exec.CommandContext(ctx, "smartctl", args...).Output()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode()&(exitCommandLine|exitDeviceOpen) == 0 {
		return string(out), nil
	}

	if err != nil {
		return string(out), fmt.Errorf("smartctl %s: %w: %s", strings.Join(args, " "), err, lastLine(string(out)))
	}

	return string(out), nil
}

// Attribute is a SMART attribute's raw value.
type Attribute struct {
	// Name is the attribute's name, lowercased with spaces replaced by underscores.
	Name string
	// ID is the attribute's ATA identifier, or -1 for a SAS drive, whose attributes have none.
	ID int
	// Raw is the attribute's raw value.
	Raw float64
}

// Device is a disk and what smartctl reports about it.
type Device struct {
	// Info is the information section, keyed by field name lowercased with spaces replaced by underscores, plus the
	// device path under "device".
	Info map[string]string
	// Attributes are the SMART attributes with a numeric raw value.
	Attributes []Attribute
}

// Client queries disks through smartctl.
type Client struct {
	Run Runner
}

var sdDevice = regexp.MustCompile(`/dev/s.+`)

// Scan lists the SCSI and SATA disks smartctl can open, those named /dev/s*.
func (c *Client) Scan(ctx context.Context) ([]string, error) {
	out, err := c.Run(ctx, "--scan-open")

	return ParseScan(out), err
}

// Device queries a disk's information section and its attributes.
func (c *Client) Device(ctx context.Context, path string) (Device, error) {
	out, err := c.Run(ctx, "-i", path)
	if err != nil {
		return Device{}, err
	}

	info := ParseInfo(out)
	if _, ok := info["device"]; !ok {
		info["device"] = path
	}

	out, err = c.Run(ctx, "-A", "-f", "brief", path)
	if err != nil {
		return Device{}, err
	}

	// smartctl names a SAS drive's transport protocol and no ATA drive's, and reports the two kinds of attributes in
	// different shapes.
	if _, ok := info["transport_protocol"]; ok {
		return Device{Info: info, Attributes: ParseSASAttributes(out)}, nil
	}

	return Device{Info: info, Attributes: ParseATAAttributes(out)}, nil
}

// ParseScan returns the /dev/s* devices from smartctl --scan-open output, which lists one device per line followed by
// its options, and comments out the devices it could not open.
func ParseScan(out string) []string {
	var devices []string

	for line := range lines(out) {
		device, _, _ := strings.Cut(line, " ")
		if sdDevice.MatchString(device) {
			devices = append(devices, device)
		}
	}

	return devices
}

// ParseInfo parses smartctl -i output, a "Field Name: value" pair per line.
func ParseInfo(out string) map[string]string {
	info := map[string]string{}

	for line := range lines(out) {
		field, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}

		info[normalise(field)] = strings.TrimSpace(value)
	}

	return info
}

// ataAttribute matches a row of smartctl -A -f brief output for an ATA drive:
//
//	ID# ATTRIBUTE_NAME          FLAGS    VALUE WORST THRESH FAIL RAW_VALUE
//	194 Temperature_Celsius     -O---K   119   103   000    -    31 (Min/Max 20/45)
var ataAttribute = regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+\S+\s+\S+\s+\S+\s+\S+\s+\S+\s+(\S+)`)

// hours matches a raw value in hours, minutes and seconds, e.g. 59342h+32m+09.624s.
var hours = regexp.MustCompile(`^(\d+)h`)

// ParseATAAttributes parses smartctl -A -f brief output for an ATA drive. Only the first word of a raw value is read,
// so a temperature of "31 (Min/Max 20/45)" reads as 31, and a duration of "59342h+32m+09.624s" reads as its hours.
// Attributes whose raw value is not a number are left out.
func ParseATAAttributes(out string) []Attribute {
	var attributes []Attribute

	for line := range lines(out) {
		match := ataAttribute.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		id, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}

		raw, ok := parseNumber(match[3])
		if !ok {
			if h := hours.FindStringSubmatch(match[3]); h != nil {
				raw, ok = parseNumber(h[1])
			}
		}

		if !ok {
			continue
		}

		attributes = append(attributes, Attribute{Name: strings.ToLower(match[2]), ID: id, Raw: raw})
	}

	return attributes
}

// ParseSASAttributes parses smartctl -A -f brief output for a SAS drive, a "Description: value" pair per line, e.g.
// "Current Drive Temperature:     31 C". Only the first word of a value is read, and lines whose value is not a number
// are left out.
func ParseSASAttributes(out string) []Attribute {
	var attributes []Attribute

	for line := range lines(out) {
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		first, _, _ := strings.Cut(strings.TrimSpace(value), " ")

		raw, ok := parseNumber(first)
		if !ok {
			continue
		}

		attributes = append(attributes, Attribute{Name: normalise(name), ID: -1, Raw: raw})
	}

	return attributes
}

var spaces = regexp.MustCompile(` +`)

func normalise(field string) string {
	return strings.ToLower(spaces.ReplaceAllString(strings.TrimSpace(field), "_"))
}

// parseNumber reads a decimal or 0x-prefixed hexadecimal number, the forms smartctl prints raw values in.
func parseNumber(s string) (float64, bool) {
	if strings.HasPrefix(s, "0x") {
		n, err := strconv.ParseUint(s[2:], 16, 64)

		return float64(n), err == nil
	}

	n, err := strconv.ParseFloat(s, 64)

	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func lines(out string) func(func(string) bool) {
	return func(yield func(string) bool) {
		scanner := bufio.NewScanner(strings.NewReader(out))
		for scanner.Scan() {
			if !yield(scanner.Text()) {
				return
			}
		}
	}
}

func lastLine(out string) string {
	var last string

	for line := range lines(out) {
		if strings.TrimSpace(line) != "" {
			last = strings.TrimSpace(line)
		}
	}

	return last
}
