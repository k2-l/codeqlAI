import { createI18n } from 'vue-i18n'
import { messages } from './messages'

export const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem('locale') || 'zh',
  fallbackLocale: 'zh',
  messages,
})
