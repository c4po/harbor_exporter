package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectScanMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()

	type scanMetric struct {
		Total     float64 `json:"total"`
		Completed float64 `json:"completed"`
		Metrics   struct {
		  Error   float64 `json:"Error"`
		  Success float64 `json:"Success"`
		} `json:"metrics"`
		Requester string `json:"requester"`
		Ongoing   bool   `json:"ongoing"`
	}
       

	body, _ := h.request("/scans/all/metrics")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	scanRequester, _ := strconv.ParseFloat(data.Requester, 64)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_requester"].Desc, allMetrics["scans_requester"].Type, float64(scanRequester),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_total"].Desc, allMetrics["scans_total"].Type, float64(data.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_error"].Desc, allMetrics["scans_error"].Type, float64(data.Metrics.Error),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_success"].Desc, allMetrics["scans_success"].Type, float64(data.Metrics.Success),
	)


	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_completed"].Desc, allMetrics["scans_completed"].Type, float64(data.Completed),
	)

	reportLatency(start, "scans_latency", ch)
	return true
}
