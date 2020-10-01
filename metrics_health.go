package main

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *HarborExporter) collectHealthMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()
	type scanMetric struct {
		Status     string `json:"status"`
		Components []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
	}
	body, _ := e.request("/health")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		allMetrics["health"].Desc, allMetrics["health"].Type, float64(Status2i(data.Status)),
	)

	for _, c := range data.Components {
		ch <- prometheus.MustNewConstMetric(
			allMetrics["components_health"].Desc, allMetrics["components_health"].Type, float64(Status2i(c.Status)), c.Name,
		)
	}

	reportLatency(start, "health_latency", ch)
	return true
}
