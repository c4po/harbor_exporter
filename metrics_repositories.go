package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectRepositoriesMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()
	type projectsMetrics []struct {
		ProjectID  float64 `json:"project_id"`
		OwnerID    float64 `json:"owner_id"`
		Name       string  `json:"name"`
		RepoCount  float64 `json:"repo_count"`
		ChartCount float64 `json:"chart_count"`
	}
	type repositoriesMetric []struct {
		ID           float64   `json:"id"`
		Name         string    `json:"name"`
		ProjectID    float64   `json:"project_id"`
		Description  string    `json:"description"`
		PullCount    float64   `json:"pull_count"`
		StarCount    float64   `json:"star_count"`
		TagsCount    float64   `json:"tags_count"`
		CreationTime time.Time `json:"creation_time"`
		UpdateTime   time.Time `json:"update_time"`
		labels       []struct {
			ID           float64   `json:"id"`
			Name         string    `json:"name"`
			ProjectID    float64   `json:"project_id"`
			Description  string    `json:"description"`
			Color        string    `json:"color"`
			Deleted      bool      `json:"deleted"`
			Scope        string    `json:"scope"`
			CreationTime time.Time `json:"creation_time"`
			UpdateTime   time.Time `json:"update_time"`
		}
	}
	type repositoriesMetricV2 []struct {
		ID            float64   `json:"id"`
		Name          string    `json:"name"`
		ProjectID     float64   `json:"project_id"`
		Description   string    `json:"description"`
		PullCount     float64   `json:"pull_count"`
		ArtifactCount float64   `json:"artifact_count"`
		CreationTime  time.Time `json:"creation_time"`
		UpdateTime    time.Time `json:"update_time"`
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
		projectID := strconv.FormatFloat(projectsData[i].ProjectID, 'f', 0, 32)
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

			for i := range data {
				repoID := strconv.FormatFloat(data[i].ID, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_pull_total"].Desc, allMetrics["repositories_pull_total"].Type, data[i].PullCount, data[i].Name, repoID,
				)
				// ch <- prometheus.MustNewConstMetric(
				// 	allMetrics["repositories_star_total"].Desc, allMetrics["repositories_star_total"].Type, data[i].Star_count, data[i].Name, repoId,
				// )
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_tags_total"].Desc, allMetrics["repositories_tags_total"].Type, data[i].ArtifactCount, data[i].Name, repoID,
				)
			}

		} else {
			var data repositoriesMetric
			err := h.requestAll("/repositories?project_id="+projectID, func(pageBody []byte) error {
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
				repoID := strconv.FormatFloat(data[i].ID, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_pull_total"].Desc, allMetrics["repositories_pull_total"].Type, data[i].PullCount, data[i].Name, repoID,
				)
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_star_total"].Desc, allMetrics["repositories_star_total"].Type, data[i].StarCount, data[i].Name, repoID,
				)
				ch <- prometheus.MustNewConstMetric(
					allMetrics["repositories_tags_total"].Desc, allMetrics["repositories_tags_total"].Type, data[i].TagsCount, data[i].Name, repoID,
				)
			}
		}
	}

	reportLatency(start, "repositories_latency", ch)
	return true
}
