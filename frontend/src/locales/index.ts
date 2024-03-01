import { createI18n } from 'vue-i18n'
import en from './en'
import fa from './fa'
import ch from './ch'


export const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem("locale") ?? 'en',
  fallbackLocale: 'en',
  messages: {
    en,
    fa,
    ch
  },
})

export const languages = [
  { title: 'English', value: 'en' },
  { title: '简体中文', value: 'ch' },
  { title: 'فارسی', value: 'fa' },
]
