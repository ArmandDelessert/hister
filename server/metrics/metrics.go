// SPDX-License-Identifier: AGPL-3.0-or-later

// Package metrics provides Prometheus instrumentation for Hister.
// All collectors are registered on a dedicated registry so that the
// default process/Go runtime metrics are included only when the
// /metrics endpoint is enabled.
package metrics

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"

	"net/http"
)

// Collector
var (
	QueriesTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hister",
		Name:      "queries_total",
		Help:      "Total number of search queries executed.",
	}, []string{"result"})

	SearchDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "hister",
		Name:      "search_duration_seconds",
		Help:      "Histogram of search query latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	DocumentsIndexedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "hister",
		Name:      "documents_indexed_total",
		Help:      "Total number of documents indexed.",
	}, []string{"type"})

	IndexingDuration = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "hister",
		Name:      "indexing_duration_seconds",
		Help:      "Histogram of single-document indexing latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	})

	DatastoreSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "hister",
		Name:      "datastore_size_bytes",
		Help:      "Total size of the data directory in bytes.",
	})

	IndexDocumentCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "hister",
		Name:      "index_document_count",
		Help:      "Number of documents currently in the search index.",
	})
)

var registry *prometheus.Registry

type GaugeSource interface {
	Total() uint64
	DataDir() string
}

var (
	initOnce    sync.Once
	gaugeSource GaugeSource
	stopTicker  chan struct{}
)

// Init creates the custom Prometheus registry, registers all collectors,
// and starts the background ticker that refreshes gauge values.
func Init(src GaugeSource) {
	initOnce.Do(func() {
		registry = prometheus.NewRegistry()

		registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
		registry.MustRegister(collectors.NewGoCollector())

		registry.MustRegister(
			QueriesTotal,
			SearchDuration,
			DocumentsIndexedTotal,
			IndexingDuration,
			DatastoreSizeBytes,
			IndexDocumentCount,
		)

		gaugeSource = src
		stopTicker = make(chan struct{})
		go runTicker()
	})
}

func Handler() http.Handler {
	if registry == nil {
		return promhttp.Handler()
	}
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

func Stop() {
	if stopTicker != nil {
		close(stopTicker)
	}
}

// Background ticker

const gaugeRefreshInterval = 30 * time.Second

func runTicker() {
	refreshGauges()

	ticker := time.NewTicker(gaugeRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			refreshGauges()
		case <-stopTicker:
			return
		}
	}
}

func refreshGauges() {
	if gaugeSource == nil {
		return
	}
	IndexDocumentCount.Set(float64(gaugeSource.Total()))

	dir := gaugeSource.DataDir()
	if dir != "" {
		size, err := dirSize(dir)
		if err != nil {
			log.Debug().Err(err).Msg("metrics: failed to compute datastore size")
		} else {
			DatastoreSizeBytes.Set(float64(size))
		}
	}
}

func dirSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
