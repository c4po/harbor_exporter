package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type StatsCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateStatsCollector(e *HarborExporter) *StatsCollector {
	sc := StatsCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	sc.metrics["repo_count_total"] = newMetricInfo(e.instance, "repo_count_total", "repositories number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	sc.metrics["project_count_total"] = newMetricInfo(e.instance, "project_count_total", "projects number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	return &sc
}

func (sc *StatsCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range sc.metrics {
		ch <- m.Desc
	}
}

func (sc *StatsCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

	type statisticsMetric struct {
		Total_project_count   float64
		Public_project_count  float64
		Private_project_count float64
		Public_repo_count     float64
		Total_repo_count      float64
		Private_repo_count    float64
	}

	body, _ := sc.exporter.request("/statistics")

	var data statisticsMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(sc.exporter.logger).Log(err.Error())
		sc.exporter.statsChan <- false
		return
	}

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["project_count_total"].Desc, sc.metrics["project_count_total"].Type, data.Total_project_count, "total_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["project_count_total"].Desc, sc.metrics["project_count_total"].Type, data.Public_project_count, "public_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["project_count_total"].Desc, sc.metrics["project_count_total"].Type, data.Private_project_count, "private_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["repo_count_total"].Desc, sc.metrics["repo_count_total"].Type, data.Public_repo_count, "public_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["repo_count_total"].Desc, sc.metrics["repo_count_total"].Type, data.Total_repo_count, "total_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.metrics["repo_count_total"].Desc, sc.metrics["repo_count_total"].Type, data.Private_repo_count, "private_repo",
	)
	reportLatency(start, "statistics_latency", ch)
	sc.exporter.statsChan <- true
}
