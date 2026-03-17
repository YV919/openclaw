package main

import (
	"io"
	"os"
	"testing"
)

// suppressStdout 将 os.Stdout 重定向到 pipe，用于抑制测试中的警告输出
func suppressStdout(f func()) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	io.ReadAll(r) //nolint:errcheck
}

func TestDetectFormatFromModels(t *testing.T) {
	tests := []struct {
		name     string
		models   []string
		expected string
	}{
		{"claude 前缀", []string{"claude-sonnet-4-6"}, "anthropic-messages"},
		{"-cc 后缀", []string{"MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"gemini 前缀", []string{"gemini-3.1-pro-preview"}, "google-generative-ai"},
		{"gpt-5 前缀", []string{"gpt-5.2"}, "openai-responses"},
		{"gpt-5 带后缀变体", []string{"gpt-5.3-codex"}, "openai-responses"},
		{"o1 裸前缀", []string{"o1"}, "openai-responses"},
		{"o3-mini 带连字符", []string{"o3-mini"}, "openai-responses"},
		{"默认 openai-completions", []string{"qwen-turbo"}, "openai-completions"},
		{"单模型无冲突路径", []string{"claude-opus-4-6"}, "anthropic-messages"},
		{"多模型一致格式", []string{"claude-sonnet-4-6", "MiniMax-M2.5-cc"}, "anthropic-messages"},
		{"冲突时取第一个", []string{"claude-sonnet-4-6", "gpt-5.2"}, "anthropic-messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			suppressStdout(func() {
				got = detectFormatFromModels(tt.models)
			})
			if got != tt.expected {
				t.Errorf("detectFormatFromModels(%v) = %q, want %q", tt.models, got, tt.expected)
			}
		})
	}
}
