package config

import (
	"testing"
)

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		rawURL    string
		apiFormat string
		want      string
	}{
		// openai-completions：无路径 → 追加 /v1
		{"https://api.example.com", "openai-completions", "https://api.example.com/v1"},
		// 末尾斜杠 → 追加 /v1
		{"https://api.example.com/", "openai-completions", "https://api.example.com/v1"},
		// 已有 /v1 → 不重复
		{"https://api.example.com/v1", "openai-completions", "https://api.example.com/v1"},
		// /v1/ → 去末尾斜杠
		{"https://api.example.com/v1/", "openai-completions", "https://api.example.com/v1"},
		// 大写 scheme/host/path → 全部规范化
		{"HTTPS://API.EXAMPLE.COM/V1", "openai-completions", "https://api.example.com/v1"},
		// 自定义路径 → 保留
		{"https://api.example.com/my-path", "openai-completions", "https://api.example.com/my-path"},
		// openai-responses
		{"https://api.example.com", "openai-responses", "https://api.example.com/v1"},
		{"https://api.example.com/v1", "openai-responses", "https://api.example.com/v1"},
		// anthropic-messages
		{"https://api.example.com", "anthropic-messages", "https://api.example.com/v1"},
		{"https://api.example.com/v1", "anthropic-messages", "https://api.example.com/v1"},
		{"https://api.example.com/my-path", "anthropic-messages", "https://api.example.com/my-path"},
		// google-generative-ai：不追加 /v1
		{"https://generativelanguage.googleapis.com", "google-generative-ai", "https://generativelanguage.googleapis.com"},
		// google 末尾斜杠 → 去掉
		{"https://generativelanguage.googleapis.com/", "google-generative-ai", "https://generativelanguage.googleapis.com"},
		// google 代理 URL → 保留原样
		{"https://my-gemini-proxy.example.com", "google-generative-ai", "https://my-gemini-proxy.example.com"},
		// 空字符串 → 原样
		{"", "openai-completions", ""},
		{"  ", "openai-completions", ""},
	}
	for _, tc := range tests {
		got := NormalizeBaseURL(tc.rawURL, tc.apiFormat)
		if got != tc.want {
			t.Errorf("NormalizeBaseURL(%q, %q)\n  got  %q\n  want %q", tc.rawURL, tc.apiFormat, got, tc.want)
		}
	}
}

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

func TestMigrateProviders_EmptyKey_UsesBaseUrl(t *testing.T) {
	raw := map[string]any{
		"": map[string]any{
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
	if providers[0].Name != "api-example-com" {
		t.Errorf("expected name %q, got %q", "api-example-com", providers[0].Name)
	}
	if len(logs) == 0 {
		t.Error("expected migration log entry for empty key")
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

func TestMigrateProviders_PrimaryMatchesOriginalKey(t *testing.T) {
	raw := map[string]any{
		"My Provider": map[string]any{
			"baseUrl": "https://api.example.com/v1",
			"apiKey":  "sk-test",
			"api":     "openai-completions",
			"models":  []any{},
		},
	}
	// primary 里存的是原始 key "My Provider/gpt-4o"
	providers, logs := MigrateProviders(raw, "My Provider/gpt-4o")
	if len(providers[0].Models) != 1 || providers[0].Models[0] != "gpt-4o" {
		t.Errorf("expected model inferred from original key, got %v", providers[0].Models)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for model inference")
	}
}

func TestMigrateProviders_GoogleFormatNoV1(t *testing.T) {
	raw := map[string]any{
		"gemini": map[string]any{
			"baseUrl": "https://generativelanguage.googleapis.com",
			"apiKey":  "AIza-test",
			"api":     "google-generative-ai",
			"models": []any{
				map[string]any{"id": "gemini-2.0-flash"},
			},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].BaseUrl != "https://generativelanguage.googleapis.com" {
		t.Errorf("expected google URL unchanged, got %q", providers[0].BaseUrl)
	}
	if len(logs) != 0 {
		t.Errorf("expected no migration logs for google format, got %v", logs)
	}
}

func TestMigrateProviders_SlugCollision_BothPreserved(t *testing.T) {
	raw := map[string]any{
		"my-proxy": map[string]any{
			"baseUrl": "https://api1.example.com/v1",
			"apiKey":  "sk-1",
			"api":     "openai-completions",
			"models":  []any{map[string]any{"id": "gpt-4o"}},
		},
		"my.proxy": map[string]any{
			"baseUrl": "https://api2.example.com/v1",
			"apiKey":  "sk-2",
			"api":     "openai-completions",
			"models":  []any{map[string]any{"id": "gpt-3.5-turbo"}},
		},
	}
	providers, logs := MigrateProviders(raw, "")
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	names := map[string]bool{providers[0].Name: true, providers[1].Name: true}
	if !names["my-proxy"] {
		t.Errorf("expected one provider named %q, got names: %v", "my-proxy", names)
	}
	if !names["my-proxy-2"] {
		t.Errorf("expected deduped provider named %q, got names: %v", "my-proxy-2", names)
	}
	if len(logs) == 0 {
		t.Error("expected migration log for slug collision")
	}
}
