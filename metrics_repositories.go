package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *Exporter) collectRepositoriesMetric(ch chan<- prometheus.Metric) bool {
	type projectsMetrics []struct {
		Project_id  float64
		Owner_id    float64
		Name        string
		Repo_count  float64
		Chart_count float64
	}

	type tagsMetric []struct {
		Name string `json:"name"`
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

	projectsBody := e.client.request("/api/projects")
	var projectsData projectsMetrics

	if err := json.Unmarshal(projectsBody, &projectsData); err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	for i := range projectsData {
		projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)

		body := e.client.request("/api/repositories?project_id=" + projectId)
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

			repoName := data[i].Name
			tagBody := e.client.request("/api/repositories/" + repoName + "/tags?detail=true")

			var scanData []map[string]interface{}
			json.Unmarshal([]byte(tagBody), &scanData)

			if len(scanData) == 0 {
				continue
			}

			var tagData tagsMetric
			if err := json.Unmarshal(tagBody, &tagData); err != nil {
				level.Error(e.logger).Log(err.Error())
				return false
			}

			for key, scanData := range scanData {
				tagName := tagData[key].Name
				//
				if len(scanData) < 16 {

					continue
				}
				scan_overview := scanData["scan_overview"].(map[string]interface{})
				severity := scan_overview["application/vnd.scanner.adapter.vuln.report.harbor+json; version=1.0"].(map[string]interface{})

				//fmt.Println("Reading Value for Key :", key)
				//fmt.Println("Reading Value for Key :", tagData)

				var vurnerability float64
				if severity["severity"] == "None" {
					vurnerability = 0
				}
				if severity["severity"] == "Low" {
					vurnerability = 1
				}
				if severity["severity"] == "Medium" {
					vurnerability = 2
				}
				if severity["severity"] == "High" {
					vurnerability = 3
				}
				//		fmt.Println("Address :", vurnerability)
				ch <- prometheus.MustNewConstMetric(
					scanSeverity, prometheus.GaugeValue, vurnerability, repoName+":"+tagName, repoId,
				)
			}

		}
	}
	return true
}
