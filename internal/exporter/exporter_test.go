package exporter

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/antilax-3/docker-smartexporter/internal/config"
	"github.com/antilax-3/docker-smartexporter/internal/smartctl"
)

type fakeDisks struct {
	paths   []string
	devices map[string]smartctl.Device
}

func (f *fakeDisks) Scan(_ context.Context) ([]string, error) {
	return f.paths, nil
}

func (f *fakeDisks) Device(_ context.Context, path string) (smartctl.Device, error) {
	device, ok := f.devices[path]
	if !ok {
		return smartctl.Device{}, errors.New(path + ": open failed")
	}

	return device, nil
}

var (
	ata = smartctl.Device{
		Info: map[string]string{"device": "/dev/sda", "device_model": "WDC WD40EFRX", "serial_number": "WD-1"},
		Attributes: []smartctl.Attribute{
			{Name: "airflow_temperature_cel", ID: 190, Raw: 31},
			{Name: "temperature_celsius", ID: 194, Raw: 33},
		},
	}
	sas = smartctl.Device{
		Info:       map[string]string{"device": "/dev/sdb", "product": "ST4000NM0023", "serial_number": "Z1"},
		Attributes: []smartctl.Attribute{{Name: "current_drive_temperature", ID: -1, Raw: 29}},
	}
)

func attributes() []config.Attribute {
	return []config.Attribute{
		{AttributeName: "temperature_celsius", Name: "temperature", Help: "Temperature", LabelNames: []string{"device", "serial_number"}},
		{AttributeID: 190, Name: "airflow_temperature", Help: "Airflow", LabelNames: []string{"device", "device_model"}},
		{AttributeName: "current_drive_temperature", Name: "sas_temperature", Help: "SAS", LabelNames: []string{"device"}},
		{Name: "unset", Help: "Neither an ID nor a name", LabelNames: []string{"device"}},
	}
}

func discard() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func TestScrapeReportsEveryDiskThatAnswers(t *testing.T) {
	t.Parallel()

	disks := &fakeDisks{
		paths:   []string{"/dev/sda", "/dev/sdb", "/dev/sdc"},
		devices: map[string]smartctl.Device{"/dev/sda": ata, "/dev/sdb": sas},
	}

	e, err := New(disks, attributes(), discard())
	if err != nil {
		t.Fatal(err)
	}

	e.Scrape(context.Background())

	// The model label is empty for the SAS drive, which smartctl reports under product.
	want := `
# HELP smartexporter_airflow_temperature Airflow
# TYPE smartexporter_airflow_temperature gauge
smartexporter_airflow_temperature{device="/dev/sda",device_model="WDC WD40EFRX"} 31
# HELP smartexporter_sas_temperature SAS
# TYPE smartexporter_sas_temperature gauge
smartexporter_sas_temperature{device="/dev/sdb"} 29
# HELP smartexporter_temperature Temperature
# TYPE smartexporter_temperature gauge
smartexporter_temperature{device="/dev/sda",serial_number="WD-1"} 33
`
	if err := testutil.CollectAndCompare(e, strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

func TestScrapeDropsDisksThatAreGone(t *testing.T) {
	t.Parallel()

	disks := &fakeDisks{paths: []string{"/dev/sda"}, devices: map[string]smartctl.Device{"/dev/sda": ata}}

	e, err := New(disks, attributes(), discard())
	if err != nil {
		t.Fatal(err)
	}

	e.Scrape(context.Background())

	if n := testutil.CollectAndCount(e); n != 2 {
		t.Fatalf("got %d samples, want 2", n)
	}

	disks.paths = nil
	e.Scrape(context.Background())

	if n := testutil.CollectAndCount(e); n != 0 {
		t.Fatalf("got %d samples after the disk was removed, want 0", n)
	}
}

func TestScrapeReportsDisksWithIdenticalLabelsOnce(t *testing.T) {
	t.Parallel()

	second := ata
	second.Info = map[string]string{"device": "/dev/sdc"}

	disks := &fakeDisks{
		paths:   []string{"/dev/sda", "/dev/sdc"},
		devices: map[string]smartctl.Device{"/dev/sda": ata, "/dev/sdc": second},
	}

	attribute := config.Attribute{AttributeName: "temperature_celsius", Name: "temperature", Help: "Temperature"}

	e, err := New(disks, []config.Attribute{attribute}, discard())
	if err != nil {
		t.Fatal(err)
	}

	e.Scrape(context.Background())

	if n := testutil.CollectAndCount(e); n != 1 {
		t.Fatalf("got %d samples, want 1", n)
	}
}

func TestNewRejectsDuplicateLabelNames(t *testing.T) {
	t.Parallel()

	// "Serial Number" and "serial number" both normalise to serial_number.
	attribute := config.Attribute{Name: "temperature", Help: "x", LabelNames: []string{"serial_number", "serial_number"}}

	if _, err := New(&fakeDisks{}, []config.Attribute{attribute}, discard()); err == nil {
		t.Fatal("got no error for duplicate label names")
	}
}

func TestIndex(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	Index().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody))

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Point Prometheus here") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}
