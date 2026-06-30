package main

import (
	"fmt"

	"openclaw_config/internal/config"
	"openclaw_config/internal/models"
)

func presetModelSet() map[string]bool {
	set := make(map[string]bool, len(models.PresetModels))
	for _, m := range models.PresetModels {
		set[m] = true
	}
	return set
}

func detectFormatFromModels(modelIDs []string) string {
	seen := make(map[string]bool, len(modelIDs))
	for _, m := range modelIDs {
		seen[config.DetectAPIFormat(m)] = true
	}
	if len(seen) == 1 {
		for f := range seen {
			return f
		}
	}
	// 冲突：打印警告
	printWarning("所选模型包含不同 API 格式，将使用第一个模型的格式：")
	for _, m := range modelIDs {
		fmt.Printf("    %s· %s → %s%s\n", cDim, m, config.DetectAPIFormat(m), cReset)
	}
	fmt.Println("  建议：将不同格式的模型拆分为独立 Provider")
	return config.DetectAPIFormat(modelIDs[0])
}
