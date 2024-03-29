package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectReplicationsMetric(ch chan<- prometheus.Metric) bool {
	start := time.Now()
	type policiesMetrics []struct {
		ID      float64 `json:"id"`
		Name    string  `json:"name"`
		Enabled bool    `json:"enabled"`
		Trigger struct {
			Type string `json:"type"`
		}
		// Extra fields omitted for maintainability: not relevant for current metrics
	}
	type policyMetric []struct {
		Status     string  `json:"status"`
		Failed     float64 `json:"failed"`
		Succeed    float64 `json:"succeed"`
		InProgress float64 `json:"in_progress"`
		Stopped    float64 `json:"stopped"`
		// Extra fields omitted for maintainability: not relevant for current metrics
	}

	var policiesData policiesMetrics
	err := h.requestAll("/replication/policies", func(pageBody []byte) error {
		var pageData policiesMetrics
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		policiesData = append(policiesData, pageData...)

		return nil
	})
	if err != nil {
		level.Error(h.logger).Log("msg", "Error retrieving replication policies", "err", err.Error())
		return false
	}

	for i := range policiesData {
		if policiesData[i].Enabled == true {
			policyID := strconv.FormatFloat(policiesData[i].ID, 'f', 0, 32)
			policyName := policiesData[i].Name
			triggerType := policiesData[i].Trigger.Type

			body, _ := h.request("/replication/executions?policy_id=" + policyID + "&page=1&page_size=2")
			var data policyMetric

			if err := json.Unmarshal(body, &data); err != nil {
				level.Error(h.logger).Log("msg", "Error retrieving replication data for policy "+policyName+" (ID "+policyID+")", "err", err.Error())
				return false
			}

			if len(data) == 0 {
				level.Debug(h.logger).Log("msg", "Policy "+policyName+" (ID "+policyID+") has no executions yet")
				continue
			}

			var j int = 0
			if len(data) > 1 && data[j].Status == "InProgress" {
				// Current is in progress: check previous replication execution
				j = 1
			}

			var replStatus float64
			replStatus = 0
			if data[j].Status == "Succeed" {
				replStatus = 1
			}
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_status"].Desc, allMetrics["replication_status"].Type, replStatus, policyName, triggerType,
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[j].Failed, policyName, triggerType, "failed",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[j].Succeed, policyName, triggerType, "succeed",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[j].InProgress, policyName, triggerType, "in_progress",
			)
			ch <- prometheus.MustNewConstMetric(
				allMetrics["replication_tasks"].Desc, allMetrics["replication_tasks"].Type, data[j].Stopped, policyName, triggerType, "stopped",
			)
		}
	}

	reportLatency(start, "replication_latency", ch)
	return true
}
