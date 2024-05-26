<template>
  <v-dialog transition="dialog-bottom-transition" width="400">
    <v-card class="rounded-lg" id="qrcode-modal">
      <v-card-title>
        <v-row>
          <v-col>QrCode</v-col>
          <v-spacer></v-spacer>
          <v-col cols="1"><v-icon icon="mdi-close-box" @click="$emit('close')" ></v-icon></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col style="text-align: center;" @click="copyToClipboard(clientSub)">
            <v-chip>{{ $t('setting.sub') }}</v-chip>
            <QrcodeVue :value="clientSub" :size="300" :margin="1" style="border-radius: 1rem;" />
          </v-col>
        </v-row>
        <v-row v-for="l in clientLinks">
          <v-col style="text-align: center;" @click="copyToClipboard(l.uri)">
            <v-chip>{{ l.remark }}</v-chip><br />
            <QrcodeVue :value="l.uri" :size="300" :margin="1" style="border-radius: 1rem;" />
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import QrcodeVue from 'qrcode.vue'
import Data from '@/store/modules/data'
import Clipboard from 'clipboard'
import Message from '@/store/modules/message'
import { i18n } from '@/locales'

export default {
  props: ['index'],
  data() {
    return {
      msg: Message(),
    }
  },
  methods: {
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
        this.msg.showMessage(i18n.global.t('copyToClipboard') + " : " + i18n.global.t('success'),'success',5000)
      })

      clipboard.on('error', () => {
        clipboard.destroy()
        this.msg.showMessage(i18n.global.t('copyToClipboard') + " : " + i18n.global.t('failed'),'error',5000)
      })

      // Perform click on hidden button to trigger copy
      hiddenButton.click()
      document.body.removeChild(hiddenButton)
    }
  },
  computed: {
    clients() { return Data().clients },
    client() {
      if ( typeof this.$props.index != 'number' ) return <any>{}
      return this.clients[this.$props.index]
    },
    clientSub() {
      return Data().subURI + this.client.name
    },
    clientLinks() {
      return JSON.parse(this.client.links?? "[]")
    }
  },
  components: { QrcodeVue }
}
</script>