package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

func (e *HarborExporter) collectStatisticsMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()

	type statisticsMetric struct {
		Total_project_count   float64
		Public_project_count  float64
		Private_project_count float64
		Public_repo_count     float64
		Total_repo_count      float64
		Private_repo_count    float64
	}

	body, _ := e.request("/statistics")

	var data statisticsMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.Total_project_count, "total_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.Public_project_count, "public_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.Private_project_count, "private_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.Public_repo_count, "public_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.Total_repo_count, "total_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.Private_repo_count, "private_repo",
	)

	reportLatency(start, "statistics_latency", ch)
	return true
}
