package main

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type HealthCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateHealthCollector(e *HarborExporter) *HealthCollector {
	hc := HealthCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	hc.metrics["health"] = newMetricInfo(e.instance, "health", "Harbor overall health status: Healthy = 1, Unhealthy = 0", prometheus.GaugeValue, nil, nil)
	hc.metrics["components_health"] = newMetricInfo(e.instance, "components_health", "Harbor components health status: Healthy = 1, Unhealthy = 0", prometheus.GaugeValue, componentLabelNames, nil)
	return &hc
}

func (hc *HealthCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range hc.metrics {
		ch <- m.Desc
	}
}

func (hc *HealthCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	type scanMetric struct {
		Status     string `json:"status"`
		Components []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		}
	}
	body, _ := hc.exporter.request("/health")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(hc.exporter.logger).Log(err.Error())
		return
	}

	ch <- prometheus.MustNewConstMetric(
		hc.metrics["health"].Desc, hc.metrics["health"].Type, float64(Status2i(data.Status)),
	)

	for _, c := range data.Components {
		ch <- prometheus.MustNewConstMetric(
			hc.metrics["components_health"].Desc, hc.metrics["components_health"].Type, float64(Status2i(c.Status)), c.Name,
		)
	}
	reportLatency(start, "health_latency", ch)
}
