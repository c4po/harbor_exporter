package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

type QuotasCollector struct {
	client    *HarborClient
	logger    log.Logger
	upChannel chan<- bool

	quotasUp    *prometheus.Desc
	quotasCount *prometheus.Desc
	quotasSize  *prometheus.Desc
}

func NewQuotasCollector(c *HarborClient, l log.Logger, u chan<- bool, instance string) *QuotasCollector {
	return &QuotasCollector{
		client:    c,
		logger:    l,
		upChannel: u,
		quotasUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "quotas_up"),
			"Was the last query of harbor quotas successful.",
			nil, nil,
		),
		quotasCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "quotas_count_total"),
			"quotas",
			[]string{"type", "repo_name", "repo_id"}, nil,
		),
		quotasSize: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "quotas_size_bytes"),
			"quotas",
			[]string{"type", "repo_name", "repo_id"}, nil,
		),
	}
}

func (qc *QuotasCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- qc.quotasUp
	ch <- qc.quotasCount
	ch <- qc.quotasSize
}

func (qc *QuotasCollector) Collect(ch chan<- prometheus.Metric) {

	type quotaMetric []struct {
		Id  float64
		Ref struct {
			Id         float64
			Name       string
			Owner_name string
		}
		Creation_time time.Time
		Update_time   time.Time
		Hard          struct {
			Count   float64
			Storage float64
		}
		Used struct {
			Count   float64
			Storage float64
		}
	}
	body := qc.client.request("/quotas")
	var data quotaMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(qc.logger).Log(err.Error())
		ch <- prometheus.MustNewConstMetric(
			qc.quotasUp, prometheus.GaugeValue, 0.0,
		)
		qc.upChannel <- false
		return
	}

	level.Debug(qc.logger).Log("body", body)

	for i := range data {
		if data[i].Ref.Name == "" || data[i].Ref.Id == 0 {
			level.Debug(qc.logger).Log(data[i].Ref.Id, data[i].Ref.Name)
		} else {
			repoid := strconv.FormatFloat(data[i].Ref.Id, 'f', 0, 32)
			ch <- prometheus.MustNewConstMetric(
				qc.quotasCount, prometheus.GaugeValue, data[i].Hard.Count, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.quotasCount, prometheus.GaugeValue, data[i].Used.Count, "used", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.quotasSize, prometheus.GaugeValue, data[i].Hard.Storage, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.quotasSize, prometheus.GaugeValue, data[i].Used.Storage, "used", data[i].Ref.Name, repoid,
			)
		}
	}
	ch <- prometheus.MustNewConstMetric(
		qc.quotasUp, prometheus.GaugeValue, 1.0,
	)
	qc.upChannel <- true
}
