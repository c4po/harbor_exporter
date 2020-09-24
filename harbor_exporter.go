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

	// "strings"
	// "net/url"

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
	metricsGroupHealth       = "health"
	metricsGroupScans        = "scans"
	metricsGroupStatistics   = "statistics"
	metricsGroupQuotas       = "quotas"
	metricsGroupRepositories = "repositories"
	metricsGroupReplication  = "replication"
)

func MetricsGroup_Values() []string {
	return []string{
		metricsGroupHealth,
		metricsGroupScans,
		metricsGroupStatistics,
		metricsGroupQuotas,
		metricsGroupRepositories,
		metricsGroupReplication,
	}
}

var (
	allMetrics          map[string]metricInfo
	collectMetricsGroup map[string]bool

	componentLabelNames       = []string{"component"}
	typeLabelNames            = []string{"type"}
	quotaLabelNames           = []string{"type", "repo_name", "repo_id"}
	serverLabelNames          = []string{"storage"}
	repoLabelNames            = []string{"repo_name", "repo_id"}
	storageLabelNames         = []string{"storage"}
	replicationLabelNames     = []string{"repl_pol_name"}
	replicationTaskLabelNames = []string{"repl_pol_name", "result"}
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
	allMetrics["scans_total"] = newMetricInfo(instanceName, "scans_total", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_completed"] = newMetricInfo(instanceName, "scans_completed", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["scans_requester"] = newMetricInfo(instanceName, "scans_requester", "metrics of the latest scan all process", prometheus.GaugeValue, nil, nil)
	allMetrics["project_count_total"] = newMetricInfo(instanceName, "project_count_total", "projects number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	allMetrics["repo_count_total"] = newMetricInfo(instanceName, "repo_count_total", "repositories number relevant to the user", prometheus.GaugeValue, typeLabelNames, nil)
	allMetrics["quotas_count_total"] = newMetricInfo(instanceName, "quotas_count_total", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	allMetrics["quotas_size_bytes"] = newMetricInfo(instanceName, "quotas_size_bytes", "quotas", prometheus.GaugeValue, quotaLabelNames, nil)
	allMetrics["system_volumes_bytes"] = newMetricInfo(instanceName, "system_volumes_bytes", "Get system volume info (total/free size).", prometheus.GaugeValue, storageLabelNames, nil)
	allMetrics["repositories_pull_total"] = newMetricInfo(instanceName, "repositories_pull_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["repositories_star_total"] = newMetricInfo(instanceName, "repositories_star_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["repositories_tags_total"] = newMetricInfo(instanceName, "repositories_tags_total", "Get public repositories which are accessed most.).", prometheus.GaugeValue, repoLabelNames, nil)
	allMetrics["replication_status"] = newMetricInfo(instanceName, "replication_status", "Get status of the last execution of this replication policy: Succeed = 1, any other status = 0.", prometheus.GaugeValue, replicationLabelNames, nil)
	allMetrics["replication_tasks"] = newMetricInfo(instanceName, "replication_tasks", "Get number of replication tasks, with various results, in the latest execution of this replication policy.", prometheus.GaugeValue, replicationTaskLabelNames, nil)
}

type promHTTPLogger struct {
	logger log.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	level.Error(l.logger).Log("msg", fmt.Sprint(v...))
}

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
	// Cache-releated
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

func getHttpClient(skipVerify bool) (*http.Client, error) {
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

func (h HarborExporter) request(endpoint string) ([]byte, error) {
	body, _, err := h.fetch(endpoint)
	return body, err
}

func (h HarborExporter) requestAll(endpoint string, callback func([]byte) error) error {
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

func (h HarborExporter) fetch(endpoint string) ([]byte, http.Header, error) {
	level.Debug(h.logger).Log("endpoint", endpoint)
	req, err := http.NewRequest("GET", h.uri+h.apiPath+endpoint, nil)
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return nil, nil, err
	}
	req.SetBasicAuth(h.username, h.password)

	client, err := getHttpClient(h.insecure)
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return nil, nil, err
	}

	resp, err := client.Do(req)
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
	client, err := getHttpClient(h.insecure)
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return err
	}

	resp, err := client.Get(h.uri + "/api/systeminfo")
	if err == nil {
		level.Info(h.logger).Log("msg", "check v1 with /api/systeminfo", "code", resp.StatusCode)
		if resp.StatusCode == 200 {
			h.apiPath = "/api"
			h.isV2 = false
		}
	} else {
		level.Info(h.logger).Log("msg", "check v1 with /api/systeminfo", "err", err)
	}

	resp, err = client.Get(h.uri + "/api/v2.0/systeminfo")
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

// Describe describes all the metrics ever exported by the Harbor exporter. It
// implements prometheus.Collector.
func (e *HarborExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range allMetrics {
		ch <- m.Desc
	}
}

// Collect fetches the stats from configured Harbor location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *HarborExporter) Collect(outCh chan<- prometheus.Metric) {
	if e.cacheEnabled {
		e.collectMutex.Lock()
		defer e.collectMutex.Unlock()
		expiry := e.lastCollectTime.Add(e.cacheDuration)
		if time.Now().Before(expiry) {
			// Return cached
			for _, cachedMetric := range e.cache {
				outCh <- cachedMetric
			}
			return
		}
		// Reset cache for fresh sampling, but re-use underlying array
		e.cache = e.cache[:0]
	}

	samplesCh := make(chan prometheus.Metric)
	// Use WaitGroup to ensure outCh isn't closed before the goroutine is finished
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for metric := range samplesCh {
			outCh <- metric
			if e.cacheEnabled {
				e.cache = append(e.cache, metric)
			}
		}
		wg.Done()
	}()

	ok := true
	if collectMetricsGroup[metricsGroupHealth] {
		ok = e.collectHealthMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupScans] {
		ok = e.collectScanMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupStatistics] {
		ok = e.collectStatisticsMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupStatistics] {
		ok = e.collectSystemVolumesMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupQuotas] {
		ok = e.collectQuotasMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupRepositories] {
		ok = e.collectRepositoriesMetric(samplesCh) && ok
	}
	if collectMetricsGroup[metricsGroupReplication] {
		ok = e.collectReplicationsMetric(samplesCh) && ok
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
	e.lastCollectTime = time.Now()
	wg.Wait()
}

// Status2i converts health status to int8
func Status2i(s string) int8 {
	if s == "healthy" {
		return 1
	}
	return 0
}

func main() {
	var (
		listenAddress  = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9107").String()
		metricsPath    = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()
		harborInstance = NewHarborExporter()
	)

	kingpin.Flag("harbor.instance", "Logical name for the Harbor instance to monitor").Envar("HARBOR_INSTANCE").Default("").StringVar(&harborInstance.instance)
	kingpin.Flag("harbor.server", "HTTP API address of a harbor server or agent. (prefix with https:// to connect over HTTPS)").Envar("HARBOR_URI").Default("http://localhost:8500").StringVar(&harborInstance.uri)
	kingpin.Flag("harbor.username", "username").Envar("HARBOR_USERNAME").Default("admin").StringVar(&harborInstance.username)
	kingpin.Flag("harbor.password", "password").Envar("HARBOR_PASSWORD").Default("password").StringVar(&harborInstance.password)
	kingpin.Flag("harbor.timeout", "Timeout on HTTP requests to the harbor API.").Default("500ms").DurationVar(&harborInstance.timeout)
	kingpin.Flag("harbor.insecure", "Disable TLS host verification.").Default("false").BoolVar(&harborInstance.insecure)
	kingpin.Flag("harbor.page.size", "Page size on requests to the harbor API.").Default("50").IntVar(&harborInstance.pageSize)
	skip := kingpin.Flag("skip.metrics", "Skip these metrics groups").Enums(MetricsGroup_Values()...)
	kingpin.Flag("cache.enabled", "Enable metrics caching.").Default("false").BoolVar(&harborInstance.cacheEnabled)
	kingpin.Flag("cache.duration", "Time duration collected values are cached for.").Default("20s").DurationVar(&harborInstance.cacheDuration)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("CacheEnabled", harborInstance.cacheEnabled)
	level.Info(logger).Log("CacheDuration", harborInstance.cacheDuration)

	level.Info(logger).Log("msg", "Starting harbor_exporter", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	collectMetricsGroup = make(map[string]bool)
	for _, v := range MetricsGroup_Values() {
		collectMetricsGroup[v] = true
	}
	for _, v := range *skip {
		level.Debug(logger).Log("skip", v)
		collectMetricsGroup[v] = false
	}
	for k, v := range collectMetricsGroup {
		level.Info(logger).Log("metrics_group", k, "collect", v)
	}

	harborInstance.logger = logger

	err := checkHarborVersion(harborInstance)
	if err != nil {
		level.Error(logger).Log("msg", "cannot get harbor api version", "err", err)
		os.Exit(1)
	}

	createMetrics(harborInstance.instance)

	prometheus.MustRegister(harborInstance)
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
