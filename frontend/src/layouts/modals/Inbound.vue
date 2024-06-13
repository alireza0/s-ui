<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.inbound') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
            hide-details
            :label="$t('type')"
            :items="Object.keys(inTypes).map((key,index) => ({title: key, value: Object.values(inTypes)[index]}))"
            v-model="inbound.type"
            @update:modelValue="changeType">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="inbound.tag" :label="$t('objects.tag')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <Listen :inbound="inbound" :inTags="inTags" />
        <Direct v-if="inbound.type == inTypes.Direct" direction="in" :data="inbound" />
        <Shadowsocks v-if="inbound.type == inTypes.Shadowsocks" direction="in" :data="inbound" />
        <Hysteria v-if="inbound.type == inTypes.Hysteria" direction="in" :data="inbound" />
        <Hysteria2 v-if="inbound.type == inTypes.Hysteria2" direction="in" :data="inbound" />
        <Naive v-if="inbound.type == inTypes.Naive" :inbound="inbound" />
        <ShadowTls v-if="inbound.type == inTypes.ShadowTLS" direction="in" :data="inbound" :outTags="outTags" />
        <Tuic v-if="inbound.type == inTypes.TUIC" direction="in" :data="inbound" />
        <TProxy v-if="inbound.type == inTypes.TProxy" :inbound="inbound" />
        <Transport v-if="Object.hasOwn(inbound,'transport')" :data="inbound" />
        <Users v-if="HasOptionalUser.includes(inbound.type)" :inbound="inbound" :id="id" />
        <InTls v-if="Object.hasOwn(inbound,'tls')"  :inbound="inbound" :tlsConfigs="tlsConfigs" :tls_id="tls_id" />
        <Multiplex v-if="Object.hasOwn(inbound,'multiplex')" direction="in" :data="inbound" />
        <v-switch v-model="inboundStats" color="primary" :label="$t('stats.enable')" hide-details></v-switch>
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
import { InTypes, createInbound } from '@/types/inbounds'
import Listen from '@/components/Listen.vue'
import Direct from '@/components/protocols/Direct.vue'
import Users from '@/components/Users.vue'
import Shadowsocks from '@/components/protocols/Shadowsocks.vue'
import Hysteria from '@/components/protocols/Hysteria.vue'
import Hysteria2 from '@/components/protocols/Hysteria2.vue'
import Naive from '@/components/protocols/Naive.vue'
import ShadowTls from '@/components/protocols/ShadowTls.vue'
import Tuic from '@/components/protocols/Tuic.vue'
import InTls from '@/components/InTLS.vue'
import TProxy from '@/components/protocols/TProxy.vue'
import RandomUtil from '@/plugins/randomUtil'
import Multiplex from '@/components/Multiplex.vue'
import Transport from '@/components/Transport.vue'
export default {
  props: ['visible', 'data', 'id', 'stats', 'inTags', 'outTags', 'tlsConfigs'],
  emits: ['close', 'save'],
  data() {
    return {
      inbound: createInbound("direct",{ "tag": "" }),
      title: "add",
      loading: false,
      inTypes: InTypes,
      inboundStats: false,
      tls_id: { value: 0 },
      HasOptionalUser: [InTypes.Mixed,InTypes.SOCKS,InTypes.HTTP,InTypes.Shadowsocks],
    }
  },
  methods: {
    updateData() {
      if (this.$props.id != -1) {
        const newData = JSON.parse(this.$props.data)
        this.inbound = createInbound(newData.type, newData)
        this.tls_id.value = this.$props.tlsConfigs?.findLast((t:any) => t.inbounds?.includes(this.inbound.tag))?.id?? 0
        this.title = "edit"
      }
      else {
        const port = RandomUtil.randomIntRange(10000, 60000)
        this.inbound = createInbound("direct",{ tag: "direct-"+port ,listen: "::", listen_port: port })
        this.tls_id.value = 0
        this.title = "add"
      }
      this.inboundStats = this.$props.stats
    },
    changeType() {
      // Tag change only in add outbound
      const tag = this.$props.id != -1 ? this.inbound.tag : this.inbound.type + "-" + this.inbound.listen_port
      // Use previous data
      const prevConfig = { tag: tag ,listen: this.inbound.listen, listen_port: this.inbound.listen_port }
      this.inbound = createInbound(this.inbound.type, prevConfig)
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.inbound, this.inboundStats, this.tls_id.value)
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
  components: { Listen, InTls, Hysteria2, Naive, Direct, Shadowsocks, Users, Hysteria, ShadowTls, TProxy, Multiplex, Tuic, Transport }
}
</script>