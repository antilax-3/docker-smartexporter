package smartctl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func TestParseScanKeepsOnlyOpenedSDDevices(t *testing.T) {
	t.Parallel()

	got := ParseScan(fixture(t, "scan.txt"))
	if want := []string{"/dev/sda", "/dev/sdb"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		want    map[string]string
	}{
		{
			name:    "ATA",
			fixture: "ata-info.txt",
			want: map[string]string{
				"device_model":     "WDC WD40EFRX-68N32N0",
				"serial_number":    "WD-WCC7K1234567",
				"user_capacity":    "4,000,787,030,016 bytes [4.00 TB]",
				"local_time_is":    "Wed Sep 30 11:00:00 2026 AEST",
				"smart_support_is": "Enabled",
			},
		},
		{
			name:    "SAS",
			fixture: "sas-info.txt",
			want: map[string]string{
				"product":            "ST4000NM0023",
				"serial_number":      "Z1Z12345",
				"transport_protocol": "SAS (SPL-4)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			info := ParseInfo(fixture(t, tt.fixture))

			for field, want := range tt.want {
				if got := info[field]; got != want {
					t.Errorf("%s is %q, want %q", field, got, want)
				}
			}

			for field := range info {
				if field == "" || strings.ContainsAny(field, " =") {
					t.Errorf("unexpected field %q from a line that is not a field", field)
				}
			}
		})
	}
}

func TestParseATAAttributes(t *testing.T) {
	t.Parallel()

	got := ParseATAAttributes(fixture(t, "ata-attributes.txt"))
	want := []Attribute{
		{Name: "raw_read_error_rate", ID: 1, Raw: 0},
		{Name: "spin_up_time", ID: 3, Raw: 6083},
		{Name: "power_on_hours", ID: 9, Raw: 59342},
		{Name: "power_cycle_count", ID: 12, Raw: 47},
		{Name: "airflow_temperature_cel", ID: 190, Raw: 31},
		{Name: "temperature_celsius", ID: 194, Raw: 33},
		{Name: "head_flying_hours", ID: 240, Raw: 41234},
		{Name: "total_lbas_written", ID: 241, Raw: 54321098765},
		{Name: "total_lbas_read", ID: 242, Raw: 12345678901},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseSASAttributes(t *testing.T) {
	t.Parallel()

	got := ParseSASAttributes(fixture(t, "sas-attributes.txt"))
	want := []Attribute{
		{Name: "current_drive_temperature", ID: -1, Raw: 31},
		{Name: "drive_trip_temperature", ID: -1, Raw: 60},
		{Name: "specified_cycle_count_over_device_lifetime", ID: -1, Raw: 10000},
		{Name: "accumulated_start-stop_cycles", ID: -1, Raw: 52},
		{Name: "elements_in_grown_defect_list", ID: -1, Raw: 0},
	}

	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// fakeRunner answers smartctl invocations from fixtures, keyed by their arguments.
type fakeRunner map[string]string

func (f fakeRunner) run(_ context.Context, args ...string) (string, error) {
	out, ok := f[strings.Join(args, " ")]
	if !ok {
		return "", errors.New("smartctl " + strings.Join(args, " ") + ": no such device")
	}

	return out, nil
}

func TestDevice(t *testing.T) {
	t.Parallel()

	client := &Client{Run: fakeRunner{
		"-i /dev/sda":          fixture(t, "ata-info.txt"),
		"-A -f brief /dev/sda": fixture(t, "ata-attributes.txt"),
		"-i /dev/sdb":          fixture(t, "sas-info.txt"),
		"-A -f brief /dev/sdb": fixture(t, "sas-attributes.txt"),
		"-i /dev/sdc":          fixture(t, "ata-info.txt"),
	}.run}

	tests := []struct {
		name      string
		path      string
		attribute Attribute
		err       string
	}{
		{name: "ATADrive", path: "/dev/sda", attribute: Attribute{Name: "temperature_celsius", ID: 194, Raw: 33}},
		{name: "SASDrive", path: "/dev/sdb", attribute: Attribute{Name: "current_drive_temperature", ID: -1, Raw: 31}},
		{name: "AttributesFail", path: "/dev/sdc", err: "no such device"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			device, err := client.Device(context.Background(), tt.path)

			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("got %v, want an error containing %q", err, tt.err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if device.Info["device"] != tt.path {
				t.Errorf("device is %q, want %q", device.Info["device"], tt.path)
			}

			if !slices.Contains(device.Attributes, tt.attribute) {
				t.Errorf("attributes %+v do not contain %+v", device.Attributes, tt.attribute)
			}
		})
	}
}

func TestScanReturnsTheDevicesAlongsideAnError(t *testing.T) {
	t.Parallel()

	client := &Client{Run: func(_ context.Context, _ ...string) (string, error) {
		return "/dev/sda -d sat # /dev/sda [SAT], ATA device\n", errors.New("smartctl --scan-open: exit status 4")
	}}

	devices, err := client.Scan(context.Background())
	if err == nil || !reflect.DeepEqual(devices, []string{"/dev/sda"}) {
		t.Fatalf("got %v, %v, want /dev/sda and the error", devices, err)
	}
}

// Exec finds smartctl on the PATH, which t.Setenv replaces for the whole process, so this test can't run in parallel.
func TestExecToleratesHealthStatusBits(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"$*\"\nexit ${SMARTCTL_EXIT}\n"

	if err := os.WriteFile(filepath.Join(dir, "smartctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		exit string
		fail bool
	}{
		{name: "Success", exit: "0"},
		{name: "ErrorLogHasErrors", exit: "64"},
		{name: "SomeCommandFailed", exit: "4"},
		{name: "DeviceOpenFailed", exit: "2", fail: true},
		{name: "CommandLineInvalid", exit: "1", fail: true},
	}

	path := os.Getenv("PATH")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", dir+string(os.PathListSeparator)+path)
			t.Setenv("SMARTCTL_EXIT", tt.exit)

			out, err := Exec(context.Background(), "-i", "/dev/sda")

			if tt.fail != (err != nil) {
				t.Fatalf("got %v for exit status %s, want failure %t", err, tt.exit, tt.fail)
			}

			if out != "-i /dev/sda\n" {
				t.Fatalf("got output %q", out)
			}
		})
	}
}
