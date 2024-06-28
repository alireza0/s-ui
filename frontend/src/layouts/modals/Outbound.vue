<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.outbound') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
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
        <Direct v-if="outbound.type == outTypes.Direct" direction="out" :data="outbound" />
        <Socks v-if="outbound.type == outTypes.SOCKS" :data="outbound" />
        <Http v-if="outbound.type == outTypes.HTTP" :data="outbound" />
        <Shadowsocks v-if="outbound.type == outTypes.Shadowsocks" direction="out" :data="outbound" />
        <Vmess v-if="outbound.type == outTypes.VMess" :data="outbound" />
        <Trojan v-if="outbound.type == outTypes.Trojan" :data="outbound" />
        <Wireguard v-if="outbound.type == outTypes.Wireguard" :data="outbound" />
        <Hysteria v-if="outbound.type == outTypes.Hysteria" direction="out" :data="outbound" />
        <ShadowTls v-if="outbound.type == outTypes.ShadowTLS" :data="outbound" />
        <Vless v-if="outbound.type == outTypes.VLESS" :data="outbound" />
        <Tuic v-if="outbound.type == outTypes.TUIC" direction="out" :data="outbound" />
        <Hysteria2 v-if="outbound.type == outTypes.Hysteria2" direction="out" :data="outbound" />
        <Tor v-if="outbound.type == outTypes.Tor" :data="outbound" />
        <Ssh v-if="outbound.type == outTypes.SSH" :data="outbound" />
        <Selector v-if="outbound.type == outTypes.Selector" :data="outbound" :tags="tags" />
        <UrlTest v-if="outbound.type == outTypes.URLTest" :data="outbound" :tags="tags" />

        <Transport v-if="Object.hasOwn(outbound,'transport')" :data="outbound" />
        <OutTLS v-if="Object.hasOwn(outbound,'tls')" :outbound="outbound" />
        <Multiplex v-if="Object.hasOwn(outbound,'multiplex')" direction="out" :data="outbound" />
        <Dial v-if="!NoDial.includes(outbound.type)" :dial="outbound" :outTags="tags" />
        <v-switch v-model="outboundStats" color="primary" :label="$t('stats.enable')" hide-details></v-switch>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="blue-darken-1"
          variant="text"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="blue-darken-1"
          variant="text"
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
export default {
  props: ['visible', 'data', 'id', 'stats', 'tags'],
  emits: ['close', 'save'],
  data() {
    return {
      outbound: createOutbound("direct",{ "tag": "" }),
      title: "add",
      loading: false,
      outTypes: OutTypes,
      outboundStats: false,
      NoDial: [OutTypes.Block, OutTypes.DNS, OutTypes.Selector, OutTypes.URLTest],
      NoServer: [OutTypes.Direct, OutTypes.Block, OutTypes.DNS, OutTypes.Selector, OutTypes.URLTest, OutTypes.Tor],
    }
  },
  methods: {
    updateData() {
      if (this.$props.id != -1) {
        const newData = JSON.parse(this.$props.data)
        this.outbound = createOutbound(newData.type, newData)
        this.title = "edit"
      }
      else {
        this.outbound = createOutbound("direct",{ tag: "direct-" + RandomUtil.randomSeq(3) })
        this.title = "add"
      }
      this.outboundStats = this.$props.stats
    },
    changeType() {
      // Tag change only in add outbound
      const tag = this.$props.id != -1 ? this.outbound.tag : this.outbound.type + "-" + RandomUtil.randomSeq(3)
      // Use previous data
      const prevConfig = { tag: tag ,listen: this.outbound.listen, listen_port: this.outbound.listen_port }
      this.outbound = createOutbound(this.outbound.type, prevConfig)
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.outbound, this.outboundStats)
      this.loading = false
    },
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.updateData()
      }
    },
  },
  components: { Dial, Multiplex, Transport, OutTLS,
    Direct, Socks, Http, Shadowsocks, Vmess, Trojan,
    Wireguard, Hysteria, ShadowTls, Vless, Tuic,
    Hysteria2, Tor, Ssh, Selector, UrlTest }
}
</script>