<template>
  <div class="settings-view fade-in-up">
    <div class="page-header">
      <h1 class="page-title font-display">{{ t('settings.title') }}</h1>
    </div>

    <div class="settings-layout">
      <!-- 左侧表单 -->
      <div class="settings-main">
        <!-- 重启提示 -->
        <div class="restart-notice" v-if="isDirty">
          <el-icon><WarningFilled /></el-icon>
          {{ t('settings.restartNotice') }}
        </div>

        <div class="settings-card">
          <div class="card-title font-display">
            <el-icon><MagicStick /></el-icon>
            {{ t('settings.aiTitle') }}
          </div>

          <el-form
            :model="form"
            ref="formRef"
            label-position="top"
            :disabled="loading"
            v-if="!loading"
          >
            <!-- 供应商 + 接口地址 -->
            <div class="form-row">
              <el-form-item :label="t('settings.provider')" prop="provider">
                <el-input v-model="form.provider" placeholder="openai-compatible" @input="markDirty" />
              </el-form-item>
              <el-form-item :label="t('settings.baseUrl')" prop="base_url">
                <el-input v-model="form.base_url" placeholder="https://api.openai.com/v1" @input="markDirty">
                  <template #prefix>
                    <el-icon><Link /></el-icon>
                  </template>
                </el-input>
              </el-form-item>
            </div>

            <!-- API Key -->
            <el-form-item :label="t('settings.apiKey')" prop="api_key">
              <el-input
                v-model="form.api_key"
                type="password"
                show-password
                :placeholder="t('settings.apiKeyTip')"
                @input="markDirty"
              >
                <template #prefix>
                  <el-icon><Key /></el-icon>
                </template>
              </el-input>
              <div class="field-hint font-mono">
                {{ t('settings.apiKeyTip') }}
              </div>
            </el-form-item>

            <!-- 模型名 -->
            <el-form-item :label="t('settings.model')" prop="model">
              <el-input v-model="form.model" placeholder="gpt-4o / z-ai/glm-4.7" @input="markDirty">
                <template #prefix>
                  <el-icon><Cpu /></el-icon>
                </template>
              </el-input>
              <!-- 快捷选择常用模型 -->
              <div class="model-presets">
                <button
                  v-for="preset in modelPresets"
                  :key="preset"
                  type="button"
                  class="preset-btn font-mono"
                  :class="{ active: form.model === preset }"
                  @click="selectPreset(preset)"
                >{{ preset }}</button>
              </div>
            </el-form-item>

            <!-- 高级参数 -->
            <div class="advanced-section">
              <div class="advanced-title" @click="showAdvanced = !showAdvanced">
                <el-icon><component :is="showAdvanced ? 'ArrowUp' : 'ArrowDown'" /></el-icon>
                Advanced Parameters
              </div>
              <transition name="slide-down">
                <div v-if="showAdvanced" class="advanced-fields">
                  <div class="form-row three-col">
                    <el-form-item :label="t('settings.maxTokens')">
                      <el-input-number
                        v-model="form.max_tokens"
                        :min="256" :max="32768" :step="256"
                        @change="markDirty"
                        style="width: 100%"
                      />
                    </el-form-item>
                    <el-form-item :label="t('settings.timeoutSec')">
                      <el-input-number
                        v-model="form.timeout_sec"
                        :min="30" :max="600" :step="30"
                        @change="markDirty"
                        style="width: 100%"
                      />
                    </el-form-item>
                    <el-form-item :label="t('settings.rateLimit')">
                      <el-input-number
                        v-model="form.rate_limit"
                        :min="1" :max="100" :step="1"
                        @change="markDirty"
                        style="width: 100%"
                      />
                    </el-form-item>
                  </div>
                </div>
              </transition>
            </div>

            <!-- 操作按钮 -->
            <div class="form-actions">
              <button
                type="button"
                class="btn-save"
                :class="{ loading: saving }"
                :disabled="saving || !isDirty"
                @click="handleSave"
              >
                <span class="btn-inner">
                  <el-icon v-if="!saving"><Check /></el-icon>
                  <span v-if="saving" class="spinner" />
                  {{ saving ? t('settings.saving') : t('settings.save') }}
                </span>
              </button>
              <button
                type="button"
                class="btn-reset"
                :disabled="!isDirty"
                @click="resetForm"
              >
                <el-icon><RefreshLeft /></el-icon>
                Reset
              </button>
            </div>
          </el-form>

          <div v-else class="loading-state">
            <span class="spinner-lg" />
            <span>{{ t('common.loading') }}</span>
          </div>
        </div>
      </div>

      <!-- 右侧说明 -->
      <div class="settings-aside">
        <!-- 当前配置快照 -->
        <div class="aside-card" v-if="savedForm">
          <div class="aside-title font-display">
            <el-icon><Document /></el-icon>
            Current Config
          </div>
          <div class="config-snapshot">
            <div class="snapshot-row" v-for="(val, key) in snapshotItems" :key="key">
              <span class="snapshot-key font-mono">{{ key }}</span>
              <span class="snapshot-val font-mono">{{ val }}</span>
            </div>
          </div>
        </div>

        <!-- 提示说明 -->
        <div class="aside-card notice-card">
          <div class="aside-title font-display">
            <el-icon><InfoFilled /></el-icon>
            Notes
          </div>
          <ul class="notice-list">
            <li>API Key 仅显示前 4 位，提交时留空则保留原有值</li>
            <li>配置写入 <code class="font-mono">configs/config.yaml</code></li>
            <li>修改后需重启后端服务才能生效</li>
            <li>仅支持兼容 OpenAI Chat API 格式的接口</li>
          </ul>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import * as api from '@/api'
import type { AISettings } from '@/api/types'

const { t } = useI18n()

const loading  = ref(true)
const saving   = ref(false)
const isDirty  = ref(false)
const showAdvanced = ref(false)

const savedForm = ref<AISettings | null>(null)

const form = reactive({
  provider:    '',
  base_url:    '',
  api_key:     '',
  model:       '',
  max_tokens:  2048,
  timeout_sec: 180,
  rate_limit:  10,
})

const modelPresets = [
  'z-ai/glm-4.7',
  'z-ai/glm-5',
  'gpt-4o',
  'gpt-4o-mini',
  'deepseek-chat',
]

const snapshotItems = computed(() => {
  if (!savedForm.value) return {}
  return {
    'provider':    savedForm.value.provider,
    'base_url':    savedForm.value.base_url,
    'api_key':     savedForm.value.api_key,
    'model':       savedForm.value.model,
    'max_tokens':  savedForm.value.max_tokens,
    'timeout_sec': savedForm.value.timeout_sec + 's',
    'rate_limit':  savedForm.value.rate_limit + '/min',
  }
})

function markDirty() {
  isDirty.value = true
}

function selectPreset(model: string) {
  form.model = model
  markDirty()
}

function fillForm(data: AISettings) {
  form.provider    = data.provider
  form.base_url    = data.base_url
  form.api_key     = ''  // 不回填脱敏的 key，让用户重新输入才更新
  form.model       = data.model
  form.max_tokens  = data.max_tokens
  form.timeout_sec = data.timeout_sec
  form.rate_limit  = data.rate_limit
}

function resetForm() {
  if (savedForm.value) fillForm(savedForm.value)
  isDirty.value = false
}

async function loadSettings() {
  loading.value = true
  try {
    const data = await api.getAISettings()
    savedForm.value = data
    fillForm(data)
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    await api.updateAISettings({
      provider:    form.provider,
      base_url:    form.base_url,
      api_key:     form.api_key || undefined,
      model:       form.model,
      max_tokens:  form.max_tokens,
      timeout_sec: form.timeout_sec,
      rate_limit:  form.rate_limit,
    })
    ElMessage.success(t('settings.saveSuccess'))
    isDirty.value = false
    // 重新拉取，更新快照
    await loadSettings()
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.settings-view { max-width: 1000px; }

.page-header { margin-bottom: 24px; }
.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.settings-layout {
  display: grid;
  grid-template-columns: 1fr 260px;
  gap: 20px;
  align-items: start;
}

/* 重启提示 */
.restart-notice {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(234,179,8,0.08);
  border: 1px solid rgba(234,179,8,0.25);
  border-radius: var(--radius-md);
  color: var(--severity-medium);
  font-size: 13px;
  margin-bottom: 16px;
}

/* 主卡片 */
.settings-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 20px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle);
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-row.three-col {
  grid-template-columns: 1fr 1fr 1fr;
}

/* Field hint */
.field-hint {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 4px;
}

/* 模型快捷选择 */
.model-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.preset-btn {
  font-size: 11px;
  padding: 3px 10px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.preset-btn:hover {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

.preset-btn.active {
  border-color: var(--accent-primary);
  background: var(--accent-glow);
  color: var(--accent-primary);
}

/* 高级参数折叠 */
.advanced-section {
  margin-bottom: 20px;
}

.advanced-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  user-select: none;
  margin-bottom: 12px;
  transition: color var(--transition-fast);
}

.advanced-title:hover { color: var(--accent-primary); }

.slide-down-enter-active,
.slide-down-leave-active {
  transition: all 0.25s ease;
  overflow: hidden;
}
.slide-down-enter-from,
.slide-down-leave-to {
  opacity: 0;
  max-height: 0;
}
.slide-down-enter-to,
.slide-down-leave-from {
  opacity: 1;
  max-height: 200px;
}

/* 操作按钮 */
.form-actions {
  display: flex;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
  margin-top: 4px;
}

.btn-save, .btn-reset {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 9px 20px;
  border-radius: var(--radius-md);
  font-size: 13px;
  font-weight: 600;
  font-family: var(--font-body);
  cursor: pointer;
  transition: all var(--transition-fast);
  border: 1px solid transparent;
}

.btn-save {
  background: var(--accent-primary);
  color: var(--text-inverse);
  box-shadow: 0 0 16px rgba(14,165,233,0.3);
  min-width: 120px;
}

.btn-save:hover:not(:disabled) {
  background: #38bdf8;
  box-shadow: 0 0 24px rgba(14,165,233,0.5);
}

.btn-save:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-reset {
  background: var(--bg-elevated);
  color: var(--text-secondary);
  border-color: var(--border-default);
}

.btn-reset:hover:not(:disabled) {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
}

.btn-reset:disabled { opacity: 0.4; cursor: not-allowed; }

.btn-inner {
  display: flex;
  align-items: center;
  gap: 6px;
}

.spinner {
  width: 12px; height: 12px;
  border: 2px solid rgba(0,0,0,0.15);
  border-top-color: var(--text-inverse);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

/* Loading state */
.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 40px;
  color: var(--text-muted);
}

.spinner-lg {
  width: 20px; height: 20px;
  border: 2px solid var(--border-default);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

/* 右侧 */
.settings-aside {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.aside-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 16px;
}

.aside-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 12px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-bottom: 14px;
}

.config-snapshot {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.snapshot-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.snapshot-key {
  font-size: 10px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.snapshot-val {
  font-size: 11px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: right;
  max-width: 150px;
}

.notice-card { border-color: rgba(14,165,233,0.15); }

.notice-list {
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.notice-list li {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  padding-left: 12px;
  position: relative;
}

.notice-list li::before {
  content: '›';
  position: absolute;
  left: 0;
  color: var(--accent-primary);
}

code {
  background: var(--bg-elevated);
  padding: 1px 5px;
  border-radius: var(--radius-sm);
  font-size: 11px;
  color: var(--accent-primary);
}

/* Element Plus InputNumber override */
:deep(.el-input-number) {
  width: 100%;
}
:deep(.el-input-number .el-input__wrapper) {
  background: var(--bg-surface) !important;
}
:deep(.el-input-number__decrease),
:deep(.el-input-number__increase) {
  background: var(--bg-elevated) !important;
  border-color: var(--border-default) !important;
  color: var(--text-secondary) !important;
}

@media (max-width: 800px) {
  .settings-layout { grid-template-columns: 1fr; }
  .settings-aside { display: none; }
  .form-row.three-col { grid-template-columns: 1fr; }
}
</style>
