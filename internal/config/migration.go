package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	slugInvalid   = regexp.MustCompile(`[^a-z0-9-]+`)
	slugMultiDash = regexp.MustCompile(`-{2,}`)
)

// NormalizeSlug 将任意字符串转为合法的 provider slug（小写字母、数字、连字符）
func NormalizeSlug(s string) string {
	s = strings.ToLower(s)
	s = slugInvalid.ReplaceAllString(s, "-")
	s = slugMultiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// SlugFromURL 从 BaseUrl 的 hostname 生成 slug
func SlugFromURL(baseUrl string) string {
	u, err := url.Parse(baseUrl)
	if err != nil || u.Hostname() == "" {
		return NormalizeSlug(baseUrl)
	}
	return NormalizeSlug(u.Hostname())
}

// MigrateProviders 对从 openclaw.json 解析出的 provider map 执行兼容性检测与修复。
// 返回修复后的 []ProviderConfig 和每次修复的描述日志。
// primary 参数是 agents.defaults.model.primary 的值，用于 Models 为空时反推模型 ID。
func MigrateProviders(
	rawProviders map[string]any,
	primary string,
) ([]ProviderConfig, []string) {
	var providers []ProviderConfig
	var logs []string
	seen := make(map[string]int) // slug → 已使用次数，用于去重

	for key, val := range rawProviders {
		pMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		name := key

		// 修复 1：provider key 含非法字符或大写 → 规范化
		normalized := NormalizeSlug(name)
		if normalized == "" {
			// key 为空或规范化后为空，从 baseUrl 生成
			rawBaseUrl, _ := pMap["baseUrl"].(string)
			normalized = SlugFromURL(rawBaseUrl)
			if normalized == "" {
				normalized = "provider"
			}
			if key == "" {
				logs = append(logs, fmt.Sprintf("provider key 为空，已从 baseUrl 生成 %q", normalized))
			} else {
				logs = append(logs, fmt.Sprintf("provider key %q 规范化失败，已重命名为 %q", key, normalized))
			}
		} else if normalized != name {
			logs = append(logs, fmt.Sprintf("provider key %q 含非法字符，已规范化为 %q", key, normalized))
		}
		name = normalized

		// 去重：若规范化后的 slug 已被占用，追加数字后缀
		if count := seen[name]; count > 0 {
			deduped := fmt.Sprintf("%s-%d", name, count+1)
			logs = append(logs, fmt.Sprintf("provider slug %q 冲突，已重命名为 %q", name, deduped))
			name = deduped
		}
		seen[normalized]++

		baseUrl, _ := pMap["baseUrl"].(string)
		apiKey, _ := pMap["apiKey"].(string)
		apiFormat, _ := pMap["api"].(string)

		// 修复 2：旧默认 URL（无 /v1）
		if baseUrl == "https://www.dmxapi.cn" {
			baseUrl = "https://www.dmxapi.cn/v1"
			logs = append(logs, fmt.Sprintf("provider %q 的 baseUrl 已补全 /v1 路径", name))
		}

		// 解析已有模型列表
		var modelIDs []string
		if rawModels, ok := pMap["models"].([]any); ok {
			for _, m := range rawModels {
				if mMap, ok := m.(map[string]any); ok {
					if id, ok := mMap["id"].(string); ok && id != "" {
						modelIDs = append(modelIDs, id)
					}
				}
			}
		}

		// 修复 3：models 数组为空 → 从 primary 反推
		// 同时尝试规范化后的 name 和原始 key，以应对 primary 中存的是原始 key 的情况
		if len(modelIDs) == 0 && primary != "" {
			for _, candidate := range []string{name, key} {
				if candidate == "" {
					continue
				}
				prefix := candidate + "/"
				if strings.HasPrefix(primary, prefix) {
					inferredID := primary[len(prefix):]
					modelIDs = append(modelIDs, inferredID)
					logs = append(logs, fmt.Sprintf("provider %q 的模型列表为空，已从 primary 推断补全: %q", name, inferredID))
					break
				}
			}
		}

		if apiFormat == "" && len(modelIDs) > 0 {
			apiFormat = DetectAPIFormat(modelIDs[0])
		}
		if apiFormat == "" {
			apiFormat = "openai-completions"
		}

		providers = append(providers, ProviderConfig{
			Name:      name,
			BaseUrl:   baseUrl,
			ApiKey:    apiKey,
			Models:    modelIDs,
			ApiFormat: apiFormat,
		})
	}

	return providers, logs
}
