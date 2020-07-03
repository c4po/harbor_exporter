package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

type ScansCollector struct {
	client *HarborClient
	logger log.Logger
	upChannel chan<- bool

	scanUp *prometheus.Desc
	scanTotalCount *prometheus.Desc
	scanCompletedCount *prometheus.Desc
	scanRequesterCount *prometheus.Desc
}

func NewScansCollector(c *HarborClient, l log.Logger, u chan<- bool, instance string) *ScansCollector {
	return &ScansCollector{
		client: c,
		logger: l,
		upChannel: u,
		scanUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "scan_up"),
			"Was the last query of harbor scans successful.",
			nil, nil,
		),
		scanTotalCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "scans_total"),
			"metrics of the latest scan all process",
			nil, nil,
		),
		scanCompletedCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "scans_completed"),
			"metrics of the latest scan all process",
			nil, nil,
		),
		scanRequesterCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "scans_requester"),
			"metrics of the latest scan all process",
			nil, nil,
		),
	}
}

func (sc *ScansCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sc.scanUp
	ch <- sc.scanTotalCount
	ch <- sc.scanCompletedCount
	ch <- sc.scanRequesterCount
}

func (sc *ScansCollector) Collect(ch chan<- prometheus.Metric) {

	type scanMetric struct {
		Total     float64
		Completed float64
		metrics   []interface{}
		Requester string
		Ongoing   bool
	}
	body := sc.client.request("/scans/all/metrics")
	var data scanMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(sc.logger).Log(err.Error())
		ch <- prometheus.MustNewConstMetric(
			sc.scanUp, prometheus.GaugeValue, 0.0,
		)
		sc.upChannel <- false
		return
	}

	scan_requester, _ := strconv.ParseFloat(data.Requester, 64)
	ch <- prometheus.MustNewConstMetric(
		sc.scanRequesterCount, prometheus.GaugeValue, float64(scan_requester),
	)

	ch <- prometheus.MustNewConstMetric(
		sc.scanTotalCount, prometheus.GaugeValue, float64(data.Total),
	)

	ch <- prometheus.MustNewConstMetric(
		sc.scanCompletedCount, prometheus.GaugeValue, float64(data.Completed),
	)
	ch <- prometheus.MustNewConstMetric(
		sc.scanUp, prometheus.GaugeValue, 1.0,
	)
	sc.upChannel <- true
}
