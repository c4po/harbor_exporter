package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type RepositoryCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateRepositoryCollector(e *HarborExporter) *RepositoryCollector {
	rc := RepositoryCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	rc.metrics["repositories_pull_total"] = newMetricInfo(e.instance, "repositories_pull_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	rc.metrics["repositories_star_total"] = newMetricInfo(e.instance, "repositories_star_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	rc.metrics["repositories_tags_total"] = newMetricInfo(e.instance, "repositories_tags_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	return &rc
}

func (rc *RepositoryCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range rc.metrics {
		ch <- m.Desc
	}
}

func (rc *RepositoryCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	type projectsMetrics []struct {
		Project_id  float64
		Owner_id    float64
		Name        string
		Repo_count  float64
		Chart_count float64
	}
	type repositoriesMetric []struct {
		Id            float64
		Name          string
		Project_id    float64
		Description   string
		Pull_count    float64
		Star_count    float64
		Tags_count    float64
		Creation_time time.Time
		Update_time   time.Time
		labels        []struct {
			Id            float64
			Name          string
			Project_id    float64
			Description   string
			Color         string
			Deleted       bool
			Scope         string
			Creation_time time.Time
			Update_time   time.Time
		}
	}
	type repositoriesMetricV2 []struct {
		Id             float64
		Name           string
		Project_id     float64
		Description    string
		Pull_count     float64
		Artifact_count float64
		Creation_time  time.Time
		Update_time    time.Time
	}

	var projectsData projectsMetrics
	err := rc.exporter.requestAll("/projects", func(pageBody []byte) error {
		var pageData projectsMetrics
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		projectsData = append(projectsData, pageData...)

		return nil
	})
	if err != nil {
		level.Error(rc.exporter.logger).Log(err.Error())
		rc.exporter.repositoryChan <- false
		return
	}

	for i := range projectsData {
		projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)
		if rc.exporter.isV2 {
			var data repositoriesMetricV2
			err := rc.exporter.requestAll("/projects/"+projectsData[i].Name+"/repositories", func(pageBody []byte) error {
				var pageData repositoriesMetricV2
				if err := json.Unmarshal(pageBody, &pageData); err != nil {
					return err
				}

				data = append(data, pageData...)

				return nil
			})
			if err != nil {
				level.Error(rc.exporter.logger).Log(err)
				return
			}

			for i := range data {
				repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					rc.metrics["repositories_pull_total"].Desc, rc.metrics["repositories_pull_total"].Type, data[i].Pull_count, data[i].Name, repoId,
				)
				// ch <- prometheus.MustNewConstMetric(
				// 	rc.metrics["repositories_star_total"].Desc, rc.metrics["repositories_star_total"].Type, data[i].Star_count, data[i].Name, repoId,
				// )
				ch <- prometheus.MustNewConstMetric(
					rc.metrics["repositories_tags_total"].Desc, rc.metrics["repositories_tags_total"].Type, data[i].Artifact_count, data[i].Name, repoId,
				)
			}

		} else {
			var data repositoriesMetric
			err := rc.exporter.requestAll("/repositories?project_id="+projectId, func(pageBody []byte) error {
				var pageData repositoriesMetric
				if err := json.Unmarshal(pageBody, &pageData); err != nil {
					return err
				}

				data = append(data, pageData...)

				return nil
			})
			if err != nil {
				level.Error(rc.exporter.logger).Log(err.Error())
				rc.exporter.repositoryChan <- false
				return
			}

			for i := range data {
				repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					rc.metrics["repositories_pull_total"].Desc, rc.metrics["repositories_pull_total"].Type, data[i].Pull_count, data[i].Name, repoId,
				)
				ch <- prometheus.MustNewConstMetric(
					rc.metrics["repositories_star_total"].Desc, rc.metrics["repositories_star_total"].Type, data[i].Star_count, data[i].Name, repoId,
				)
				ch <- prometheus.MustNewConstMetric(
					rc.metrics["repositories_tags_total"].Desc, rc.metrics["repositories_tags_total"].Type, data[i].Tags_count, data[i].Name, repoId,
				)
			}
		}
	}
	reportLatency(start, "repositories_latency", ch)
	rc.exporter.repositoryChan <- true
}
