package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"time"
)

type VolumeCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateVolumeCollector(e *HarborExporter) *VolumeCollector {
	vc := VolumeCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	vc.metrics["system_volumes_bytes"] = newMetricInfo(e.instance, "system_volumes_bytes", "Get system volume info (total/free size).", prometheus.GaugeValue, storageLabelNames, nil)
	return &vc
}

func (vc *VolumeCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range vc.metrics {
		ch <- m.Desc
	}
}

func (vc *VolumeCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	type systemVolumesMetric struct {
		Storage struct {
			Total float64
			Free  float64
		}
	}
	body, _ := vc.exporter.request("/systeminfo/volumes")
	var data systemVolumesMetric
	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(vc.exporter.logger).Log(err.Error())
		vc.exporter.volumeChan <- false
		return
	}

	ch <- prometheus.MustNewConstMetric(
		vc.metrics["system_volumes_bytes"].Desc, vc.metrics["system_volumes_bytes"].Type, data.Storage.Total, "total",
	)
	ch <- prometheus.MustNewConstMetric(
		vc.metrics["system_volumes_bytes"].Desc, vc.metrics["system_volumes_bytes"].Type, data.Storage.Free, "free",
	)
	reportLatency(start, "system_volumes_latency", ch)
	vc.exporter.volumeChan <- true
}
