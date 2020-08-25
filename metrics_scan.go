package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

func (e *HarborExporter) collectScanMetric(ch chan<- prometheus.Metric) bool {

	type scanMetric struct {
		Total     float64
		Completed float64
		metrics   []interface{}
		Requester string
		Ongoing   bool
	}
	body, _ := e.request("/scans/all/metrics")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	scan_requester, _ := strconv.ParseFloat(data.Requester, 64)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_requester"].Desc, prometheus.GaugeValue, float64(scan_requester),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_total"].Desc, prometheus.GaugeValue, float64(data.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["scans_completed"].Desc, prometheus.GaugeValue, float64(data.Completed),
	)
	return true
}
