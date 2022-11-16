package main

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectSystemVolumesMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()
	type systemVolumesMetric struct {
		Storage []struct {
			Total float64
			Free  float64
		}
	}
	body, _ := h.request("/systeminfo/volumes")
	var data systemVolumesMetric
	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	if len(data.Storage) < 1 {
		level.Error(h.logger).Log("msg", "Error retrieving system volumes")
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_volumes_bytes"].Desc, allMetrics["system_volumes_bytes"].Type, data.Storage[0].Total, "total",
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_volumes_bytes"].Desc, allMetrics["system_volumes_bytes"].Type, data.Storage[0].Free, "free",
	)

	reportLatency(start, "system_volumes_latency", ch)
	return true
}
