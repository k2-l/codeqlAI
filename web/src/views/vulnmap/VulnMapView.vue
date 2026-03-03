<template>
  <div class="vulnmap-view fade-in-up">
    <div class="page-header">
      <h1 class="page-title font-display">漏洞地图</h1>
      <div class="header-right">
        <!-- 项目选择器 -->
        <el-select
          v-model="selectedTaskId"
          placeholder="选择已完成的扫描任务"
          style="width: 300px"
          @change="loadMap"
          filterable
        >
          <el-option
            v-for="task in completedTasks"
            :key="task.id"
            :value="task.id"
            :label="task.display_name || task.id"
          >
            <div class="task-option">
              <span class="font-mono lang-tag">{{ task.language }}</span>
              <span>{{ task.display_name || task.id }}</span>
            </div>
          </el-option>
        </el-select>
      </div>
    </div>

    <!-- 主体：图 + 右侧栏 -->
    <div class="map-layout" v-if="selectedTaskId">
      <!-- 右侧筛选/列表 -->
      <div class="map-sidebar">
        <div class="sidebar-header">
          <span class="sidebar-title font-display">数据流漏洞</span>
          <span class="sidebar-count font-mono">{{ filteredFlows.length }}/{{ flows.length }}</span>
        </div>

        <!-- 严重度筛选 -->
        <div class="sev-filters">
          <button
            v-for="sev in severities"
            :key="sev.key"
            class="sev-btn"
            :class="{ active: sevFilter === sev.key }"
            @click="sevFilter = sev.key"
          >
            <span class="sev-dot" :class="`dot-${sev.key}`" />
            {{ sev.label }}
            <span class="sev-cnt font-mono">{{ sev.count }}</span>
          </button>
        </div>

        <!-- Finding 列表 -->
        <div class="flow-list">
          <div v-if="loading" class="list-loading">
            <span class="spinner" />加载中...
          </div>
          <div v-else-if="filteredFlows.length === 0" class="list-empty">
            <el-icon><Warning /></el-icon>
            <p>该任务无数据流漏洞</p>
          </div>
          <div
            v-for="(flow, idx) in filteredFlows"
            :key="idx"
            class="flow-item"
            :class="{ active: selectedFlow === flow }"
            @click="selectFlow(flow)"
          >
            <div class="flow-item-top">
              <span class="sev-dot-sm" :class="`dot-${flow.severity}`" />
              <span class="flow-rule font-mono">{{ shortRule(flow.rule_id) }}</span>
              <span class="flow-nodes font-mono">{{ flow.flows[0]?.nodes.length ?? 0 }} nodes</span>
            </div>
            <div class="flow-file font-mono">{{ basename(flow.file_path) }}:{{ flow.line }}</div>
            <div class="flow-msg">{{ truncate(flow.message, 60) }}</div>
          </div>
        </div>
      </div>

      <!-- 中间：图形区域 -->
      <div class="map-canvas" ref="canvasRef">
        <!-- 空提示 -->
        <div v-if="!selectedFlow" class="canvas-empty">
          <div class="canvas-empty-inner">
            <div class="grid-bg" />
            <div class="canvas-hint">
              <el-icon><Share /></el-icon>
              <p>← 从左侧选择一条数据流漏洞</p>
              <p class="hint-sub font-mono">只展示含完整 codeFlows 的 finding</p>
            </div>
          </div>
        </div>

        <!-- SVG 数据流图 -->
        <div v-else class="flow-graph-wrapper">
          <div class="graph-title">
            <span class="sev-tag-lg" :class="`tag-${selectedFlow.severity}`">
              {{ selectedFlow.severity.toUpperCase() }}
            </span>
            <span class="graph-rule font-mono">{{ selectedFlow.rule_id }}</span>
          </div>

          <!-- 路径选择（多路径时） -->
          <div class="path-tabs" v-if="selectedFlow.flows.length > 1">
            <button
              v-for="(_, i) in selectedFlow.flows"
              :key="i"
              class="path-tab font-mono"
              :class="{ active: selectedPathIdx === i }"
              @click="selectedPathIdx = i"
            >Path {{ i + 1 }}</button>
          </div>

          <!-- 节点链路图 -->
          <div class="flow-chain" v-if="currentPath">
            <div
              v-for="(node, i) in currentPath.nodes"
              :key="i"
              class="flow-chain-item"
            >
              <!-- 节点卡片 -->
              <div class="node-card" :class="getNodeClass(i, currentPath.nodes.length)">
                <div class="node-badge font-mono">
                  <span v-if="i === 0">SOURCE</span>
                  <span v-else-if="i === currentPath.nodes.length - 1">SINK</span>
                  <span v-else>STEP {{ i }}</span>
                </div>
                <div class="node-file font-mono">{{ basename(node.file_path) }}</div>
                <div class="node-line font-mono">Line {{ node.line }}{{ node.column ? ', Col ' + node.column : '' }}</div>
                <div class="node-msg" v-if="node.message">{{ node.message }}</div>
                <div class="node-path font-mono">{{ truncatePath(node.file_path) }}</div>
              </div>

              <!-- 连接箭头（最后一个节点后不加） -->
              <div class="node-arrow" v-if="i < currentPath.nodes.length - 1">
                <div class="arrow-line" />
                <div class="arrow-head">▼</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- 未选择任务的初始状态 -->
    <div v-else class="initial-state">
      <div class="initial-inner">
        <div class="initial-grid" />
        <el-icon class="initial-icon"><Share /></el-icon>
        <p class="initial-title font-display">选择一个已完成的扫描任务</p>
        <p class="initial-sub">系统将解析 SARIF 文件，展示有完整数据流（Source → Sink）的漏洞链路</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import * as api from '@/api'
import type { Task, FindingFlow, FlowPath } from '@/api/types'

const completedTasks  = ref<Task[]>([])
const selectedTaskId  = ref<string>('')
const flows           = ref<FindingFlow[]>([])
const selectedFlow    = ref<FindingFlow | null>(null)
const selectedPathIdx = ref(0)
const sevFilter       = ref('all')
const loading         = ref(false)
const canvasRef       = ref<HTMLElement | null>(null)

const severities = computed(() => {
  const counts: Record<string, number> = { all: flows.value.length }
  flows.value.forEach(f => { counts[f.severity] = (counts[f.severity] || 0) + 1 })
  return [
    { key: 'all',    label: 'All',    count: counts.all    || 0 },
    { key: 'high',   label: 'High',   count: counts.high   || 0 },
    { key: 'medium', label: 'Medium', count: counts.medium || 0 },
    { key: 'low',    label: 'Low',    count: counts.low    || 0 },
  ].filter(s => s.key === 'all' || s.count > 0)
})

const filteredFlows = computed(() =>
  sevFilter.value === 'all'
    ? flows.value
    : flows.value.filter(f => f.severity === sevFilter.value)
)

const currentPath = computed<FlowPath | null>(() =>
  selectedFlow.value?.flows[selectedPathIdx.value] ?? null
)

async function loadTasks() {
  const res = await api.listTasks('completed')
  completedTasks.value = res.items
}

async function loadMap() {
  if (!selectedTaskId.value) return
  loading.value = true
  selectedFlow.value = null
  try {
    const res = await api.getVulnMap(selectedTaskId.value)
    flows.value = res.items || []
  } finally {
    loading.value = false
  }
}

function selectFlow(flow: FindingFlow) {
  selectedFlow.value = flow
  selectedPathIdx.value = 0
}

function getNodeClass(idx: number, total: number) {
  if (idx === 0)         return 'node-source'
  if (idx === total - 1) return 'node-sink'
  return 'node-step'
}

const basename     = (p: string) => p.split('/').pop() || p
const truncate     = (s: string, n: number) => s.length > n ? s.slice(0, n) + '…' : s
const truncatePath = (p: string) => p.length > 50 ? '...' + p.slice(-47) : p
const shortRule    = (r: string) => r.split('/').pop()?.split('.').pop() || r

onMounted(loadTasks)
</script>

<style scoped>
.vulnmap-view { max-width: 1400px; height: calc(100vh - 100px); display: flex; flex-direction: column; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px; flex-shrink: 0;
}

.page-title { font-size: 22px; font-weight: 700; color: var(--text-primary); }

.task-option { display: flex; align-items: center; gap: 8px; }
.lang-tag {
  font-size: 10px; font-weight: 700; color: var(--accent-primary);
  background: var(--accent-glow); padding: 1px 6px;
  border-radius: var(--radius-sm); border: 1px solid rgba(14,165,233,0.2);
}

/* 主布局 */
.map-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
  flex: 1;
  min-height: 0;
}

/* 右侧边栏 */
.map-sidebar {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 14px 16px;
  border-bottom: 1px solid var(--border-subtle);
}

.sidebar-title { font-size: 13px; font-weight: 700; color: var(--text-primary); }
.sidebar-count { font-size: 11px; color: var(--text-muted); }

.sev-filters {
  padding: 10px 10px 6px;
  display: flex; flex-direction: column; gap: 3px;
}

.sev-btn {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; background: none;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  color: var(--text-secondary); font-size: 12px;
  font-family: var(--font-body); cursor: pointer;
  transition: all var(--transition-fast);
}

.sev-btn:hover { background: var(--bg-hover); }
.sev-btn.active { background: var(--bg-elevated); border-color: var(--border-default); color: var(--text-primary); }

.sev-dot {
  width: 7px; height: 7px; border-radius: 50%; flex-shrink: 0;
}

.dot-high     { background: var(--severity-high); }
.dot-medium   { background: var(--severity-medium); }
.dot-low      { background: var(--severity-low); }
.dot-critical { background: var(--severity-critical); }

.sev-cnt { margin-left: auto; font-size: 11px; color: var(--text-muted); }

.flow-list {
  flex: 1; overflow-y: auto;
  padding: 6px 10px;
  display: flex; flex-direction: column; gap: 4px;
}

.list-loading, .list-empty {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 8px;
  padding: 30px; color: var(--text-muted); font-size: 13px;
}

.flow-item {
  padding: 9px 10px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.flow-item:hover { background: var(--bg-hover); }
.flow-item.active { background: var(--bg-elevated); border-color: var(--accent-primary); }

.flow-item-top {
  display: flex; align-items: center; gap: 6px; margin-bottom: 3px;
}

.sev-dot-sm { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }

.flow-rule { font-size: 11px; color: var(--accent-primary); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.flow-nodes { font-size: 10px; color: var(--text-muted); white-space: nowrap; }

.flow-file { font-size: 10px; color: var(--text-muted); margin-bottom: 2px; }
.flow-msg  { font-size: 11px; color: var(--text-secondary); line-height: 1.4; }

/* 图形画布 */
.map-canvas {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: auto;
  position: relative;
}

.canvas-empty {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center;
}

.canvas-empty-inner { position: relative; width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; }

.grid-bg {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(var(--border-subtle) 1px, transparent 1px),
    linear-gradient(90deg, var(--border-subtle) 1px, transparent 1px);
  background-size: 32px 32px;
  opacity: 0.4;
}

.canvas-hint {
  position: relative; z-index: 1;
  text-align: center; color: var(--text-muted);
  display: flex; flex-direction: column; align-items: center; gap: 8px;
}

.canvas-hint .el-icon { font-size: 40px; opacity: 0.4; }
.canvas-hint p { font-size: 14px; }
.hint-sub { font-size: 11px; color: var(--text-muted); }

/* 流图 */
.flow-graph-wrapper {
  padding: 24px;
  min-height: 100%;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.graph-title {
  display: flex; align-items: center; gap: 10px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle);
}

.sev-tag-lg {
  font-size: 11px; font-family: var(--font-mono); font-weight: 700;
  padding: 3px 10px; border-radius: var(--radius-sm);
}

.tag-high     { background: rgba(249,115,22,0.15); color: var(--severity-high); }
.tag-medium   { background: rgba(234,179,8,0.15);  color: var(--severity-medium); }
.tag-low      { background: rgba(34,197,94,0.15);  color: var(--severity-low); }
.tag-critical { background: rgba(244,63,94,0.15);  color: var(--severity-critical); }

.graph-rule { font-size: 13px; font-weight: 600; color: var(--text-primary); }

.path-tabs {
  display: flex; gap: 6px;
}

.path-tab {
  padding: 4px 12px; background: var(--bg-elevated);
  border: 1px solid var(--border-default); border-radius: var(--radius-sm);
  font-size: 11px; color: var(--text-secondary); cursor: pointer;
  transition: all var(--transition-fast);
}

.path-tab.active { border-color: var(--accent-primary); color: var(--accent-primary); background: var(--accent-glow); }

/* 节点链路 */
.flow-chain {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  max-width: 600px;
  margin: 0 auto;
}

.flow-chain-item {
  display: flex; flex-direction: column; align-items: center;
  width: 100%;
}

.node-card {
  width: 100%;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  border: 1px solid;
  transition: all var(--transition-fast);
}

.node-source {
  background: rgba(14,165,233,0.08);
  border-color: rgba(14,165,233,0.3);
  box-shadow: 0 0 16px rgba(14,165,233,0.1);
}

.node-sink {
  background: rgba(244,63,94,0.08);
  border-color: rgba(244,63,94,0.3);
  box-shadow: 0 0 16px rgba(244,63,94,0.1);
}

.node-step {
  background: var(--bg-elevated);
  border-color: var(--border-default);
}

.node-badge {
  font-size: 9px; font-weight: 700; letter-spacing: 1px;
  margin-bottom: 6px;
}

.node-source .node-badge { color: var(--accent-primary); }
.node-sink   .node-badge { color: var(--severity-critical); }
.node-step   .node-badge { color: var(--text-muted); }

.node-file  { font-size: 12px; color: var(--text-primary); font-weight: 600; margin-bottom: 2px; }
.node-line  { font-size: 11px; color: var(--text-secondary); margin-bottom: 4px; }
.node-msg   { font-size: 12px; color: var(--text-primary); margin-bottom: 4px; }
.node-path  { font-size: 10px; color: var(--text-muted); }

.node-arrow {
  display: flex; flex-direction: column; align-items: center;
  padding: 4px 0;
}

.arrow-line {
  width: 1px; height: 20px;
  background: linear-gradient(to bottom, var(--border-default), var(--accent-primary));
}

.arrow-head {
  font-size: 10px; color: var(--accent-primary); line-height: 1;
  margin-top: -2px;
}

/* 初始状态 */
.initial-state {
  flex: 1;
  display: flex; align-items: center; justify-content: center;
}

.initial-inner {
  text-align: center; position: relative;
  width: 500px; height: 320px;
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 12px;
}

.initial-grid {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(var(--border-subtle) 1px, transparent 1px),
    linear-gradient(90deg, var(--border-subtle) 1px, transparent 1px);
  background-size: 32px 32px; opacity: 0.3; border-radius: var(--radius-lg);
}

.initial-icon { font-size: 52px; color: var(--accent-primary); opacity: 0.3; position: relative; }
.initial-title { font-size: 16px; color: var(--text-primary); position: relative; }
.initial-sub { font-size: 12px; color: var(--text-muted); position: relative; max-width: 340px; line-height: 1.6; }

.spinner {
  width: 14px; height: 14px;
  border: 2px solid var(--border-default);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  display: inline-block;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
