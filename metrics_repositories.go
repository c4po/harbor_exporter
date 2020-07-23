package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *Exporter) collectRepositoriesMetric(ch chan<- prometheus.Metric, version string, pageSize int) bool {
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
	projectsPage := 1
	result := true
	for {
		projectsBody := e.client.request(fmt.Sprintf("/projects?page=%d&page_size=%d", projectsPage, pageSize))
		projectsPage++
		var projectsData projectsMetrics

		if err := json.Unmarshal(projectsBody, &projectsData); err != nil {
			level.Error(e.logger).Log(err.Error())
			result = false
			continue
		}

		for i := range projectsData {
			reposPage := 1
			projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)
			for {
				url := fmt.Sprintf("/repositories?project_id=%s&page=%d&page_size=%d", projectId, reposPage, pageSize)
				if version == "/api/v2.0" {
					url = fmt.Sprintf("/projects/"+projectsData[i].Name+"/repositories?page=%d&page_size=%d", reposPage, pageSize)
				}
				reposPage++
				body := e.client.request(url)
				var data repositoriesMetric

				if err := json.Unmarshal(body, &data); err != nil {
					level.Error(e.logger).Log(err.Error())
					result = false
					continue
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
				if len(data) < pageSize {
					break
				}
			}
		}
		if len(projectsData) < pageSize {
			break
		}
	}
	return result
}
