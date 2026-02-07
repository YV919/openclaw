<script lang="ts" setup>
import ConfigForm from './components/ConfigForm.vue'
</script>

<template>
  <div class="container">
    <h1>OpenClaw DMXAPI 配置工具</h1>

    <div class="warning-banner">
      <span>注意：只能使用兼容 v1/messages 接口的模型</span>
    </div>

    <ConfigForm />

    <div class="help-section">
      <h4>常用命令</h4>
      <ul>
        <li><code>openclaw gateway restart</code> - 重启网关使配置生效</li>
        <li><code>/model</code> - 在聊天中查看/切换模型</li>
        <li><code>openclaw models list</code> - 列出所有可用模型</li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
</style>
