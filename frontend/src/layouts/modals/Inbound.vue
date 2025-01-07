<template>
  <v-dialog transition="dialog-bottom-transition" width="800" @after-enter="updateData">
    <v-card class="rounded-lg" :loading="loading">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.inbound') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-skeleton-loader
          class="mx-auto border"
          width="95%"
          type="card, text, divider, list-item-two-line"
          v-if="loading"
        ></v-skeleton-loader>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;" :hidden="loading">
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
          <v-tabs
            v-if="HasInData.includes(inbound.type)"
            v-model="side"
            density="compact"
            fixed-tabs
            align-tabs="center"
          >
            <v-tab value="s">{{ $t('in.sSide') }}</v-tab>
            <v-tab value="c">{{ $t('in.cSide') }}</v-tab>
          </v-tabs>
          <v-window v-model="side" style="margin-top: 10px;">
            <v-window-item value="s">
              <Listen :inbound="inbound" :inTags="inTags" />
              <Direct v-if="inbound.type == inTypes.Direct" :data="inbound" />
              <Shadowsocks v-if="inbound.type == inTypes.Shadowsocks" direction="in" :data="inbound" />
              <Hysteria v-if="inbound.type == inTypes.Hysteria" direction="in" :data="inbound" />
              <Hysteria2 v-if="inbound.type == inTypes.Hysteria2" direction="in" :data="inbound" />
              <Naive v-if="inbound.type == inTypes.Naive" :inbound="inbound" />
              <ShadowTls v-if="inbound.type == inTypes.ShadowTLS" direction="in" :data="inbound" :outTags="outTags" />
              <Tuic v-if="inbound.type == inTypes.TUIC" direction="in" :data="inbound" />
              <Tun v-if="inbound.type == inTypes.Tun" :data="inbound" />
              <TProxy v-if="inbound.type == inTypes.TProxy" :inbound="inbound" />
              <Transport v-if="Object.hasOwn(inbound,'transport')" :data="inbound" />
              <Users v-if="HasOptionalUser.includes(inbound.type)" :inbound="inbound" />
              <InTls v-if="HasTls.includes(inbound.type)"  :inbound="inbound" :tlsConfigs="tlsConfigs" :tls_id="inbound.tls_id" />
              <Multiplex v-if="Object.hasOwn(inbound,'multiplex')" direction="in" :data="inbound" />
            </v-window-item>
            <v-window-item value="c">
              <OutJsonVue :inData="inbound" :type="inbound.type" />
              <Multiplex v-if="Object.hasOwn(inbound,'multiplex')" direction="out" :data="inbound.out_json" />
              <v-card>
                <v-card-subtitle>{{ $t('in.multiDomain') }}
                  <v-icon @click="add_addr" icon="mdi-plus"></v-icon>
                </v-card-subtitle>
                <template v-for="addr,index in inbound.addrs">
                  {{ $t('in.addr') }} #{{ (index+1) }} <v-icon icon="mdi-delete" @click="inbound.addrs?.splice(index,1)" />
                  <v-divider></v-divider>
                  <AddrVue :addr="addr" :hasTls="HasTls.includes(inbound.type)" />
                </template>
              </v-card>
            </v-window-item>
          </v-window>
        </v-container>
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
          :disabled="!validate"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { InTypes, createInbound, Addr } from '@/types/inbounds'
import RandomUtil from '@/plugins/randomUtil'

import Listen from '@/components/Listen.vue'
import Direct from '@/components/protocols/Direct.vue'
import Users from '@/components/Users.vue'
import Shadowsocks from '@/components/protocols/Shadowsocks.vue'
import Hysteria from '@/components/protocols/Hysteria.vue'
import Hysteria2 from '@/components/protocols/Hysteria2.vue'
import Naive from '@/components/protocols/Naive.vue'
import ShadowTls from '@/components/protocols/ShadowTls.vue'
import Tuic from '@/components/protocols/Tuic.vue'
import Tun from '@/components/protocols/Tun.vue'
import InTls from '@/components/tls/InTLS.vue'
import TProxy from '@/components/protocols/TProxy.vue'
import Multiplex from '@/components/Multiplex.vue'
import Transport from '@/components/Transport.vue'
import AddrVue from '@/components/Addr.vue'
import OutJsonVue from '@/components/OutJson.vue'
import Data from '@/store/modules/data'
export default {
  props: ['visible', 'id', 'inTags', 'outTags', 'tlsConfigs'],
  emits: ['close', 'save'],
  data() {
    return {
      inbound: createInbound("direct",{ id:0, "tag": "" }),
      title: "add",
      loading: false,
      side: "s",
      inTypes: InTypes,
      HasOptionalUser: [InTypes.Mixed,InTypes.SOCKS,InTypes.HTTP,InTypes.Shadowsocks],
      HasInData: [
        InTypes.SOCKS,
        InTypes.HTTP,
        InTypes.Mixed,
        InTypes.Shadowsocks,
        InTypes.VMess,
        InTypes.ShadowTLS,
        InTypes.Trojan,
        InTypes.Hysteria,
        InTypes.VLESS,
        InTypes.TUIC,
        InTypes.Hysteria2,
        InTypes.Naive,
      ],
      HasTls: [
        InTypes.HTTP,
        InTypes.VMess,
        InTypes.Trojan,
        InTypes.Naive,
        InTypes.Hysteria,
        InTypes.TUIC,
        InTypes.Hysteria2,
        InTypes.VLESS,
      ],
      OnlyTLS: [InTypes.Hysteria, InTypes.Hysteria2, InTypes.TUIC, InTypes.Naive ],
    }
  },
  methods: {
    async loadData() {
      this.loading = true
      const inboundArray = await Data().loadInbounds([this.$props.id])
      this.inbound = inboundArray[0]
      this.loading = false
    },
    updateData() {
      if (this.$props.id > 0) {
        this.loadData()
        this.title = "edit"
      }
      else {
        const port = RandomUtil.randomIntRange(10000, 60000)
        this.inbound = createInbound("direct",{ id: 0, tag: "direct-"+port ,listen: "::", listen_port: port })
        if (this.HasInData.includes(this.inbound.type)){
          this.inbound.addrs = []
          this.inbound.out_json = {}
        } else {
          delete this.inbound.addrs
          delete this.inbound.out_json
        }
        this.title = "add"
        this.loading = false
      }
      this.side = "s"
    },
    changeType() {
      if (!this.inbound.listen_port) this.inbound.listen_port = RandomUtil.randomIntRange(10000, 60000)
      // Tag change only in add inbound
      const tag = this.$props.id > 0 ? this.inbound.tag : this.inbound.type + "-" + this.inbound.listen_port
      // Use previous data
      const prevConfig = { id: this.inbound.id, tag: tag ,listen: this.inbound.listen?? "::", listen_port: this.inbound.listen_port }
      this.inbound = createInbound(this.inbound.type, this.inbound.type != this.inTypes.Tun ? prevConfig : { tag: tag })
      if (this.HasInData.includes(this.inbound.type)){
        this.inbound.addrs = []
        this.inbound.out_json = {}
      } else {
        delete this.inbound.addrs
        delete this.inbound.out_json
      }
      this.side = "s"
    },
    add_addr() {
      this.inbound.addrs?.push(<Addr>{ server: location.hostname, server_port: this.inbound.listen_port })
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.inbound)
      this.loading = false
    },
  },
  computed: {
    validate() {
      if (this.inbound == undefined) return false
      if (this.inbound.tag == "") return false
      if (this.inbound.listen_port > 65535 || this.inbound.listen_port < 1) return false
      if (this.OnlyTLS.includes(this.inbound.type) && this.inbound.tls_id == 0) return false
      return true
    },
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.loading = true
      }
    },
  },
  components: {
    Listen, InTls, Hysteria2, Naive, Direct, Shadowsocks,
    Users, Hysteria, ShadowTls, TProxy, Multiplex, Tuic, Tun, Transport,
    AddrVue, OutJsonVue
  }
}
</script>