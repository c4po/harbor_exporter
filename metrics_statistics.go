package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *Exporter) collectStatisticsMetric(ch chan<- prometheus.Metric) bool {

	type statisticsMetric struct {
		Total_project_count   float64
		Public_project_count  float64
		Private_project_count float64
		Public_repo_count     float64
		Total_repo_count      float64
		Private_repo_count    float64
	}

	body := e.client.request("/api/v2.0/statistics")

	var data statisticsMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	ch <- prometheus.MustNewConstMetric(
		projectCount, prometheus.GaugeValue, data.Total_project_count, "total_project",
	)

	ch <- prometheus.MustNewConstMetric(
		projectCount, prometheus.GaugeValue, data.Public_project_count, "public_project",
	)

	ch <- prometheus.MustNewConstMetric(
		projectCount, prometheus.GaugeValue, data.Private_project_count, "private_project",
	)

	ch <- prometheus.MustNewConstMetric(
		repoCount, prometheus.GaugeValue, data.Public_repo_count, "public_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		repoCount, prometheus.GaugeValue, data.Total_repo_count, "total_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		repoCount, prometheus.GaugeValue, data.Private_repo_count, "private_repo",
	)

	return true
}
