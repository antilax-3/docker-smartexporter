package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "smartexporter.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestLoadWritesTheDefaultWhenMissing(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "smartexporter.json")

	config, warning, err := Load(path)
	if err != nil || !strings.Contains(warning, "copied the default config") {
		t.Fatalf("got %q, %v, want a warning that the default was copied", warning, err)
	}

	if !reflect.DeepEqual(config, Default()) {
		t.Fatalf("got %+v, want the default config", config)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(data, defaultJSON) {
		t.Fatalf("wrote %s, want the default config", data)
	}

	reloaded, warning, err := Load(path)
	if err != nil || warning != "" || !reflect.DeepEqual(reloaded, config) {
		t.Fatalf("reloaded %+v, %q, %v, want the default config and no warning", reloaded, warning, err)
	}
}

func TestDefault(t *testing.T) {
	t.Parallel()

	config := Default()

	if config.Interval() != 10*time.Second || config.ListenPort() != 9120 {
		t.Fatalf("got interval %s and port %d, want 10s and 9120", config.Interval(), config.ListenPort())
	}

	names := make([]string, 0, len(config.Attributes))
	for _, attribute := range config.Attributes {
		names = append(names, attribute.MetricName())
	}

	want := []string{
		"smartexporter_temperature", "smartexporter_airflow_temperature",
		"smartexporter_lbas_written", "smartexporter_lbas_read",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("got metrics %v, want %v", names, want)
	}
}

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		want     Config
		warning  string
	}{
		{
			name: "NamesAreNormalised",
			contents: `{"attributes":[{"attributeName":" Power On  Hours ","name":"Power Hours","help":"Hours",` +
				`"labelNames":["Serial Number","Device"]}],"scrapeInterval":30,"port":9000}`,
			want: Config{
				Attributes: []Attribute{{
					AttributeName: "power_on_hours",
					Name:          "Power_Hours",
					Help:          "Hours",
					LabelNames:    []string{"serial_number", "device"},
				}},
				ScrapeInterval: 30,
				Port:           9000,
			},
		},
		{
			name:     "AttributeIDIsClamped",
			contents: `{"attributes":[{"attributeID":300,"name":"x","help":"x","labelNames":[]}]}`,
			want:     Config{Attributes: []Attribute{{AttributeID: 255, Name: "x", Help: "x", LabelNames: []string{}}}},
		},
		{
			name:     "AttributesAreRequired",
			contents: `{"scrapeInterval":30}`,
			want:     Default(),
			warning:  "Missing required field 'attributes'",
		},
		{
			name:     "InvalidJSON",
			contents: `{`,
			want:     Default(),
			warning:  "please check JSON validity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := write(t, tt.contents)

			config, warning, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(warning, tt.warning) || (tt.warning == "") != (warning == "") {
				t.Fatalf("got warning %q, want one containing %q", warning, tt.warning)
			}

			if !reflect.DeepEqual(config, tt.want) {
				t.Fatalf("got %+v, want %+v", config, tt.want)
			}

			// A config that can't be used is left for the user to fix rather than overwritten.
			if data, err := os.ReadFile(path); err != nil || string(data) != tt.contents {
				t.Fatalf("config file is now %q, %v, want it unchanged", data, err)
			}
		})
	}
}

func TestIntervalAndPortDefaultWhenUnset(t *testing.T) {
	t.Parallel()

	var config Config

	if config.Interval() != DefaultScrapeInterval || config.ListenPort() != DefaultPort {
		t.Fatalf("got interval %s and port %d", config.Interval(), config.ListenPort())
	}
}
