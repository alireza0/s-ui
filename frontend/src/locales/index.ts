import { createI18n } from 'vue-i18n'
import en from './en'
import fa from './fa'
import zhcn from './zhcn'
import zhcn from './vi'


export const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") ?? 'en',
  fallbackLocale: 'en',
  messages: {
    en,
    fa,
    zhcn,
    vi
  },
})

export const languages = [
  { title: 'English', value: 'en' },
  { title: 'فارسی', value: 'fa' },
  { title: '简体中文', value: 'zhcn' },
  { title: 'Tiếng Việt', value: 'vi' },
]
