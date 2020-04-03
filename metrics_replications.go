package main

import (
	"encoding/json"
	"strconv"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *Exporter) collectReplicationsMetric(ch chan<- prometheus.Metric) bool {
	type policiesMetrics []struct {
		Id   float64
		Name string
		// Extra fields omitted for maintainability: not relevant for current metrics
	}
	type policyMetric []struct {
		Status      string
		Failed      float64
		Succeed     float64
		In_progress float64
		Stopped     float64
		// Extra fields omitted for maintainability: not relevant for current metrics
	}

	policiesBody := e.client.request("/api/replication/policies")
	var policiesData policiesMetrics

	if err := json.Unmarshal(policiesBody, &policiesData); err != nil {
		level.Error(e.logger).Log("msg", "Error retrieving replication policies", "err", err.Error())
		return false
	}

	for i := range policiesData {
		policyId := strconv.FormatFloat(policiesData[i].Id, 'f', 0, 32)
		policyName := policiesData[i].Name

		body := e.client.request("/api/replication/executions?policy_id=" + policyId + "&page=1&page_size=1")
		var data policyMetric

		if err := json.Unmarshal(body, &data); err != nil {
			level.Error(e.logger).Log("msg", "Error retrieving replication data for policy "+policyId, "err", err.Error())
			return false
		}

		for i := range data {
			var replStatus float64
			replStatus = 0
			if data[i].Status == "Succeed" {
				replStatus = 1
			}
			ch <- prometheus.MustNewConstMetric(
				replicationStatus, prometheus.GaugeValue, replStatus, policyName,
			)
			ch <- prometheus.MustNewConstMetric(
				replicationTasks, prometheus.GaugeValue, data[i].Failed, policyName, "failed",
			)
			ch <- prometheus.MustNewConstMetric(
				replicationTasks, prometheus.GaugeValue, data[i].Succeed, policyName, "succeed",
			)
			ch <- prometheus.MustNewConstMetric(
				replicationTasks, prometheus.GaugeValue, data[i].In_progress, policyName, "in_progress",
			)
			ch <- prometheus.MustNewConstMetric(
				replicationTasks, prometheus.GaugeValue, data[i].Stopped, policyName, "stopped",
			)
		}
	}
	return true
}
