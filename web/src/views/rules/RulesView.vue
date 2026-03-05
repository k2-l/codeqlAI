<template>
  <div class="rules-view fade-in-up">
    <div class="page-header">
      <h1 class="page-title font-display">自定义 QL 规则</h1>
      <button class="btn-primary" @click="openCreate">
        <el-icon><Plus /></el-icon> 新建规则
      </button>
    </div>

    <!-- 语言筛选 -->
    <div class="filter-bar">
      <el-radio-group v-model="langFilter" size="small" @change="loadRules">
        <el-radio-button label="">All</el-radio-button>
        <el-radio-button v-for="l in languages" :key="l" :label="l">{{ l }}</el-radio-button>
      </el-radio-group>
      <span class="rule-count font-mono">{{ rules.length }} rules</span>
    </div>

    <!-- 规则列表 + 详情编辑 双栏 -->
    <div class="rules-layout">
      <!-- 左侧列表 -->
      <div class="rules-list">
        <div v-if="rules.length === 0" class="empty-state">
          <el-icon class="empty-icon"><Edit /></el-icon>
          <p>暂无自定义规则</p>
          <button class="btn-primary btn-sm" @click="openCreate">创建第一条规则</button>
        </div>

        <div
          v-for="rule in rules"
          :key="rule.id"
          class="rule-item"
          :class="{ active: selected?.id === rule.id, disabled: !rule.is_enabled }"
          @click="selectRule(rule)"
        >
          <div class="rule-item-top">
            <span class="rule-lang font-mono">{{ rule.language }}</span>
            <span class="rule-status" :class="rule.is_enabled ? 'status-on' : 'status-off'">
              {{ rule.is_enabled ? 'ON' : 'OFF' }}
            </span>
          </div>
          <div class="rule-name">{{ rule.name }}</div>
          <div class="rule-desc" v-if="rule.description">{{ rule.description }}</div>
          <div class="rule-meta font-mono">{{ formatTime(rule.created_at) }}</div>
        </div>
      </div>

      <!-- 右侧编辑器 -->
      <div class="rule-editor" v-if="selected || creating">
        <div class="editor-header">
          <span class="editor-title font-display">
            {{ creating ? '新建规则' : '编辑规则' }}
          </span>
          <div class="editor-actions">
            <button class="btn-save" :disabled="saving" @click="handleSave">
              <span class="spinner" v-if="saving" />
              <el-icon v-else><Check /></el-icon>
              {{ saving ? '保存中...' : '保存' }}
            </button>
            <button v-if="!creating && selected" class="btn-toggle" @click="toggleEnabled">
              {{ selected.is_enabled ? '禁用' : '启用' }}
            </button>
            <button v-if="!creating && selected" class="btn-delete" @click="handleDelete">
              <el-icon><Delete /></el-icon>
            </button>
          </div>
        </div>

        <el-form :model="form" label-position="top">
          <div class="form-row">
            <el-form-item label="规则名称">
              <el-input v-model="form.name" placeholder="e.g. Custom SQL Injection Check" />
            </el-form-item>
            <el-form-item label="语言" v-if="creating">
              <el-select v-model="form.language" style="width:100%">
                <el-option v-for="l in languages" :key="l" :label="l" :value="l" />
              </el-select>
            </el-form-item>
          </div>
          <el-form-item label="描述">
            <el-input v-model="form.description" placeholder="规则用途说明（可选）" />
          </el-form-item>
          <el-form-item label="QL 查询内容">
            <div class="ql-editor-wrapper">
              <!-- 行号 -->
              <div class="line-numbers font-mono">
                <div v-for="n in lineCount" :key="n" class="line-num">{{ n }}</div>
              </div>
              <textarea
                v-model="form.content"
                class="ql-textarea font-mono"
                spellcheck="false"
                placeholder="import java
from Method m
where m.getName() = &quot;exec&quot;
select m, &quot;Potential command injection&quot;"
                @input="updateLineCount"
              />
            </div>
          </el-form-item>
        </el-form>

        <!-- QL 语法提示 -->
        <div class="ql-tips">
          <div class="tips-title font-mono">// QL Quick Reference</div>
          <div class="tips-grid">
            <div class="tip-item" v-for="tip in qlTips" :key="tip.code">
              <code class="font-mono">{{ tip.code }}</code>
              <span>{{ tip.desc }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 空状态右侧 -->
      <div class="editor-empty" v-else>
        <div class="editor-empty-inner">
          <el-icon><EditPen /></el-icon>
          <p>选择规则进行编辑，或新建一条规则</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as api from '@/api'
import type { CustomRule } from '@/api/types'
import dayjs from 'dayjs'

const rules    = ref<CustomRule[]>([])
const selected = ref<CustomRule | null>(null)
const creating = ref(false)
const saving   = ref(false)
const langFilter = ref('')

const languages = ref<string[]>([])

// 表单默认语言在 languages 加载后再设置
const form = reactive({
  name: '', description: '', language: '', content: '',
})

const lineCount = computed(() => {
  const lines = form.content.split('\n').length
  return Math.max(lines, 10)
})

const qlTips = [
  { code: 'import java',        desc: '导入 Java 语言库' },
  { code: 'from X x',          desc: '声明变量' },
  { code: 'where condition',    desc: '筛选条件' },
  { code: 'select x, "msg"',   desc: '输出结果' },
  { code: 'x.getName()',        desc: '获取名称' },
  { code: 'x.getLocation()',    desc: '获取位置' },
]

async function loadRules() {
  const res = await api.listRules(langFilter.value || undefined)
  rules.value = res.items
}

function selectRule(rule: CustomRule) {
  creating.value = false
  selected.value = rule
  form.name        = rule.name
  form.description = rule.description
  form.language    = rule.language
  form.content     = rule.content
}

function openCreate() {
  creating.value = true
  selected.value = null
  form.name = ''; form.description = ''; form.language = languages.value[0] ?? ''; form.content = ''
}

function updateLineCount() {}

async function handleSave() {
  if (!form.name || !form.content) {
    ElMessage.warning('规则名称和 QL 内容不能为空')
    return
  }
  saving.value = true
  try {
    if (creating.value) {
      const rule = await api.createRule({ name: form.name, description: form.description, language: form.language, content: form.content })
      rules.value.unshift(rule)
      selected.value = rule
      creating.value = false
      ElMessage.success('规则已创建')
    } else if (selected.value) {
      const rule = await api.updateRule(selected.value.id, { name: form.name, description: form.description, content: form.content })
      const idx = rules.value.findIndex(r => r.id === rule.id)
      if (idx >= 0) rules.value[idx] = rule
      selected.value = rule
      ElMessage.success('规则已更新')
    }
  } finally {
    saving.value = false
  }
}

async function toggleEnabled() {
  if (!selected.value) return
  const newVal = !selected.value.is_enabled
  const rule = await api.updateRule(selected.value.id, { is_enabled: newVal })
  const idx = rules.value.findIndex(r => r.id === rule.id)
  if (idx >= 0) rules.value[idx] = rule
  selected.value = rule
  ElMessage.success(newVal ? '规则已启用' : '规则已禁用')
}

async function handleDelete() {
  if (!selected.value) return
  await ElMessageBox.confirm('确认删除此规则？', '删除规则', { type: 'warning' })
  await api.deleteRule(selected.value.id)
  rules.value = rules.value.filter(r => r.id !== selected.value!.id)
  selected.value = null
  ElMessage.success('规则已删除')
}

const formatTime = (s: string) => dayjs(s).format('YYYY-MM-DD HH:mm')

onMounted(async () => {
  const [langs] = await Promise.all([
    api.listLanguages(),
    loadRules(),
  ])
  languages.value = langs
  if (langs.length > 0 && !form.language) form.language = langs[0]
})
</script>

<style scoped>
.rules-view { max-width: 1300px; }

.page-header {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 20px;
}

.page-title { font-size: 22px; font-weight: 700; color: var(--text-primary); }

.btn-primary {
  display: flex; align-items: center; gap: 6px;
  padding: 8px 16px;
  background: var(--accent-primary); border: none;
  border-radius: var(--radius-md);
  color: var(--text-inverse); font-size: 13px; font-weight: 600;
  font-family: var(--font-body); cursor: pointer;
  box-shadow: 0 0 16px rgba(14,165,233,0.3);
  transition: all var(--transition-fast);
}
.btn-primary:hover { background: #38bdf8; }
.btn-sm { padding: 6px 12px; font-size: 12px; }

.filter-bar {
  display: flex; align-items: center; gap: 16px;
  margin-bottom: 16px;
}

.rule-count { font-size: 11px; color: var(--text-muted); }

/* 双栏布局 */
.rules-layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  gap: 16px;
  height: calc(100vh - 200px);
}

/* 左侧列表 */
.rules-list {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.rule-item {
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.rule-item:hover { background: var(--bg-hover); border-color: var(--border-subtle); }
.rule-item.active { background: var(--bg-elevated); border-color: var(--accent-primary); }
.rule-item.disabled { opacity: 0.5; }

.rule-item-top {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 4px;
}

.rule-lang {
  font-size: 10px; font-weight: 700;
  color: var(--accent-primary);
  background: var(--accent-glow);
  padding: 1px 6px; border-radius: var(--radius-sm);
  border: 1px solid rgba(14,165,233,0.2);
}

.rule-status {
  font-size: 9px; font-family: var(--font-mono); font-weight: 700;
  padding: 1px 5px; border-radius: 3px;
}
.status-on  { background: rgba(34,197,94,0.1); color: var(--status-completed); }
.status-off { background: rgba(107,114,128,0.1); color: var(--text-muted); }

.rule-name { font-size: 13px; font-weight: 500; color: var(--text-primary); margin-bottom: 3px; }
.rule-desc { font-size: 11px; color: var(--text-muted); margin-bottom: 4px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rule-meta { font-size: 10px; color: var(--text-muted); }

/* 右侧编辑器 */
.rule-editor {
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.editor-header {
  display: flex; align-items: center; justify-content: space-between;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--border-subtle);
}

.editor-title { font-size: 15px; font-weight: 700; color: var(--text-primary); }

.editor-actions { display: flex; gap: 8px; }

.btn-save, .btn-toggle, .btn-delete {
  display: flex; align-items: center; gap: 5px;
  padding: 6px 14px; border-radius: var(--radius-md);
  font-size: 12px; font-weight: 600; font-family: var(--font-body);
  cursor: pointer; transition: all var(--transition-fast);
  border: 1px solid transparent;
}

.btn-save {
  background: var(--accent-primary); color: var(--text-inverse);
  box-shadow: 0 0 12px rgba(14,165,233,0.25);
}
.btn-save:hover:not(:disabled) { background: #38bdf8; }
.btn-save:disabled { opacity: 0.6; cursor: not-allowed; }

.btn-toggle {
  background: var(--bg-elevated); color: var(--text-secondary);
  border-color: var(--border-default);
}
.btn-toggle:hover { border-color: var(--accent-primary); color: var(--accent-primary); }

.btn-delete {
  background: rgba(244,63,94,0.08); color: var(--severity-critical);
  border-color: rgba(244,63,94,0.2);
}
.btn-delete:hover { background: rgba(244,63,94,0.15); }

.form-row { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }

/* QL 编辑器 */
.ql-editor-wrapper {
  display: flex;
  background: var(--bg-base);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  overflow: hidden;
  min-height: 240px;
  width: 100%;
}

.line-numbers {
  padding: 12px 8px;
  background: var(--bg-elevated);
  border-right: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  min-width: 40px;
  user-select: none;
}

.line-num {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.7;
  text-align: right;
  padding-right: 4px;
}

.ql-textarea {
  flex: 1;
  padding: 12px;
  background: transparent;
  border: none;
  outline: none;
  color: #7dd3fc;
  font-size: 12.5px;
  line-height: 1.7;
  resize: none;
  white-space: pre;
  overflow-x: auto;
  min-height: 240px;
}

.ql-textarea::placeholder { color: var(--text-muted); }

/* QL 提示 */
.ql-tips {
  background: var(--bg-elevated);
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  padding: 12px 14px;
}

.tips-title {
  font-size: 11px;
  color: var(--text-muted);
  margin-bottom: 10px;
}

.tips-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.tip-item {
  display: flex; flex-direction: column; gap: 2px;
}

.tip-item code {
  font-size: 11px; color: var(--accent-primary);
  background: var(--accent-glow);
  padding: 2px 6px; border-radius: var(--radius-sm);
}

.tip-item span {
  font-size: 11px; color: var(--text-muted);
}

/* 空状态 */
.empty-state {
  display: flex; flex-direction: column; align-items: center;
  justify-content: center; gap: 12px; padding: 40px;
  text-align: center; color: var(--text-muted);
}
.empty-icon { font-size: 36px; }

.editor-empty {
  background: var(--bg-card);
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-lg);
  display: flex; align-items: center; justify-content: center;
}

.editor-empty-inner {
  text-align: center; color: var(--text-muted);
  display: flex; flex-direction: column; align-items: center; gap: 10px;
}
.editor-empty-inner .el-icon { font-size: 32px; }

.spinner {
  width: 12px; height: 12px;
  border: 2px solid rgba(0,0,0,0.15);
  border-top-color: var(--text-inverse);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
