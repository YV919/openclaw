package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultReleasesURL = "https://cnb.cool/dmxapi/openclaw_config/-/releases"

var releaseTagPattern = regexp.MustCompile(`/releases/tag/(v?\d+\.\d+\.\d+)`)

type releaseUpdateStatus struct {
	CurrentVersion string
	LatestVersion  string
	ReleasesURL    string
	HasUpdate      bool
}

func displayVersion(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return strings.TrimPrefix(trimmed, "v")
}

func checkForUpdates(client *http.Client, currentVersion string, releaseURL string) (releaseUpdateStatus, error) {
	status := releaseUpdateStatus{
		CurrentVersion: displayVersion(currentVersion),
		ReleasesURL:    releaseURL,
	}

	latestVersion, err := fetchLatestReleaseVersion(client, releaseURL)
	if err != nil {
		return status, err
	}

	status.LatestVersion = latestVersion
	status.HasUpdate = isVersionNewer(latestVersion, status.CurrentVersion)
	return status, nil
}

func fetchLatestReleaseVersion(client *http.Client, releaseURL string) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "openclaw-config/"+displayVersion(Version))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("release page returned %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	return parseLatestReleaseVersion(string(body))
}

func parseLatestReleaseVersion(page string) (string, error) {
	match := releaseTagPattern.FindStringSubmatch(page)
	if len(match) < 2 {
		return "", fmt.Errorf("latest release tag not found")
	}
	return displayVersion(match[1]), nil
}

func buildUpdateStatusLine(status releaseUpdateStatus) string {
	if status.CurrentVersion == "" || status.LatestVersion == "" {
		return ""
	}
	if status.HasUpdate {
		return fmt.Sprintf("更新检查  ·  发现新版本 %s（当前 %s）  ·  %s", status.LatestVersion, status.CurrentVersion, status.ReleasesURL)
	}
	return fmt.Sprintf("更新检查  ·  已是最新版本 %s", status.CurrentVersion)
}

func isVersionNewer(latest string, current string) bool {
	return compareVersions(latest, current) > 0
}

func compareVersions(left string, right string) int {
	leftParts, leftOK := parseVersionParts(left)
	rightParts, rightOK := parseVersionParts(right)
	if !leftOK || !rightOK {
		return 0
	}

	size := len(leftParts)
	if len(rightParts) > size {
		size = len(rightParts)
	}

	for i := 0; i < size; i++ {
		var leftPart int
		if i < len(leftParts) {
			leftPart = leftParts[i]
		}

		var rightPart int
		if i < len(rightParts) {
			rightPart = rightParts[i]
		}

		switch {
		case leftPart > rightPart:
			return 1
		case leftPart < rightPart:
			return -1
		}
	}

	return 0
}

func parseVersionParts(raw string) ([]int, bool) {
	trimmed := displayVersion(raw)
	if trimmed == "" {
		return nil, false
	}

	chunks := strings.Split(trimmed, ".")
	parts := make([]int, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk == "" {
			return nil, false
		}

		value, err := strconv.Atoi(chunk)
		if err != nil {
			return nil, false
		}
		parts = append(parts, value)
	}

	return parts, true
}
