package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

type ScanCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateScanCollector(e *HarborExporter) *ScanCollector {
	sc := ScanCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	sc.metrics["scans_total"] = newMetricInfo(e.instance, "scans_total", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	sc.metrics["scans_completed"] = newMetricInfo(e.instance, "scans_completed", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	sc.metrics["scans_requester"] = newMetricInfo(e.instance, "scans_requester", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	return &sc
}

func (sc *ScanCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range sc.metrics {
		ch <- m.Desc
	}
}

func (sc *ScanCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	type scanMetric struct {
		Total     float64
		Completed float64
		metrics   []interface{}
		Requester string
		Ongoing   bool
	}
	body, _ := sc.exporter.request("/scans/all/metrics")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(sc.exporter.logger).Log(err.Error())
		sc.exporter.scanChan <- false
		return
	}

	scan_requester, _ := strconv.ParseFloat(data.Requester, 64)
	ch <- prometheus.MustNewConstMetric(
		sc.metrics["scans_requester"].Desc, sc.metrics["scans_requester"].Type, float64(scan_requester),
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["scans_total"].Desc, sc.metrics["scans_total"].Type, float64(data.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["scans_completed"].Desc, sc.metrics["scans_completed"].Type, float64(data.Completed),
	)
	reportLatency(start, "scans_latency", ch)
	sc.exporter.scanChan <- true
}
