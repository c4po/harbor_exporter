package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectQuotasMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()

	type quotaMetric []struct {
		ID  float64 `json:"id"`
		Ref struct {
			ID        float64 `json:"id"`
			Name      string  `json:"name"`
			OwnerName string  `json:"owner_name"`
		}
		CreationTime time.Time
		UpdateTime   time.Time
		Hard         struct {
			Count   float64 `json:"count"`
			Storage float64 `json:"storage"`
		}
		Used struct {
			Count   float64 `json:"count"`
			Storage float64 `json:"storage"`
		}
	}
	var data quotaMetric
	err := h.requestAll("/quotas", func(pageBody []byte) error {
		var pageData quotaMetric
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
		if data[i].Ref.Name == "" || data[i].Ref.ID == 0 {
			level.Debug(h.logger).Log(data[i].Ref.ID, data[i].Ref.Name)
		} else {
			repoid := strconv.FormatFloat(data[i].Ref.ID, 'f', 0, 32)
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
