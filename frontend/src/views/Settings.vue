<template>
  <v-card :loading="loading">
    <v-tabs
    v-model="tab"
    align-tabs="center"
    bg-color="secondary"
    show-arrows
  >
    <v-tab value="t1">{{ $t('setting.interface') }}</v-tab>
    <v-tab value="t2">{{ $t('setting.sub') }}</v-tab>
    <v-tab value="t3">Language</v-tab>
  </v-tabs>
  <v-card-text>
    <v-row align="center" justify="center" style="margin-bottom: 10px;">
      <v-col cols="auto">
        <v-btn color="primary" @click="saveChanges" :loading="loading" :disabled="!stateChange">
          {{ $t('actions.save') }}
        </v-btn>
      </v-col>
      <v-col cols="auto">
        <v-btn variant="outlined" color="warning" @click="restartApp" :loading="loading" :disabled="stateChange">
          {{ $t('actions.restartApp') }}
        </v-btn>
      </v-col>
    </v-row>
    <v-window v-model="tab">
      <v-window-item value="t1">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="webPort" :label="$t('setting.port')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webPath" :label="$t('setting.webPath')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.webURI" :label="$t('setting.webUri')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="sessionMaxAge"
              :label="$t('setting.sessionAge')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.timeLocation" :label="$t('setting.timeLoc')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t2">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subEncode" :label="$t('setting.subEncode')" hide-details />
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="subShowInfo" :label="$t('setting.subInfo')" hide-details />
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subListen" :label="$t('setting.addr')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model="subPort"
              min="1"
              :label="$t('setting.port')"
              hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subKeyFile" :label="$t('setting.sslKey')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subCertFile" :label="$t('setting.sslCert')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subDomain" :label="$t('setting.domain')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subPath" :label="$t('setting.path')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field
              type="number"
              v-model.number="subUpdates"
              :label="$t('setting.update')"
              hide-details
              ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="settings.subURI" :label="$t('setting.subUri')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="t3">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              label="Language"
              :items="languages"
              v-model="$i18n.locale"
              @update:modelValue="changeLocale">
            </v-select>
          </v-col>
        </v-row>
      </v-window-item>
    </v-window>
  </v-card-text>
</v-card>
</template>

<script lang="ts" setup>
import { useLocale } from "vuetify"
import { languages } from '@/locales'
import { Ref, computed, inject, onMounted, ref } from "vue"
import HttpUtils from "@/plugins/httputil"
import { FindDiff } from "@/plugins/utils"
const locale = useLocale()
const tab = ref("t1")
const loading:Ref = inject('loading')?? ref(false)
const oldSettings = ref({})

const settings = ref({
	webListen: "",
	webDomain: "",
	webPort: "2095",
	webCertFile: "",
	webKeyFile: "",
  webPath: "/app/",
  webURI: "",
	sessionMaxAge: "0",
	timeLocation: "Asia/Tehran",
  subListen: "",
	subPort: "2096",
	subPath: "/sub/",
	subDomain: "",
	subCertFile: "",
	subKeyFile: "",
	subUpdates: "12",
	subEncode: "true",
	subShowInfo: "false",
	subURI: "",
})

onMounted(async () => {loadData()})

const changeLocale = (l: any) => {
  locale.current.value = l ?? 'en'
  localStorage.setItem('locale', locale.current.value)
}

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/setting')
  loading.value = false
  if (msg.success) {
    settings.value = msg.obj
    oldSettings.value = { ...msg.obj }
  }
}

const saveChanges = async () => {
  loading.value = true
  const diff = {
    settings: JSON.stringify(FindDiff.Settings(settings.value,oldSettings.value)),
  }
  const msg = await HttpUtils.post('api/save', diff)
  if (msg.success) {
    loadData()
  }
  loading.value = false
}

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const restartApp = async () => {
  loading.value = true
  const msg = await HttpUtils.post('api/restartApp',{})
  if (msg.success) {
    let url = settings.value.webURI
    if (url !== "") {
      const isTLS = settings.value.webCertFile !== "" || settings.value.webKeyFile !== ""
      url = buildURL(settings.value.webDomain,settings.value.webPort.toString(),isTLS, settings.value.webPath)
    }
    await sleep(3000)
    window.location.replace(url)
  }
  loading.value = false
}

const buildURL = (host: string, port: string, isTLS: boolean, path: string) => {
  if (!host || host.length == 0) host = window.location.hostname
  if (!port || port.length == 0) port = window.location.port

  const protocol = isTLS ? "https:" : "http:"

  if (port === "" || (isTLS && port === "443") || (!isTLS && port === "80")) {
      port = ""
  } else {
      port = `:${port}`
  }

  return `${protocol}//${host}${port}${path}settings`
}

const subEncode = computed({
  get: () => { return settings.value.subEncode == "true" },
  set: (v:boolean) => { settings.value.subEncode = v ? "true" : "false" }
})

const subShowInfo = computed({
  get: () => { return settings.value.subShowInfo == "true" },
  set: (v:boolean) => { settings.value.subShowInfo = v ? "true" : "false" }
})

const webPort = computed({
  get: () => { return parseInt(settings.value.webPort) },
  set: (v:number) => { settings.value.webPort = v.toString() }
})

const sessionMaxAge = computed({
  get: () => { return parseInt(settings.value.sessionMaxAge) },
  set: (v:number) => { settings.value.sessionMaxAge = v.toString() }
})

const subPort = computed({
  get: () => { return parseInt(settings.value.subPort) },
  set: (v:number) => { settings.value.subPort = v.toString() }
})

const subUpdates = computed({
  get: () => { return parseInt(settings.value.subUpdates) },
  set: (v:number) => { settings.value.subUpdates = v.toString() }
})

const stateChange = computed(() => {
  return !FindDiff.deepCompare(settings.value,oldSettings.value)
})
</script>
