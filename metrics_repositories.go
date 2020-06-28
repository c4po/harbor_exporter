package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

func (e *Exporter) collectRepositoriesMetric(ch chan<- prometheus.Metric) bool {
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
	projectsBody := e.client.request("/projects")
	var projectsData projectsMetrics

	if err := json.Unmarshal(projectsBody, &projectsData); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	for i := range projectsData {
		projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)

		body := e.client.request("/repositories?project_id=" + projectId)
		var data repositoriesMetric

		if err := json.Unmarshal(body, &data); err != nil {
			level.Error(e.logger).Log(err.Error())
			return false
		}

		for i := range data {
			repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
			ch <- prometheus.MustNewConstMetric(
				repositoriesPullCount, prometheus.GaugeValue, data[i].Pull_count, data[i].Name, repoId,
			)
			ch <- prometheus.MustNewConstMetric(
				repositoriesStarCount, prometheus.GaugeValue, data[i].Star_count, data[i].Name, repoId,
			)
			ch <- prometheus.MustNewConstMetric(
				repositoriesTagsCount, prometheus.GaugeValue, data[i].Tags_count, data[i].Name, repoId,
			)
		}
	}
	return true
}
