<template>
  <v-card subtitle="ACME" style="background-color: inherit;">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch color="primary" :label="$t('enable')" v-model="enabled" hide-details></v-switch>
      </v-col>
      <v-col cols="12" md="8" v-if="enabled">
        <v-text-field
          :label="$t('rule.domain') + ' ' + $t('commaSeparated')"
          hide-details
          v-model="domains">
        </v-text-field>
      </v-col>
    </v-row>
    <template v-if="enabled">
      <v-row>
        <v-col cols="12" sm="6" md="4" v-if="optionDir">
          <v-text-field
            :label="$t('tls.acme.dataDir')"
            hide-details
            v-model="acme.data_directory">
          </v-text-field>
        </v-col>
        <v-col cols="12" sm="6" md="4" v-if="optionDefault">
          <v-combobox
            v-model="acme.default_server_name"
            :items="acme.domain"
            :label="$t('tls.acme.defaultDomain')"
            hide-details
          ></v-combobox>
        </v-col>
        <v-col cols="12" sm="6" md="4" v-if="optionEmail">
          <v-text-field
            :label="$t('email')"
            hide-details
            v-model="acme.email">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-if="optionChallenge">
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" :label="$t('tls.acme.httpChallenge')" v-model="acme.disable_http_challenge" hide-details></v-switch>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-switch color="primary" :label="$t('tls.acme.tlsChallenge')" v-model="acme.disable_tls_alpn_challenge" hide-details></v-switch>
        </v-col>
      </v-row>
      <v-row v-if="optionPorts">
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          :label="$t('tls.acme.altHport')"
          hide-details
          type="number"
          min=1
          max="65532"
          v-model.number="acme.alternative_http_port">
          </v-text-field>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          :label="$t('tls.acme.altTport')"
          hide-details
          type="number"
          min=1
          max="65532"
          v-model.number="acme.alternative_tls_port">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-if="optionProvider">
        <v-col cols="12" sm="6" md="4">
          <v-select
            v-model="caProvider"
            :items="providerList"
            :label="$t('tls.acme.caProvider')"
            hide-details
          ></v-select>
        </v-col>
        <v-col cols="12" md="8" v-if="caProvider == ''">
          <v-text-field
            :label="$t('tls.acme.customCa')"
            hide-details
            v-model="acme.provider">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-if="acme.external_account != undefined">
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          label="Key ID"
          hide-details
          v-model="acme.external_account.key_id">
          </v-text-field>
        </v-col>
        <v-col cols="12" sm="6" md="4">
          <v-text-field
          label="MAC Key"
          hide-details
          v-model="acme.external_account.mac_key">
          </v-text-field>
        </v-col>
      </v-row>
      <v-row v-if="acme.dns01_challenge != undefined">
        <v-col cols="12" sm="6" md="4">
          <v-select
          :label="$t('tls.acme.dns01Provider')"
          hide-details
          :items="dnsProviders.map(d => d.provider)"
          @update:model-value="acme.dns01_challenge = { provider: $event }"
          v-model="acme.dns01_challenge.provider">
          </v-select>
        </v-col>
        <v-col cols="12" sm="6" md="4"
        v-for="item in dnsProviders.filter(d => d.provider == acme.dns01_challenge?.provider)[0]?.params"
        :key="item">
          <v-text-field
          :label="item"
          hide-details
          v-model="acme.dns01_challenge[item]">
          </v-text-field>
        </v-col>
      </v-row>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-menu v-model="menu" :close-on-content-click="false" location="start">
          <template v-slot:activator="{ props }">
            <v-btn v-bind="props" hide-details variant="tonal">{{ $t('tls.acme.options') }}</v-btn>
          </template>
          <v-card>
            <v-list>
              <v-list-item>
                <v-switch v-model="optionDir" color="primary" :label="$t('tls.acme.dataDir')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionDefault" color="primary" :label="$t('tls.acme.defaultDomain')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionEmail" color="primary" :label="$t('email')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionChallenge" color="primary" :label="$t('tls.acme.disableChallenges')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionPorts" color="primary" :label="$t('tls.acme.altPorts')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionProvider" color="primary" :label="$t('tls.acme.caProvider')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionExt" color="primary" :label="$t('tls.acme.extAcc')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionDns01" color="primary" :label="$t('tls.acme.dns01')" hide-details></v-switch>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>
      </v-card-actions>
    </template>
  </v-card>
</template>

<script lang="ts">
import { acme } from '@/types/tls'

export default {
  props: ['tls'],
  data() {
    return {
      menu: false,
      providerList: [
        { title: "Let's Encrypt", value: "letsencrypt" },
        { title: "ZeroSSL", value: "zerossl" },
        { title: "Custom", value: "" }
      ],
      dnsProviders: [
        { provider: "cloudflare", params: [ "api_token" ] },
        { provider: "alidns", params: [ "access_key_id","access_key_secret","region_id" ] }
      ]
    }
  },
  computed: {
    acme() {
      return <acme>this.$props.tls.acme
    },
    enabled: {
      get() { return this.acme != undefined },
      set(v: boolean) { this.$props.tls.acme = v ? { domain: [] } : undefined }
    },
    domains: {
      get() { return this.acme?.domain ? this.acme.domain.join(',') : "" },
      set(v: string) {
        if(!v.endsWith(',')) {
          this.acme.domain = v.length > 0 ? v.split(',') : []
        }
      }
    },
    caProvider: {
      get() { return this.acme?.provider && ['letsencrypt','zerossl'].includes(this.acme.provider) ? this.acme?.provider : '' },
      set(v: string) { this.acme.provider = ['letsencrypt','zerossl'].includes(v) ? v : 'https://' }
    },
    optionDir: {
      get(): boolean { return this.acme?.data_directory != undefined },
      set(v:boolean) { this.acme.data_directory = v ? '' : undefined }
    },
    optionDefault: {
      get(): boolean { return this.acme?.default_server_name != undefined },
      set(v:boolean) { this.acme.default_server_name = v ? this.domains.length>0 ? this.domains[0] : '' : undefined }
    },
    optionEmail: {
      get(): boolean { return this.acme?.email != undefined },
      set(v:boolean) { this.acme.email = v ? '' : undefined }
    },
    optionChallenge: {
      get(): boolean { return this.acme?.disable_http_challenge != undefined || this.acme?.disable_tls_alpn_challenge != undefined },
      set(v:boolean) { 
        if (v) {
          this.acme.disable_http_challenge = false
          this.acme.disable_tls_alpn_challenge = false
        } else {
          delete this.acme.disable_http_challenge
          delete this.acme.disable_tls_alpn_challenge
        }
      }
    },
    optionPorts: {
      get(): boolean { return this.acme?.alternative_http_port != undefined || this.acme?.alternative_tls_port != undefined },
      set(v:boolean) { 
        if (v) {
          this.acme.alternative_http_port = 80
          this.acme.alternative_tls_port = 443
        } else {
          delete this.acme.alternative_http_port
          delete this.acme.alternative_tls_port
        }
      }
    },
    optionProvider: {
      get(): boolean { return this.acme?.provider != undefined },
      set(v:boolean) { this.acme.provider = v ? 'letsencrypt' : undefined }
    },
    optionExt: {
      get(): boolean { return this.acme?.external_account != undefined },
      set(v:boolean) { this.acme.external_account = v ? { key_id: '', mac_key: '' } : undefined }
    },
    optionDns01: {
      get(): boolean { return this.acme?.dns01_challenge != undefined },
      set(v:boolean) { this.acme.dns01_challenge = v ? { provider: 'cloudflare' } : undefined }
    },
  }
}
</script>