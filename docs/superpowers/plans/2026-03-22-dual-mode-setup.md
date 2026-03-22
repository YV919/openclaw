# Dual-Mode Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make startup offer `快速配置` and `高级配置`, where quick setup creates/edits exactly one Provider, lets the user choose exactly one primary model from that Provider's `models`, and leaves all other agents to inherit the main model via existing empty-value semantics.

**Architecture:** Keep `Run()` as the top-level router that loads config, shows the banner, chooses the mode, and saves once at the end. Advanced setup keeps the current 4-step flow unchanged; quick setup uses its own 2+1 step flow with a runtime snapshot, single-Provider convergence, single-primary-model selection, and a finish/restore confirmation page.

**Tech Stack:** Go, `github.com/charmbracelet/huh`, Bubble Tea, Go test

---

## Implementation Context

### Spec to follow
- `docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md`

### Files to modify
- `app.go`
- `app_test.go`

### Existing code to preserve
- `runAdvancedSetup(...)` and the current advanced Step 1~4 semantics
- `internal/config/manager.go`
- `internal/config/types.go`
- existing `provider/model` config format
- existing empty-value inheritance behavior for `SubAgent`

### Existing code paths already present
- `Run()` already calls `pickSetupMode(...)`, `runQuickSetup(...)`, and `runAdvancedSetup(...)`
- `runQuickSetup(...)` already exists but currently still reuses `runStep1Providers(...)` and `runStep2MainAgent(...)`, which is the behavior this plan must correct
- `prepareQuickSetup(...)` / `restoreQuickSetupSnapshot(...)` exist but currently do not snapshot `Providers`

### Required behavioral end state
After quick setup finishes, `fullCfg` must satisfy all of these:
- exactly 1 Provider remains
- `MainAgent.Primary` is `provider/model`
- `MainAgent.Fallback == ""`
- `SubAgent.Primary == ""`
- `SubAgent.Fallback == ""`
- `NamedAgents` is empty / nil
- quick mode never exposes fallback selection
- quick mode never offers models from other Providers

---

## File Structure / Responsibility Lock-In

- `app.go`
  - keep top-level routing in `Run()`
  - expand quick-setup snapshot helpers
  - add quick-only Provider convergence and quick-only primary-model selection helpers
  - keep advanced setup untouched except for routing boundaries
- `app_test.go`
  - add pure-logic tests first for snapshot/restore and quick-mode convergence
  - add narrow UI-text tests for mode entry and quick confirmation copy
  - keep existing advanced-flow tests passing

---

### Task 1: Fix quick-setup snapshot semantics first

**Files:**
- Modify: `app_test.go`
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write the failing snapshot tests**

Add these tests to `app_test.go` near the other quick-setup helper tests:

```go
func TestPrepareQuickSetupBacksUpProvidersMainSubAndNamedAgents(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{
			{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}},
			{Name: "anthropic", BaseUrl: "https://api.anthropic.com", Models: []string{"claude-sonnet-4-6"}},
		},
		MainAgent: config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "anthropic/claude-sonnet-4-6"},
		SubAgent:  config.AgentModelConfig{Primary: "anthropic/claude-sonnet-4-6"},
		NamedAgents: []config.NamedAgentConfig{{
			ID: "writer",
			Model: config.AgentModelConfig{Primary: "openai/gpt-5"},
		}},
	}

	snapshot := prepareQuickSetup(cfg)

	if !reflect.DeepEqual(snapshot.Providers, []config.ProviderConfig{
		{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}},
		{Name: "anthropic", BaseUrl: "https://api.anthropic.com", Models: []string{"claude-sonnet-4-6"}},
	}) {
		t.Fatalf("snapshot providers = %#v", snapshot.Providers)
	}
	if snapshot.MainAgent != (config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "anthropic/claude-sonnet-4-6"}) {
		t.Fatalf("snapshot main = %#v", snapshot.MainAgent)
	}
	if snapshot.SubAgent != (config.AgentModelConfig{Primary: "anthropic/claude-sonnet-4-6"}) {
		t.Fatalf("snapshot sub = %#v", snapshot.SubAgent)
	}
	if len(snapshot.NamedAgents) != 1 || snapshot.NamedAgents[0].ID != "writer" {
		t.Fatalf("snapshot named agents = %#v", snapshot.NamedAgents)
	}

	if cfg.SubAgent != (config.AgentModelConfig{}) {
		t.Fatalf("sub agent = %#v, want empty", cfg.SubAgent)
	}
	if cfg.NamedAgents != nil {
		t.Fatalf("named agents = %#v, want nil", cfg.NamedAgents)
	}
}

func TestRestoreQuickSetupSnapshotRestoresProvidersMainSubAndNamedAgents(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{{Name: "replacement", Models: []string{"replacement-model"}}},
		MainAgent: config.AgentModelConfig{Primary: "replacement/replacement-model"},
	}
	snapshot := quickSetupSnapshot{
		Providers: []config.ProviderConfig{{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5"}}},
		MainAgent: config.AgentModelConfig{Primary: "openai/gpt-5", Fallback: "openai/gpt-4.1"},
		SubAgent:  config.AgentModelConfig{Primary: "openai/gpt-5"},
		NamedAgents: []config.NamedAgentConfig{{
			ID: "reviewer",
			Model: config.AgentModelConfig{Primary: "openai/gpt-5"},
		}},
	}

	restoreQuickSetupSnapshot(cfg, snapshot)

	if !reflect.DeepEqual(cfg.Providers, snapshot.Providers) {
		t.Fatalf("providers = %#v, want %#v", cfg.Providers, snapshot.Providers)
	}
	if cfg.MainAgent != snapshot.MainAgent {
		t.Fatalf("main = %#v, want %#v", cfg.MainAgent, snapshot.MainAgent)
	}
	if cfg.SubAgent != snapshot.SubAgent {
		t.Fatalf("sub = %#v, want %#v", cfg.SubAgent, snapshot.SubAgent)
	}
	if !reflect.DeepEqual(cfg.NamedAgents, snapshot.NamedAgents) {
		t.Fatalf("named agents = %#v, want %#v", cfg.NamedAgents, snapshot.NamedAgents)
	}
}
```

- [ ] **Step 2: Run the targeted tests to verify RED**

Run: `go test ./... -run 'TestPrepareQuickSetupBacksUpProvidersMainSubAndNamedAgents|TestRestoreQuickSetupSnapshotRestoresProvidersMainSubAndNamedAgents'`

Expected: FAIL because `quickSetupSnapshot` does not yet include `Providers`, and restore does not yet restore them.

- [ ] **Step 3: Write the minimal snapshot implementation**

In `app.go`:
- extend `quickSetupSnapshot` to include `Providers []config.ProviderConfig`
- add a `cloneProviders(...) []config.ProviderConfig` helper that deep-copies the Provider slice and each Provider's `Models`
- update `prepareQuickSetup(...)` to snapshot `Providers`
- update `restoreQuickSetupSnapshot(...)` to restore `Providers`

Implementation shape:

```go
type quickSetupSnapshot struct {
	Providers   []config.ProviderConfig
	MainAgent   config.AgentModelConfig
	SubAgent    config.AgentModelConfig
	NamedAgents []config.NamedAgentConfig
}
```

```go
func cloneProviders(providers []config.ProviderConfig) []config.ProviderConfig {
	if len(providers) == 0 {
		return nil
	}
	cloned := make([]config.ProviderConfig, len(providers))
	for i, provider := range providers {
		cloned[i] = provider
		if len(provider.Models) > 0 {
			cloned[i].Models = append([]string(nil), provider.Models...)
		}
	}
	return cloned
}
```

- [ ] **Step 4: Run the targeted tests to verify GREEN**

Run: `go test ./... -run 'TestPrepareQuickSetupBacksUpProvidersMainSubAndNamedAgents|TestRestoreQuickSetupSnapshotRestoresProvidersMainSubAndNamedAgents'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app.go app_test.go
git commit -m "test(快速配置): 覆盖快照与恢复语义"
```

---

### Task 2: Lock down quick-mode final config semantics

**Files:**
- Modify: `app_test.go`
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write the failing convergence tests**

Add these tests to `app_test.go`:

```go
func TestApplyQuickProviderSelectionReplacesProvidersWithSingleProvider(t *testing.T) {
	cfg := &config.FullConfig{
		Providers: []config.ProviderConfig{
			{Name: "old-one", Models: []string{"a"}},
			{Name: "old-two", Models: []string{"b"}},
		},
	}
	selected := config.ProviderConfig{Name: "openai", BaseUrl: "https://api.openai.com/v1", Models: []string{"gpt-5", "gpt-4.1"}}

	applyQuickProviderSelection(cfg, selected)

	want := []config.ProviderConfig{selected}
	if !reflect.DeepEqual(cfg.Providers, want) {
		t.Fatalf("providers = %#v, want %#v", cfg.Providers, want)
	}
}

func TestApplyQuickPrimaryModelSetsProviderQualifiedPrimaryAndClearsAllQuickOverrides(t *testing.T) {
	cfg := &config.FullConfig{
		MainAgent: config.AgentModelConfig{Primary: "old/model", Fallback: "old/fallback"},
		SubAgent:  config.AgentModelConfig{Primary: "custom/sub", Fallback: "custom/sub-fallback"},
		NamedAgents: []config.NamedAgentConfig{{
			ID: "writer",
			Model: config.AgentModelConfig{Primary: "custom/writer"},
		}},
	}

	applyQuickPrimaryModel(cfg, "openai", "gpt-5")

	if cfg.MainAgent.Primary != "openai/gpt-5" {
		t.Fatalf("main primary = %q, want %q", cfg.MainAgent.Primary, "openai/gpt-5")
	}
	if cfg.MainAgent.Fallback != "" {
		t.Fatalf("main fallback = %q, want empty", cfg.MainAgent.Fallback)
	}
	if cfg.SubAgent != (config.AgentModelConfig{}) {
		t.Fatalf("sub = %#v, want empty", cfg.SubAgent)
	}
	if cfg.NamedAgents != nil {
		t.Fatalf("named agents = %#v, want nil", cfg.NamedAgents)
	}
}

func TestQuickPrimaryModelOptionsUseOnlySelectedProviderModels(t *testing.T) {
	providers := []config.ProviderConfig{
		{Name: "openai", Models: []string{"gpt-5", "gpt-4.1"}},
		{Name: "anthropic", Models: []string{"claude-sonnet-4-6"}},
	}

	got := buildQuickPrimaryModelOptions(providers[0])
	want := []huh.Option[string]{
		huh.NewOption("gpt-5", "gpt-5"),
		huh.NewOption("gpt-4.1", "gpt-4.1"),
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("options = %#v, want %#v", got, want)
	}
}

func TestQuickPrimaryModelOptionsEmptyWhenProviderHasNoModels(t *testing.T) {
	got := buildQuickPrimaryModelOptions(config.ProviderConfig{Name: "openai"})
	if len(got) != 0 {
		t.Fatalf("len(options) = %d, want 0", len(got))
	}
}
```

- [ ] **Step 2: Run the targeted tests to verify RED**

Run: `go test ./... -run 'TestApplyQuickProviderSelectionReplacesProvidersWithSingleProvider|TestApplyQuickPrimaryModelSetsProviderQualifiedPrimaryAndClearsAllQuickOverrides|TestQuickPrimaryModelOptions'`

Expected: FAIL because these helpers either do not exist yet or do not enforce the final quick-mode semantics.

- [ ] **Step 3: Write the minimal implementation**

In `app.go`, add these helpers:

```go
func applyQuickProviderSelection(cfg *config.FullConfig, provider config.ProviderConfig) {
	cfg.Providers = []config.ProviderConfig{provider}
}

func applyQuickPrimaryModel(cfg *config.FullConfig, providerName string, model string) {
	cfg.MainAgent.Primary = providerName + "/" + model
	cfg.MainAgent.Fallback = ""
	cfg.SubAgent = config.AgentModelConfig{}
	cfg.NamedAgents = nil
}

func buildQuickPrimaryModelOptions(provider config.ProviderConfig) []huh.Option[string] {
	opts := make([]huh.Option[string], 0, len(provider.Models))
	for _, model := range provider.Models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		opts = append(opts, huh.NewOption(trimmed, trimmed))
	}
	return opts
}
```

Do not add fallback logic or cross-Provider model pooling.

- [ ] **Step 4: Run the targeted tests to verify GREEN**

Run: `go test ./... -run 'TestApplyQuickProviderSelectionReplacesProvidersWithSingleProvider|TestApplyQuickPrimaryModelSetsProviderQualifiedPrimaryAndClearsAllQuickOverrides|TestQuickPrimaryModelOptions'`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add app.go app_test.go
git commit -m "feat(快速配置): 固定单 provider 与单主模型语义"
```

---

### Task 3: Replace reused advanced pages with quick-only steps

**Files:**
- Modify: `app_test.go`
- Modify: `app.go`
- Test: `app_test.go`

- [ ] **Step 1: Write the failing copy tests for quick-mode entry and confirmation**

Add these tests:

```go
func TestSetupModeDescriptionMentionsQuickAndAdvancedResponsibilities(t *testing.T) {
	got := setupModeDescription(&config.FullConfig{})
	if !strings.Contains(got, "快速配置：只设置 Provider 和主模型") {
		t.Fatalf("description = %q, want quick-setup copy", got)
	}
	if !strings.Contains(got, "高级配置：完整设置主 Agent、子 Agent 和命名 Agent") {
		t.Fatalf("description = %q, want advanced-setup copy", got)
	}
}

func TestQuickSetupSummaryDescriptionMentionsSingleProviderInheritanceAndRestore(t *testing.T) {
	got := quickSetupSummaryDescription(true)
	if !strings.Contains(got, "只保留 1 个 Provider") {
		t.Fatalf("description = %q, want single-provider copy", got)
	}
	if !strings.Contains(got, "默认继承主 Agent") {
		t.Fatalf("description = %q, want inheritance copy", got)
	}
	if !strings.Contains(got, "恢复原样") {
		t.Fatalf("description = %q, want restore copy", got)
	}
}
```

- [ ] **Step 2: Run the targeted tests to verify RED**

Run: `go test ./... -run 'TestSetupModeDescriptionMentionsQuickAndAdvancedResponsibilities|TestQuickSetupSummaryDescriptionMentionsSingleProviderInheritanceAndRestore'`

Expected: FAIL because the quick summary copy does not yet mention the final single-Provider constraint clearly enough.

- [ ] **Step 3: Write the minimal quick-only flow implementation**

In `app.go`:
1. keep `runAdvancedSetup(...)` unchanged
2. rewrite `runQuickSetup(...)` so it no longer calls:
   - `runStep1Providers(...)`
   - `runStep2MainAgent(...)`
3. instead, create and use quick-only functions:

```go
func (a *App) runQuickStep1Provider(fullCfg *config.FullConfig) (config.ProviderConfig, bool, error)
func (a *App) runQuickStep2PrimaryModel(provider config.ProviderConfig, fullCfg *config.FullConfig) (bool, error)
```

Required semantics for `runQuickStep1Provider(...)`:
- allow the user to create or edit exactly one Provider using the existing provider editor capability (`editProvider(...)` is the intended reuse point)
- after success, call `applyQuickProviderSelection(...)`
- never expose multi-Provider management choices in quick mode

Required semantics for `runQuickStep2PrimaryModel(...)`:
- build options from `buildQuickPrimaryModelOptions(provider)` only
- if the Provider has no models, return to step 1 with a clear prompt instead of allowing submission
- only select one model
- call `applyQuickPrimaryModel(fullCfg, provider.Name, selectedModel)`
- never show fallback UI

Required semantics for `runQuickSetup(...)`:
- snapshot first
- keep a 2+1 step loop: provider -> primary model -> finish/restore
- on `__back__` from confirmation, return to quick primary-model step
- on `__restore__`, call `restoreQuickSetupSnapshot(...)` and return nil
- on `__finish__`, return nil with current quick config preserved

- [ ] **Step 4: Run the full test suite to verify GREEN**

Run: `go test ./...`

Expected: PASS, with the new quick-only copy tests and all prior tests passing.

- [ ] **Step 5: Commit**

```bash
git add app.go app_test.go
git commit -m "feat(快速配置): 改为专用步骤流程"
```

---

### Task 4: Verify no advanced-flow regression and finish branch hygiene

**Files:**
- Modify: `app.go`
- Modify: `app_test.go`
- Modify: `docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md`
- Modify: `docs/superpowers/plans/2026-03-22-dual-mode-setup.md`

- [ ] **Step 1: Format Go files**

Run: `gofmt -w app.go app_test.go`

Expected: no output; files formatted in place.

- [ ] **Step 2: Run the full verification suite**

Run: `go test ./...`

Expected: all tests pass.

- [ ] **Step 3: Review git changes and ignore rules**

Run:

```bash
git status --short
git diff -- app.go app_test.go docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md docs/superpowers/plans/2026-03-22-dual-mode-setup.md
```

Expected:
- only intended files are changed
- no generated or ignored junk is included
- advanced-flow code paths were not unintentionally modified beyond routing boundaries

- [ ] **Step 4: Create the final commit**

```bash
git add app.go app_test.go docs/superpowers/specs/2026-03-22-dual-mode-setup-design.md docs/superpowers/plans/2026-03-22-dual-mode-setup.md
git commit -m "feat(配置向导): 修正快速配置双模式语义"
```

Expected: local commit created successfully.

---

## Notes for the implementer

- Do not change `internal/config/*` to force quick-mode behavior; all required behavior can be expressed in `app.go`.
- Do not add fallback support to quick mode.
- Do not keep hidden extra Providers in quick mode; the final quick config must truly converge to one Provider.
- Do not explicitly copy the main model into sub-agent or named-agent configs; rely on existing inheritance semantics.
- Prefer pure helper tests first. Only add narrow UI-copy tests where they guard the agreed user-facing contract.
