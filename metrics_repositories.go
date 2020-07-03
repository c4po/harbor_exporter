package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"sync"
	"time"
)

type RepositoriesCollector struct {
	client    *HarborClient
	logger    log.Logger
	upChannel chan<- bool
	threads   int

	repositoryUp          *prometheus.Desc
	repositoryLatency     *prometheus.Desc
	repositoriesPullCount *prometheus.Desc
	repositoriesStarCount *prometheus.Desc
	repositoriesTagsCount *prometheus.Desc
}

func NewRepositoriesCollector(c *HarborClient, l log.Logger, u chan<- bool, instance string, threads int) *RepositoriesCollector {
	return &RepositoriesCollector{
		client:    c,
		logger:    l,
		upChannel: u,
		threads:   threads,
		repositoryUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repositories_up"),
			"Was the last query of harbor repositories successful.",
			nil, nil,
		),
		repositoryLatency: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repositories_latency"),
			"Time in seconds to collect repository metrics",
			nil, nil,
		),
		repositoriesPullCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repositories_pull_total"),
			"Get public repositories which are accessed most.).",
			[]string{"repo_name", "repo_id"}, nil,
		),
		repositoriesStarCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repositories_star_total"),
			"Get public repositories which are accessed most.).",
			[]string{"repo_name", "repo_id"}, nil,
		),
		repositoriesTagsCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "repositories_tags_total"),
			"Get public repositories which are accessed most.).",
			[]string{"repo_name", "repo_id"}, nil,
		),
	}
}

func (rc *RepositoriesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.repositoryUp
	ch <- rc.repositoriesPullCount
	ch <- rc.repositoriesStarCount
	ch <- rc.repositoriesTagsCount
}

func (rc *RepositoriesCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	type projectMetric struct {
		Project_id  float64
		Owner_id    float64
		Name        string
		Repo_count  float64
		Chart_count float64
	}
	type projectsMetrics []projectMetric
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
	projectsBody := rc.client.request("/projects")
	var projectsData projectsMetrics

	if err := json.Unmarshal(projectsBody, &projectsData); err != nil {
		level.Error(rc.logger).Log(err.Error())
		ch <- prometheus.MustNewConstMetric(
			rc.repositoryUp, prometheus.GaugeValue, 0.0,
		)
		rc.upChannel <- false
		return
	}

	projectChan := make(chan projectMetric, len(projectsData))
	defer close(projectChan)
	for _, p := range projectsData {
		projectChan <- p
	}
	threadgroup := sync.WaitGroup{}
	threadgroup.Add(rc.threads)
	projectgroup := sync.WaitGroup{}
	projectgroup.Add(len(projectsData))
	for i := 0; i < rc.threads; i++ {
		go func() {
			for {
				exit := false
				select {
				case project := <-projectChan:

					projectId := strconv.FormatFloat(project.Project_id, 'f', 0, 32)

					body := rc.client.request("/repositories?project_id=" + projectId)
					var data repositoriesMetric

					if err := json.Unmarshal(body, &data); err != nil {
						level.Error(rc.logger).Log(err.Error())
						ch <- prometheus.MustNewConstMetric(
							rc.repositoryUp, prometheus.GaugeValue, 0.0,
						)
						rc.upChannel <- false
						return
					}

					for i := range data {
						repoId := strconv.FormatFloat(data[i].Id, 'f', 0, 32)
						ch <- prometheus.MustNewConstMetric(
							rc.repositoriesPullCount, prometheus.GaugeValue, data[i].Pull_count, data[i].Name, repoId,
						)
						ch <- prometheus.MustNewConstMetric(
							rc.repositoriesStarCount, prometheus.GaugeValue, data[i].Star_count, data[i].Name, repoId,
						)
						ch <- prometheus.MustNewConstMetric(
							rc.repositoriesTagsCount, prometheus.GaugeValue, data[i].Tags_count, data[i].Name, repoId,
						)
					}
					projectgroup.Done()
				default:
					exit = true
					break
				}
				if exit {
					break
				}
			}
			threadgroup.Done()
		}()
	}

	projectgroup.Wait()
	threadgroup.Wait()

	end := time.Now()
	latency := end.Sub(start).Seconds()
	ch <- prometheus.MustNewConstMetric(
		rc.repositoryLatency, prometheus.GaugeValue, latency,
	)
	ch <- prometheus.MustNewConstMetric(
		rc.repositoryUp, prometheus.GaugeValue, 1.0,
	)
	rc.upChannel <- true
}
