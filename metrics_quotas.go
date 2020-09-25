package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

type QuotaCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateQuotaCollector(e *HarborExporter) *QuotaCollector {
	qc := QuotaCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	qc.metrics["quotas_count_total"] = newMetricInfo(e.instance, "quotas_count_total", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	qc.metrics["quotas_size_bytes"] = newMetricInfo(e.instance, "quotas_size_bytes", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	return &qc
}

func (qc *QuotaCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range qc.metrics {
		ch <- m.Desc
	}
}

func (qc *QuotaCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()

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

	var data quotaMetric
	err := qc.exporter.requestAll("/quotas", func(pageBody []byte) error {
		var pageData quotaMetric
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		data = append(data, pageData...)

		return nil
	})
	if err != nil {
		level.Error(qc.exporter.logger).Log(err.Error())
		qc.exporter.quotaChan <- false
		return
	}

	for i := range data {
		if data[i].Ref.Name == "" || data[i].Ref.Id == 0 {
			level.Debug(qc.exporter.logger).Log(data[i].Ref.Id, data[i].Ref.Name)
		} else {
			repoid := strconv.FormatFloat(data[i].Ref.Id, 'f', 0, 32)
			ch <- prometheus.MustNewConstMetric(
				qc.metrics["quotas_count_total"].Desc, qc.metrics["quotas_count_total"].Type, data[i].Hard.Count, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.metrics["quotas_count_total"].Desc, qc.metrics["quotas_count_total"].Type, data[i].Used.Count, "used", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.metrics["quotas_size_bytes"].Desc, qc.metrics["quotas_size_bytes"].Type, data[i].Hard.Storage, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				qc.metrics["quotas_size_bytes"].Desc, qc.metrics["quotas_size_bytes"].Type, data[i].Used.Storage, "used", data[i].Ref.Name, repoid,
			)
		}
	}
	reportLatency(start, "quotas_latency", ch)
	qc.exporter.quotaChan <- true
}
