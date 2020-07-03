package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type StatisticsCollector struct {
	client *HarborClient
	logger log.Logger
	upChannel chan<- bool

	statsUp *prometheus.Desc
	projectCount *prometheus.Desc
	repoCount *prometheus.Desc
}

func NewStatisticsCollector(c *HarborClient, l log.Logger, u chan<- bool, instance string) *StatisticsCollector {
	return &StatisticsCollector{
		client: c,
		logger: l,
		upChannel: u,
		statsUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "statistics_up"),
			"Was the last query of harbor project and repo counts successful.",
			nil, nil,
		),
		projectCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "project_count_total"),
			"projects number relevant to the user",
			[]string{"type"}, nil,
		),
		repoCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repo_count_total"),
			"repositories number relevant to the user",
			[]string{"type"}, nil,
		),
	}
}

func (sc *StatisticsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sc.statsUp
	ch <- sc.projectCount
	ch <- sc.repoCount
}

func (sc *StatisticsCollector) Collect(ch chan<- prometheus.Metric) {
	type statisticsMetric struct {
		Total_project_count   float64
		Public_project_count  float64
		Private_project_count float64
		Public_repo_count     float64
		Total_repo_count      float64
		Private_repo_count    float64
	}

	body := sc.client.request("/statistics")

	var data statisticsMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(sc.logger).Log(err.Error())
		ch <- prometheus.MustNewConstMetric(
			sc.statsUp, prometheus.GaugeValue, 0.0,
		)
		sc.upChannel <- false
		return
	}

	ch <- prometheus.MustNewConstMetric(
		sc.projectCount, prometheus.GaugeValue, data.Total_project_count, "total_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.projectCount, prometheus.GaugeValue, data.Public_project_count, "public_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.projectCount, prometheus.GaugeValue, data.Private_project_count, "private_project",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.repoCount, prometheus.GaugeValue, data.Public_repo_count, "public_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.repoCount, prometheus.GaugeValue, data.Total_repo_count, "total_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.repoCount, prometheus.GaugeValue, data.Private_repo_count, "private_repo",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.statsUp, prometheus.GaugeValue, 1.0,
	)
	sc.upChannel <- true
}
