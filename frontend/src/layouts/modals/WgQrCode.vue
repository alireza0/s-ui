<template>
  <v-dialog transition="dialog-bottom-transition" width="400">
    <v-card class="rounded-lg" id="qrcode-modal" :loading="loading">
      <v-card-title>
        <v-row>
          <v-col>Wireguard QrCode</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close-box" @click="$emit('close')" /></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-row v-for="l, i in wgLinks">
        <v-col style="text-align: center;" v-if="l.length>0">
          <v-chip>{{ $t('types.wg.peer') + ' ' + (i+1) }}</v-chip><br />
          <QrcodeVue :value="l" :size="size" @click="copyToClipboard(l)" :margin="1" style="border-radius: .5rem; cursor: copy;" />
        </v-col>
      </v-row>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import QrcodeVue from 'qrcode.vue'
import Clipboard from 'clipboard'
import { i18n } from '@/locales'
import { push } from 'notivue'

export default {
  props: ['data', 'visible'],
  data() {
    return {
      wgData: <any>{},
      wgLinks: <string[]>[],
      loading: false,
    }
  },
  methods: {
    async load() {
      this.wgData = this.$props.data
      this.wgLinks = []
      const address = location.hostname
      this.wgData.peers.forEach((_: any, index: number) => {
        this.wgLinks.push(this.getWireguardLink(index, address))
      })
    },
    getWireguardLink(peerId: number, address: string) {
      const peerData = this.wgData.peers[peerId]
      const keys = this.wgData.ext?.keys?.find((key: any) => key.public_key == peerData.public_key)
      if (!keys || !this.wgData.ext?.public_key) return ''
      let txt = `[Interface]\n`
      txt += `PrivateKey = ${keys.private_key}\n`
      txt += `Address = ${peerData.allowed_ips.join(',')}\n`
      txt += `DNS = 1.1.1.1, 9.9.9.9\n`
      if (this.wgData.mtu) {
          txt += `MTU = ${this.wgData.mtu}\n`
      }
      txt += `\n# ${this.wgData.tag} - ${peerId}\n`
      txt += `[Peer]\n`
      txt += `PublicKey = ${this.wgData.ext.public_key}\n`
      txt += `AllowedIPs = 0.0.0.0/0, ::/0\n`
      txt += `Endpoint = ${address}:${this.wgData.listen_port}\n`
      if (peerData.pre_shared_key) {
          txt += `\nPresharedKey = ${peerData.pre_shared_key}`
      }
      if (peerData.persistent_keepalive_interval) {
          txt += `\nPersistentKeepalive = ${peerData.persistent_keepalive_interval}\n`
      }
      return txt;
    },
    copyToClipboard(txt:string) {
      const hiddenButton = document.createElement('button')
      hiddenButton.className = 'clipboard-btn'
      document.body.appendChild(hiddenButton)

      const clipboard = new Clipboard('.clipboard-btn', {
        text: () => txt,
        container: document.getElementById('qrcode-modal')?? undefined
      });

      clipboard.on('success', () => {
        clipboard.destroy()
        push.success({
          message: i18n.global.t('success') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        push.error({
          message: i18n.global.t('failed') + ": " + i18n.global.t('copyToClipboard'),
          duration: 5000,
        })
      })

      // Perform click on hidden button to trigger copy
      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    }
  },
  computed: {
    size() {
      if (window.innerWidth > 380) return 300
      if (window.innerWidth > 330) return 280
      return 250
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.load()
      }
    },
  },
  components: { QrcodeVue }
}
</script>