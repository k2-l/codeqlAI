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

// 响应拦截器：统一错误处理
http.interceptors.response.use(
  res => res,
  err => {
    const msg = err.response?.data?.error || err.message || 'Network Error'
    ElMessage.error(msg)
    return Promise.reject(err)
  }
)

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

// ===== Health =====
export const checkHealth = () =>
  axios.get<{ status: string }>('/health').then(r => r.data)
