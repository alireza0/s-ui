/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Composables
import { createApp, ref } from 'vue'

// Components
import App from './App.vue'

// Use router
import router from './router'

// Store
import store from './store'

// Plugins
import { registerPlugins } from '@/plugins'

// Locale
import { i18n } from '@/locales'
import Vue3PersianDatetimePicker from 'vue3-persian-datetime-picker'

const loading = ref(false)

const app = createApp(App)
app.provide('loading', loading)

registerPlugins(app)

app
  .use(router)
  .use(store)
  .use(i18n)
  .component('DatePicker', Vue3PersianDatetimePicker)
  .mount('#app')
