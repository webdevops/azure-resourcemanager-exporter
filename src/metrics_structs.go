package main

import "github.com/prometheus/client_golang/prometheus"

type prometheusMetricRow struct {
	labels prometheus.Labels
	value float64
}

type prometheusMetricsList struct {
	list []prometheusMetricRow
}

func (m *prometheusMetricsList) Add(labels prometheus.Labels, value float64) {
	m.list = append(m.list, prometheusMetricRow{labels:labels, value:value})
}

func (m *prometheusMetricsList) GaugeSet(gauge *prometheus.GaugeVec) {
	for _, metric := range m.list {
		gauge.With(metric.labels).Set(metric.value)
	}
}
