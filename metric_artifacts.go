package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
)

type project struct {
	ProjectID int64  `json:"project_id,omitempty"`
	Name      string `json:"name,omitempty"`

	repositories repositories
}

type projects []project

type repository struct {
	ID   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`

	artifacts artifacts
}

type repositories []repository

type scanOverview struct {
	Duration   int       `json:"duration"`
	EndTime    time.Time `json:"end_time"`
	ReportID   string    `json:"report_id"`
	ScanStatus string    `json:"scan_status"`
	Severity   string    `json:"severity"`
	StartTime  time.Time `json:"start_time"`
	Summary    struct {
		Fixable int `json:"fixable"`
		Summary struct {
			High   int `json:"High"`
			Low    int `json:"Low"`
			Medium int `json:"Medium"`
		} `json:"summary"`
		Total int `json:"total"`
	} `json:"summary"`
}

type artifact struct {
	Digest       string       `json:"digest"`
	ID           int64        `json:"id"`
	ProjectID    int64        `json:"project_id"`
	RepositoryID int64        `json:"repository_id"`
	ScanOverview scanOverview `json:"scan_overview"`
	Size         int64        `json:"size"`
	Tags         []struct {
		ArtifactID   int64  `json:"artifact_id"`
		ID           int64  `json:"id"`
		Immutable    bool   `json:"immutable"`
		Name         string `json:"name"`
		RepositoryID int64  `json:"repository_id"`
		Signed       bool   `json:"signed"`
	} `json:"tags"`
	Type string `json:"type"`
}

type artifacts []artifact

func (h *HarborExporter) collectArtifactsMetric(ch chan<- prometheus.Metric) bool {
	// Do not load V1 API.
	// ToDo: Implement V1 support.
	if !h.isV2 {
		return true
	}

	start := time.Now()

	// Load Projects.
	prData, err := h.loadProjects()
	if err != nil {
		return false
	}

	// Load Repositories.
	prData, err = h.loadRepositories(prData)
	if err != nil {
		return false
	}

	// Make metrics.
	var (
		sizeMI     = allMetrics["artifacts_size"]
		vulnMI     = allMetrics["artifacts_vulnerabilities"]
		scansMI    = allMetrics["artifacts_vulnerabilities_scans"]
		scansDurMI = allMetrics["artifacts_vulnerabilities_scan_duration"]
	)

	for pi := range prData {
		var (
			pp = &prData[pi]

			projectName = pp.Name
			projectID   = strconv.FormatInt(pp.ProjectID, 10)
		)

		for ri := range pp.repositories {
			var (
				rp = &pp.repositories[ri]

				repoName = rp.Name
				repoID   = strconv.FormatInt(rp.ID, 10)
			)

			for ai := range rp.artifacts {
				var (
					ap = &rp.artifacts[ai]

					artID   = strconv.FormatInt(ap.ID, 10)
					artName = ap.Digest
				)

				// Size.
				ch <- prometheus.MustNewConstMetric(sizeMI.Desc, sizeMI.Type, float64(ap.Size), projectName, projectID, repoName, repoID, artName, artID)

				// Vulnerabilities.
				var scanInfo = &ap.ScanOverview

				// No scan performed.
				var reportID = scanInfo.ReportID

				if reportID == "" {
					continue
				}

				ch <- prometheus.MustNewConstMetric(vulnMI.Desc, vulnMI.Type, float64(scanInfo.Summary.Fixable), projectName, projectID, repoName, repoID, artName, artID, reportID, "fixable")
				ch <- prometheus.MustNewConstMetric(vulnMI.Desc, vulnMI.Type, float64(scanInfo.Summary.Total), projectName, projectID, repoName, repoID, artName, artID, reportID, "total")
				ch <- prometheus.MustNewConstMetric(vulnMI.Desc, vulnMI.Type, float64(scanInfo.Summary.Summary.Low), projectName, projectID, repoName, repoID, artName, artID, reportID, "low")
				ch <- prometheus.MustNewConstMetric(vulnMI.Desc, vulnMI.Type, float64(scanInfo.Summary.Summary.Medium), projectName, projectID, repoName, repoID, artName, artID, reportID, "medium")
				ch <- prometheus.MustNewConstMetric(vulnMI.Desc, vulnMI.Type, float64(scanInfo.Summary.Summary.High), projectName, projectID, repoName, repoID, artName, artID, reportID, "high")

				// Scan Status.
				ch <- prometheus.MustNewConstMetric(scansDurMI.Desc, scansDurMI.Type, float64(scanInfo.Duration), projectName, projectID, repoName, repoID, artName, artID, reportID, "duration_sec")
				ch <- prometheus.MustNewConstMetric(scansDurMI.Desc, scansDurMI.Type, float64(scanInfo.StartTime.Unix()), projectName, projectID, repoName, repoID, artName, artID, reportID, "start_time")

				var scanRes float64

				switch strings.ToLower(scanInfo.ScanStatus) {
				case "success":
					scanRes = 1
				case "running":
					scanRes = 2
				}

				ch <- prometheus.MustNewConstMetric(scansMI.Desc, scansMI.Type, scanRes, projectName, projectID, repoName, repoID, artName, artID)
			}
		}
	}

	reportLatency(start, "artifacts_latency", ch)

	return true
}

func (h *HarborExporter) loadProjects() (projects, error) {
	// Load Projects.
	var projectsData projects

	err := h.requestAll("/projects", func(pageBody []byte) error {
		var pageData projects
		if err := json.Unmarshal(pageBody, &pageData); err != nil {
			return err
		}

		projectsData = append(projectsData, pageData...)

		return nil
	})
	if err != nil {
		level.Error(h.logger).Log(err.Error())

		return nil, err
	}

	return projectsData, nil
}

func (h *HarborExporter) loadRepositories(projectsData projects) (projects, error) {
	// Load Repositories for Projects.
	for i := range projectsData {
		projectID := strconv.FormatInt(projectsData[i].ProjectID, 10)

		var reqURL string
		if h.isV2 {
			reqURL = "/projects/" + projectsData[i].Name + "/repositories"
		} else {
			reqURL = "/repositories?project_id=" + projectID
		}

		var pageData repositories
		err := h.requestAll(reqURL, func(pageBody []byte) error {
			if err := json.Unmarshal(pageBody, &pageData); err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			level.Error(h.logger).Log(err.Error())

			return nil, err
		}

		// Load Artifacts for Repositories.
		af, err := h.loadArtifacts(projectsData[i].Name, pageData)
		if err != nil {
			level.Error(h.logger).Log(err.Error())

			return nil, err
		}

		// Save.
		projectsData[i].repositories = af
	}

	return projectsData, nil
}

func (h *HarborExporter) loadArtifacts(projectName string, repoData repositories) (repositories, error) {
	type rawArtifacts []struct {
		Digest       string                  `json:"digest"`
		ID           int64                   `json:"id"`
		ProjectID    int64                   `json:"project_id"`
		RepositoryID int64                   `json:"repository_id"`
		ScanOverview map[string]scanOverview `json:"scan_overview"`
		Size         int64                   `json:"size"`
		Tags         []struct {
			ArtifactID   int64  `json:"artifact_id"`
			ID           int64  `json:"id"`
			Immutable    bool   `json:"immutable"`
			Name         string `json:"name"`
			RepositoryID int64  `json:"repository_id"`
			Signed       bool   `json:"signed"`
		} `json:"tags"`
		Type string `json:"type"`
	}

	for i := range repoData {
		var reqURL string
		if h.isV2 {
			reqURL = "/projects/" + projectName +
				"/repositories" + strings.TrimLeft(repoData[i].Name, projectName) +
				"/artifacts?with_tag=true&with_scan_overview=true"
		} else {
			panic("No v1 API support")
		}

		if err := h.requestAll(reqURL, func(b []byte) error {
			var pageData rawArtifacts

			if err := json.Unmarshal(b, &pageData); err != nil {
				return err
			}

			// Convert.
			var repoArts artifacts

			for pi := range pageData {
				pp := &pageData[pi]

				// Parse Scan Overview Convert.
				var parsedScanOverview scanOverview

				for _, v := range pp.ScanOverview {
					parsedScanOverview = v

					break
				}

				repoArts = append(repoArts, artifact{
					Digest:       pp.Digest,
					ID:           pp.ID,
					ProjectID:    pp.ProjectID,
					RepositoryID: pp.RepositoryID,
					Size:         pp.Size,
					Tags:         pp.Tags,
					Type:         pp.Type,
					ScanOverview: parsedScanOverview,
				})
			}

			repoData[i].artifacts = repoArts

			return nil
		}); err != nil {
			level.Error(h.logger).Log(err.Error())

			return nil, err
		}
	}

	return repoData, nil
}
