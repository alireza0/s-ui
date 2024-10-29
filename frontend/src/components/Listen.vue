<template>
  <v-card :subtitle="$t('objects.listen')">
    <v-row v-if="inbound.type != 'tun'">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('in.addr')"
        hide-details
        required
        v-model="inbound.listen">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('in.port')"
        hide-details
        type="number"
        required
        v-model.number="inbound.listen_port"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionDetour">
        <v-select
        :label="$t('listen.detourText')"
        hide-details
        :items="inTags"
        v-model="inbound.detour">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="inbound.sniff" color="primary" :label="$t('listen.sniffing')" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="inbound.sniff">
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="inbound.sniff_override_destination" color="primary" :label="$t('listen.sniffingOverride')" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('listen.sniffingTimeout')"
        hide-details
        type="number"
        min="50"
        step="50"
        :suffix="$t('date.ms')"
        v-model.number="sniffTimeout"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionTCP">
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="inbound.tcp_fast_open" color="primary" label="TCP Fast Open" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="inbound.tcp_multi_path" color="primary" label="TCP Multi Path" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="optionUDP">
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="inbound.udp_fragment" color="primary" label="UDP Fragment" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="UDP NAT expiration"
        hide-details
        type="number"
        min="1"
        :suffix="$t('date.m')"
        v-model.number="udpTimeout"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionDS">
      <v-col cols="12" sm="6" md="4">
        <v-select
            hide-details
            :label="$t('listen.domainStrategy')"
            :items="['prefer_ipv4','prefer_ipv6','ipv4_only','ipv6_only']"
            v-model="inbound.domain_strategy">
          </v-select>
      </v-col>
    </v-row>
    <v-card-actions class="pt-0" v-if="inbound.type != 'tun'">
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('listen.options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionDetour" color="primary" :label="$t('listen.detour')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionTCP" color="primary" :label="$t('listen.tcpOptions')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionUDP" color="primary" :label="$t('listen.udpOptions')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionDS" color="primary" :label="$t('listen.domainStrategy')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
export default {
  props: ['inbound', 'inTags'],
  data() {
    return {
      menu: false
    }
  },
  computed: {
    udpTimeout: {
      get() { return this.$props.inbound.udp_timeout ? parseInt(this.$props.inbound.udp_timeout.replace('m','')) : 5 },
      set(newValue:number) { this.$props.inbound.udp_timeout = newValue > 0 ? newValue + 'm' : '5m' }
    },
    sniffTimeout: {
      get() { return this.$props.inbound.sniff_timeout ? parseInt(this.$props.inbound.sniff_timeout.replace('ms','')) : 300 },
      set(newValue:number) { this.$props.inbound.sniff_timeout = newValue > 0 ? newValue + 'ms' : '300ms' }
    },
    optionTCP: {
      get(): boolean { 
        return this.$props.inbound.tcp_fast_open != undefined && 
               this.$props.inbound.tcp_multi_path != undefined
      },
      set(v:boolean) {
        this.$props.inbound.tcp_fast_open = v ? false : undefined
        this.$props.inbound.tcp_multi_path = v ? false : undefined
      }
    },
    optionUDP: {
      get(): boolean { 
        return this.$props.inbound.udp_fragment != undefined &&
               this.$props.inbound.udp_timeout != undefined
      },
      set(v:boolean) {
        this.$props.inbound.udp_fragment = v ? false : undefined
        this.$props.inbound.udp_timeout = v ? '5m' : undefined 
      }
    },
    optionDetour: {
      get(): boolean { return this.$props.inbound.detour != undefined },
      set(v:boolean) { this.$props.inbound.detour = v ? this.inTags[0]?? '' : undefined }
    },
    optionDS: {
      get(): boolean { return this.$props.inbound.domain_strategy != undefined },
      set(v:boolean) { this.$props.inbound.domain_strategy = v ? 'prefer_ipv4' : undefined }
    }
  }
}
</script>