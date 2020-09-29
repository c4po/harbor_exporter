package main

import (
	"encoding/json"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

func (e *HarborExporter) collectQuotasMetric(ch chan<- prometheus.Metric) bool {
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
	err := e.requestAll("/quotas", func(pageBody []byte) error {
		var pageData quotaMetric
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		data = append(data, pageData...)

		return nil
	})
	if err != nil {
		level.Error(e.logger).Log(err.Error())
		return false
	}

	for i := range data {
		if data[i].Ref.Name == "" || data[i].Ref.Id == 0 {
			level.Debug(e.logger).Log(data[i].Ref.Id, data[i].Ref.Name)
		} else {
			repoid := strconv.FormatFloat(data[i].Ref.Id, 'f', 0, 32)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["quotas_count_total"].Desc, allMetrics["quotas_count_total"].Type, data[i].Hard.Count, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["quotas_count_total"].Desc, allMetrics["quotas_count_total"].Type, data[i].Used.Count, "used", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["quotas_size_bytes"].Desc, allMetrics["quotas_size_bytes"].Type, data[i].Hard.Storage, "hard", data[i].Ref.Name, repoid,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["quotas_size_bytes"].Desc, allMetrics["quotas_size_bytes"].Type, data[i].Used.Storage, "used", data[i].Ref.Name, repoid,
			)
		}
	}

	reportLatency(start, "quotas_latency", ch)
	return true
}
