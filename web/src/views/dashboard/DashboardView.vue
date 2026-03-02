<template>
  <div class="dashboard fade-in-up">
    <!-- 页头 -->
    <div class="page-header">
      <h1 class="page-title font-display">{{ t('dashboard.title') }}</h1>
      <div class="header-meta">
        <span class="meta-dot" />
        <span class="meta-text font-mono">LIVE</span>
      </div>
    </div>

    <!-- 统计卡片 -->
    <div class="stats-grid">
      <div class="stat-card" v-for="card in statCards" :key="card.key" :class="`card-${card.color}`">
        <div class="stat-card-inner">
          <div class="stat-icon">
            <el-icon><component :is="card.icon" /></el-icon>
          </div>
          <div class="stat-info">
            <div class="stat-value font-mono">{{ card.value }}</div>
            <div class="stat-label">{{ t(card.label) }}</div>
          </div>
          <div class="stat-bg-icon">
            <el-icon><component :is="card.icon" /></el-icon>
          </div>
        </div>
      </div>
    </div>

    <!-- 任务列表（最近5条） -->
    <div class="section">
      <div class="section-header">
        <h2 class="section-title font-display">Recent Tasks</h2>
        <router-link to="/tasks" class="section-link">
          {{ t('task.title') }} →
        </router-link>
      </div>

      <div class="task-timeline">
        <div v-if="taskStore.tasks.length === 0" class="empty-state">
          <el-icon class="empty-icon"><DocumentRemove /></el-icon>
          <p>{{ t('task.noTasks') }}</p>
          <router-link to="/new-scan" class="empty-action">{{ t('nav.newScan') }}</router-link>
        </div>
        <div
          v-for="task in recentTasks"
          :key="task.id"
          class="timeline-item"
          @click="$router.push(`/tasks/${task.id}`)"
        >
          <div class="timeline-status" :class="`status-${task.status}`">
            <span v-if="isRunning(task.status)" class="status-pulse" />
          </div>
          <div class="timeline-content">
            <div class="timeline-name">{{ task.display_name || task.id }}</div>
            <div class="timeline-meta">
              <span class="font-mono lang-badge">{{ task.language }}</span>
              <span class="timeline-time">{{ formatTime(task.created_at) }}</span>
            </div>
          </div>
          <div class="timeline-status-text" :class="`text-status-${task.status}`">
            {{ t(`status.${task.status}`) }}
          </div>
        </div>
      </div>
    </div>

    <!-- 漏洞严重度分布 -->
    <div class="section" v-if="taskStore.findings.length > 0">
      <div class="section-header">
        <h2 class="section-title font-display">Severity Distribution</h2>
      </div>
      <div class="severity-bars">
        <div v-for="s in severityList" :key="s.key" class="severity-row">
          <span class="severity-label font-mono" :class="`sev-${s.key}`">{{ t(`severity.${s.key}`) }}</span>
          <div class="severity-track">
            <div
              class="severity-fill"
              :class="`fill-${s.key}`"
              :style="{ width: `${s.percent}%` }"
            />
          </div>
          <span class="severity-count font-mono">{{ s.count }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTaskStore } from '@/stores'
import dayjs from 'dayjs'
import type { TaskStatus } from '@/api/types'

const { t } = useI18n()
const taskStore = useTaskStore()

const statCards = computed(() => [
  { key: 'total',      label: 'dashboard.totalTasks',     value: taskStore.stats.total_tasks,          icon: 'List',         color: 'blue'   },
  { key: 'completed',  label: 'dashboard.completedTasks', value: taskStore.stats.completed_tasks,      icon: 'CircleCheck',  color: 'green'  },
  { key: 'running',    label: 'dashboard.runningTasks',   value: taskStore.stats.running_tasks,        icon: 'Loading',      color: 'cyan'   },
  { key: 'failed',     label: 'dashboard.failedTasks',    value: taskStore.stats.failed_tasks,         icon: 'CircleClose',  color: 'red'    },
  { key: 'findings',   label: 'dashboard.totalFindings',  value: taskStore.stats.total_findings,       icon: 'Warning',      color: 'orange' },
  { key: 'high',       label: 'dashboard.highFindings',   value: taskStore.stats.high_findings,        icon: 'AlarmClock',   color: 'red'    },
  { key: 'audited',    label: 'dashboard.auditedFindings',value: taskStore.stats.audited_findings,     icon: 'Finished',     color: 'purple' },
  { key: 'exploitable',label: 'dashboard.exploitable',    value: taskStore.stats.exploitable_findings, icon: 'MagicStick',   color: 'danger' },
])

const recentTasks = computed(() => taskStore.tasks.slice(0, 5))

const severityList = computed(() => {
  const total = taskStore.findings.length || 1
  const counts: Record<string, number> = { critical: 0, high: 0, medium: 0, low: 0, note: 0 }
  taskStore.findings.forEach(f => { counts[f.severity] = (counts[f.severity] || 0) + 1 })
  return Object.entries(counts).map(([key, count]) => ({
    key, count, percent: Math.round((count / total) * 100),
  }))
})

const isRunning = (status: TaskStatus) =>
  ['pending','cloning','building','analyzing'].includes(status)

const formatTime = (t: string) => dayjs(t).format('MM-DD HH:mm')

onMounted(() => {
  // 如果有 tasks 但没有 findings，尝试加载第一个已完成任务的 findings
  const completed = taskStore.tasks.find(t => t.status === 'completed')
  if (completed && taskStore.findings.length === 0) {
    taskStore.loadFindings(completed.id)
  }
})
</script>

<style scoped>
.dashboard {
  max-width: 1200px;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.header-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.meta-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--accent-primary);
  animation: pulse-glow 2s ease-in-out infinite;
}

.meta-text {
  font-size: 10px;
  color: var(--accent-primary);
  letter-spacing: 2px;
}

/* 统计卡片 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-bottom: 28px;
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all var(--transition-normal);
  cursor: default;
}

.stat-card:hover {
  border-color: var(--border-accent);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0,0,0,0.3);
}

.stat-card-inner {
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  position: relative;
  overflow: hidden;
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.card-blue   .stat-icon { background: rgba(14,165,233,0.12); color: #0ea5e9; }
.card-green  .stat-icon { background: rgba(34,197,94,0.12);  color: #22c55e; }
.card-cyan   .stat-icon { background: rgba(6,182,212,0.12);  color: #06b6d4; }
.card-red    .stat-icon { background: rgba(244,63,94,0.12);  color: #f43f5e; }
.card-orange .stat-icon { background: rgba(249,115,22,0.12); color: #f97316; }
.card-purple .stat-icon { background: rgba(168,85,247,0.12); color: #a855f7; }
.card-danger .stat-icon { background: rgba(244,63,94,0.12);  color: #f43f5e; }

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1;
}

.stat-label {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 3px;
}

.stat-bg-icon {
  position: absolute;
  right: 12px;
  font-size: 52px;
  opacity: 0.04;
  color: var(--text-primary);
}

/* Section */
.section {
  margin-bottom: 28px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.section-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.section-link {
  font-size: 12px;
  color: var(--accent-primary);
  text-decoration: none;
  font-family: var(--font-mono);
}

/* Timeline */
.task-timeline {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.timeline-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle);
  cursor: pointer;
  transition: background var(--transition-fast);
}

.timeline-item:last-child { border-bottom: none; }

.timeline-item:hover { background: var(--bg-hover); }

.timeline-status {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  position: relative;
}

.status-pending    { background: var(--status-pending); }
.status-cloning,
.status-building,
.status-analyzing  { background: var(--status-running); }
.status-completed  { background: var(--status-completed); }
.status-failed     { background: var(--status-failed); }

.status-pulse {
  position: absolute;
  inset: -3px;
  border-radius: 50%;
  border: 1px solid var(--accent-primary);
  animation: pulse-glow 1.5s ease-in-out infinite;
}

.timeline-content { flex: 1; min-width: 0; }

.timeline-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.timeline-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
}

.lang-badge {
  font-size: 10px;
  color: var(--accent-primary);
  background: var(--accent-glow);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid rgba(14,165,233,0.2);
}

.timeline-time {
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.timeline-status-text {
  font-size: 11px;
  font-family: var(--font-mono);
  font-weight: 600;
}

.text-status-completed { color: var(--status-completed); }
.text-status-failed    { color: var(--status-failed); }
.text-status-pending   { color: var(--status-pending); }
.text-status-cloning,
.text-status-building,
.text-status-analyzing { color: var(--status-running); }

/* Empty */
.empty-state {
  padding: 40px;
  text-align: center;
  color: var(--text-muted);
}

.empty-icon { font-size: 40px; margin-bottom: 12px; display: block; }
.empty-action {
  display: inline-block;
  margin-top: 12px;
  color: var(--accent-primary);
  text-decoration: none;
  font-size: 13px;
}

/* Severity bars */
.severity-bars {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.severity-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.severity-label {
  width: 48px;
  font-size: 11px;
  font-weight: 600;
  text-align: right;
}

.sev-critical { color: var(--severity-critical); }
.sev-high     { color: var(--severity-high); }
.sev-medium   { color: var(--severity-medium); }
.sev-low      { color: var(--severity-low); }
.sev-note     { color: var(--severity-note); }

.severity-track {
  flex: 1;
  height: 6px;
  background: var(--bg-elevated);
  border-radius: 3px;
  overflow: hidden;
}

.severity-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.6s ease;
}

.fill-critical { background: var(--severity-critical); }
.fill-high     { background: var(--severity-high); }
.fill-medium   { background: var(--severity-medium); }
.fill-low      { background: var(--severity-low); }
.fill-note     { background: var(--severity-note); }

.severity-count {
  width: 32px;
  font-size: 12px;
  color: var(--text-secondary);
  text-align: right;
}

@media (max-width: 1024px) {
  .stats-grid { grid-template-columns: repeat(2, 1fr); }
}
</style>
