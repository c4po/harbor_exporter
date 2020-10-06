package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

func (h *HarborExporter) collectScanMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()

	type scanMetric struct {
		Total     float64
		Completed float64
		metrics   []interface{}
		Requester string
		Ongoing   bool
	}
	body, _ := h.request("/scans/all/metrics")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	scan_requester, _ := strconv.ParseFloat(data.Requester, 64)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_requester"].Desc, allMetrics["scans_requester"].Type, float64(scan_requester),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_total"].Desc, allMetrics["scans_total"].Type, float64(data.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_completed"].Desc, allMetrics["scans_completed"].Type, float64(data.Completed),
	)

	reportLatency(start, "scans_latency", ch)
	return true
}
