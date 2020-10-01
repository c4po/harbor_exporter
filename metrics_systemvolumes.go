package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

func (e *HarborExporter) collectSystemVolumesMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()
	type systemVolumesMetric struct {
		Storage struct {
			Total float64
			Free  float64
		}
	}
	body, _ := e.request("/systeminfo/volumes")
	var data systemVolumesMetric
	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_volumes_bytes"].Desc, allMetrics["system_volumes_bytes"].Type, data.Storage.Total, "total",
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_volumes_bytes"].Desc, allMetrics["system_volumes_bytes"].Type, data.Storage.Free, "free",
	)

	reportLatency(start, "system_volumes_latency", ch)
	return true
}
