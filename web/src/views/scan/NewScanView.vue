<template>
  <div class="new-scan fade-in-up">
    <div class="page-header">
      <h1 class="page-title font-display">{{ t('newScan.title') }}</h1>
    </div>

    <div class="form-layout">
      <!-- 表单区 -->
      <div class="form-card">
        <el-form :model="form" :rules="rules" ref="formRef" label-position="top">

          <!-- 项目名 / 任务名 -->
          <div class="form-row">
            <el-form-item :label="t('newScan.projectName')" prop="project_name">
              <el-input v-model="form.project_name" placeholder="my-project" />
            </el-form-item>
            <el-form-item :label="t('newScan.taskName')">
              <el-input v-model="form.task_name" :placeholder="t('newScan.taskNameTip')" />
            </el-form-item>
          </div>

          <!-- 语言选择 -->
          <el-form-item :label="t('newScan.language')" prop="language">
            <div class="lang-selector">
              <button
                v-for="lang in languages"
                :key="lang.value"
                type="button"
                class="lang-option"
                :class="{ active: form.language === lang.value }"
                @click="form.language = lang.value"
              >
                <span class="lang-icon">{{ lang.icon }}</span>
                <span>{{ lang.label }}</span>
              </button>
            </div>
          </el-form-item>

          <!-- 来源类型切换 -->
          <el-form-item :label="t('newScan.sourceType')">
            <div class="source-toggle">
              <button
                type="button"
                class="toggle-btn"
                :class="{ active: sourceType === 'local' }"
                @click="sourceType = 'local'"
              >
                <el-icon><FolderOpened /></el-icon>
                {{ t('newScan.sourceLocal') }}
              </button>
              <button
                type="button"
                class="toggle-btn"
                :class="{ active: sourceType === 'git' }"
                @click="sourceType = 'git'"
              >
                <el-icon><Connection /></el-icon>
                {{ t('newScan.sourceGit') }}
              </button>
            </div>
          </el-form-item>

          <!-- 本地路径 -->
          <el-form-item
            v-if="sourceType === 'local'"
            :label="t('newScan.localPath')"
            prop="source_path"
          >
            <el-input v-model="form.source_path" placeholder="/path/to/project" prefix-icon="FolderOpened" />
          </el-form-item>

          <!-- Git 配置 -->
          <template v-if="sourceType === 'git'">
            <el-form-item :label="t('newScan.gitUrl')" prop="git_url">
              <el-input v-model="form.git_url" placeholder="https://github.com/user/repo.git" prefix-icon="Link" />
            </el-form-item>

            <div class="form-row">
              <el-form-item :label="t('newScan.gitBranch')">
                <el-input v-model="form.git_branch" :placeholder="t('newScan.gitBranchTip')" prefix-icon="Connection" />
              </el-form-item>
              <el-form-item :label="t('newScan.gitToken')">
                <el-input
                  v-model="form.git_token"
                  type="password"
                  show-password
                  :placeholder="t('newScan.gitTokenTip')"
                  prefix-icon="Key"
                />
              </el-form-item>
            </div>

            <el-form-item :label="t('newScan.gitSshKey')">
              <el-input v-model="form.git_ssh_key" placeholder="/root/.ssh/id_rsa" prefix-icon="Lock" />
            </el-form-item>
          </template>

          <!-- 自定义规则选择 -->
          <el-form-item label="扫描规则">
            <div class="rule-selector">
              <button
                type="button"
                class="toggle-btn"
                :class="{ active: !form.custom_rule_id }"
                @click="form.custom_rule_id = ''"
              >
                <el-icon><Star /></el-icon>
                官方规则套件
              </button>
              <button
                type="button"
                class="toggle-btn"
                :class="{ active: !!form.custom_rule_id || showRuleList }"
                @click="showRuleList = !showRuleList; if(!showRuleList) form.custom_rule_id = ''"
              >
                <el-icon><Edit /></el-icon>
                自定义规则
              </button>
            </div>
            <transition name="slide-down">
              <el-select
                v-if="showRuleList"
                v-model="form.custom_rule_id"
                placeholder="选择已保存的自定义规则"
                style="width:100%; margin-top: 8px"
                clearable
              >
                <el-option
                  v-for="rule in availableRules"
                  :key="rule.id"
                  :value="rule.id"
                  :label="rule.name"
                >
                  <div style="display:flex;align-items:center;gap:8px">
                    <span style="font-size:10px;color:var(--accent-primary);font-family:var(--font-mono)">{{ rule.language }}</span>
                    <span>{{ rule.name }}</span>
                  </div>
                </el-option>
              </el-select>
            </transition>
          </el-form-item>

          <!-- 提交按钮 -->
          <el-form-item style="margin-top: 8px;">
            <button
              type="button"
              class="submit-btn"
              :class="{ loading: submitting }"
              :disabled="submitting"
              @click="handleSubmit"
            >
              <span v-if="!submitting" class="submit-inner">
                <el-icon><VideoPlay /></el-icon>
                {{ t('newScan.submit') }}
              </span>
              <span v-else class="submit-inner">
                <span class="submit-spinner" />
                {{ t('newScan.submitting') }}
              </span>
            </button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 右侧说明面板 -->
      <div class="info-panel">
        <div class="info-card">
          <div class="info-title font-display">
            <el-icon><InfoFilled /></el-icon>
            Quick Guide
          </div>
          <div class="info-steps">
            <div class="info-step" v-for="(step, i) in guideSteps" :key="i">
              <div class="step-num font-mono">{{ String(i + 1).padStart(2, '0') }}</div>
              <div class="step-text">{{ step }}</div>
            </div>
          </div>
        </div>

        <div class="info-card supported-langs">
          <div class="info-title font-display">
            <el-icon><Code /></el-icon>
            Supported Languages
          </div>
          <div class="lang-list">
            <span
              v-for="lang in languages"
              :key="lang.value"
              class="lang-chip font-mono"
              :class="{ 'chip-active': form.language === lang.value }"
            >
              {{ lang.icon }} {{ lang.value }}
            </span>
          </div>
        </div>

        <!-- SSRF 提示 -->
        <div class="info-card ssrf-notice">
          <div class="info-title font-display">
            <el-icon><Shield /></el-icon>
            Security Notice
          </div>
          <p class="notice-text">
            Private IP ranges and cloud metadata endpoints (169.254.x.x) are blocked to prevent SSRF attacks.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, type FormInstance } from 'element-plus'
import { useTaskStore } from '@/stores'
import * as api from '@/api'
import type { Language, CustomRule } from '@/api/types'

const { t } = useI18n()
const router = useRouter()
const taskStore = useTaskStore()

const formRef = ref<FormInstance>()
const submitting = ref(false)
const sourceType = ref<'local' | 'git'>('git')
const showRuleList = ref(false)
const availableRules = ref<CustomRule[]>([])

const form = reactive({
  project_name:   '',
  task_name:      '',
  language:       '' as Language,
  source_path:    '',
  git_url:        '',
  git_branch:     '',
  git_token:      '',
  git_ssh_key:    '',
  custom_rule_id: '',
})

// 从后端动态加载，config.yaml 里加一行即可支持新语言
const languages = ref<{ value: string; label: string }[]>([])

const rules = {
  project_name: [{ required: true, message: 'Project name is required', trigger: 'blur' }],
  language:     [{ required: true }],
  source_path:  [{ required: sourceType.value === 'local', message: 'Path is required' }],
  git_url:      [{ required: sourceType.value === 'git', message: 'Git URL is required' }],
}

const guideSteps = [
  'Fill in the project name and optional task name',
  'Select the programming language to analyze',
  'Choose source: local path or Git repository',
  'Submit and monitor progress in Task List',
  'View findings and trigger AI audit per finding',
]

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const req: any = {
      project_name: form.project_name,
      task_name:    form.task_name || undefined,
      language:     form.language,
    }
    if (sourceType.value === 'local') {
      req.source_path = form.source_path
    } else {
      req.git_url    = form.git_url
      req.git_branch = form.git_branch || undefined
      req.git_token  = form.git_token  || undefined
      req.git_ssh_key = form.git_ssh_key || undefined
    }
    if (form.custom_rule_id) req.custom_rule_id = form.custom_rule_id

    const res = await api.submitScan(req)
    ElMessage.success(`${t('newScan.success')}: ${res.display_name}`)

    // 立即加载新任务到 store
    const task = await api.getTask(res.task_id)
    taskStore.addTask(task)

    // 跳转到任务列表
    router.push('/tasks')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  // 并行加载语言列表和自定义规则
  const [langs, rulesRes] = await Promise.all([
    api.listLanguages(),
    api.listRules(),
  ])
  languages.value = langs.map(l => ({ value: l, label: l.charAt(0).toUpperCase() + l.slice(1) }))
  if (langs.length > 0 && !form.language) form.language = langs[0]
  availableRules.value = rulesRes.items.filter((r: any) => r.is_enabled)
})
</script>

<style scoped>
.new-scan { max-width: 1100px; }

.page-header {
  margin-bottom: 24px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.form-layout {
  display: grid;
  grid-template-columns: 1fr 280px;
  gap: 20px;
  align-items: start;
}

/* 表单卡片 */
.form-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

/* 语言选择器 */
.lang-selector {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.lang-option {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: 13px;
  font-family: var(--font-body);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.lang-option:hover {
  border-color: var(--accent-primary);
  color: var(--text-primary);
}

.lang-option.active {
  border-color: var(--accent-primary);
  background: var(--accent-glow);
  color: var(--accent-primary);
  font-weight: 600;
  box-shadow: 0 0 10px rgba(14,165,233,0.2);
}

.lang-icon { font-size: 15px; }

/* 来源切换 */
.source-toggle {
  display: flex;
  gap: 8px;
}

.toggle-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 16px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  font-size: 13px;
  font-family: var(--font-body);
  cursor: pointer;
  transition: all var(--transition-fast);
}

.toggle-btn.active {
  border-color: var(--accent-primary);
  background: var(--accent-glow);
  color: var(--accent-primary);
  font-weight: 600;
}

/* 提交按钮 */
.submit-btn {
  width: 100%;
  padding: 12px;
  background: var(--accent-primary);
  border: none;
  border-radius: var(--radius-md);
  color: var(--text-inverse);
  font-size: 14px;
  font-weight: 700;
  font-family: var(--font-display);
  cursor: pointer;
  transition: all var(--transition-normal);
  box-shadow: 0 0 24px rgba(14,165,233,0.35);
  position: relative;
  overflow: hidden;
}

.submit-btn::before {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.1), transparent);
}

.submit-btn:hover:not(:disabled) {
  background: #38bdf8;
  box-shadow: 0 0 36px rgba(14,165,233,0.55);
  transform: translateY(-1px);
}

.submit-btn:disabled { opacity: 0.7; cursor: not-allowed; }

.submit-inner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.submit-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(0,0,0,0.2);
  border-top-color: var(--text-inverse);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

/* 右侧信息面板 */
.info-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.info-card {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 16px;
}

.info-title {
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

.info-steps {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.info-step {
  display: flex;
  gap: 10px;
  align-items: flex-start;
}

.step-num {
  font-size: 10px;
  color: var(--accent-primary);
  font-weight: 700;
  flex-shrink: 0;
  margin-top: 1px;
}

.step-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.lang-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.lang-chip {
  font-size: 11px;
  padding: 3px 8px;
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-sm);
  color: var(--text-muted);
}

.lang-chip.chip-active {
  border-color: var(--accent-primary);
  color: var(--accent-primary);
  background: var(--accent-glow);
}

.ssrf-notice .info-title { color: var(--severity-medium); }
.ssrf-notice { border-color: rgba(234,179,8,0.2); }

.notice-text {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.6;
}

@media (max-width: 900px) {
  .form-layout { grid-template-columns: 1fr; }
  .info-panel { display: none; }
}
</style>
