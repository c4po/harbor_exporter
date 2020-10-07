package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectRepositoriesMetric(ch chan<- prometheus.Metric) bool {
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
	type artifactsMetric []struct {
		Id   float64
		Tags []struct {
			Name string
		}
	}
	var projectsData projectsMetrics
	err := h.requestAll("/projects", func(pageBody []byte) error {
		var pageData projectsMetrics
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		projectsData = append(projectsData, pageData...)

		return nil
	})
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	for i := range projectsData {
		projectId := strconv.FormatFloat(projectsData[i].Project_id, 'f', 0, 32)
		if h.isV2 {
			var data repositoriesMetricV2
			err := h.requestAll("/projects/"+projectsData[i].Name+"/repositories", func(pageBody []byte) error {
				var pageData repositoriesMetricV2
				if err := json.Unmarshal(pageBody, &pageData); err != nil {
					return err
				}

				data = append(data, pageData...)

				return nil
			})
			if err != nil {
				level.Error(h.logger).Log(err.Error())
				return false
			}
			projectName := projectsData[i].Name

			for i := range data {
				repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_pull_total"].Desc, allMetrics["repositories_pull_total"].Type, data[i].Pull_count, data[i].Name, repoId,
				)
				// ch <- prometheus.MustNewConstMetric(
				// 	allMetrics["repositories_star_total"].Desc, allMetrics["repositories_star_total"].Type, data[i].Star_count, data[i].Name, repoId,
				// )
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_tags_total"].Desc, allMetrics["repositories_tags_total"].Type, data[i].Artifact_count, data[i].Name, repoId,
				)

				repoName := strings.Split(data[i].Name, "/")
				tagBody, _ := h.request("/projects/" + projectName + "/repositories/" + repoName[1] + "/artifacts?with_tag=true&with_scan_overview=true")
				var scanData []map[string]interface{}

				var tagData artifactsMetric
				if err := json.Unmarshal(tagBody, &tagData); err != nil {
					level.Error(h.logger).Log(err.Error())
					return false
				}

				json.Unmarshal([]byte(tagBody), &scanData)
				if len(scanData) == 0 {
					continue
				}
				for key, scanData := range scanData {
					tagName := tagData[key].Tags[0]

					if len(scanData) < 16 {

						continue
					}
					scan_overview := scanData["scan_overview"].(map[string]interface{})
					severity := scan_overview["application/vnd.scanner.adapter.vuln.report.harbor+json; version=1.0"].(map[string]interface{})

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
					if severity["severity"] == "Critical" {
						vurnerability = 4
					}
					image := data[i].Name + ":" + tagName.Name
					ch <- prometheus.MustNewConstMetric(
						allMetrics["image_vulnerability"].Desc, allMetrics["image_vulnerability"].Type, vurnerability, image,
					)
				}
			}

		} else {
			var data repositoriesMetric
			err := h.requestAll("/repositories?project_id="+projectId, func(pageBody []byte) error {
				var pageData repositoriesMetric
				if err := json.Unmarshal(pageBody, &pageData); err != nil {
					return err
				}

				data = append(data, pageData...)

				return nil
			})
			if err != nil {
				level.Error(h.logger).Log(err.Error())
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
	}

	reportLatency(start, "repositories_latency", ch)
	return true
}
