// Package config loads the smartexporter.json file from the /config volume.
package config

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"
	"time"
)

// DefaultPort is the port the exporter listens on when the config sets none.
const DefaultPort = 9120

// DefaultScrapeInterval is how often the disks are scraped when the config sets no interval. The default config sets
// its own, shorter interval, so this applies only to a config that leaves the field out.
const DefaultScrapeInterval = 15 * time.Second

// MetricPrefix is prepended to every configured metric name.
const MetricPrefix = "smartexporter_"

//go:embed default.json
var defaultJSON []byte

// Config is the smartexporter.json file.
type Config struct {
	// Attributes are the SMART attributes reported to Prometheus, one gauge each.
	Attributes []Attribute `json:"attributes"`
	// ScrapeInterval is the number of seconds between scrapes of the disks.
	ScrapeInterval float64 `json:"scrapeInterval"`
	// Port is the port the metrics are served on.
	Port int `json:"port"`
}

// Attribute is a SMART attribute reported as a gauge.
type Attribute struct {
	// AttributeName is the attribute as smartctl names it, e.g. Temperature_Celsius. Either it or AttributeID is set.
	AttributeName string `json:"attributeName,omitempty"`
	// AttributeID is the attribute's ATA identifier, e.g. 194.
	AttributeID int `json:"attributeID,omitempty"`
	// Name is the metric's name, prefixed with MetricPrefix.
	Name string `json:"name"`
	// Help is the metric's help text.
	Help string `json:"help"`
	// LabelNames are fields of smartctl's information section, e.g. "Serial Number", or "Device" for the device path.
	LabelNames []string `json:"labelNames"`
}

// Interval is the time between scrapes.
func (c Config) Interval() time.Duration {
	if c.ScrapeInterval <= 0 {
		return DefaultScrapeInterval
	}

	return time.Duration(c.ScrapeInterval * float64(time.Second))
}

// ListenPort is the port the metrics are served on.
func (c Config) ListenPort() int {
	if c.Port == 0 {
		return DefaultPort
	}

	return c.Port
}

// MetricName is the name the attribute is reported to Prometheus under.
func (a Attribute) MetricName() string {
	return MetricPrefix + strings.ToLower(a.Name)
}

// Default returns the config used when none exists or the existing one can't be used.
func Default() Config {
	var config Config
	if err := json.Unmarshal(defaultJSON, &config); err != nil {
		panic(fmt.Sprintf("default config is invalid: %v", err))
	}

	return config.sanitize()
}

// Load reads the config at path. When there is none, the default config is written there and returned. When the
// existing config can't be parsed or has no attributes, the default config is returned and the file is left for the
// user to fix. Either way the returned warning says why the default is in use, and err is set only when the default
// could not be written.
func Load(path string) (config Config, warning string, err error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the fixed config location, never request input.

	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := os.WriteFile(path, defaultJSON, 0o644); err != nil { //nolint:gosec // the config holds no secrets.
			return Default(), "", fmt.Errorf("unable to write default configuration file %s: %w", path, err)
		}

		return Default(), "Unable to find configuration file, copied the default config to " + path + ".", nil
	case err != nil:
		return Default(), fmt.Sprintf("Unable to read configuration file %s, using defaults for now: %v", path, err), nil
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return Default(), fmt.Sprintf("Unable to parse configuration file, please check JSON validity. Using defaults "+
			"for now. Error: %v", err), nil
	}

	if len(config.Attributes) == 0 {
		return Default(), "Missing required field 'attributes' from the configuration file. Using defaults for now.", nil
	}

	return config.sanitize(), "", nil
}

var spaces = regexp.MustCompile(` +`)

// sanitize normalises the attributes the way smartctl's output is normalised, so the two compare equal: attribute and
// label names are lowercased with runs of spaces replaced by an underscore, and attribute IDs are clamped to the ATA
// range.
func (c Config) sanitize() Config {
	attributes := make([]Attribute, 0, len(c.Attributes))

	for _, attribute := range c.Attributes {
		attribute.AttributeName = strings.ToLower(spaces.ReplaceAllString(strings.TrimSpace(attribute.AttributeName), "_"))
		attribute.AttributeID = max(min(attribute.AttributeID, 255), 0)
		attribute.Name = spaces.ReplaceAllString(strings.TrimSpace(attribute.Name), "_")

		labelNames := make([]string, 0, len(attribute.LabelNames))
		for _, labelName := range attribute.LabelNames {
			labelNames = append(labelNames, strings.ToLower(spaces.ReplaceAllString(labelName, "_")))
		}

		attribute.LabelNames = labelNames
		attributes = append(attributes, attribute)
	}

	c.Attributes = attributes

	return c
}
