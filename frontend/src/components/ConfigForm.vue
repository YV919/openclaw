<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { GetPresetModels, GetCurrentConfig, SaveConfig } from '../../wailsjs/go/main/App'

const baseUrl = ref('https://www.dmxapi.cn')
const apiKey = ref('')
const selectedModel = ref('')
const customModel = ref('')
const showPassword = ref(false)
const saving = ref(false)
const saveSuccess = ref(false)
const errorMessage = ref('')
const presetModels = ref<string[]>([])

const useCustomModel = ref(false)

onMounted(async () => {
  try {
    // 加载预设模型列表
    const models = await GetPresetModels()
    presetModels.value = models

    // 加载当前配置
    const config = await GetCurrentConfig()
    if (config) {
      baseUrl.value = config.baseUrl || 'https://www.dmxapi.cn'
      apiKey.value = config.apiKey || ''

      // 检查当前模型是否在预设列表中
      if (config.currentModel) {
        if (models.includes(config.currentModel)) {
          selectedModel.value = config.currentModel
        } else {
          useCustomModel.value = true
          customModel.value = config.currentModel
          selectedModel.value = '__custom__'
        }
      }
    }
  } catch (e) {
    console.error('加载配置失败:', e)
  }
})

function togglePassword() {
  showPassword.value = !showPassword.value
}

function onModelChange() {
  if (selectedModel.value === '__custom__') {
    useCustomModel.value = true
  } else {
    useCustomModel.value = false
    customModel.value = ''
  }
}

async function handleSave() {
  saving.value = true
  saveSuccess.value = false
  errorMessage.value = ''

  try {
    const model = useCustomModel.value ? customModel.value : selectedModel.value

    if (!model) {
      throw new Error('请选择或输入模型')
    }

    if (!apiKey.value) {
      throw new Error('请输入 API Key')
    }

    await SaveConfig(baseUrl.value, apiKey.value, model)
    saveSuccess.value = true
  } catch (e: any) {
    errorMessage.value = e.message || String(e)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="config-form">
    <div class="form-group">
      <label>Base URL</label>
      <input
        v-model="baseUrl"
        type="text"
        placeholder="https://www.dmxapi.cn"
      />
    </div>

    <div class="form-group">
      <label>API Key</label>
      <div class="password-input">
        <input
          v-model="apiKey"
          :type="showPassword ? 'text' : 'password'"
          placeholder="sk-..."
        />
        <button class="toggle-password" @click="togglePassword">
          {{ showPassword ? '隐藏' : '显示' }}
        </button>
      </div>
    </div>

    <div class="form-group">
      <label>模型</label>
      <select v-model="selectedModel" @change="onModelChange">
        <option value="" disabled>请选择模型</option>
        <option v-for="model in presetModels" :key="model" :value="model">
          {{ model }}
        </option>
        <option value="__custom__">自定义模型...</option>
      </select>

      <div v-if="useCustomModel" class="custom-model-input">
        <input
          v-model="customModel"
          type="text"
          placeholder="输入自定义模型名称"
        />
      </div>
    </div>

    <button
      class="btn-save"
      @click="handleSave"
      :disabled="saving"
    >
      {{ saving ? '保存中...' : '保存配置' }}
    </button>

    <div v-if="saveSuccess" class="success-message">
      <h4>配置已保存，模型已切换！</h4>
    </div>

    <div v-if="errorMessage" class="error-message">
      <strong>保存失败：</strong> {{ errorMessage }}
    </div>
  </div>
</template>

<style scoped>
</style>
