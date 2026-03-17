package config

import (
	"testing"
)

func TestNormalizeSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dmxapi", "dmxapi"},
		{"MyProxy", "myproxy"},
		{"my proxy", "my-proxy"},
		{"my.proxy.com", "my-proxy-com"},
		{"api.example.com:8080", "api-example-com-8080"},
		{"  --bad-- ", "bad"},
		{"", ""},
	}
	for _, tc := range tests {
		got := NormalizeSlug(tc.input)
		if got != tc.want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestSlugFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://api.dmxapi.cn/v1", "api-dmxapi-cn"},
		{"https://192.168.1.1:8080/v1", "192-168-1-1"},
		{"https://my.proxy.com", "my-proxy-com"},
		{"not-a-url", "not-a-url"},
	}
	for _, tc := range tests {
		got := SlugFromURL(tc.input)
		if got != tc.want {
			t.Errorf("SlugFromURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMigrateProviders_NormalizesKey(t *testing.T) {
	raw := map[string]any{
		"My Provider": map[string]any{
			"baseUrl": "https://api.example.com/v1",
			"apiKey":  "sk-test",
			"api":     "openai-completions",
			"models": []any{
				map[string]any{"id": "gpt-4o"},
			},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name != "my-provider" {
		t.Errorf("expected name %q, got %q", "my-provider", providers[0].Name)
	}
	if len(logs) == 0 {
		t.Error("expected migration log entry for key normalization")
	}
}

func TestMigrateProviders_FixesOldDefaultURL(t *testing.T) {
	raw := map[string]any{
		"dmxapi": map[string]any{
			"baseUrl": "https://www.dmxapi.cn",
			"apiKey":  "sk-test",
			"api":     "anthropic-messages",
			"models": []any{
				map[string]any{"id": "claude-opus-4-6"},
			},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if providers[0].BaseUrl != "https://www.dmxapi.cn/v1" {
		t.Errorf("expected URL to be fixed, got %q", providers[0].BaseUrl)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for URL fix")
	}
}

func TestMigrateProviders_InfersModelsFromPrimary(t *testing.T) {
	raw := map[string]any{
		"dmxapi": map[string]any{
			"baseUrl": "https://api.dmxapi.cn/v1",
			"apiKey":  "sk-test",
			"api":     "anthropic-messages",
			"models":  []any{}, // 空
		},
	}
	providers, logs := MigrateProviders(raw, "dmxapi/claude-opus-4-6")
	if len(providers[0].Models) != 1 || providers[0].Models[0] != "claude-opus-4-6" {
		t.Errorf("expected models to be inferred, got %v", providers[0].Models)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for model inference")
	}
}

func TestMigrateProviders_EmptyModels_NoInference(t *testing.T) {
	raw := map[string]any{
		"dmxapi": map[string]any{
			"baseUrl": "https://api.dmxapi.cn/v1",
			"apiKey":  "sk-test",
			"api":     "openai-completions",
			"models":  []any{},
		},
	}
	// primary 来自不同 provider，不应推断
	providers, _ := MigrateProviders(raw, "other/some-model")
	if len(providers[0].Models) != 0 {
		t.Errorf("expected no models inferred, got %v", providers[0].Models)
	}
}
