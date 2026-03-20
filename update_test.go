package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisplayVersionStripsReleasePrefix(t *testing.T) {
	got := displayVersion("v1.2.2")
	want := "1.2.2"

	if got != want {
		t.Fatalf("displayVersion() = %q, want %q", got, want)
	}
}

func TestCheckForUpdatesDetectsNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases" {
			t.Fatalf("request path = %q, want %q", r.URL.Path, "/releases")
		}
		_, _ = w.Write([]byte(`
			<html>
				<body>
					<a href="/dmxapi/openclaw_config/-/releases/tag/v1.2.3">OpenClaw Config v1.2.3</a>
					<a href="/dmxapi/openclaw_config/-/releases/tag/v1.2.2">OpenClaw Config v1.2.2</a>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	got, err := checkForUpdates(server.Client(), "1.2.2", server.URL+"/releases")
	if err != nil {
		t.Fatalf("checkForUpdates() error = %v", err)
	}

	if got.CurrentVersion != "1.2.2" {
		t.Fatalf("CurrentVersion = %q, want %q", got.CurrentVersion, "1.2.2")
	}
	if got.LatestVersion != "1.2.3" {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, "1.2.3")
	}
	if !got.HasUpdate {
		t.Fatal("HasUpdate = false, want true")
	}
}

func TestCheckForUpdatesMarksCurrentVersionAsLatest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
			<html>
				<body>
					<a href="/dmxapi/openclaw_config/-/releases/tag/v1.2.2">OpenClaw Config v1.2.2</a>
				</body>
			</html>
		`))
	}))
	defer server.Close()

	got, err := checkForUpdates(server.Client(), "v1.2.2", server.URL)
	if err != nil {
		t.Fatalf("checkForUpdates() error = %v", err)
	}

	if got.LatestVersion != "1.2.2" {
		t.Fatalf("LatestVersion = %q, want %q", got.LatestVersion, "1.2.2")
	}
	if got.HasUpdate {
		t.Fatal("HasUpdate = true, want false")
	}
}

func TestBuildUpdateStatusLineForAvailableRelease(t *testing.T) {
	got := buildUpdateStatusLine(releaseUpdateStatus{
		CurrentVersion: "1.2.2",
		LatestVersion:  "1.2.3",
		ReleasesURL:    "https://cnb.cool/dmxapi/openclaw_config/-/releases",
		HasUpdate:      true,
	})
	want := "更新检查  ·  发现新版本 1.2.3（当前 1.2.2）  ·  https://cnb.cool/dmxapi/openclaw_config/-/releases"

	if got != want {
		t.Fatalf("buildUpdateStatusLine() = %q, want %q", got, want)
	}
}

func TestBuildUpdateStatusLineForLatestRelease(t *testing.T) {
	got := buildUpdateStatusLine(releaseUpdateStatus{
		CurrentVersion: "1.2.2",
		LatestVersion:  "1.2.2",
		ReleasesURL:    "https://cnb.cool/dmxapi/openclaw_config/-/releases",
		HasUpdate:      false,
	})
	want := "更新检查  ·  已是最新版本 1.2.2"

	if got != want {
		t.Fatalf("buildUpdateStatusLine() = %q, want %q", got, want)
	}
}
