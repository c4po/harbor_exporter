package main

import (
	"encoding/json"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectStatisticsMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()

	type statisticsMetric struct {
		TotalProjectCount   float64 `json:"total_project_count"`
		PublicProjectCount  float64 `json:"public_project_count"`
		PrivateProjectCount float64 `json:"private_project_count"`
		PublicRepoCount     float64 `json:"public_repo_count"`
		TotalRepoCount      float64 `json:"total_repo_count"`
		PrivateRepoCount    float64 `json:"private_repo_count"`
	}

	body, _ := h.request("/statistics")

	var data statisticsMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.TotalProjectCount, "total_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.PublicProjectCount, "public_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["project_count_total"].Desc, allMetrics["project_count_total"].Type, data.PrivateProjectCount, "private_project",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.PublicRepoCount, "public_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.TotalRepoCount, "total_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		allMetrics["repo_count_total"].Desc, allMetrics["repo_count_total"].Type, data.PrivateRepoCount, "private_repo",
	)

	reportLatency(start, "statistics_latency", ch)
	return true
}
