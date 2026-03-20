package main

import (
	"os"
	"strings"
	"testing"
)

func TestCNBYmlAIRoutingCoversClaudeResponsesGeminiAndChat(t *testing.T) {
	content, err := os.ReadFile(".cnb.yml")
	if err != nil {
		t.Fatalf("read .cnb.yml: %v", err)
	}

	text := string(content)
	requiredSnippets := []string{
		`if echo "${AI_MODEL_LOWER}" | grep -q '^claude'`,
		`/v1/messages"`,
		`"anthropic-version: 2023-06-01"`,
		`if echo "${AI_MODEL_LOWER}" | grep -q '^gpt-5'`,
		`/v1/responses"`,
		`if echo "${AI_MODEL_LOWER}" | grep -q '^gemini'`,
		`/v1beta/models/${AI_MODEL}:generateContent"`,
		`/v1/chat/completions"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf(".cnb.yml missing snippet %q", snippet)
		}
	}
}

func TestCNBYmlAIResponseParsingHandlesEachProviderShape(t *testing.T) {
	content, err := os.ReadFile(".cnb.yml")
	if err != nil {
		t.Fatalf("read .cnb.yml: %v", err)
	}

	text := string(content)
	requiredSnippets := []string{
		`.content[]?.text`,
		`.output[]?.content[]? | select(.type == "output_text") | .text`,
		`.candidates[0].content.parts[]?.text`,
		`.choices[0].message.content`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf(".cnb.yml missing response parser snippet %q", snippet)
		}
	}
}

func TestCNBYmlResponsesPayloadUsesCodexStyleMessages(t *testing.T) {
	content, err := os.ReadFile(".cnb.yml")
	if err != nil {
		t.Fatalf("read .cnb.yml: %v", err)
	}

	text := string(content)
	requiredSnippets := []string{
		`type: "message"`,
		`role: "developer"`,
		`role: "user"`,
		`type: "input_text"`,
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf(".cnb.yml missing codex-style responses snippet %q", snippet)
		}
	}
}
