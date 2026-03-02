<template>
  <div class="findings-view fade-in-up">
    <!-- 任务信息头 -->
    <div class="task-header" v-if="task">
      <div class="task-header-left">
        <button class="back-btn" @click="$router.push('/tasks')">
          <el-icon><ArrowLeft /></el-icon>
        </button>
        <div>
          <h1 class="task-name font-display">{{ task.display_name || task.id }}</h1>
          <div class="task-meta">
            <span class="lang-badge font-mono">{{ task.language }}</span>
            <span class="status-chip" :class="`chip-${task.status}`">{{ t(`status.${task.status}`) }}</span>
            <span class="meta-time font-mono">{{ formatTime(task.created_at) }}</span>
            <span v-if="task.project?.source_url" class="meta-url font-mono">
              {{ truncate(task.project.source_url, 50) }}
            </span>
          </div>
        </div>
      </div>
      <div class="task-header-right">
        <div class="findings-summary">
          <div
            v-for="sev in ['high', 'medium', 'low']"
            :key="sev"
            class="summary-badge"
            :class="`badge-${sev}`"
          >
            <span class="badge-count font-mono">{{ severityCounts[sev] || 0 }}</span>
            <span class="badge-label">{{ t(`severity.${sev}`) }}</span>
          </div>
        </div>
        <button class="icon-btn" @click="loadData" :class="{ spinning: loading }">
          <el-icon><Refresh /></el-icon>
        </button>
      </div>
    </div>

    <!-- 进行中提示 -->
    <div v-if="isRunning" class="running-banner">
      <span class="running-dot" />
      <span>{{ t(`status.${task?.status}`) }} — {{ t('common.loading') }}</span>
    </div>

    <!-- 过滤栏 -->
    <div class="filter-bar" v-if="findings.length > 0">
      <el-radio-group v-model="severityFilter" size="small">
        <el-radio-button label="all">All ({{ findings.length }})</el-radio-button>
        <el-radio-button
          v-for="sev in activeSeverities"
          :key="sev"
          :label="sev"
        >{{ t(`severity.${sev}`) }} ({{ severityCounts[sev] || 0 }})</el-radio-button>
      </el-radio-group>

      <el-radio-group v-model="auditFilter" size="small" style="margin-left: auto">
        <el-radio-button label="all">All</el-radio-button>
        <el-radio-button label="pending">{{ t('audit.pending') }}</el-radio-button>
        <el-radio-button label="completed">{{ t('audit.completed') }}</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 空状态 -->
    <div v-if="!loading && findings.length === 0 && task?.status === 'completed'" class="empty-state">
      <el-icon class="empty-icon"><CircleCheck /></el-icon>
      <p>{{ t('finding.noFindings') }}</p>
    </div>

    <!-- 漏洞列表 + 详情面板（左右布局） -->
    <div class="findings-layout" v-if="filteredFindings.length > 0">
      <!-- 左：列表 -->
      <div class="findings-list">
        <div
          v-for="finding in filteredFindings"
          :key="finding.id"
          class="finding-item"
          :class="{
            active: selectedFinding?.id === finding.id,
            [`sev-border-${finding.severity}`]: true
          }"
          @click="selectFinding(finding)"
        >
          <div class="finding-item-top">
            <span class="sev-dot" :class="`dot-${finding.severity}`" />
            <span class="finding-rule font-mono">{{ finding.rule_id }}</span>
            <span class="audit-badge" :class="`audit-${finding.audit_status}`">
              {{ finding.audit_status === 'completed'
                ? (finding.ai_result?.is_exploitable ? '⚠ ' + t('finding.exploitable') : '✓ ' + t('finding.notExploitable'))
                : t(`audit.${finding.audit_status}`)
              }}
            </span>
          </div>
          <div class="finding-file font-mono">
            {{ finding.file_path }}:{{ finding.start_line }}
          </div>
          <div class="finding-msg">{{ truncate(finding.message.replace(/\[([^\]]+)\]\(\d+\)/g, '$1'), 80) }}</div>
        </div>
      </div>

      <!-- 右：详情面板 -->
      <div class="finding-detail" v-if="selectedFinding">
        <!-- 规则 & 严重度 -->
        <div class="detail-header">
          <div class="detail-rule">
            <span class="sev-tag" :class="`tag-${selectedFinding.severity}`">
              {{ t(`severity.${selectedFinding.severity}`) }}
            </span>
            <span class="detail-rule-id font-mono">{{ selectedFinding.rule_id }}</span>
          </div>
          <button
            class="close-btn"
            @click="selectedFinding = null"
          ><el-icon><Close /></el-icon></button>
        </div>

        <!-- 文件位置 -->
        <div class="detail-location font-mono">
          <el-icon><Document /></el-icon>
          {{ selectedFinding.file_path }} : {{ selectedFinding.start_line }}
        </div>

        <!-- 描述 -->
        <div class="detail-message">
          {{ selectedFinding.message.replace(/\[([^\]]+)\]\(\d+\)/g, '$1') }}
        </div>

        <!-- 代码片段 -->
        <div class="detail-section">
          <div class="detail-section-title">
            <el-icon><Code /></el-icon>
            {{ t('finding.code') }}
          </div>
          <div class="code-block">
            <pre class="code-pre font-mono">{{ selectedFinding.code_snippet }}</pre>
          </div>
        </div>

        <!-- AI 审计区 -->
        <div class="detail-section">
          <div class="detail-section-title">
            <el-icon><MagicStick /></el-icon>
            {{ t('finding.aiResult') }}
            <span class="audit-status-badge" :class="`audit-${selectedFinding.audit_status}`">
              {{ t(`audit.${selectedFinding.audit_status}`) }}
            </span>
          </div>

          <!-- 未审计：显示触发按钮 -->
          <div v-if="selectedFinding.audit_status === 'pending'" class="audit-trigger">
            <p class="audit-hint">AI will analyze the data flow and generate a PoC if exploitable.</p>
            <button
              class="trigger-btn"
              :disabled="triggering"
              @click="handleTriggerAudit(selectedFinding.id)"
            >
              <el-icon><VideoPlay /></el-icon>
              {{ triggering ? t('common.loading') : t('finding.triggerAudit') }}
            </button>
          </div>

          <!-- 审计中 -->
          <div v-else-if="selectedFinding.audit_status === 'processing'" class="audit-processing">
            <span class="processing-dot" />
            AI is analyzing this finding...
          </div>

          <!-- 审计完成 -->
          <div v-else-if="selectedFinding.audit_status === 'completed' && selectedFinding.ai_result" class="ai-result">
            <!-- 结论 -->
            <div class="result-verdict" :class="selectedFinding.ai_result.is_exploitable ? 'verdict-danger' : 'verdict-safe'">
              <el-icon>
                <component :is="selectedFinding.ai_result.is_exploitable ? 'Warning' : 'CircleCheck'" />
              </el-icon>
              <span>{{ selectedFinding.ai_result.is_exploitable ? t('finding.exploitable') : t('finding.notExploitable') }}</span>
              <span class="confidence font-mono">
                {{ t('finding.confidence') }}: {{ (selectedFinding.ai_result.confidence * 100).toFixed(0) }}%
              </span>
            </div>

            <!-- 分析逻辑 -->
            <div class="result-block">
              <div class="result-block-title">{{ t('finding.analysis') }}</div>
              <p class="result-text">{{ selectedFinding.ai_result.analysis_logic }}</p>
            </div>

            <!-- PoC -->
            <div class="result-block" v-if="selectedFinding.ai_result.poc_content !== 'N/A'">
              <div class="result-block-title">
                {{ t('finding.pocContent') }}
                <span class="poc-type-badge font-mono">{{ selectedFinding.ai_result.poc_type }}</span>
                <button class="copy-btn font-mono" @click="copyPoC(selectedFinding.ai_result!.poc_content)">
                  {{ copied ? t('common.copied') : t('common.copy') }}
                </button>
              </div>
              <div class="poc-block">
                <pre class="font-mono poc-pre">{{ selectedFinding.ai_result.poc_content }}</pre>
              </div>
            </div>

            <!-- Token 信息 -->
            <div class="result-meta font-mono">
              <span>Model: {{ selectedFinding.ai_result.model_used }}</span>
              <span>Tokens: {{ selectedFinding.ai_result.prompt_tokens + selectedFinding.ai_result.completion_tokens }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { useTaskStore } from '@/stores'
import * as api from '@/api'
import dayjs from 'dayjs'
import type { Finding, Task, TaskStatus } from '@/api/types'

const { t } = useI18n()
const route = useRoute()
const taskStore = useTaskStore()

const task = ref<Task | null>(null)
const findings = ref<Finding[]>([])
const selectedFinding = ref<Finding | null>(null)
const severityFilter = ref('all')
const auditFilter = ref('all')
const loading = ref(false)
const triggering = ref(false)
const copied = ref(false)

const taskId = route.params.id as string

const isRunning = computed(() =>
  task.value && ['pending','cloning','building','analyzing'].includes(task.value.status)
)

const severityCounts = computed(() => {
  const c: Record<string, number> = {}
  findings.value.forEach(f => { c[f.severity] = (c[f.severity] || 0) + 1 })
  return c
})

const activeSeverities = computed(() =>
  ['critical','high','medium','low','note'].filter(s => severityCounts.value[s] > 0)
)

const filteredFindings = computed(() => {
  let result = findings.value
  if (severityFilter.value !== 'all') result = result.filter(f => f.severity === severityFilter.value)
  if (auditFilter.value !== 'all') result = result.filter(f => f.audit_status === auditFilter.value)
  return result
})

async function loadData() {
  loading.value = true
  try {
    task.value = await api.getTask(taskId)
    taskStore.addTask(task.value)
    if (task.value.status === 'completed' || findings.value.length === 0) {
      const res = await api.getFindings(taskId)
      findings.value = res.items
      taskStore.loadFindings(taskId)
    }
  } finally {
    loading.value = false
  }
}

function selectFinding(f: Finding) {
  selectedFinding.value = f
}

async function handleTriggerAudit(findingId: string) {
  triggering.value = true
  try {
    await api.triggerAudit(findingId)
    ElMessage.success(t('finding.auditQueued'))
    // 更新该 finding 状态为 processing
    const idx = findings.value.findIndex(f => f.id === findingId)
    if (idx >= 0) findings.value[idx] = { ...findings.value[idx], audit_status: 'processing' }
    if (selectedFinding.value?.id === findingId) {
      selectedFinding.value = { ...selectedFinding.value, audit_status: 'processing' }
    }
  } finally {
    triggering.value = false
  }
}

async function copyPoC(content: string) {
  await navigator.clipboard.writeText(content)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

const formatTime = (s: string) => dayjs(s).format('YYYY-MM-DD HH:mm')
const truncate = (s: string, n: number) => s.length > n ? s.slice(0, n) + '...' : s

// 轮询：进行中的任务每 5 秒刷新一次，审计中的 finding 也轮询
let pollTimer: ReturnType<typeof setInterval>
onMounted(async () => {
  await loadData()
  pollTimer = setInterval(async () => {
    if (!task.value) return
    // 刷新任务状态
    if (['pending','cloning','building','analyzing'].includes(task.value.status)) {
      await loadData()
    }
    // 刷新 processing 状态的 finding
    const processingIds = findings.value.filter(f => f.audit_status === 'processing').map(f => f.id)
    if (processingIds.length > 0) {
      const res = await api.getFindings(taskId)
      findings.value = res.items
      // 同步 selectedFinding
      if (selectedFinding.value) {
        const updated = res.items.find(f => f.id === selectedFinding.value!.id)
        if (updated) selectedFinding.value = updated
      }
    }
  }, 5000)
})
onUnmounted(() => clearInterval(pollTimer))
</script>

<style scoped>
.findings-view { max-width: 1400px; }

/* 任务头部 */
.task-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border-subtle);
}

.task-header-left { display: flex; align-items: flex-start; gap: 14px; }

.back-btn {
  width: 36px; height: 36px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all var(--transition-fast);
  flex-shrink: 0;
  margin-top: 2px;
}

.back-btn:hover { border-color: var(--accent-primary); color: var(--accent-primary); }

.task-name {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
}

.task-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.lang-badge {
  font-size: 10px;
  padding: 2px 7px;
  background: var(--accent-glow);
  color: var(--accent-primary);
  border: 1px solid rgba(14,165,233,0.2);
  border-radius: var(--radius-sm);
  font-weight: 600;
}

.status-chip {
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: var(--radius-sm);
}

.chip-completed { background: rgba(34,197,94,0.1); color: var(--status-completed); }
.chip-failed    { background: rgba(244,63,94,0.1);  color: var(--status-failed); }
.chip-analyzing,
.chip-building  { background: var(--accent-glow);   color: var(--accent-primary); }

.meta-time, .meta-url {
  font-size: 11px;
  color: var(--text-muted);
}

.task-header-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.findings-summary {
  display: flex;
  gap: 8px;
}

.summary-badge {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 6px 12px;
  border-radius: var(--radius-md);
  border: 1px solid;
}

.badge-high   { border-color: rgba(249,115,22,0.3); background: rgba(249,115,22,0.08); }
.badge-medium { border-color: rgba(234,179,8,0.3);  background: rgba(234,179,8,0.08); }
.badge-low    { border-color: rgba(34,197,94,0.3);  background: rgba(34,197,94,0.08); }

.badge-count {
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}

.badge-high .badge-count   { color: var(--severity-high); }
.badge-medium .badge-count { color: var(--severity-medium); }
.badge-low .badge-count    { color: var(--severity-low); }

.badge-label {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 2px;
}

.icon-btn {
  width: 36px; height: 36px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all var(--transition-fast);
}
.icon-btn:hover { border-color: var(--accent-primary); color: var(--accent-primary); }
.icon-btn.spinning .el-icon { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

/* 进行中横幅 */
.running-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  background: var(--accent-glow);
  border: 1px solid rgba(14,165,233,0.2);
  border-radius: var(--radius-md);
  margin-bottom: 16px;
  font-size: 13px;
  color: var(--accent-primary);
}

.running-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--accent-primary);
  animation: pulse-glow 1.5s ease-in-out infinite;
}

/* 过滤栏 */
.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

/* 漏洞布局 */
.findings-layout {
  display: grid;
  grid-template-columns: 380px 1fr;
  gap: 16px;
  align-items: start;
}

/* 漏洞列表 */
.findings-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: calc(100vh - 260px);
  overflow-y: auto;
}

.finding-item {
  padding: 10px 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-left-width: 3px;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.finding-item:hover { background: var(--bg-hover); }
.finding-item.active { background: var(--bg-elevated); border-color: var(--accent-primary); border-left-color: var(--accent-primary); }

.sev-border-critical { border-left-color: var(--severity-critical); }
.sev-border-high     { border-left-color: var(--severity-high); }
.sev-border-medium   { border-left-color: var(--severity-medium); }
.sev-border-low      { border-left-color: var(--severity-low); }
.sev-border-note     { border-left-color: var(--severity-note); }

.finding-item-top {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 4px;
}

.sev-dot {
  width: 6px; height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-critical { background: var(--severity-critical); }
.dot-high     { background: var(--severity-high); }
.dot-medium   { background: var(--severity-medium); }
.dot-low      { background: var(--severity-low); }
.dot-note     { background: var(--severity-note); }

.finding-rule {
  font-size: 11px;
  color: var(--accent-primary);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audit-badge {
  font-size: 10px;
  font-family: var(--font-mono);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  font-weight: 600;
  white-space: nowrap;
}

.audit-pending    { background: rgba(107,114,128,0.15); color: var(--text-muted); }
.audit-processing { background: var(--accent-glow); color: var(--accent-primary); }
.audit-completed  { background: rgba(34,197,94,0.1); color: var(--status-completed); }

.finding-file {
  font-size: 10px;
  color: var(--text-muted);
  margin-bottom: 3px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.finding-msg {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

/* 详情面板 */
.finding-detail {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 20px;
  max-height: calc(100vh - 260px);
  overflow-y: auto;
  position: sticky;
  top: 0;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.detail-rule {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sev-tag {
  font-size: 10px;
  font-family: var(--font-mono);
  font-weight: 700;
  padding: 2px 8px;
  border-radius: var(--radius-sm);
}

.tag-critical { background: rgba(244,63,94,0.15); color: var(--severity-critical); }
.tag-high     { background: rgba(249,115,22,0.15); color: var(--severity-high); }
.tag-medium   { background: rgba(234,179,8,0.15);  color: var(--severity-medium); }
.tag-low      { background: rgba(34,197,94,0.15);  color: var(--severity-low); }
.tag-note     { background: rgba(107,114,128,0.15);color: var(--severity-note); }

.detail-rule-id {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  width: 28px; height: 28px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  transition: all var(--transition-fast);
}
.close-btn:hover { border-color: var(--severity-critical); color: var(--severity-critical); }

.detail-location {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 10px;
  background: var(--bg-elevated);
  padding: 6px 10px;
  border-radius: var(--radius-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-message {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.6;
  margin-bottom: 16px;
}

.detail-section { margin-bottom: 16px; }

.detail-section-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.8px;
  margin-bottom: 8px;
}

.audit-status-badge {
  font-family: var(--font-mono);
  font-size: 10px;
  padding: 1px 6px;
  border-radius: var(--radius-sm);
}

.code-block {
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: auto;
  max-height: 240px;
}

.code-pre {
  font-size: 11.5px;
  line-height: 1.7;
  color: var(--text-secondary);
  padding: 12px;
  white-space: pre;
  margin: 0;
}

/* AI 审计区 */
.audit-trigger {
  background: var(--bg-elevated);
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-md);
  padding: 16px;
  text-align: center;
}

.audit-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 12px;
}

.trigger-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 20px;
  background: var(--accent-primary);
  border: none;
  border-radius: var(--radius-md);
  color: var(--text-inverse);
  font-size: 13px;
  font-weight: 600;
  font-family: var(--font-body);
  cursor: pointer;
  transition: all var(--transition-fast);
  box-shadow: 0 0 16px rgba(14,165,233,0.3);
}

.trigger-btn:hover:not(:disabled) {
  background: #38bdf8;
  box-shadow: 0 0 24px rgba(14,165,233,0.5);
}

.trigger-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.audit-processing {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  background: var(--accent-glow);
  border-radius: var(--radius-md);
  font-size: 13px;
  color: var(--accent-primary);
}

.processing-dot {
  width: 8px; height: 8px;
  border-radius: 50%;
  background: var(--accent-primary);
  animation: pulse-glow 1.2s ease-in-out infinite;
}

.result-verdict {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: var(--radius-md);
  margin-bottom: 14px;
  font-weight: 600;
  font-size: 14px;
}

.verdict-danger {
  background: rgba(244,63,94,0.1);
  border: 1px solid rgba(244,63,94,0.25);
  color: var(--severity-critical);
}

.verdict-safe {
  background: rgba(34,197,94,0.1);
  border: 1px solid rgba(34,197,94,0.25);
  color: var(--status-completed);
}

.confidence {
  margin-left: auto;
  font-size: 12px;
  opacity: 0.8;
}

.result-block { margin-bottom: 14px; }

.result-block-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.result-text {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.7;
}

.poc-type-badge {
  font-size: 10px;
  padding: 1px 6px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  color: var(--text-muted);
}

.copy-btn {
  margin-left: auto;
  font-size: 11px;
  padding: 2px 8px;
  background: none;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.copy-btn:hover { border-color: var(--accent-primary); color: var(--accent-primary); }

.poc-block {
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: auto;
  max-height: 200px;
}

.poc-pre {
  font-size: 11.5px;
  line-height: 1.7;
  color: #7dd3fc;
  padding: 12px;
  white-space: pre;
  margin: 0;
}

.result-meta {
  display: flex;
  gap: 16px;
  font-size: 10px;
  color: var(--text-muted);
  padding-top: 10px;
  border-top: 1px solid var(--border-subtle);
}

/* 空状态 */
.empty-state {
  text-align: center;
  padding: 60px;
  color: var(--text-muted);
}
.empty-icon { font-size: 48px; display: block; margin-bottom: 12px; color: var(--status-completed); }

@media (max-width: 900px) {
  .findings-layout { grid-template-columns: 1fr; }
}
</style>
