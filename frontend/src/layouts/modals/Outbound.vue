<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.outbound') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">
          <v-tabs
            v-model="tab"
            align-tabs="center"
          >
            <v-tab value="t1">{{ $t('client.basics') }}</v-tab>
            <v-tab value="t2">{{ $t('client.external') }}</v-tab>
          </v-tabs>
          <v-window v-model="tab">
            <v-window-item value="t1">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                  hide-details
                  :label="$t('type')"
                  :items="Object.keys(outTypes).map((key,index) => ({title: key, value: Object.values(outTypes)[index]}))"
                  v-model="outbound.type"
                  @update:modelValue="changeType">
                  </v-select>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="outbound.tag" :label="$t('objects.tag')" hide-details></v-text-field>
                </v-col>
              </v-row>
              <v-row v-if="!NoServer.includes(outbound.type)">
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                  :label="$t('out.addr')"
                  hide-details
                  v-model="outbound.server">
                  </v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field
                  :label="$t('out.port')"
                  type="number"
                  min="0"
                  hide-details
                  v-model.number="outbound.server_port">
                  </v-text-field>
                </v-col>
              </v-row>
              <Socks v-if="outbound.type == outTypes.SOCKS" :data="outbound" />
              <Http v-if="outbound.type == outTypes.HTTP" :data="outbound" />
              <Shadowsocks v-if="outbound.type == outTypes.Shadowsocks" direction="out" :data="outbound" />
              <Vmess v-if="outbound.type == outTypes.VMess" :data="outbound" />
              <Trojan v-if="outbound.type == outTypes.Trojan" :data="outbound" />
              <Hysteria v-if="outbound.type == outTypes.Hysteria" direction="out" :data="outbound" />
              <ShadowTls v-if="outbound.type == outTypes.ShadowTLS" :data="outbound" />
              <Vless v-if="outbound.type == outTypes.VLESS" :data="outbound" />
              <Tuic v-if="outbound.type == outTypes.TUIC" direction="out" :data="outbound" />
              <Hysteria2 v-if="outbound.type == outTypes.Hysteria2" direction="out" :data="outbound" />
              <AnyTls v-if="outbound.type == outTypes.AnyTls" :data="outbound" direction="out" />
              <Tor v-if="outbound.type == outTypes.Tor" :data="outbound" />
              <Ssh v-if="outbound.type == outTypes.SSH" :data="outbound" />
              <Selector v-if="outbound.type == outTypes.Selector" :data="outbound" :tags="tags" />
              <UrlTest v-if="outbound.type == outTypes.URLTest" :data="outbound" :tags="tags" />

              <Transport v-if="Object.hasOwn(outbound,'transport')" :data="outbound" />
              <OutTLS v-if="Object.hasOwn(outbound,'tls')" :outbound="outbound" />
              <Multiplex v-if="Object.hasOwn(outbound,'multiplex')" direction="out" :data="outbound" />
              <Dial v-if="!NoDial.includes(outbound.type)" :dial="outbound" />
            </v-window-item>
            <v-window-item value="t2">
              <v-row>
                <v-col cols="12">
                  <v-text-field v-model="link" :label="$t('client.external')" hide-details />
                </v-col>
                <v-col cols="12" align="center">
                  <v-btn hide-details variant="tonal" :loading="loading" @click="linkConvert">{{ $t('submit') }}</v-btn>
                </v-col>
              </v-row>
            </v-window-item>
          </v-window>
        </v-container>
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
import { OutTypes, createOutbound } from '@/types/outbounds'
import RandomUtil from '@/plugins/randomUtil'
import Dial from '@/components/Dial.vue'
import Multiplex from '@/components/Multiplex.vue'
import Transport from '@/components/Transport.vue'
import OutTLS from '@/components/tls/OutTLS.vue'
import Direct from '@/components/protocols/Direct.vue'
import Socks from '@/components/protocols/Socks.vue'
import Http from '@/components/protocols/Http.vue'
import Shadowsocks from '@/components/protocols/Shadowsocks.vue'
import Vmess from '@/components/protocols/Vmess.vue'
import Trojan from '@/components/protocols/Trojan.vue'
import Wireguard from '@/components/protocols/Wireguard.vue'
import Hysteria from '@/components/protocols/Hysteria.vue'
import ShadowTls from '@/components/protocols/OutShadowTls.vue'
import Vless from '@/components/protocols/Vless.vue'
import Tuic from '@/components/protocols/Tuic.vue'
import Hysteria2 from '@/components/protocols/Hysteria2.vue'
import Tor from '@/components/protocols/Tor.vue'
import Ssh from '@/components/protocols/Ssh.vue'
import Selector from '@/components/protocols/Selector.vue'
import UrlTest from '@/components/protocols/UrlTest.vue'
import HttpUtils from '@/plugins/httputil'
import AnyTls from '@/components/protocols/AnyTls.vue'
import Data from '@/store/modules/data'
export default {
  props: ['visible', 'data', 'id', 'tags'],
  emits: ['close'],
  data() {
    return {
      outbound: createOutbound("direct",{ "tag": "" }),
      title: "add",
      tab: "t1",
      link: "",
      loading: false,
      outTypes: OutTypes,
      NoDial: [OutTypes.Selector, OutTypes.URLTest],
      NoServer: [OutTypes.Direct, OutTypes.Selector, OutTypes.URLTest, OutTypes.Tor],
    }
  },
  methods: {
    updateData(id: number) {
      if (id > 0) {
        const newData = JSON.parse(this.$props.data)
        this.outbound = createOutbound(newData.type, newData)
        this.title = "edit"
      }
      else {
        this.outbound = createOutbound("direct",{ tag: "direct-" + RandomUtil.randomSeq(3) })
        this.title = "add"
      }
      this.tab = "t1"
    },
    changeType() {
      // Tag change only in add outbound
      const tag = this.$props.id > 0 ? this.outbound.tag : this.outbound.type + "-" + RandomUtil.randomSeq(3)
      // Use previous data
      const prevConfig = { id: this.outbound.id, tag: tag, listen: this.outbound.listen, listen_port: this.outbound.listen_port }
      this.outbound = createOutbound(this.outbound.type, prevConfig)
    },
    closeModal() {
      this.updateData(0) // reset
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.$props.visible) return
      // check duplicate tag
      const isDuplicatedTag = Data().checkTag("outbound",this.outbound.id, this.outbound.tag)
      if (isDuplicatedTag) return

      // save data
      this.loading = true
      const success = await Data().save("outbounds", this.$props.id == 0 ? "new" : "edit", this.outbound)
      if (success) this.closeModal()
      this.loading = false
    },
    async linkConvert() {
      if (this.link.length>0){
        this.loading = true
        const msg = await HttpUtils.post('api/linkConvert', { link: this.link })
        this.loading = false
        if (msg.success) {
          this.outbound = msg.obj
          this.tab = "t1"
          this.link = ""
        }
      }
    }
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.updateData(this.$props.id)
      }
    },
  },
  components: { Dial, Multiplex, Transport, OutTLS,
    Direct, Socks, Http, Shadowsocks, Vmess, Trojan,
    Wireguard, Hysteria, ShadowTls, Vless, Tuic,
    Hysteria2, AnyTls, Tor, Ssh, Selector, UrlTest }
}
</script>