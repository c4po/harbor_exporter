package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *HarborExporter) collectRepositoriesMetric(ch chan<- prometheus.Metric) bool {
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
	projectsBody, _ := e.request("/projects")
	var projectsData projectsMetrics
	var url string

	if err := json.Unmarshal(projectsBody, &projectsData); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	for i := range projectsData {
		projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)
		if e.isV2 {
			url = "/projects/" + projectsData[i].Name + "/repositories"
		} else {
			url = "/repositories?project_id=" + projectId
		}
		body, _ := e.request(url)
		var data repositoriesMetric

		if err := json.Unmarshal(body, &data); err != nil {
			level.Error(e.logger).Log(err.Error())
			return false
		}

		for i := range data {
			repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["repositories_pull_total"].Desc, allMetrics["repositories_pull_total"].Type, data[i].Pull_count, data[i].Name, repoId,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["repositories_star_total"].Desc, allMetrics["repositories_star_total"].Type, data[i].Star_count, data[i].Name, repoId,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["repositories_tags_total"].Desc, allMetrics["repositories_tags_total"].Type, data[i].Tags_count, data[i].Name, repoId,
			)
		}
	}
	return true
}
