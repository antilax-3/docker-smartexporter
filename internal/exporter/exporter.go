// Package exporter scrapes the disks' SMART attributes on an interval and serves them as Prometheus gauges.
package exporter

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/antilax-3/docker-smartexporter/internal/config"
	"github.com/antilax-3/docker-smartexporter/internal/smartctl"
)

// Disks queries the disks, which smartctl.Client does.
type Disks interface {
	Scan(ctx context.Context) ([]string, error)
	Device(ctx context.Context, path string) (smartctl.Device, error)
}

// Exporter is a prometheus.Collector reporting the configured attributes of every disk as of the last scrape.
type Exporter struct {
	disks   Disks
	metrics []metric
	logger  *log.Logger

	mu      sync.RWMutex
	samples []sample
}

type metric struct {
	attribute config.Attribute
	desc      *prometheus.Desc
}

type sample struct {
	metric *metric
	labels []string
	value  float64
}

// New returns an Exporter for the attributes, which fails if an attribute's metric or label names are not valid
// Prometheus names, or an attribute names the same label twice.
func New(disks Disks, attributes []config.Attribute, logger *log.Logger) (*Exporter, error) {
	e := &Exporter{disks: disks, logger: logger}

	for _, attribute := range attributes {
		desc := prometheus.NewDesc(attribute.MetricName(), attribute.Help, attribute.LabelNames, nil)
		if err := desc.Err(); err != nil {
			return nil, fmt.Errorf("attribute %q: %w", attribute.Name, err)
		}

		e.metrics = append(e.metrics, metric{attribute: attribute, desc: desc})
	}

	return e, nil
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	for i := range e.metrics {
		ch <- e.metrics[i].desc
	}
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, s := range e.samples {
		ch <- prometheus.MustNewConstMetric(s.metric.desc, prometheus.GaugeValue, s.value, s.labels...)
	}
}

// scrapeTimeout bounds a scrape, so a disk that stops answering can't stall every scrape after it.
const scrapeTimeout = time.Minute

// Run scrapes the disks immediately and then every interval, until ctx is done. A scrape that overruns the interval
// delays the next rather than overlapping it.
func (e *Exporter) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		scrapeCtx, cancel := context.WithTimeout(ctx, scrapeTimeout)
		e.Scrape(scrapeCtx)
		cancel()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Scrape queries every disk and replaces the reported samples with what it found. A disk that can't be queried is
// logged and left out, and the other disks are still reported.
func (e *Exporter) Scrape(ctx context.Context) {
	paths, err := e.disks.Scan(ctx)
	if err != nil {
		e.logger.Print(err)
	}

	devices := make([]smartctl.Device, len(paths))

	var wg sync.WaitGroup

	for i, path := range paths {
		wg.Go(func() {
			device, err := e.disks.Device(ctx, path)
			if err != nil {
				e.logger.Print(err)

				return
			}

			devices[i] = device
		})
	}

	wg.Wait()

	// Samples are keyed by metric and label values, so that disks whose configured labels are identical report once,
	// the last scraped winning, rather than colliding in the registry and failing the whole response.
	var samples []sample

	index := map[string]int{}

	for _, device := range devices {
		if device.Info == nil {
			continue
		}

		for i := range e.metrics {
			m := &e.metrics[i]

			value, ok := find(device.Attributes, m.attribute)
			if !ok {
				continue
			}

			labels := make([]string, len(m.attribute.LabelNames))
			for j, name := range m.attribute.LabelNames {
				labels[j] = device.Info[name]
			}

			key := m.attribute.MetricName() + "\xff" + strings.Join(labels, "\xff")
			if j, ok := index[key]; ok {
				samples[j].value = value

				continue
			}

			index[key] = len(samples)
			samples = append(samples, sample{metric: m, labels: labels, value: value})
		}
	}

	e.mu.Lock()
	e.samples = samples
	e.mu.Unlock()
}

// find returns the raw value of the first attribute matching the configured attribute's ID or name. An attribute
// configured with neither matches nothing.
func find(attributes []smartctl.Attribute, configured config.Attribute) (float64, bool) {
	if configured.AttributeID == 0 && configured.AttributeName == "" {
		return 0, false
	}

	for _, attribute := range attributes {
		if (configured.AttributeID != 0 && attribute.ID == configured.AttributeID) ||
			(configured.AttributeName != "" && attribute.Name == configured.AttributeName) {
			return attribute.Raw, true
		}
	}

	return 0, false
}

// Index answers on / with a pointer to the metrics.
func Index() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "Point Prometheus here for your HDD statistics")
	})
}
