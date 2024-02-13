<template>
  <v-card subtitle="Hysteria">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Uplink Limit"
        hide-details
        type="number"
        suffix="Mbps"
        v-model.number="up_mbps">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
        label="Downlink Limit"
        hide-details
        type="number"
        suffix="Mbps"
        min="0"
        v-model.number="down_mbps">
        </v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
       <v-text-field
        label="obfs Password"
        hide-details
        v-model="inbound.obfs">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['inbound'],
  data() {
    return {
    }
  },
  computed: {
    down_mbps: {
      get() { return this.$props.inbound.down_mbps ? this.$props.inbound.down_mbps : 0 },
      set(newValue:any) {
        if (newValue.length != 0 ){
          this.$props.inbound.down_mbps = newValue
          this.$props.inbound.down = "" + newValue + " Mbps"
        } else {
          this.$props.inbound.down_mbps = 0
          this.$props.inbound.down = "0 Mbps"
        }
      }
    },
    up_mbps: {
      get() { return this.$props.inbound.up_mbps ? this.$props.inbound.up_mbps : 0 },
      set(newValue:number) { this.$props.inbound.up_mbps = newValue > 0 ? newValue : 0 }
    },
  },
}
</script>