<template>
  <Editor
    v-model="enableEditor"
    :data="settings.subClashExt"
    :visible="enableEditor"
    :title="$t('editor') + ' - ' + $t('setting.clashSub')"
    @close="enableEditor = false"
    @save="saveEditor"
    />
  <v-card>
    <v-row>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionMixed">
        <v-text-field type="number" v-model.number="mixedPort" min="1" max="65535" :label="$t('setting.mixedPort')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionMixed">
        <v-switch color="primary" v-model="allowLan" :label="$t('types.ts.allowLanAccess')" hide-details />
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionExt">
        <v-text-field v-model="externalController" :label="$t('basic.exp.extController')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionLog">
        <v-select v-model="logLevel" :items="['debug', 'info', 'warning', 'error']" :label="$t('basic.log.title') + ' - ' + $t('basic.log.level')" hide-details></v-select>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionTun">
        <v-switch color="primary" v-model="tun" :label="$t('setting.tun')" hide-details />
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2" v-if="optionDns">
        <v-switch color="primary" v-model="dns" :label="$t('pages.dns')" hide-details />
      </v-col>
    </v-row>
    <v-row v-if="optionRules">
      <v-col cols="12" sm="12" md="6" lg="4">
        <v-select
          v-model="rules"
          :items="rulesIP"
          chips
          closable-chips
          multiple
          hide-details
          :label="$t('pages.rules')"
        ></v-select>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-btn @click="openEditor" variant="outlined" hide-details>{{ $t('editor') }}</v-btn>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('setting.jsonSubOptions') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionMixed" color="primary" :label="$t('setting.mixedPort')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionTun" color="primary" :label="$t('setting.tun')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionExt" color="primary" :label="$t('basic.exp.extController')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionLog" color="primary" :label="$t('basic.log.title')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionDns" color="primary" :label="$t('pages.dns')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionRules" color="primary" :label="$t('pages.rules')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
import { push } from 'notivue'
import Editor from './Editor.vue'
import yaml from 'yaml'
import { i18n } from '@/locales'
export default {
  props: ['settings'],
  data() {
    return {
      enableEditor: false,
      menu: false,
      defaultConfig: {
        "mixed-port": 7890,
        "allow-lan": false,
        "mode": "rule",
        "log-level": "info",
        "external-controller": "127.0.0.1:9090",
        "tun": {
          "enable": true,
          "stack": "system",
          "auto-route": true,
          "auto-detect-interface": true,
          "dns-hijack": ["any:53"],
        },
        "dns": {
          "enable": true,
          "ipv6": false,
          "enhanced-mode": "fake-ip",
          "fake-ip-range": "198.18.0.1/16",
          "default-nameserver": ["8.8.8.8","1.1.1.1"],
          "nameserver": [
            "https://doh.pub/dns-query",
            "https://1.0.0.1/dns-query"
          ],
          "fallback": ["tcp://9.9.9.9:53"],
          "fake-ip-filter": ["*.lan", "localhost", "*.local"]
        },
        "rules": [
          "GEOIP,Private,DIRECT",
          "MATCH,Proxy"
        ]
      },
      geoList: [
        { title: "Site-Private", value: "geoip-private" },
        { title: "IP-Private", value: "geosite-private" },
        { title: "Site-Ads", value: "geosite-ads" },
        { title: "🇮🇷 Site-Iran", value: "geosite-ir" },
        { title: "🇮🇷 IP-Iran", value: "geoip-ir" },
        { title: "🇨🇳 Site-China", value: "geosite-cn" },
        { title: "🇨🇳 IP-China", value: "geoip-cn" },
        { title: "🇻🇳 Site-Vietnam", value: "geosite-vn" },
        { title: "🇻🇳 IP-Vietnam", value: "geoip-vn" },
      ],
      rulesIP: [
        { title: 'Private-Direct', value: 'GEOIP,Private,DIRECT' },
        { title: 'Private-Block', value: 'GEOIP,Private,REJECT' },
        { title: 'Ads-Direct', value: 'GEOIP,Ads,DIRECT' },
        { title: 'Ads-Block', value: 'GEOIP,Ads,REJECT' },
        { title: '🇨🇳 China-Direct', value: 'GEOIP,CN,DIRECT' },
        { title: '🇨🇳 China-Block', value: 'GEOIP,CN,REJECT' },
        { title: '🇮🇷 Iran-Direct', value: 'GEOIP,CATEGORY-IR,DIRECT' },
        { title: '🇮🇷 Iran-Block', value: 'GEOIP,CATEGORY-IR,REJECT' },
        { title: '🇻🇳 Vietnam-Direct', value: 'GEOIP,CATEGORY-VN,DIRECT' },
        { title: '🇻🇳 Vietnam-Block', value: 'GEOIP,CATEGORY-VN,REJECT' },
      ],
    }
  },
  methods: {
    openEditor() {
      this.enableEditor = true
    },
    saveEditor(data:string) {
      try {
        const result = yaml.parse(data)
        if (typeof result != 'object' || Array.isArray(result)) throw new Error()
      } catch (e) {
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('error.invalidData'),
          duration: 5000,
        })
        return
      }
      this.$props.settings.subClashExt = data
      this.enableEditor = false
    },
    updateMetaJson(data:any, key:string) {
      let newMetaJson = this.metaJson
      if (data==null) {
        delete newMetaJson[key]
      } else {
        newMetaJson[key] = data
      }
      this.metaJson = newMetaJson
    }
  },
  computed: {
    metaJson: {
      get() {
        try {
          return yaml.parse(this.settings.subClashExt)??{}
        } catch (e) {
          return {}
        }
      },
      set(v:any) {
        this.settings.subClashExt = Object.keys(v).length==0 ? "" : yaml.stringify(v)
      }
    },
    optionMixed: {
      get() { return this.metaJson['mixed-port']>0 },
      set(v:boolean) {
        this.updateMetaJson(v ? this.defaultConfig['mixed-port'] : null, 'mixed-port')
        this.updateMetaJson(v ? this.defaultConfig['allow-lan'] : null, 'allow-lan')
      }
    },
    optionTun: {
      get() { return this.metaJson['tun']?.['enable']?? false },
      set(v:boolean) { this.updateMetaJson(v ? this.defaultConfig['tun'] : null, 'tun') }
    },
    optionExt: {
      get() { return this.metaJson['external-controller']?.length>0 },
      set(v:boolean) { this.updateMetaJson(v ? this.defaultConfig['external-controller'] : null, 'external-controller') }
    },
    optionLog: {
      get() { return this.metaJson['log-level']?.length>0 },
      set(v:boolean) { this.updateMetaJson(v ? this.defaultConfig['log-level'] : null, 'log-level') }
    },
    optionDns: {
      get() { return this.metaJson['dns']?.['enable']?? false },
      set(v:boolean) { this.updateMetaJson(v ? this.defaultConfig['dns'] : null, 'dns') }
    },
    optionRules: {
      get() { return this.metaJson['rules']?.length>0 },
      set(v:boolean) {
        this.updateMetaJson(v ? this.defaultConfig['rules'] : null, 'rules')
        this.updateMetaJson(v ? this.defaultConfig['mode'] : null, 'mode')
      }
    },
    mixedPort: {
      get() { return this.metaJson['mixed-port'] },
      set(v:number) { this.updateMetaJson(v, 'mixed-port') }
    },
    allowLan: {
      get() { return this.metaJson['allow-lan'] },
      set(v:boolean) { this.updateMetaJson(v, 'allow-lan') }
    },
    externalController: {
      get() { return this.metaJson['external-controller'] },
      set(v:string) { this.updateMetaJson(v, 'external-controller') }
    },
    logLevel: {
      get() { return this.metaJson['log-level'] },
      set(v:string) { this.updateMetaJson(v, 'log-level') }
    },
    dns: {
      get() { return this.metaJson['dns']?.['enable'] ?? false },
      set(v:boolean) { this.updateMetaJson({ ...this.metaJson['dns'], 'enable': v }, 'dns') }
    },
    tun: {
      get() { return this.metaJson['tun']?.['enable'] ?? false },
      set(v:boolean) { this.updateMetaJson({ ...this.metaJson['tun'], 'enable': v }, 'tun') }
    },
    rules: {
      get() { return this.metaJson.rules.length > 0 ? this.metaJson.rules.filter((r:string) => r != "MATCH,Proxy") : [] },
      set(v:string[]) {
        let newRules = <string[]>[]
        v.forEach((r:string) => { newRules.push(r) })          
        this.updateMetaJson([ ...newRules, "MATCH,Proxy" ], 'rules')
      }
    }
  },
  components: { Editor }
}
</script>