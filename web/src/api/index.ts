import axios from 'axios'
import { ElMessage } from 'element-plus'
import type {
  Task, Finding, FindingsResponse,
  SubmitScanRequest, SubmitScanResponse, AiResult
} from './types'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

// 请求拦截器：自动携带 JWT Token
http.interceptors.request.use(config => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：401 清 token 跳登录，其他错误统一提示
http.interceptors.response.use(
  res => res,
  err => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('username')
      // 跳转登录页（避免循环依赖不直接 import router）
      if (!window.location.pathname.includes('/login')) {
        window.location.href = '/login'
      }
      return Promise.reject(err)
    }
    const msg = err.response?.data?.error || err.message || 'Network Error'
    ElMessage.error(msg)
    return Promise.reject(err)
  }
)

// ===== Auth =====
export const getCaptcha = () =>
  http.get<{ captcha_id: string; captcha_code: string }>('/auth/captcha').then(r => r.data)

export const login = (data: { username: string; password: string; captcha_id: string; captcha_code: string }) =>
  http.post<{ token: string; expires_at: number; username: string }>('/auth/login', data).then(r => r.data)

export const logout = () =>
  http.post('/auth/logout').then(r => r.data)

// ===== Scan =====
export const submitScan = (data: SubmitScanRequest) =>
  http.post<SubmitScanResponse>('/scan', data).then(r => r.data)

// ===== Task =====
export const getTask = (id: string) =>
  http.get<Task>(`/task/${id}`).then(r => r.data)

export const getTaskByName = (name: string) =>
  http.get<Task>(`/task/name/${encodeURIComponent(name)}`).then(r => r.data)

export const deleteTask = (id: string) =>
  http.delete(`/task/${id}`).then(r => r.data)

// ===== Findings =====
export const getFindings = (taskId: string) =>
  http.get<FindingsResponse>(`/task/${taskId}/results`).then(r => r.data)

// ===== AI Audit =====
export const triggerAudit = (findingId: string) =>
  http.post<{ message: string; finding_id: string }>(`/finding/${findingId}/audit`).then(r => r.data)

// ===== Settings =====
export const getAISettings = () =>
  http.get<any>('/settings/ai').then(r => r.data)

export const updateAISettings = (data: any) =>
  http.put<{ message: string }>('/settings/ai', data).then(r => r.data)

// ===== Custom Rules =====
export const listRules = (language?: string) =>
  http.get<{ total: number; items: any[] }>('/rules', { params: language ? { language } : {} }).then(r => r.data)

export const getRule = (id: string) =>
  http.get<any>(`/rules/${id}`).then(r => r.data)

export const createRule = (data: any) =>
  http.post<any>('/rules', data).then(r => r.data)

export const updateRule = (id: string, data: any) =>
  http.put<any>(`/rules/${id}`, data).then(r => r.data)

export const deleteRule = (id: string) =>
  http.delete(`/rules/${id}`).then(r => r.data)

// ===== VulnMap =====
export const getVulnMap = (taskId: string) =>
  http.get<{ task_id: string; total: number; items: any[] }>(`/task/${taskId}/vulnmap`).then(r => r.data)

export const listTasks = (status?: string) =>
  http.get<{ total: number; items: any[] }>('/tasks', { params: status ? { status } : {} }).then(r => r.data)

export const listLanguages = () =>
  http.get<{ items: string[] }>('/languages').then(r => r.data.items)

// ===== Health =====
export const checkHealth = () =>
  axios.get<{ status: string }>('/health').then(r => r.data)

