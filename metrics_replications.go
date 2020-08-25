package main

import (
	"encoding/json"
	"strconv"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (e *HarborExporter) collectReplicationsMetric(ch chan<- prometheus.Metric) bool {
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

	policiesBody, _ := e.request("/replication/policies")
	var policiesData policiesMetrics

	if err := json.Unmarshal(policiesBody, &policiesData); err != nil {
		level.Error(e.logger).Log("msg", "Error retrieving replication policies", "err", err.Error())
		return false
	}

	for i := range policiesData {
		policyId := strconv.FormatFloat(policiesData[i].Id, 'f', 0, 32)
		policyName := policiesData[i].Name

		body, _ := e.request("/replication/executions?policy_id=" + policyId + "&page=1&page_size=1")
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
				allMetrics["replication_status"].Desc, allMetrics["replication_status"].Type, replStatus, policyName,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[i].Failed, policyName, "failed",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[i].Succeed, policyName, "succeed",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[i].In_progress, policyName, "in_progress",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[i].Stopped, policyName, "stopped",
			)
		}
	}
	return true
}
