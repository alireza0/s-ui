<template>
  <v-card :subtitle="$t('objects.listen')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('in.addr')"
        hide-details
        required
        v-model="data.listen">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        :label="$t('in.port')"
        hide-details
        type="number"
        min="1"
        max="65535"
        required
        v-model.number="data.listen_port"></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="optionDetour">
        <v-select
        :label="$t('listen.detourText')"
        hide-details
        :items="inTags"
        v-model="data.detour">
        </v-select>
      </v-col>
    </v-row>
    <v-row v-if="optionTCP">
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.tcp_fast_open" color="primary" label="TCP Fast Open" hide-details></v-switch>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.tcp_multi_path" color="primary" label="TCP Multi Path" hide-details></v-switch>
      </v-col>
    </v-row>
    <v-row v-if="optionUDP">
      <v-col cols="12" sm="6" md="4">
        <v-switch v-model="data.udp_fragment" color="primary" label="UDP Fragment" hide-details></v-switch>
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
    <v-card-actions class="pt-0">
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
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
export default {
  props: ['data', 'inTags'],
  data() {
    return {
      menu: false
    }
  },
  computed: {
    udpTimeout: {
      get() { return this.$props.data.udp_timeout ? parseInt(this.$props.data.udp_timeout.replace('m','')) : 5 },
      set(newValue:number) { this.$props.data.udp_timeout = newValue > 0 ? newValue + 'm' : '5m' }
    },
    optionTCP: {
      get(): boolean { 
        return this.$props.data.tcp_fast_open != undefined && 
               this.$props.data.tcp_multi_path != undefined
      },
      set(v:boolean) {
        this.$props.data.tcp_fast_open = v ? false : undefined
        this.$props.data.tcp_multi_path = v ? false : undefined
      }
    },
    optionUDP: {
      get(): boolean { 
        return this.$props.data.udp_fragment != undefined &&
               this.$props.data.udp_timeout != undefined
      },
      set(v:boolean) {
        this.$props.data.udp_fragment = v ? false : undefined
        this.$props.data.udp_timeout = v ? '5m' : undefined 
      }
    },
    optionDetour: {
      get(): boolean { return this.$props.data.detour != undefined },
      set(v:boolean) { this.$props.data.detour = v ? this.inTags[0]?? '' : undefined }
    }
  }
}
</script>