package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type SystemVolumesCollector struct {
	client *HarborClient
	logger log.Logger
	upChannel chan<- bool

	volumesUp *prometheus.Desc
	systemVolumes *prometheus.Desc
}

func NewSystemVolumesCollector(c *HarborClient, l log.Logger, u chan<- bool, instance string) *SystemVolumesCollector {
	return &SystemVolumesCollector{
		client: c,
		logger: l,
		upChannel: u,
		volumesUp: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "system_volumes_up"),
			"Was the last query of harbor system volumes successful.",
			nil, nil,
		),
		systemVolumes: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instance, "system_volumes_bytes"),
			"Get system volume info (total/free size).",
			[]string{"storage"}, nil,
		),
	}
}

func (sc *SystemVolumesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- sc.volumesUp
	ch <- sc.systemVolumes
}

func (sc *SystemVolumesCollector) Collect(ch chan<- prometheus.Metric) {
	type systemVolumesMetric struct {
		Storage struct {
			Total float64
			Free  float64
		}
	}
	body := sc.client.request("/systeminfo/volumes")
	var data systemVolumesMetric
	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(sc.logger).Log(err.Error())
		ch <- prometheus.MustNewConstMetric(
			sc.volumesUp, prometheus.GaugeValue, 0.0,
		)
		sc.upChannel <- false
		return
	}

	ch <- prometheus.MustNewConstMetric(
		sc.systemVolumes, prometheus.GaugeValue, data.Storage.Total, "total",
	)
	ch <- prometheus.MustNewConstMetric(
		sc.systemVolumes, prometheus.GaugeValue, data.Storage.Free, "free",
	)

	ch <- prometheus.MustNewConstMetric(
		sc.volumesUp, prometheus.GaugeValue, 1.0,
	)
	sc.upChannel <- true
}
