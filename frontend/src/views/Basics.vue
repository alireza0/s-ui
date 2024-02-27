<template>
  <v-expansion-panels>
    <v-expansion-panel title="Log">
      <v-expansion-panel-text>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="appConfig.log.disabled" color="primary" :label="$t('disable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-select
              hide-details
              label="Level"
              :items="levels"
              v-model="appConfig.log.level">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-text-field
              v-model="appConfig.log.output"
              hide-details
              label="Output"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="appConfig.log.timestamp" color="primary" label="Timestamp" hide-details></v-switch>
          </v-col>
        </v-row>
      </v-expansion-panel-text>
    </v-expansion-panel>
    <v-expansion-panel title="DNS">
      <v-expansion-panel-text>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-select
              hide-details
              label="Final"
              :items="[ {title: 'First Server', value: ''}, ...dnsServersTags]"
              v-model="finalDns">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-select
              hide-details
              label="Domain to IP Strategy"
              clearable
              @click:clear="delete appConfig.dns.strategy"
              :items="['prefer_ipv4','prefer_ipv6','ipv4_only','ipv6_only']"
              v-model="appConfig.dns.strategy">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="3" align-self="center">
            <v-btn @click="addDnsServer" rounded>
              <v-icon icon="mdi-plus" />Server
            </v-btn>
          </v-col>
        </v-row>
        <template v-for="(s, index) in appConfig.dns.servers">
          Server {{ index+1 }} <v-icon icon="mdi-delete" @click="appConfig.dns.servers.splice(index,1)" />
          <v-divider></v-divider>
          <v-row>
            <v-col cols="12" sm="6" md="3">
              <v-text-field
                v-model="s.tag"
                hide-details
                clearable
                @click:clear="delete s.tag"
                label="Tag"
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="3">
              <v-text-field
                v-model="s.address"
                hide-details
                label="Address"
              ></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="3">
              <v-select
                hide-details
                label="Outbound"
                clearable
                @click:clear="delete s.detour"
                :items="outboundTags"
                v-model="s.detour">
              </v-select>
            </v-col>
            <v-col cols="12" sm="6" md="3">
              <v-select
                hide-details
                label="Domain Strategy"
                clearable
                @click:clear="delete s.strategy"
                :items="['prefer_ipv4','prefer_ipv6','ipv4_only','ipv6_only']"
                v-model="s.strategy">
              </v-select>
            </v-col>
          </v-row>
        </template>
      </v-expansion-panel-text>
    </v-expansion-panel>
    <v-expansion-panel title="NTP">
      <v-expansion-panel-text>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="enableNtp" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="appConfig.ntp.server"
              hide-details
              label="Server"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="appConfig.ntp.server_port"
              hide-details
              type="number"
              clearable
              @click:clear="delete appConfig.ntp.server_port"
              label="Server Port"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.ntp?.enabled">
            <v-text-field
              v-model="ntpInterval"
              hide-details
              suffix="m"
              min="0"
              type="number"
              label="Interval"
            ></v-text-field>
          </v-col>
        </v-row>
        <Dial :dial="appConfig.ntp" v-if="appConfig.ntp?.enabled" />
      </v-expansion-panel-text>
    </v-expansion-panel>
    <v-expansion-panel title="Experimental">
      <v-expansion-panel-text>
        Cache File
        <v-divider></v-divider>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="enableCacheFile" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.cache_file">
            <v-text-field
              v-model="appConfig.experimental.cache_file.path"
              hide-details
              label="Path"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.cache_file">
            <v-text-field
              v-model="appConfig.experimental.cache_file.cache_id"
              hide-details
              label="Cache ID"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.cache_file">
            <v-switch v-model="appConfig.experimental.cache_file.store_fakeip"
              color="primary"
              label="Store Fake IP"
              hide-details></v-switch>
          </v-col>
        </v-row>
        Clash API
        <v-divider></v-divider>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="enableClashApi" color="primary" :label="$t('enable')" hide-details></v-switch>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_controller"
              hide-details
              label="External Controller"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_ui"
              hide-details
              label="External UI"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_ui_download_url"
              hide-details
              label="UI Download URL"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.external_ui_download_detour"
              hide-details
              label="UI Download detour"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.secret"
              hide-details
              label="Secret"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" v-if="appConfig.experimental.clash_api">
            <v-text-field
              v-model="appConfig.experimental.clash_api.default_mode"
              hide-details
              label="Default Mode"
            ></v-text-field>
          </v-col>
        </v-row>
        V2Ray API
        <v-divider></v-divider>
        <v-row>
          <v-col cols="12" sm="6" md="3">
            <v-text-field
              v-model="appConfig.experimental.v2ray_api.listen"
              hide-details
              label="Listen"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3">
            <v-switch v-model="appConfig.experimental.v2ray_api.stats.enabled"
              color="primary"
              :label="$t('stats.enable')"
              hide-details></v-switch>
          </v-col>
        </v-row>
        <v-row v-if="appConfig.experimental.v2ray_api.stats.enabled">
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.inbounds')"
              multiple chips closable-chips
              :items="inboundTags"
              v-model="appConfig.experimental.v2ray_api.stats.inbounds">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.outbounds')"
              multiple chips closable-chips
              :items="outboundTags"
              v-model="appConfig.experimental.v2ray_api.stats.outbounds">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6">
            <v-select
              hide-details
              :label="$t('pages.clients')"
              multiple chips closable-chips
              :items="clientNames"
              v-model="appConfig.experimental.v2ray_api.stats.users">
            </v-select>
          </v-col>
        </v-row>
      </v-expansion-panel-text>
    </v-expansion-panel>
  </v-expansion-panels>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data';
import Dial from '@/components/Dial.vue';
import { computed } from 'vue';
import { Config, Ntp } from '@/types/config';
import { Client } from '@/types/clients';

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const inboundTags = computed((): string[] => {
  return appConfig.value.inbounds.map(i => i.tag)
})

const outboundTags = computed((): string[] => {
  return appConfig.value.outbounds.map(o => o.tag)
})

const clientNames = computed((): string[] => {
  const clients = <Client[]>Data().clients
  return clients?.map(c => c.name)
})

const levels = ["trace", "debug", "info", "warn", "error", "fatal", "panic"]

const dnsServersTags = computed((): string[] => {
  const s = <string[]>appConfig.value.dns.servers?.filter(s => s.tag && s.tag != "")?.map(s => s.tag)
  return s?? <string[]>[]
})

const finalDns = computed({
  get() { return appConfig.value.dns.final?? '' },
  set(v:string) { appConfig.value.dns.final = v.length>0 ? v : undefined }
})

const addDnsServer = () => {
  if (!appConfig.value.dns.servers) appConfig.value.dns.servers = []
  appConfig.value.dns.servers.push({address: 'local'})
}

const enableNtp = computed({
  get() { return appConfig.value.ntp?.enabled?? false },
  set(v:boolean) { 
    if (v){
      appConfig.value.ntp = <Ntp>{ enabled: true, server: 'time.apple.com', server_port: 123, interval: '30m'}
    } else { appConfig.value.ntp = <Ntp>{}  }
  }
})

const ntpInterval = computed({
  get():any { return appConfig.value.ntp?.interval? parseInt(appConfig.value.ntp?.interval.replace('m','')) : null },
  set(v:number) { if (appConfig.value.ntp) v>0 ? appConfig.value.ntp.interval =  v + 'm' : delete appConfig.value.ntp.interval }
})

const enableCacheFile = computed({
  get() { return appConfig.value.experimental.cache_file?.enabled?? false },
  set(v:boolean) { 
    if (v){
      appConfig.value.experimental.cache_file = { enabled: true }
    } else { delete appConfig.value.experimental.cache_file  }
  }
})

const enableClashApi = computed({
  get() { return appConfig.value.experimental.clash_api != undefined },
  set(v:boolean) { 
    if (v){
      appConfig.value.experimental.clash_api = {}
    } else { delete appConfig.value.experimental.clash_api  }
  }
})
</script>