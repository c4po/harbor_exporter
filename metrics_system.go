package main

import (
	"encoding/json"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

func (h *HarborExporter) collectSystemMetric(ch chan<- prometheus.Metric) bool {

	type systemInfoMetric struct {
		WithNotary                  bool   `json:"with_notary"`
		AuthMode                    string `json:"auth_mode"`
		ProjectCreationRestriction  string `json:"project_creation_restriction"`
		SelfRegistration            bool   `json:"self_registration"`
		HasCaRoot                   bool   `json:"has_ca_root"`
		HarborVersion               string `json:"harbor_version"`
		RegistryStorageProviderName string `json:"registry_storage_provider_name"`
		ReadOnly                    bool   `json:"read_only"`
		WithChartmuseum             bool   `json:"with_chartmuseum"`
		NotificationEnable          bool   `json:"notification_enable"`
	}
	body, _ := h.request("/systeminfo")
	var data systemInfoMetric

	if err := json.Unmarshal(body, &data); err != nil {
		level.Error(h.logger).Log(err.Error())
		return false
	}

	// Set string values as labels for general system_info
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_info"].Desc, allMetrics["system_info"].Type, 1, data.AuthMode, data.ProjectCreationRestriction, data.HarborVersion, data.RegistryStorageProviderName,
	)

	// Set all bool values as separate metrics
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_with_notary"].Desc, allMetrics["system_with_notary"].Type, float64(Btoi(data.WithNotary)),
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_self_registration"].Desc, allMetrics["system_self_registration"].Type, float64(Btoi(data.SelfRegistration)),
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_has_ca_root"].Desc, allMetrics["system_has_ca_root"].Type, float64(Btoi(data.HasCaRoot)),
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_read_only"].Desc, allMetrics["system_read_only"].Type, float64(Btoi(data.ReadOnly)),
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_with_chartmuseum"].Desc, allMetrics["system_with_chartmuseum"].Type, float64(Btoi(data.WithChartmuseum)),
	)
	ch <- prometheus.MustNewConstMetric(
		allMetrics["system_notification_enable"].Desc, allMetrics["system_notification_enable"].Type, float64(Btoi(data.NotificationEnable)),
	)

	return true
}
