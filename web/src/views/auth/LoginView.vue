<template>
  <div class="login-page">
    <!-- 背景网格 -->
    <div class="bg-grid" />
    <!-- 垂直扫描线 -->
    <div class="bg-scan-line" />
    <!-- 角落装饰 -->
    <div class="corner corner-tl" />
    <div class="corner corner-br" />

    <!-- 登录卡片 -->
    <div class="login-card" :class="{ shake: shaking }">
      <!-- Logo -->
      <div class="card-logo">
        <div class="logo-diamond">
          <svg width="36" height="36" viewBox="0 0 36 36" fill="none">
            <path d="M5 18 L18 5 L31 18 L18 31 Z" stroke="var(--accent-primary)" stroke-width="1.5" fill="none"/>
            <path d="M11 18 L18 11 L25 18 L18 25 Z" fill="var(--accent-primary)" opacity="0.5"/>
            <circle cx="18" cy="18" r="3" fill="var(--accent-primary)"/>
          </svg>
        </div>
        <div class="logo-text">
          <span class="logo-main">CodeQL</span>
          <span class="logo-tag font-mono">AI Scanner</span>
        </div>
      </div>

      <div class="card-title font-display">Security Console</div>
      <div class="card-sub font-mono">// authenticate to continue</div>

      <!-- 表单 -->
      <form class="login-form" @submit.prevent="handleLogin">
        <!-- 用户名 -->
        <div class="field-group" :class="{ focused: focus.username, 'has-error': errors.username }">
          <label class="field-label font-mono">USER</label>
          <div class="field-input-wrap">
            <el-icon class="field-icon"><User /></el-icon>
            <input
              v-model="form.username"
              type="text"
              class="field-input font-mono"
              placeholder="username"
              autocomplete="username"
              @focus="focus.username = true"
              @blur="focus.username = false; validateField('username')"
            />
          </div>
          <div class="field-error" v-if="errors.username">{{ errors.username }}</div>
        </div>

        <!-- 密码 -->
        <div class="field-group" :class="{ focused: focus.password, 'has-error': errors.password }">
          <label class="field-label font-mono">PASS</label>
          <div class="field-input-wrap">
            <el-icon class="field-icon"><Lock /></el-icon>
            <input
              v-model="form.password"
              :type="showPass ? 'text' : 'password'"
              class="field-input font-mono"
              placeholder="password"
              autocomplete="current-password"
              @focus="focus.password = true"
              @blur="focus.password = false; validateField('password')"
            />
            <button type="button" class="pass-toggle" @click="showPass = !showPass" tabindex="-1">
              <el-icon><component :is="showPass ? 'Hide' : 'View'" /></el-icon>
            </button>
          </div>
          <div class="field-error" v-if="errors.password">{{ errors.password }}</div>
        </div>

        <!-- 验证码 -->
        <div class="field-group" :class="{ focused: focus.captcha, 'has-error': errors.captcha }">
          <label class="field-label font-mono">CODE</label>
          <div class="captcha-row">
            <div class="field-input-wrap" style="flex:1">
              <el-icon class="field-icon"><Key /></el-icon>
              <input
                v-model="form.captchaCode"
                type="text"
                class="field-input font-mono"
                placeholder="0000"
                maxlength="4"
                autocomplete="off"
                @focus="focus.captcha = true"
                @blur="focus.captcha = false; validateField('captcha')"
              />
            </div>
            <!-- 验证码展示框，点击刷新 -->
            <div class="captcha-box" @click="refreshCaptcha" title="点击刷新验证码">
              <div v-if="captchaLoading" class="captcha-loading">
                <span class="captcha-spin" />
              </div>
              <div v-else class="captcha-digits">
                <span
                  v-for="(digit, i) in captchaCode.split('')"
                  :key="`${captchaKey}-${i}`"
                  class="captcha-digit font-mono"
                  :style="{ animationDelay: `${i * 70}ms`, transform: `rotate(${rotations[i]}deg)` }"
                >{{ digit }}</span>
              </div>
              <span class="refresh-hint">↻</span>
            </div>
          </div>
          <div class="field-error" v-if="errors.captcha">{{ errors.captcha }}</div>
        </div>

        <!-- 登录错误 -->
        <transition name="err-slide">
          <div class="login-error font-mono" v-if="loginError">
            <el-icon><CircleClose /></el-icon>
            {{ loginError }}
          </div>
        </transition>

        <!-- 提交 -->
        <button type="submit" class="submit-btn" :disabled="submitting">
          <span class="btn-inner">
            <span v-if="submitting" class="btn-spin" />
            <el-icon v-else><Unlock /></el-icon>
            {{ submitting ? 'AUTHENTICATING...' : 'ACCESS SYSTEM' }}
          </span>
        </button>
      </form>

      <div class="card-footer font-mono">
        <span class="dot" /><span>Single-user system</span><span class="dot" />
      </div>
    </div>

    <div class="version font-mono">CodeQL AI Scanner · v1.0</div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import * as api from '@/api'

const router = useRouter()

const form = reactive({ username: '', password: '', captchaCode: '' })
const focus = reactive({ username: false, password: false, captcha: false })
const errors = reactive({ username: '', password: '', captcha: '' })

const showPass      = ref(false)
const submitting    = ref(false)
const shaking       = ref(false)
const loginError    = ref('')
const captchaId     = ref('')
const captchaCode   = ref('----')
const captchaKey    = ref(0)          // 驱动 digit 动画重放
const captchaLoading = ref(false)
const rotations     = ref([0, 0, 0, 0])

async function refreshCaptcha() {
  captchaLoading.value = true
  form.captchaCode = ''
  errors.captcha = ''
  try {
    const res = await api.getCaptcha()
    captchaId.value   = res.captcha_id
    captchaCode.value = res.captcha_code
    captchaKey.value++
    // 每次刷新随机生成小旋转角度（-6° ~ +6°），让数字更有手写感
    rotations.value = Array.from({ length: 4 }, () => Math.floor(Math.random() * 12) - 6)
  } catch {
    captchaCode.value = 'ERR!'
  } finally {
    captchaLoading.value = false
  }
}

function validateField(field: 'username' | 'password' | 'captcha') {
  if (field === 'username') errors.username = form.username.trim() ? '' : '请输入用户名'
  if (field === 'password') errors.password = form.password ? '' : '请输入密码'
  if (field === 'captcha')  errors.captcha  = form.captchaCode.length === 4 ? '' : '请输入4位验证码'
}

function doShake() {
  shaking.value = true
  setTimeout(() => { shaking.value = false }, 480)
}

async function handleLogin() {
  validateField('username')
  validateField('password')
  validateField('captcha')
  if (errors.username || errors.password || errors.captcha) { doShake(); return }

  submitting.value = true
  loginError.value = ''
  try {
    const res = await api.login({
      username:     form.username,
      password:     form.password,
      captcha_id:   captchaId.value,
      captcha_code: form.captchaCode,
    })
    localStorage.setItem('token',    res.token)
    localStorage.setItem('username', res.username)
    router.push('/')
  } catch (err: any) {
    loginError.value = err?.response?.data?.error || '登录失败，请重试'
    doShake()
    await refreshCaptcha()
  } finally {
    submitting.value = false
  }
}

onMounted(refreshCaptcha)
</script>

<style scoped>
/* ===== 页面 ===== */
.login-page {
  min-height: 100vh;
  background: var(--bg-base);
  display: flex; align-items: center; justify-content: center;
  position: relative; overflow: hidden;
}

.bg-grid {
  position: absolute; inset: 0;
  background-image:
    linear-gradient(rgba(14,165,233,0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(14,165,233,0.04) 1px, transparent 1px);
  background-size: 48px 48px;
}

.bg-scan-line {
  position: absolute; left: 0; right: 0; height: 2px;
  background: linear-gradient(90deg, transparent, rgba(14,165,233,0.35), transparent);
  animation: vscan 7s linear infinite;
}
@keyframes vscan {
  0%   { top: -2px; opacity: 0; }
  5%   { opacity: 1; }
  95%  { opacity: 1; }
  100% { top: 100vh; opacity: 0; }
}

.corner { position: absolute; width: 72px; height: 72px; }
.corner-tl { top: 20px; left: 20px; border-top: 1px solid rgba(14,165,233,0.25); border-left: 1px solid rgba(14,165,233,0.25); }
.corner-br { bottom: 20px; right: 20px; border-bottom: 1px solid rgba(14,165,233,0.25); border-right: 1px solid rgba(14,165,233,0.25); }

/* ===== 卡片 ===== */
.login-card {
  width: 400px;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  padding: 36px 32px 28px;
  position: relative; z-index: 1;
  box-shadow: 0 0 0 1px rgba(14,165,233,0.06), 0 24px 64px rgba(0,0,0,0.5), 0 0 80px rgba(14,165,233,0.05);
}

/* 顶部光线 */
.login-card::before {
  content: '';
  position: absolute; top: 0; left: 12%; right: 12%; height: 1px;
  background: linear-gradient(90deg, transparent, var(--accent-primary), transparent);
}

.shake { animation: shake 0.44s ease; }
@keyframes shake {
  0%,100%{ transform: translateX(0); }
  20%    { transform: translateX(-8px); }
  40%    { transform: translateX( 8px); }
  60%    { transform: translateX(-5px); }
  80%    { transform: translateX( 5px); }
}

/* Logo */
.card-logo { display: flex; align-items: center; gap: 12px; margin-bottom: 22px; }
.logo-diamond {
  width: 46px; height: 46px;
  display: flex; align-items: center; justify-content: center;
  background: rgba(14,165,233,0.06); border: 1px solid rgba(14,165,233,0.18);
  border-radius: 10px;
}
.logo-text { display: flex; flex-direction: column; }
.logo-main { font-family: var(--font-display); font-size: 18px; font-weight: 800; color: var(--text-primary); line-height: 1; }
.logo-tag  { font-size: 10px; color: var(--accent-primary); letter-spacing: 1px; margin-top: 3px; }

.card-title { font-size: 22px; font-weight: 700; color: var(--text-primary); margin-bottom: 4px; }
.card-sub   { font-size: 11px; color: var(--text-muted); margin-bottom: 28px; letter-spacing: 0.5px; }

/* ===== 表单 ===== */
.login-form { display: flex; flex-direction: column; gap: 16px; }

.field-group { display: flex; flex-direction: column; gap: 5px; }

.field-label {
  font-size: 10px; font-weight: 700; color: var(--text-muted);
  letter-spacing: 2px; transition: color var(--transition-fast);
}
.field-group.focused  .field-label { color: var(--accent-primary); }
.field-group.has-error .field-label { color: var(--severity-critical); }

.field-input-wrap {
  position: relative; display: flex; align-items: center;
  background: var(--bg-elevated); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); transition: all var(--transition-fast);
}
.field-group.focused   .field-input-wrap { border-color: var(--accent-primary); box-shadow: 0 0 0 2px rgba(14,165,233,0.12); }
.field-group.has-error .field-input-wrap { border-color: var(--severity-critical); box-shadow: 0 0 0 2px rgba(244,63,94,0.1); }

.field-icon {
  position: absolute; left: 12px; font-size: 15px;
  color: var(--text-muted); pointer-events: none; transition: color var(--transition-fast);
}
.field-group.focused .field-icon { color: var(--accent-primary); }

.field-input {
  width: 100%; padding: 11px 12px 11px 38px;
  background: transparent; border: none; outline: none;
  color: var(--text-primary); font-size: 13px; letter-spacing: 0.5px;
}
.field-input::placeholder { color: var(--text-muted); }

.pass-toggle {
  position: absolute; right: 10px; background: none; border: none;
  color: var(--text-muted); cursor: pointer; padding: 4px;
  display: flex; align-items: center; transition: color var(--transition-fast);
}
.pass-toggle:hover { color: var(--accent-primary); }

.field-error { font-size: 11px; color: var(--severity-critical); font-family: var(--font-mono); padding-left: 2px; }

/* ===== 验证码 ===== */
.captcha-row { display: flex; gap: 10px; }

.captcha-box {
  width: 116px; min-width: 116px; height: 42px;
  background: var(--bg-base); border: 1px solid var(--border-default);
  border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center;
  cursor: pointer; position: relative; transition: border-color var(--transition-fast);
  user-select: none;
}
.captcha-box:hover { border-color: var(--accent-primary); }
.captcha-box:hover .refresh-hint { opacity: 1; }

.refresh-hint {
  position: absolute; top: 2px; right: 5px;
  font-size: 11px; color: var(--text-muted); opacity: 0;
  transition: opacity var(--transition-fast);
}

.captcha-loading { display: flex; align-items: center; justify-content: center; }
.captcha-spin {
  width: 16px; height: 16px;
  border: 2px solid var(--border-default); border-top-color: var(--accent-primary);
  border-radius: 50%; animation: spin 0.8s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

.captcha-digits { display: flex; align-items: center; gap: 5px; padding: 0 8px; }

.captcha-digit {
  font-size: 21px; font-weight: 700; color: var(--accent-primary);
  line-height: 1; display: inline-block;
  text-shadow: 0 0 10px rgba(14,165,233,0.4);
  animation: digit-pop 0.3s ease backwards;
}
@keyframes digit-pop {
  from { opacity: 0; transform: translateY(-6px) scale(0.7); }
  to   { opacity: 1; transform: translateY(0)    scale(1); }
}

/* ===== 错误提示 ===== */
.login-error {
  display: flex; align-items: center; gap: 8px;
  padding: 10px 12px;
  background: rgba(244,63,94,0.08); border: 1px solid rgba(244,63,94,0.25);
  border-radius: var(--radius-md); color: var(--severity-critical); font-size: 12px;
}
.err-slide-enter-active, .err-slide-leave-active { transition: all 0.22s ease; }
.err-slide-enter-from, .err-slide-leave-to { opacity: 0; transform: translateY(-6px); }

/* ===== 提交按钮 ===== */
.submit-btn {
  width: 100%; padding: 13px; margin-top: 4px;
  background: var(--accent-primary); border: none;
  border-radius: var(--radius-md);
  color: var(--bg-base); font-family: var(--font-display);
  font-size: 13px; font-weight: 700; letter-spacing: 2px;
  cursor: pointer; position: relative; overflow: hidden;
  transition: all var(--transition-normal);
  box-shadow: 0 0 24px rgba(14,165,233,0.35);
}
.submit-btn::before {
  content: ''; position: absolute; inset: 0;
  background: linear-gradient(135deg, rgba(255,255,255,0.12), transparent 60%);
}
.submit-btn:hover:not(:disabled) {
  background: #38bdf8;
  box-shadow: 0 0 40px rgba(14,165,233,0.55);
  transform: translateY(-1px);
}
.submit-btn:disabled { opacity: 0.7; cursor: not-allowed; transform: none; }

.btn-inner {
  position: relative; display: flex; align-items: center; justify-content: center; gap: 8px;
}
.btn-spin {
  width: 14px; height: 14px;
  border: 2px solid rgba(0,0,0,0.2); border-top-color: var(--bg-base);
  border-radius: 50%; animation: spin 0.8s linear infinite;
}

/* ===== 底部 ===== */
.card-footer {
  display: flex; align-items: center; justify-content: center; gap: 8px;
  margin-top: 22px; font-size: 11px; color: var(--text-muted); letter-spacing: 0.5px;
}
.dot { width: 3px; height: 3px; border-radius: 50%; background: var(--border-default); }

.version {
  position: fixed; bottom: 14px; right: 18px;
  font-size: 10px; color: var(--text-muted); letter-spacing: 0.5px;
}
</style>
