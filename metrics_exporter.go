package main

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
)

type MetricsCollectorExporter struct {
	CollectorProcessorCustom

	prometheus struct {
		stats *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorExporter) Setup(collector *CollectorCustom) {
	m.CollectorReference = collector

	m.prometheus.stats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_stats",
			Help: "Azure ResourceManager stats",
		},
		[]string{
			"name",
			"type",
		},
	)
	prometheus.MustRegister(m.prometheus.stats)
}

func (m *MetricsCollectorExporter) Collect(ctx context.Context, logger *log.Entry) {
	m.collectCollectorStats(ctx, logger)
}

func (m *MetricsCollectorExporter) collectCollectorStats(ctx context.Context, logger *log.Entry) {
	statsMetrics := prometheusCommon.NewMetricsList()

	for _, collector := range collectorGeneralList {
		if collector.LastScrapeDuration != nil {
			statsMetrics.AddDuration(prometheus.Labels{
				"name": collector.Name,
				"type": "collectorDuration",
			}, *collector.LastScrapeDuration)
		}
	}

	for _, collector := range collectorCustomList {
		if collector.LastScrapeDuration != nil {
			statsMetrics.AddDuration(prometheus.Labels{
				"name": collector.Name,
				"type": "collectorDuration",
			}, *collector.LastScrapeDuration)
		}
	}

	statsMetrics.GaugeSet(m.prometheus.stats)
}
