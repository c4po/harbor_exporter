// Copyright 2019 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "harbor"

	//These are metricsGroup enum values
	metricsGroupHealth        = "health"
	metricsGroupScans         = "scans"
	metricsGroupStatistics    = "statistics"
	metricsGroupQuotas        = "quotas"
	metricsGroupRepositories  = "repositories"
	metricsGroupReplication   = "replication"
	metricsGroupSystemInfo    = "systeminfo"
	metricsGroupArtifactsInfo = "artifacts"
)

func metricsGroupValues() []string {
	return []string{
		metricsGroupHealth,
		metricsGroupScans,
		metricsGroupStatistics,
		metricsGroupQuotas,
		metricsGroupRepositories,
		metricsGroupReplication,
		metricsGroupSystemInfo,
		metricsGroupArtifactsInfo,
	}
}

var (
	allMetrics          map[string]metricInfo
	collectMetricsGroup map[string]bool

	componentLabelNames                       = []string{"component"}
	typeLabelNames                            = []string{"type"}
	quotaLabelNames                           = []string{"type", "repo_name", "repo_id"}
	repoLabelNames                            = []string{"repo_name", "repo_id"}
	artifactLabelNames                        = []string{"project_name", "project_id", "repo_name", "repo_id", "artifact_name", "artifact_id", "tag"}
	artifactVulnerabilitiesLabelNames         = []string{"project_name", "project_id", "repo_name", "repo_id", "artifact_name", "artifact_id", "report_id", "status", "tag"}
	artifactsVulnerabilitiesScansLabelNames   = []string{"project_name", "project_id", "repo_name", "repo_id", "artifact_name", "artifact_id", "tag"}
	artifactVulnerabilitiesDurationLabelNames = []string{"project_name", "project_id", "repo_name", "repo_id", "artifact_name", "artifact_id", "report_id", "tag"}
	storageLabelNames                         = []string{"storage"}
	replicationLabelNames                     = []string{"repl_pol_name"}
	replicationTaskLabelNames                 = []string{"repl_pol_name", "result"}
	systemInfoLabelNames                      = []string{"auth_mode", "project_creation_restriction", "harbor_version", "registry_storage_provider_name"}
)

type metricInfo struct {
	Desc *prometheus.Desc
	Type prometheus.ValueType
}

func newMetricInfo(instanceName string, metricName string, docString string, t prometheus.ValueType, variableLabels []string, constLabels prometheus.Labels) metricInfo {
	return metricInfo{
		Desc: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, instanceName, metricName),
			docString,
			variableLabels,
			constLabels,
		),
		Type: t,
	}
}

func createMetrics(instanceName string) {
	allMetrics = make(map[string]metricInfo)

	allMetrics["up"] = newMetricInfo(instanceName, "up", "Was the last query of harbor successful.", prometheus.GaugeValue, nil, nil)
	allMetrics["health"] = newMetricInfo(instanceName, "health", "Harbor overall health status: Healthy = 1, Unhealthy = 0", prometheus.GaugeValue, nil, nil)
	allMetrics["components_health"] = newMetricInfo(instanceName, "components_health", "Harbor components health status: Healthy = 1, Unhealthy = 0", prometheus.GaugeValue, componentLabelNames, nil)
	allMetrics["health_latency"] = newMetricInfo(instanceName, "health_latency", "Time in seconds to collect health metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_total"] = newMetricInfo(instanceName, "scans_total", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_completed"] = newMetricInfo(instanceName, "scans_completed", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_requester"] = newMetricInfo(instanceName, "scans_requester", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_error"] = newMetricInfo(instanceName, "scans_error", "metrics of the current amount of errors during scan", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_success"] = newMetricInfo(instanceName, "scans_success", "metrics of the current amount of succeeded scans", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_latency"] = newMetricInfo(instanceName, "scans_latency", "Time in seconds to collect scan metrics", prometheus.GaugeValue, nil, nil)
        allMetrics["scans_error"] = newMetricInfo(instanceName, "scans_error", "Amount of failed scans", prometheus.GaugeValue, nil, nil)
	allMetrics["project_count_total"] = newMetricInfo(instanceName, "project_count_total", "projects number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	allMetrics["repo_count_total"] = newMetricInfo(instanceName, "repo_count_total", "repositories number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	allMetrics["statistics_latency"] = newMetricInfo(instanceName, "statistics_latency", "Time in seconds to collect statistics metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["quotas_count_total"] = newMetricInfo(instanceName, "quotas_count_total", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	allMetrics["quotas_size_bytes"] = newMetricInfo(instanceName, "quotas_size_bytes", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	allMetrics["quotas_latency"] = newMetricInfo(instanceName, "quotas_latency", "Time in seconds to collect quota metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["system_volumes_bytes"] = newMetricInfo(instanceName, "system_volumes_bytes", "Get system volume info (total/free size).", prometheus.GaugeValue, storageLabelNames, nil)
	allMetrics["system_volumes_latency"] = newMetricInfo(instanceName, "system_volumes_latency", "Time in seconds to collect system_volume metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["repositories_pull_total"] = newMetricInfo(instanceName, "repositories_pull_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["repositories_star_total"] = newMetricInfo(instanceName, "repositories_star_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["repositories_tags_total"] = newMetricInfo(instanceName, "repositories_tags_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["repositories_latency"] = newMetricInfo(instanceName, "repositories_latency", "Time in seconds to collect repository metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["artifacts_size"] = newMetricInfo(instanceName, "artifacts_size", "Size in bytes for uploaded artifacts", prometheus.GaugeValue, artifactLabelNames, nil)
	allMetrics["artifacts_vulnerabilities"] = newMetricInfo(instanceName, "artifacts_vulnerabilities", "Detected vulnerabilities for uploaded artifacts", prometheus.GaugeValue, artifactVulnerabilitiesLabelNames, nil)
	allMetrics["artifacts_vulnerabilities_scan_start"] = newMetricInfo(instanceName, "artifacts_vulnerabilities_scan_start", "Vulnerabilities scan start time", prometheus.GaugeValue, artifactVulnerabilitiesDurationLabelNames, nil)
	allMetrics["artifacts_vulnerabilities_scan_duration"] = newMetricInfo(instanceName, "artifacts_vulnerabilities_scan_duration", "Vulnerabilities scan duration", prometheus.GaugeValue, artifactVulnerabilitiesDurationLabelNames, nil)
	allMetrics["artifacts_vulnerabilities_scans"] = newMetricInfo(instanceName, "artifacts_vulnerabilities_scans", "Vulnerabilities scan operation status. Success == 1, running == 2; others == 0", prometheus.CounterValue, artifactsVulnerabilitiesScansLabelNames, nil)
	allMetrics["artifacts_latency"] = newMetricInfo(instanceName, "artifacts_latency", "Time in seconds to collect artifacts metrics", prometheus.GaugeValue, nil, nil)
	allMetrics["replication_status"] = newMetricInfo(instanceName, "replication_status", "Get status of the last execution of this replication policy: Succeed = 1, any other status = 0.", prometheus.GaugeValue, replicationLabelNames, nil)
	allMetrics["replication_tasks"] = newMetricInfo(instanceName, "replication_tasks", "Get number of replication tasks, with various results, in the latest execution of this replication policy.", prometheus.GaugeValue, replicationTaskLabelNames, nil)
	allMetrics["system_info"] = newMetricInfo(instanceName, "system_info", "A metric with a constant '1' value labeled by auth_mode, project_creation_restriction, harbor_version and registry_storage_provider_name from /systeminfo endpoint.", prometheus.GaugeValue, systemInfoLabelNames, nil)
	allMetrics["system_with_notary"] = newMetricInfo(instanceName, "system_with_notary", "If notary is used", prometheus.GaugeValue, nil, nil)
	allMetrics["system_self_registration"] = newMetricInfo(instanceName, "system_self_registration", "If self registration is enabled", prometheus.GaugeValue, nil, nil)
	allMetrics["system_has_ca_root"] = newMetricInfo(instanceName, "system_has_ca_root", "If harbor has a root ca", prometheus.GaugeValue, nil, nil)
	allMetrics["system_read_only"] = newMetricInfo(instanceName, "system_read_only", "If harbor is in read-only mode", prometheus.GaugeValue, nil, nil)
	allMetrics["system_with_chartmuseum"] = newMetricInfo(instanceName, "system_with_chartmuseum", "If harbor has chartmuseum enabled", prometheus.GaugeValue, nil, nil)
	allMetrics["system_notification_enable"] = newMetricInfo(instanceName, "system_notification_enable", "If notifications are enabled", prometheus.GaugeValue, nil, nil)
	allMetrics["replication_latency"] = newMetricInfo(instanceName, "replication_latency", "Time in seconds to collect replication metrics", prometheus.GaugeValue, nil, nil)
}

type promHTTPLogger struct {
	logger log.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	level.Error(l.logger).Log("msg", fmt.Sprint(v...))
}

// HarborExporter structure
// Connection info to harbor instance
type HarborExporter struct {
	instance string
	uri      string
	username string
	password string
	timeout  time.Duration
	insecure bool
	apiPath  string
	logger   log.Logger
	isV2     bool
	pageSize int
	client   *http.Client
	// Cache-related
	cacheEnabled    bool
	cacheDuration   time.Duration
	lastCollectTime time.Time
	cache           []prometheus.Metric
	collectMutex    sync.Mutex
}

// NewHarborExporter constructs a HarborExporter instance
func NewHarborExporter() *HarborExporter {
	return &HarborExporter{
		cache:           make([]prometheus.Metric, 0),
		lastCollectTime: time.Unix(0, 0),
		collectMutex:    sync.Mutex{},
	}
}

func getHTTPClient(skipVerify bool) (*http.Client, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	tlsClientConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}
	if skipVerify {
		tlsClientConfig.InsecureSkipVerify = true
	}
	transport := &http.Transport{
		TLSClientConfig: tlsClientConfig,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Second * 10,
	}
	return client, nil
}

func (h *HarborExporter) request(endpoint string) ([]byte, error) {
	body, _, err := h.fetch(endpoint)
	return body, err
}

func (h *HarborExporter) requestAll(endpoint string, callback func([]byte) error) error {
	page := 1
	separator := "?"
	if strings.Index(endpoint, separator) > 0 {
		separator = "&"
	}
	for {
		path := fmt.Sprintf("%s%spage=%d&page_size=%d", endpoint, separator, page, h.pageSize)
		body, headers, err := h.fetch(path)
		if err != nil {
			return err
		}

		err = callback(body)
		if err != nil {
			return err
		}

		countStr := headers.Get("x-total-count")
		if countStr == "" {
			break
		}

		count, err := strconv.Atoi(countStr)
		if err != nil {
			return err
		}

		if page*h.pageSize >= count {
			break
		}

		page++
	}
	return nil
}

func (h *HarborExporter) fetch(endpoint string) ([]byte, http.Header, error) {
	level.Debug(h.logger).Log("endpoint", endpoint)
	req, err := http.NewRequest("GET", h.uri+h.apiPath+endpoint, nil)
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return nil, nil, err
	}
	req.SetBasicAuth(h.username, h.password)

	resp, err := h.client.Do(req)
	if err != nil {
		level.Error(h.logger).Log("msg", "Error handling request for "+endpoint, "err", err.Error())
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		level.Error(h.logger).Log("msg", "Error handling request for "+endpoint, "http-statuscode", resp.Status)
		return nil, nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		level.Error(h.logger).Log("msg", "Error reading response of request for "+endpoint, "err", err.Error())
		return nil, nil, err
	}
	return body, resp.Header, nil
}

func checkHarborVersion(h *HarborExporter) error {
	resp, err := h.client.Get(h.uri + "/api/systeminfo")
	if err == nil {
		level.Info(h.logger).Log("msg", "check v1 with /api/systeminfo", "code", resp.StatusCode)
		if resp.StatusCode == 200 {
			h.apiPath = "/api"
			h.isV2 = false
		}
	} else {
		level.Info(h.logger).Log("msg", "check v1 with /api/systeminfo", "err", err)
	}

	resp, err = h.client.Get(h.uri + "/api/v2.0/systeminfo")
	if err == nil {
		level.Info(h.logger).Log("msg", "check v2 with /api/v2.0/systeminfo", "code", resp.StatusCode)
		if resp.StatusCode == 200 {
			h.apiPath = "/api/v2.0"
			h.isV2 = true
		}
	} else {
		level.Info(h.logger).Log("msg", "check v2 with /api/v2.0/systeminfo", "erro", err)
	}

	if h.apiPath == "" {
		return errors.New("unable to determine harbor version")
	}
	return nil
}

func reportLatency(start time.Time, metric string, ch chan<- prometheus.Metric) {
	end := time.Now()
	latency := end.Sub(start).Seconds()
	ch <- prometheus.MustNewConstMetric(
		allMetrics[metric].Desc, allMetrics[metric].Type, latency,
	)
}

// Describe describes all the metrics ever exported by the Harbor exporter. It
// implements prometheus.Collector.
func (h *HarborExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range allMetrics {
		ch <- m.Desc
	}
}

// Collect fetches the stats from configured Harbor location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (h *HarborExporter) Collect(outCh chan<- prometheus.Metric) {
	if h.cacheEnabled {
		h.collectMutex.Lock()
		defer h.collectMutex.Unlock()
		expiry := h.lastCollectTime.Add(h.cacheDuration)
		if time.Now().Before(expiry) {
			// Return cached
			for _, cachedMetric := range h.cache {
				outCh <- cachedMetric
			}
			return
		}
		// Reset cache for fresh sampling, but re-use underlying array
		h.cache = h.cache[:0]
	}

	samplesCh := make(chan prometheus.Metric)
	// Use WaitGroup to ensure outCh isn't closed before the goroutine is finished
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for metric := range samplesCh {
			outCh <- metric
			if h.cacheEnabled {
				h.cache = append(h.cache, metric)
			}
		}
		wg.Done()
	}()

	ok := true
	if collectMetricsGroup[metricsGroupHealth] {
		ok = h.collectHealthMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupScans] {
		ok = h.collectScanMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupStatistics] {
		ok = h.collectStatisticsMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupStatistics] {
		ok = h.collectSystemVolumesMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupQuotas] {
		ok = h.collectQuotasMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupRepositories] {
		ok = h.collectRepositoriesMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupReplication] {
		ok = h.collectReplicationsMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupSystemInfo] {
		ok = h.collectSystemMetric(samplesCh)
	}
	if collectMetricsGroup[metricsGroupArtifactsInfo] {
		ok = h.collectArtifactsMetric(samplesCh) && ok
	}

	if ok {
		samplesCh <- prometheus.MustNewConstMetric(
			allMetrics["up"].Desc, allMetrics["up"].Type,
			1.0,
		)
	} else {
		samplesCh <- prometheus.MustNewConstMetric(
			allMetrics["up"].Desc, allMetrics["up"].Type,
			0.0,
		)
	}

	close(samplesCh)
	h.lastCollectTime = time.Now()
	wg.Wait()
}

// Status2i converts health status to int8
func Status2i(s string) int8 {
	if s == "healthy" {
		return 1
	}
	return 0
}

// Btoi converts bool to int8
func Btoi(b bool) int8 {
	if b {
		return 1
	}
	return 0
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9107").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		exporter      = NewHarborExporter()
	)

	kingpin.Flag("harbor.instance", "Logical name for the Harbor instance to monitor").Envar("HARBOR_INSTANCE").Default("").StringVar(&exporter.instance)
	kingpin.Flag("harbor.server", "HTTP API address of a harbor server or agent. (prefix with https:// to connect over HTTPS)").Envar("HARBOR_URI").Default("http://localhost:8500").StringVar(&exporter.uri)
	kingpin.Flag("harbor.username", "username").Envar("HARBOR_USERNAME").Default("admin").StringVar(&exporter.username)
	kingpin.Flag("harbor.password", "password").Envar("HARBOR_PASSWORD").Default("password").StringVar(&exporter.password)
	kingpin.Flag("harbor.timeout", "Timeout on HTTP requests to the harbor API.").Default("500ms").DurationVar(&exporter.timeout)
	kingpin.Flag("harbor.insecure", "Disable TLS host verification.").Default("false").BoolVar(&exporter.insecure)
	kingpin.Flag("harbor.pagesize", "Page size on requests to the harbor API.").Envar("HARBOR_PAGESIZE").Default("100").IntVar(&exporter.pageSize)
	skip := kingpin.Flag("skip.metrics", "Skip these metrics groups").Enums(metricsGroupValues()...)
	kingpin.Flag("cache.enabled", "Enable metrics caching.").Envar("HARBOR_CACHE_ENABLED").Default("false").BoolVar(&exporter.cacheEnabled)
	kingpin.Flag("cache.duration", "Time duration collected values are cached for.").Envar("HARBOR_CACHE_DURATION").Default("20s").DurationVar(&exporter.cacheDuration)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	client, err := getHTTPClient(exporter.insecure)
	if err != nil {
		level.Error(logger).Log("msg", "Failed to create HTTP client")
		os.Exit(1)
	}
	exporter.client = client

	level.Info(logger).Log("CacheEnabled", exporter.cacheEnabled)
	level.Info(logger).Log("CacheDuration", exporter.cacheDuration)

	level.Info(logger).Log("msg", "Starting harbor_exporter", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	collectMetricsGroup = make(map[string]bool)
	for _, v := range metricsGroupValues() {
		collectMetricsGroup[v] = true
	}
	for _, v := range *skip {
		level.Debug(logger).Log("skip", v)
		collectMetricsGroup[v] = false
	}
	for k, v := range collectMetricsGroup {
		level.Info(logger).Log("metrics_group", k, "collect", v)
	}

	exporter.logger = logger

	err = checkHarborVersion(exporter)
	if err != nil {
		level.Error(logger).Log("msg", "cannot get harbor api version", "err", err)
		os.Exit(1)
	}

	createMetrics(exporter.instance)

	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("harbor_exporter"))

	http.Handle(*metricsPath, promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Harbor Exporter</title></head>
             <body>
             <h1>harbor Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             <h2>Build</h2>
             <pre>` + version.Info() + ` ` + version.BuildContext() + `</pre>
             </body>
             </html>`))
	})
	http.HandleFunc("/-/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})
	http.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	level.Info(logger).Log("msg", "Listening on address", "address", *listenAddress)
	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		level.Error(logger).Log("msg", "Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}
