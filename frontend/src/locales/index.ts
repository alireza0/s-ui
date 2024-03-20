import { createI18n } from 'vue-i18n'
import en from './en'
import fa from './fa'
import zhcn from './zhcn'
import zhtw from './zhtw'

export const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") ?? 'en',
  fallbackLocale: 'en',
  messages: {
    en,
    fa,
    zhcn,
    zhtw
  },
})

export const languages = [
  { title: 'English', value: 'en' },
  { title: 'فارسی', value: 'fa' },
  { title: '简体中文', value: 'zhcn' },
  { title: '繁體中文', value: 'zhtw' },
]
