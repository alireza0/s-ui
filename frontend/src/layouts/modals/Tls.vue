<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.tls') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-card class="rounded-lg">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                :label="$t('client.name')"
                hide-details
                v-model="tls.name">
              </v-text-field>
            </v-col>
            <v-col align="end">
              <v-btn-toggle v-model="tlsType"
              class="rounded-xl"
              density="compact"
              variant="outlined"
              @update:model-value="changeTlsType"
              shaped
              mandatory>
                <v-btn>TLS</v-btn>
                <v-btn>Reality</v-btn>
              </v-btn-toggle>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" sm="6" md="4" v-if="inTls.server_name != undefined">
              <v-text-field
                label="SNI"
                hide-details
                v-model="inTls.server_name">
              </v-text-field>
            </v-col>
            <template v-if="tlsType == 0">
              <v-col cols="12" sm="6" md="4" v-if="inTls.min_version">
                <v-select
                  hide-details
                  :label="$t('tls.minVer')"
                  :items="tlsVersions"
                  v-model="inTls.min_version">
                </v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="inTls.max_version">
                <v-select
                  hide-details
                  :label="$t('tls.maxVer')"
                  :items="tlsVersions"
                  v-model="inTls.max_version">
                </v-select>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="inTls.alpn">
                <v-select
                  hide-details
                  label="ALPN"
                  multiple
                  :items="alpn"
                  v-model="inTls.alpn">
                </v-select>
              </v-col>
              <v-col cols="12" md="8" v-if="inTls.cipher_suites != undefined">
                <v-select
                  hide-details
                  :label="$t('tls.cs')"
                  multiple
                  :items="cipher_suites"
                  v-model="inTls.cipher_suites">
                </v-select>
              </v-col>
            </template>
          </v-row>
          <template v-if="tlsType == 0">
            <v-row>
              <v-col>
                <v-btn-toggle v-model="usePath"
                class="rounded-xl"
                density="compact"
                variant="outlined"
                shaped
                mandatory>
                  <v-btn
                    @click="inTls.key=undefined; inTls.certificate=undefined"
                  >{{ $t('tls.usePath') }}</v-btn>
                  <v-btn
                    @click="inTls.key_path=undefined; inTls.certificate_path=undefined"
                  >{{ $t('tls.useText') }}</v-btn>
                </v-btn-toggle>
              </v-col>
              <v-spacer></v-spacer>
              <v-col cols="auto">
                <v-btn
                  variant="tonal"
                  density="compact"
                  icon="mdi-key-star"
                  @click="genSelfSigned"
                  :loading="loading">
                  <v-icon />
                  <v-tooltip activator="parent" location="top">
                    {{ $t('actions.generate') }}
                  </v-tooltip>
                </v-btn>
              </v-col>
            </v-row>
            <v-row v-if="usePath == 0">
              <v-col cols="12" sm="6">
                <v-text-field
                  :label="$t('tls.certPath')"
                  hide-details
                  v-model="inTls.certificate_path">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6">
                <v-text-field
                  :label="$t('tls.keyPath')"
                  hide-details
                  v-model="inTls.key_path">
                </v-text-field>
              </v-col>
            </v-row>
            <v-row v-else>
              <v-col cols="12">
                <v-textarea
                  :label="$t('tls.cert')"
                  hide-details
                  v-model="certText">
                </v-textarea>
              </v-col>
              <v-col cols="12">
                <v-textarea
                  :label="$t('tls.key')"
                  hide-details
                  v-model="keyText">
                </v-textarea>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.disableSni')" v-model="disableSni" hide-details></v-switch>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-switch color="primary" :label="$t('tls.insecure')" v-model="insecure" hide-details></v-switch>
              </v-col>
            </v-row>
          </template>
          <template v-if="outTls.reality && inTls.reality">
            <v-row>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('types.shdwTls.hs')"
                hide-details
                v-model="inTls.reality.handshake.server">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4">
                <v-text-field
                :label="$t('out.port')"
                type="number"
                min="0"
                hide-details
                v-model="server_port">
                </v-text-field>
              </v-col>
              <v-spacer></v-spacer>
              <v-col cols="auto">
                <v-btn
                  variant="tonal"
                  density="compact"
                  icon="mdi-key-star"
                  @click="genRealityKey"
                  :loading="loading">
                  <v-icon />
                  <v-tooltip activator="parent" location="top">
                    {{ $t('actions.generate') }}
                  </v-tooltip>
                </v-btn>
              </v-col>
            </v-row>
            <v-row>
              <v-col cols="12">
                <v-text-field
                  :label="$t('tls.privKey')"
                  hide-details
                  v-model="inTls.reality.private_key">
                </v-text-field>
              </v-col>
              <v-col cols="12">
                <v-text-field
                  :label="$t('tls.pubKey')"
                  hide-details
                  v-model="outTls.reality.public_key">
                </v-text-field>
              </v-col>
              <v-col cols="12">
                <v-text-field
                  label="Short IDs"
                  hide-details
                  append-icon="mdi-refresh"
                  @click:append="randomSID"
                  v-model="short_id">
                </v-text-field>
              </v-col>
              <v-col cols="12" sm="6" md="4" v-if="optionTime">
                <v-text-field
                label="Max Time Diference"
                type="number"
                min="1"
                :suffix="$t('date.m')"
                hide-details
                v-model="max_time">
                </v-text-field>
              </v-col>
            </v-row>
          </template>
          <v-row v-if="outTls.utls != undefined">
            <v-col cols="12" sm="6" md="4">
              <v-select
                hide-details
                label="Fingerprint"
                :items="fingerprints"
                v-model="outTls.utls.fingerprint">
              </v-select>
            </v-col>
          </v-row>
          <v-card-actions>
            <v-spacer></v-spacer>
            <v-menu v-model="menu" :close-on-content-click="false" location="start">
              <template v-slot:activator="{ props }">
                <v-btn v-bind="props" hide-details variant="tonal">{{ $t('tls.options') }}</v-btn>
              </template>
              <v-card>
                <v-list>
                  <template v-if="tlsType == 0">
                    <v-list-item>
                      <v-switch v-model="optionSNI" color="primary" label="SNI" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionALPN" color="primary" label="ALPN" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionMinV" color="primary" :label="$t('tls.minVer')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionMaxV" color="primary" :label="$t('tls.maxVer')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionCS" color="primary" :label="$t('tls.cs')" hide-details></v-switch>
                    </v-list-item>
                    <v-list-item>
                      <v-switch v-model="optionFP" color="primary" label="UTLS" hide-details></v-switch>
                    </v-list-item>
                  </template>
                  <template v-else>
                    <v-list-item>
                      <v-switch v-model="optionTime" color="primary" label="Max Time Difference" hide-details></v-switch>
                    </v-list-item>
                  </template>
                </v-list>
              </v-card>
            </v-menu>
          </v-card-actions>
        </v-card>
        <AcmeVue :tls="inTls" />
        <EchVue :iTls="inTls" :oTls="outTls" />
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          :loading="loading"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { tls, iTls, defaultInTls, oTls, defaultOutTls } from '@/types/tls'
import AcmeVue from '@/components/tls/Acme.vue'
import EchVue from '@/components/tls/Ech.vue'
import HttpUtils from '@/plugins/httputil'
import { push } from 'notivue'
import { i18n } from '@/locales'
import RandomUtil from '@/plugins/randomUtil'
export default {
  props: ['visible', 'data', 'id'],
  emits: ['close', 'save'],
  data() {
    return {
      tls: <tls>{ id: 0, name: '', server: <iTls>{ enabled: true }, client: <oTls>{} },
      title: "add",
      loading: false,
      menu: false,
      tlsType: 0,
      usePath: 0,
      alpn: [
        { title: "H3", value: 'h3' },
        { title: "H2", value: 'h2' },
        { title: "Http/1.1", value: 'http/1.1' },
      ],
      tlsVersions: [ '1.0', '1.1', '1.2', '1.3' ],
      cipher_suites: [
        { title: "RSA-AES128-CBC-SHA", value: "TLS_RSA_WITH_AES_128_CBC_SHA" },
        { title: "RSA-AES256-CBC-SHA", value: "TLS_RSA_WITH_AES_256_CBC_SHA" },
        { title: "RSA-AES128-GCM-SHA256", value: "TLS_RSA_WITH_AES_128_GCM_SHA256" },
        { title: "RSA-AES256-GCM-SHA384", value: "TLS_RSA_WITH_AES_256_GCM_SHA384" },
        { title: "AES128-GCM-SHA256", value: "TLS_AES_128_GCM_SHA256" },
        { title: "AES256-GCM-SHA384", value: "TLS_AES_256_GCM_SHA384" },
        { title: "CHACHA20-POLY1305-SHA256", value: "TLS_CHACHA20_POLY1305_SHA256" },
        { title: "ECDHE-ECDSA-AES128-CBC-SHA", value: "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA" },
        { title: "ECDHE-ECDSA-AES256-CBC-SHA", value: "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA" },
        { title: "ECDHE-RSA-AES128-CBC-SHA", value: "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA" },
        { title: "ECDHE-RSA-AES256-CBC-SHA", value: "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA" },
        { title: "ECDHE-ECDSA-AES128-GCM-SHA256", value: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256" },
        { title: "ECDHE-ECDSA-AES256-GCM-SHA384", value: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384" },
        { title: "ECDHE-RSA-AES128-GCM-SHA256", value: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256" },
        { title: "ECDHE-RSA-AES256-GCM-SHA384", value: "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384" },
        { title: "ECDHE-ECDSA-CHACHA20-POLY1305-SHA256", value: "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256" },
        { title: "ECDHE-RSA-CHACHA20-POLY1305-SHA256", value: "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256" }
      ],
      fingerprints: [
        { title: "Chrome", value: "chrome" },
        { title: "Firefox", value: "firefox" },
        { title: "Microsoft Edge", value: "edge" },
        { title: "Apple Safari", value: "safari" },
        { title: "360", value: "360" },
        { title: "QQ", value: "qq" },
        { title: "Apple IOS", value: "ios" },
        { title: "Android", value: "android" },
        { title: "Random", value: "random" },
        { title: "Randomized", value: "randomized" },
      ]
    }
  },
  methods: {
    updateData(id: number) {
      if (id > 0) {
        const newData = <tls>JSON.parse(this.$props.data)
        this.tls = newData
        if (this.tls.server == null) this.tls.server = { enabled: true }
        if (this.tls.client == null) this.tls.client = {}
        this.tlsType = newData.server?.reality == undefined ? 0 : 1
        this.usePath = newData.server?.key == undefined ? 0 : 1
        this.title = "edit"
      }
      else {
        this.tls = <tls>{ id: 0, name: '', server: {enabled: true}, client: {} }
        this.tlsType = 0
        this.usePath = 0
        this.title = "add"
      }
    },
    changeTlsType(){
      if (this.tlsType) {
        this.tls.server = <iTls>{
          enabled: true,
          reality: { enabled: true, handshake: { server_port: 443 }, short_id: RandomUtil.randomShortId() },
          server_name: ""
        }
        this.tls.client = <oTls>{ reality: { public_key: "" }, utls: defaultOutTls.utls }
      } else {
        this.tls.server = <iTls>{ enabled: true }
        this.tls.client = <oTls>{}
      }
    },
    closeModal() {
      this.updateData(0) // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.tls)
      this.loading = false
    },
    async genSelfSigned(){
      this.loading = true
      const msg = await HttpUtils.get('api/keypairs', { k: "tls", o: this.inTls.server_name?? "''" })
      this.loading = false
      if (msg.success) {
        this.inTls.key_path=undefined
        this.inTls.certificate_path=undefined
        this.usePath = 1
        if (msg.obj.length>0){
          let privateKey = <string[]>[]
          let publicKey = <string[]>[]
          let isPrivateKey = false
          let isPublicKey = false

          msg.obj.forEach((line:string) => {
              if (line === "-----BEGIN PRIVATE KEY-----") {
                  isPrivateKey = true
                  isPublicKey = false
                  privateKey.push(line)
              } else if (line === "-----END PRIVATE KEY-----") {
                  isPrivateKey = false
                  privateKey.push(line)
              } else if (line === "-----BEGIN CERTIFICATE-----") {
                  isPublicKey = true
                  isPrivateKey = false
                  publicKey.push(line)
              } else if (line === "-----END CERTIFICATE-----") {
                  isPublicKey = false
                  publicKey.push(line)
              } else if (isPrivateKey) {
                  privateKey.push(line)
              } else if (isPublicKey) {
                  publicKey.push(line)
              }
          })
          this.inTls.key = privateKey?? undefined
          this.inTls.certificate = publicKey?? undefined

        } else {
          push.error({
            message: i18n.global.t('error') + ": " + msg.obj
          })
        }
      }
    },
    async genRealityKey(){
      this.loading = true
      const msg = await HttpUtils.get('api/keypairs', { k: "reality" })
      this.loading = false
      if (msg.success) {
        msg.obj.forEach((line:string) => {
          if (this.inTls.reality && this.outTls.reality){
            if (line.startsWith("PrivateKey")){
              this.inTls.reality.private_key = line.substring(12)
            }
            if (line.startsWith("PublicKey")){
              this.outTls.reality.public_key = line.substring(11)
            }
          }
        })
      } else {
        push.error({
          message: i18n.global.t('error') + ": " + msg.obj
        })
      }
    },
    randomSID(){
      this.short_id = RandomUtil.randomShortId().join(',')
    }
  },
  computed: {
    inTls(): iTls {
      return this.tls.server
    },
    outTls(): oTls {
      return this.tls.client
    },
    certText: {
      get(): string { return this.inTls.certificate ? this.inTls.certificate.join('\n') : '' },
      set(v:string) { this.inTls.certificate = v.split('\n') }
    },
    keyText: {
      get(): string { return this.inTls.key ? this.inTls.key.join('\n') : '' },
      set(v:string) { this.inTls.key = v.split('\n') }
    },
    disableSni: {
      get() { return this.outTls.disable_sni ?? false },
      set(v: boolean) { this.tls.client.disable_sni = v ? true : undefined }
    },
    insecure: {
      get() { return this.outTls.insecure ?? false },
      set(v: boolean) { this.tls.client.insecure = v ? true : undefined }
    },
    server_port: {
      get() { return this.inTls.reality?.handshake?.server_port ? this.inTls.reality.handshake.server_port : 443 },
      set(v: any) {
        if (this.inTls.reality){
          this.inTls.reality.handshake.server_port = v.length == 0 || v == 0 ? 443 : parseInt(v)
        }
      }
    },
    short_id: {
      get() { return this.inTls.reality?.short_id ? this.inTls.reality.short_id.join(',') : undefined },
      set(v: string) {
        if (this.inTls.reality){
          this.inTls.reality.short_id = v.length > 0 ? v.split(',') : []
        }
      }
    },
    max_time: {
      get() { return this.inTls?.reality?.max_time_difference ? this.inTls.reality.max_time_difference.replace('m','') : 1 },
      set(v: number) {
        if (this.inTls.reality){
          this.inTls.reality.max_time_difference = v > 0 ? v + 'm' : '1m'
        }
      }
    },
    optionSNI: {
      get(): boolean { return this.inTls.server_name != undefined },
      set(v:boolean) { this.inTls.server_name = v ? '' : undefined }
    },
    optionALPN: {
      get(): boolean { return this.inTls.alpn != undefined },
      set(v:boolean) { this.inTls.alpn = v ? defaultInTls.alpn : undefined }
    },
    optionMinV: {
      get(): boolean { return this.inTls.min_version != undefined },
      set(v:boolean) { this.inTls.min_version = v ? defaultInTls.min_version : undefined }
    },
    optionMaxV: {
      get(): boolean { return this.inTls.max_version != undefined },
      set(v:boolean) { this.inTls.max_version = v ? defaultInTls.max_version : undefined }
    },
    optionCS: {
      get(): boolean { return this.inTls.cipher_suites != undefined },
      set(v:boolean) { this.inTls.cipher_suites = v ? defaultInTls.cipher_suites : undefined }
    },
    optionFP: {
      get(): boolean { return this.outTls.utls != undefined },
      set(v:boolean) { this.outTls.utls = v ? defaultOutTls.utls : undefined }
    },
    optionEch: {
      get(): boolean { return this.outTls.ech != undefined },
      set(v:boolean) { this.outTls.ech = v ? defaultOutTls.ech : undefined }
    },
    optionTime: {
      get(): boolean { return this.inTls?.reality?.max_time_difference != undefined },
      set(v:boolean) { if (this.inTls.reality) this.inTls.reality.max_time_difference = v ? "1m" : undefined }
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.updateData(this.$props.id)
      }
    },
  },
  components: { AcmeVue, EchVue }
}
</script>