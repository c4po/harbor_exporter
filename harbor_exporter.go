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
	"fmt"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"

	// "github.com/pkg/errors"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	// "regexp"
	"io/ioutil"
	"strings"
	"time"
)

const (
	namespace = "harbor"
)

var (
	up,
	scanTotalCount,
	scanCompletedCount,
	scanRequesterCount,
	projectCount,
	repoCount,
	quotasCount,
	quotasSize,
	systemVolumes,
	repositoriesPullCount,
	repositoriesStarCount,
	repositoriesTagsCount,
	replicationStatus,
	replicationTasks *prometheus.Desc
)

type promHTTPLogger struct {
	logger log.Logger
}

func (l promHTTPLogger) Println(v ...interface{}) {
	level.Error(l.logger).Log("msg", fmt.Sprint(v...))
}

// Exporter collects Consul stats from the given server and exports them using
// the prometheus metrics package.
type Exporter struct {
	client HarborClient
	opts   harborOpts
	logger log.Logger
}

type harborOpts struct {
	instance string
	uri      string
	username string
	password string
	timeout  time.Duration
	insecure bool
	version  string
}

type HarborClient struct {
	client *http.Client
	opts   harborOpts
	logger log.Logger
}

func (h HarborClient) request(endpoint string) []byte {
	req, err := http.NewRequest("GET", h.opts.uri+h.opts.version+endpoint, nil)
	if err != nil {
		level.Error(h.logger).Log(err.Error())
		return nil
	}
	req.SetBasicAuth(h.opts.username, h.opts.password)

	resp, err := h.client.Do(req)
	if err != nil {
		level.Error(h.logger).Log("msg", "Error handling request for "+endpoint, "err", err.Error())
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		level.Error(h.logger).Log("msg", "Error handling request for "+endpoint, "http-statuscode", resp.Status)
		return nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		level.Error(h.logger).Log("msg", "Error reading response of request for "+endpoint, "err", err.Error())
		return nil
	}
	return body
}

// NewExporter returns an initialized Exporter.
func NewExporter(opts harborOpts, logger log.Logger) (*Exporter, error) {
	uri := opts.uri
	if !strings.Contains(uri, "://") {
		uri = "http://" + uri
	}
	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid harbor URL: %s", err)
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid harbor URL: %s", uri)
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	tlsClientConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}
	if opts.insecure {
		tlsClientConfig.InsecureSkipVerify = true
	}

	transport := &http.Transport{
		TLSClientConfig: tlsClientConfig,
	}

	client := &http.Client{
		Transport: transport,
	}

	resp, err := client.Get(uri + "/api/systeminfo")
	if err == nil {
		level.Info(logger).Log("msg", "check v1 with /api/systeminfo", "code", resp.StatusCode)
		if resp.StatusCode == 200 {
			opts.version = "/api"
		}
	} else {
		level.Info(logger).Log("msg", "check v1 with /api/systeminfo", "err", err)
	}

	resp, err = client.Get(uri + "/api/v2.0/systeminfo")
	if err == nil {
		level.Info(logger).Log("msg", "check v2 with /api/v2.0/systeminfo", "code", resp.StatusCode)
		if resp.StatusCode == 200 {
			opts.version = "/api/v2.0"
		}
	} else {
		level.Info(logger).Log("msg", "check v2 with /api/v2.0/systeminfo", "erro", err)
	}

	if opts.version == "" {
		return nil, fmt.Errorf("unable to determine harbor version")
	}

	hc := HarborClient{client, opts, logger}

	// Init Prometheus Descriptors
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "up"),
		"Was the last query of harbor successful.",
		nil, nil,
	)
	scanTotalCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "scans_total"),
		"metrics of the latest scan all process",
		nil, nil,
	)
	scanCompletedCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "scans_completed"),
		"metrics of the latest scan all process",
		nil, nil,
	)
	scanRequesterCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "scans_requester"),
		"metrics of the latest scan all process",
		nil, nil,
	)
	projectCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "project_count_total"),
		"projects number relevant to the user",
		[]string{"type"}, nil,
	)
	repoCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "repo_count_total"),
		"repositories number relevant to the user",
		[]string{"type"}, nil,
	)
	quotasCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "quotas_count_total"),
		"quotas",
		[]string{"type", "repo_name", "repo_id"}, nil,
	)
	quotasSize = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "quotas_size_bytes"),
		"quotas",
		[]string{"type", "repo_name", "repo_id"}, nil,
	)
	systemVolumes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "system_volumes_bytes"),
		"Get system volume info (total/free size).",
		[]string{"storage"}, nil,
	)
	repositoriesPullCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "repositories_pull_total"),
		"Get public repositories which are accessed most.).",
		[]string{"repo_name", "repo_id"}, nil,
	)
	repositoriesStarCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "repositories_star_total"),
		"Get public repositories which are accessed most.).",
		[]string{"repo_name", "repo_id"}, nil,
	)
	repositoriesTagsCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "repositories_tags_total"),
		"Get public repositories which are accessed most.).",
		[]string{"repo_name", "repo_id"}, nil,
	)
	replicationStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "replication_status"),
		"Get status of the last execution of this replication policy: Succeed = 1, any other status = 0.",
		[]string{"repl_pol_name"}, nil,
	)
	replicationTasks = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, opts.instance, "replication_tasks"),
		"Get number of replication tasks, with various results, in the latest execution of this replication policy.",
		[]string{"repl_pol_name", "result"}, nil,
	)

	// Init our exporter.
	return &Exporter{
		client: hc,
		opts:   opts,
		logger: logger,
	}, nil
}

// Describe describes all the metrics ever exported by the Consul exporter. It
// implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- scanTotalCount
	ch <- scanCompletedCount
	ch <- scanRequesterCount
	ch <- projectCount
	ch <- repoCount
	ch <- quotasCount
	ch <- quotasSize
	ch <- systemVolumes
	ch <- repositoriesPullCount
	ch <- repositoriesStarCount
	ch <- repositoriesTagsCount
	ch <- replicationStatus
	ch <- replicationTasks
}

// Collect fetches the stats from configured Consul location and delivers them
// as Prometheus metrics. It implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	ok := e.collectScanMetric(ch)
	ok = e.collectStatisticsMetric(ch) && ok
	ok = e.collectQuotasMetric(ch) && ok
	ok = e.collectSystemVolumesMetric(ch) && ok
	ok = e.collectRepositoriesMetric(ch) && ok
	ok = e.collectReplicationsMetric(ch) && ok

	if ok {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 1.0,
		)
	} else {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0.0,
		)
	}
}

func init() {
	prometheus.MustRegister(version.NewCollector("harbor_exporter"))
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9107").String()
		metricsPath   = kingpin.Flag("web.telemetry-path", "Path under which to expose metrics.").Default("/metrics").String()

		opts = harborOpts{}
	)
	kingpin.Flag("harbor.instance", "Logical name for the Harbor instance to monitor").Envar("HARBOR_INSTANCE").Default("").StringVar(&opts.instance)
	kingpin.Flag("harbor.server", "HTTP API address of a harbor server or agent. (prefix with https:// to connect over HTTPS)").Envar("HARBOR_URI").Default("http://localhost:8500").StringVar(&opts.uri)
	kingpin.Flag("harbor.username", "username").Envar("HARBOR_USERNAME").Default("admin").StringVar(&opts.username)
	kingpin.Flag("harbor.password", "password").Envar("HARBOR_PASSWORD").Default("password").StringVar(&opts.password)
	kingpin.Flag("harbor.timeout", "Timeout on HTTP requests to the harbor API.").Default("500ms").DurationVar(&opts.timeout)
	kingpin.Flag("harbor.insecure", "Disable TLS host verification.").Default("false").BoolVar(&opts.insecure)

	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	level.Info(logger).Log("msg", "Starting harbor_exporter", "version", version.Info())
	level.Info(logger).Log("build_context", version.BuildContext())

	exporter, err := NewExporter(opts, logger)
	if err != nil {
		level.Error(logger).Log("msg", "Error creating the exporter", "err", err)
		os.Exit(1)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath,
		promhttp.InstrumentMetricHandler(
			prometheus.DefaultRegisterer,
			promhttp.HandlerFor(
				prometheus.DefaultGatherer,
				promhttp.HandlerOpts{
					ErrorLog: &promHTTPLogger{
						logger: logger,
					},
				},
			),
		),
	)
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
