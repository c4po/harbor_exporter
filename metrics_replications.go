package main

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type ReplicationCollector struct {
	exporter *HarborExporter
	metrics  map[string]metricInfo
}

func CreateReplicationCollector(e *HarborExporter) *ReplicationCollector {
	rc := ReplicationCollector{
		exporter: e,
		metrics:  make(map[string]metricInfo),
	}
	rc.metrics["replication_status"] = newMetricInfo(e.instance, "replication_status", "Get status of the last execution of this replication policy: Succeed = 1, any other status = 0.", prometheus.GaugeValue, replicationLabelNames, nil)
	rc.metrics["replication_tasks"] = newMetricInfo(e.instance, "replication_tasks", "Get number of replication tasks, with various results, in the latest execution of this replication policy.", prometheus.GaugeValue, replicationTaskLabelNames, nil)
	return &rc
}

func (rc *ReplicationCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range rc.metrics {
		ch <- m.Desc
	}
}

func (rc *ReplicationCollector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
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

	var policiesData policiesMetrics
	err := rc.exporter.requestAll("/replication/policies", func(pageBody []byte) error {
		var pageData policiesMetrics
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}
		policiesData = append(policiesData, pageData...)

		return nil
	})
	if err != nil {
		level.Error(rc.exporter.logger).Log("msg", "Error retrieving replication policies", "err", err.Error())
		rc.exporter.replicationChan <- false
		return
	}

	for i := range policiesData {
		policyId := strconv.FormatFloat(policiesData[i].Id, 'f', 0, 32)
		policyName := policiesData[i].Name

		body, _ := rc.exporter.request("/replication/executions?policy_id=" + policyId + "&page=1&page_size=1")
		var data policyMetric

		if err := json.Unmarshal(body, &data); err != nil {
			level.Error(rc.exporter.logger).Log("msg", "Error retrieving replication data for policy "+policyId, "err", err.Error())
			rc.exporter.replicationChan <- false
			return
		}

		for i := range data {
			var replStatus float64
			replStatus = 0
			if data[i].Status == "Succeed" {
				replStatus = 1
			}
			ch <- prometheus.MustNewConstMetric(
				rc.metrics["replication_status"].Desc, rc.metrics["replication_status"].Type, replStatus, policyName,
			)
			ch <- prometheus.MustNewConstMetric(
				rc.metrics["replication_tasks"].Desc, rc.metrics["replication_tasks"].Type, data[i].Failed, policyName, "failed",
			)
			ch <- prometheus.MustNewConstMetric(
				rc.metrics["replication_tasks"].Desc, rc.metrics["replication_tasks"].Type, data[i].Succeed, policyName, "succeed",
			)
			ch <- prometheus.MustNewConstMetric(
				rc.metrics["replication_tasks"].Desc, rc.metrics["replication_tasks"].Type, data[i].In_progress, policyName, "in_progress",
			)
			ch <- prometheus.MustNewConstMetric(
				rc.metrics["replication_tasks"].Desc, rc.metrics["replication_tasks"].Type, data[i].Stopped, policyName, "stopped",
			)
		}
	}
	reportLatency(start, "replication_latency", ch)
	rc.exporter.replicationChan <- true
}
