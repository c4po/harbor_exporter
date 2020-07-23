package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"time"
)

func (e *Exporter) collectQuotasMetric(ch chan<- prometheus.Metric, pageSize int) bool {

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
	page := 1
	result := true
	for {
		body := e.client.request(fmt.Sprintf("/quotas?page=%d&page_size=%d", page, pageSize))
		page++
		var data quotaMetric

		if err := json.Unmarshal(body, &data); err != nil {
			level.Error(e.logger).Log(err.Error())
			result = false
			continue
		}

		level.Debug(e.logger).Log("body", body)

		for i := range data {
			if data[i].Ref.Name == "" || data[i].Ref.Id == 0 {
				level.Debug(e.logger).Log(data[i].Ref.Id, data[i].Ref.Name)
			} else {
				repoid := strconv.FormatFloat(data[i].Ref.Id, 'f', 0, 32)
				ch <- prometheus.MustNewConstMetric(
					quotasCount, prometheus.GaugeValue, data[i].Hard.Count, "hard", data[i].Ref.Name, repoid,
				)
				ch <- prometheus.MustNewConstMetric(
					quotasCount, prometheus.GaugeValue, data[i].Used.Count, "used", data[i].Ref.Name, repoid,
				)
				ch <- prometheus.MustNewConstMetric(
					quotasSize, prometheus.GaugeValue, data[i].Hard.Storage, "hard", data[i].Ref.Name, repoid,
				)
				ch <- prometheus.MustNewConstMetric(
					quotasSize, prometheus.GaugeValue, data[i].Used.Storage, "used", data[i].Ref.Name, repoid,
				)
			}
		}
		if len(data) < pageSize {
			break
		}
	}
	return result
}
