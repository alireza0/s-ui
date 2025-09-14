<template>
  <Editor
    v-model="enableEditor"
    :data="settings.subJsonExt"
    :visible="enableEditor"
    :title="$t('editor') + ' - ' + $t('setting.jsonSub')"
    @close="enableEditor = false"
    @save="saveEditor"
    />
  <v-card>
    <v-row>
      <v-col cols="12" sm="6" md="3">
        <v-select
          v-model="ruleToDirect"
          :items="geoList"
          :label="$t('setting.toDirect')"
          multiple
          chips
          hide-details
        ></v-select>
      </v-col>
      <v-col cols="12" sm="6" md="3">
        <v-select
          v-model="ruleToBlock"
          :items="geoList"
          :label="$t('setting.toBlock')"
          multiple
          chips
          hide-details
        ></v-select>
      </v-col>
    </v-row>
    <v-row  v-if="enableLog">
      <v-col cols="12" sm="6" md="3" lg="2">
        <v-select
          hide-details
          :label="$t('basic.log.level')"
          :items="levels"
          v-model="subJsonExt.log.level">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2">
        <v-switch v-model="subJsonExt.log.timestamp" color="primary" :label="$t('setting.timestamp')" hide-details />
      </v-col>
    </v-row>
    <v-row v-if="enableDns">
      <v-col cols="12" sm="6" md="3" lg="2">
        <v-select
          hide-details
          :label="$t('dns.final')"
          :items="dnsTags"
          v-model="subJsonExt.dns.final">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2">
        <SimpleDNS :data="proxyDns" :label="$t('setting.globalDns')" />
      </v-col>
      <v-col cols="12" sm="6" md="3" lg="2">
        <SimpleDNS :data="directDns" :label="$t('setting.directDns')" />
      </v-col>
    </v-row>
    <v-row v-if="enableDns">
      <v-col cols="12" sm="6" md="3" lg="2">
        <v-select
          hide-details
          :label="$t('basic.routing.defaultDns')"
          :items="dnsTags"
          clearable
          @click:clear="delete subJsonExt.default_domain_resolver"
          v-model="subJsonExt.default_domain_resolver">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="3">
        <v-select
          v-model="dnsToDirect"
          :items="geositeList"
          :label="$t('setting.toDirectDns')"
          multiple
          chips
          hide-details
        ></v-select>
      </v-col>
    </v-row>
    <template v-if="enableInb">
      <v-row>
        <v-col cols="12" sm="6" md="3">
          <v-combobox
            v-model="inbounds[0].address"
            :items="defaultInb[0].address"
            chips
            multiple
            hide-details
            :label="$t('in.addr')"
          ></v-combobox>
        </v-col>
        <v-col cols="12" sm="6" md="3" lg="2">
          <v-text-field
            type="number"
            v-model.number="inbounds[0].mtu"
            hide-details
            label="MTU"
          ></v-text-field>
        </v-col>
      </v-row>
      <v-row>
        <v-col cols="12" sm="6" md="3">
          <v-combobox
            v-model="inbounds[0].exclude_package"
            :items="['ir.mci.ecareapp','com.myirancell']"
            chips
            multiple
            hide-details
            :label="$t('setting.excludePkg')"
          ></v-combobox>
        </v-col>
        <v-col cols="12" sm="6" md="3" lg="2">
          <v-switch
            v-model="platformProxy"
            hide-details
            color="primary"
            label="Platform HTTP proxy"
          ></v-switch>
        </v-col>
      </v-row>
    </template>
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
              <v-switch v-model="enableLog" color="primary" :label="$t('basic.log.title')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="enableDns" color="primary" label="DNS" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="enableInb" color="primary" :label="$t('objects.inbound')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="enableExp" color="primary" label="Experimental" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
import Editor from './Editor.vue'
import SimpleDNS from './SimpleDNS.vue'
import { push } from 'notivue'
import { i18n } from '@/locales'
export default {
  props: ['settings'],
  data() {
    return {
      menu: false,
      enableEditor: false,
      subJsonExt: <any>{},
      levels: ["trace", "debug", "info", "warn", "error", "fatal", "panic"],
      defaultLog: {
        "level": "info",
        "timestamp": true
      },
      defaultInb: [
        {
          "type": "tun",
          "address": [
            "172.19.0.1/30",
            "fdfe:dcba:9876::1/126"
          ],
          "mtu": 9000,
          "auto_route": true,
          "strict_route": false,
          "endpoint_independent_nat": false,
          "stack": "mixed",
          "exclude_package": [],
          "platform": {
            "http_proxy": {
              "enabled": true,
              "server": "127.0.0.1",
              "server_port": 2080
            }
          }
        },
        {
          "type": "mixed",
          "listen": "127.0.0.1",
          "listen_port": 2080,
          "users": []
        }
      ],
      defaultExp: {
        "clash_api": {
          "external_controller": "127.0.0.1:9090",
          "external_ui": "ui",
          "secret": "",
          "external_ui_download_url": "https://mirror.ghproxy.com/https://github.com/MetaCubeX/Yacd-meta/archive/gh-pages.zip",
          "external_ui_download_detour": "direct",
          "default_mode": "rule"
        },
        "cache_file": {
          "enabled": true,
          "store_fakeip": false
        }
      },
      defaultDns: {
        "servers": [
          {
            "type": "tcp",
            "tag": "proxy-dns",
            "server": "8.8.8.8",
            "server_port": 53,
            "detour": "proxy",
            "domain_resolver": "local-dns",
          },
          { 
            "tag": "direct-dns",
            "type": "local",
          },
          {
            "tag": "local-dns",
            "type": "local",
          }
        ],
        "rules": [
          {
            "clash_mode": "Global",
            "source_ip_cidr": [
              "172.19.0.0/30",
              "fdfe:dcba:9876::1/126"
            ],
            "action": "route",
            "server": "proxy-dns"
          },
          {
            "clash_mode": "Direct",
            "action": "route",
            "server": "direct-dns"
          },
          {
            "source_ip_cidr": [
              "172.19.0.0/30",
              "fdfe:dcba:9876::1/126"
            ],
            "action": "route",
            "server": "proxy-dns"
          },
        ],
        "final": "local-dns",
        "strategy": "prefer_ipv4"
      },
      geositeList: [
        { title: "Private", value: "geosite-private" },
        { title: "Ads", value: "geosite-ads" },
        { title: "🇮🇷 Iran", value: "geosite-ir" },
        { title: "🇨🇳 China", value: "geosite-cn" },
        { title: "🇻🇳 Vietnam", value: "geosite-vn" },
      ],
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
      geo: [
        {
          tag: "geosite-ads",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/category-ads-all.srs",
          download_detour: "direct"
        },
        {
          tag: "geosite-private",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/private.srs",
          download_detour: "direct"
        },
        {
          tag: "geosite-ir",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/category-ir.srs",
          download_detour: "direct"
        },
        {
          tag: "geosite-cn",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/cn.srs",
          download_detour: "direct"
        },
        {
          tag: "geosite-vn",
          type: "remote",
          format: "binary",
          url: "https://github.com/Thaomtam/Geosite-vn/raw/rule-set/Geosite-vn.srs",
          download_detour: "direct"
        },
        {
          tag: "geoip-private",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/private.srs",
          download_detour: "direct"
        },
        {
          tag: "geoip-ir",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/ir.srs",
          download_detour: "direct"
        },
        {
          tag: "geoip-cn",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/cn.srs",
          download_detour: "direct"
        },
        {
          tag: "geoip-vn",
          type: "remote",
          format: "binary",
          url: "https://testingcf.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geoip/vn.srs",
          download_detour: "direct"
        }
      ],
    }
  },
  computed: {
    enableLog: {
      get() :boolean { return this.subJsonExt?.log != undefined },
      set(v:boolean) { v ? this.subJsonExt.log = this.defaultLog : delete this.subJsonExt.log }
    },
    enableDns: {
      get() :boolean { return this.subJsonExt?.dns != undefined },
      set(v:boolean) {
        if (v) {
          this.subJsonExt.dns = this.defaultDns
          if (this.rules == undefined) this.subJsonExt.rules = [{ action: 'sniff' }]
          this.subJsonExt.rules.unshift({ protocol: "dns", action: "hijack-dns" })
        } else {
          delete this.subJsonExt.dns
          const rules = this.subJsonExt?.rules?.filter((r:any) => r.protocol != "dns") ?? []
          if (rules.length >= 0) this.subJsonExt.rules = rules
          if (this.rules.length == 0) delete this.subJsonExt.rules
        }
      }
    },
    enableInb: {
      get() :boolean { return this.subJsonExt?.inbounds != undefined },
      set(v:boolean) { v ? this.subJsonExt.inbounds = this.defaultInb.slice() : delete this.subJsonExt.inbounds }
    },
    enableExp: {
      get() :boolean { return this.subJsonExt?.experimental != undefined },
      set(v:boolean) { v ? this.subJsonExt.experimental = this.defaultExp : delete this.subJsonExt.experimental }
    },
    dns():any { return this.subJsonExt?.dns?? undefined },
    proxyDns: {
      get() :any { return this.dns?.servers?.findLast((d:any) => d.tag == "proxy-dns")?? {} },
      set(v:any) { 
        let sIndex = this.dns.servers.findIndex((d:any) => d.tag == "proxy-dns")
        console.log(sIndex)
        if (sIndex === -1 || sIndex == undefined) {
          this.dns.servers.push({ ...this.defaultDns.servers[0], ...v })
        } else {
          this.dns.servers[sIndex] = { ...this.defaultDns.servers[0], ...v }
        }
      }
    },
    directDns: {
      get() :any { return this.dns?.servers?.findLast((d:any) => d.tag == "direct-dns")?? {} },
      set(v:any) {
        const sIndex = this.dns.servers.findIndex((d:any) => d.tag == "direct-dns")
        if (sIndex === -1 || sIndex == undefined) {
          this.dns.servers.push({ ...this.defaultDns.servers[1], ...v })
        } else {
          this.dns.servers[sIndex] = { ...this.defaultDns.servers[1], ...v }
        }
      },
    },
    dnsTags() { return this.dns?.servers?.map((d:any) => d.tag) ?? [] },
    final: {
      get() :string { return this.dns.final?? "" },
      set(v:string) { this.dns.final = v.length>0 ? v : undefined }
    },
    dnsToDirect: {
      get() :string[] {
        const ruleIndex = this.dns?.rules?.findIndex((r:any) => r.server == "direct-dns" && Object.hasOwn(r,'rule_set'))
        return ruleIndex >= 0 ? this.dns.rules[ruleIndex].rule_set : []
      },
      set(v:string[]) {
        const ruleIndex = this.dns?.rules?.findIndex((r:any) => r.server == "direct-dns" && Object.hasOwn(r,'rule_set'))
        if (v.length>0) {
          if (ruleIndex >= 0){
            this.dns.rules[ruleIndex].rule_set = v
          } else {
            this.dns.rules.push({ rule_set: v, action: "route", server: "direct-dns" })
          }
        } else {
          if (ruleIndex != -1) this.dns.rules.splice(ruleIndex,1)
        }
        this.updateRuleSets()
      }
    },
    inbounds():any[] { return this.subJsonExt?.inbounds?? undefined },
    platformProxy: {
      get() :boolean { return this.inbounds[0]?.platform != undefined },
      set(v:boolean) { this.subJsonExt.inbounds[0].platform = v ? this.defaultInb[0].platform : undefined }
    },
    rules():any { return this.subJsonExt?.rules?? undefined },
    ruleToDirect: {
      get() :string[] {
        const ruleIndex = this.rules?.findIndex((r:any) => r.outbound == "direct" && Object.hasOwn(r,'rule_set'))
        return ruleIndex >= 0 ? this.rules[ruleIndex].rule_set : []
      },
      set(v:string[]) {
        const ruleIndex = this.rules?.findIndex((r:any) => r.outbound == "direct" && Object.hasOwn(r,'rule_set'))
        if (v.length>0) {
          if (ruleIndex >= 0){
            this.rules[ruleIndex].rule_set = v
          } else {
            if (this.rules == undefined) this.subJsonExt.rules = []
            this.rules.push({ rule_set: v, action: "route", outbound: "direct" })
          }
        } else {
          if (ruleIndex != -1) this.rules.splice(ruleIndex,1)
        }
        this.updateRuleSets()
      }
    },
    ruleToBlock: {
      get() :string[] {
        const ruleIndex = this.rules?.findIndex((r:any) => r.action == "reject" && Object.hasOwn(r,'rule_set'))
        return ruleIndex >= 0 ? this.rules[ruleIndex].rule_set : []
      },
      set(v:string[]) {
        const ruleIndex = this.rules?.findIndex((r:any) => r.action == "reject" && Object.hasOwn(r,'rule_set'))
        if (v.length>0) {
          if (ruleIndex >= 0){
            this.rules[ruleIndex].rule_set = v
          } else {
            if (this.rules == undefined) this.subJsonExt.rules = []
            this.rules.push({ rule_set: v, action: "reject" })
          }
        } else {
          if (ruleIndex != -1) this.rules.splice(ruleIndex,1)
        }
        this.updateRuleSets()
      }
    }
  },
  methods: {
    loadData() {
      if (this.$props.settings?.subJsonExt?.length>0){
        this.subJsonExt = JSON.parse(this.$props.settings.subJsonExt)
      } else {
        this.subJsonExt = <any>{}
      }
    },
    updateRuleSets(){
      let tags = <string[]>[]
      if (this.dns?.rules?.length>0) this.dns.rules.forEach((r:any) => { if (r.rule_set) tags.push(...r.rule_set) })
      if (this.rules?.length>0) this.rules.forEach((r:any) => { if (r.rule_set) tags.push(...r.rule_set) })
      if (tags.length>0){
        this.subJsonExt.rule_set = this.geo.filter((g:any) => tags.includes(g.tag))
      } else {
        delete this.subJsonExt.rule_set
      }
      if (this.rules.length == 0) delete this.subJsonExt.rules
    },
    openEditor() {
      this.enableEditor = true
    },
    saveEditor(data:string) {
      try {
        this.subJsonExt = JSON.parse(data)
      } catch (e) {
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('error.invalidData'),
          duration: 5000,
        })
        return
      }
      this.enableEditor = false
    }
  },
  mounted(){
    this.loadData()
  },
  watch:{
    subJsonExt:{
      handler(v) {
        this.$props.settings.subJsonExt = Object.keys(v).length>0 ? JSON.stringify(v, null, 2) : ""
      },
      deep: true
    },
  },
  components: { Editor, SimpleDNS }
}
</script>