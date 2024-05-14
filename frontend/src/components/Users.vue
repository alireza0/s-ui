<template>
  <v-card :subtitle="$t('pages.clients')">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-switch 
          v-model="hasUser"
          @change="() => {inbound.users = hasUser? [] : undefined}"
          color="primary"
          :label="$t('in.clients')"
          hide-details></v-switch>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['inbound', 'id'],
  data() {
    return {
      hasUser: false,
    }
  },
  computed: {
    cardTitle() {
      this.hasUser = Object.hasOwn(this.$props.inbound,'users')
      return this.$props.inbound?.type.toUpperCase()
    },
  },
  mounted() {
    this.hasUser = Object.hasOwn(this.$props.inbound,'users')
  }
}
</script>