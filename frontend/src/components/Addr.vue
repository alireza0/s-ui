<template>
  <v-row>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('out.addr')"
      hide-details
      required
      v-model="addr.server">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4">
      <v-text-field
      :label="$t('out.port')"
      hide-details
      type="number"
      required
      v-model.number="addr.server_port"></v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4" v-if="optionRemark">
      <v-text-field
      :label="$t('in.remark')"
      hide-details
      v-model="addr.remark">
      </v-text-field>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12" sm="6" md="4" v-if="optionTLS">
      <v-switch
      :label="$t('tls.enable')"
      color="primary"
      hide-details
      @update:model-value="updateTls($event)"
      v-model="addr.tls" />
    </v-col>
    <v-col cols="12" sm="6" md="4" v-if="optionSNI">
      <v-text-field
      label="SNI"
      hide-details
      v-model="addr.server_name">
      </v-text-field>
    </v-col>
    <v-col cols="12" sm="6" md="4" v-if="optionInsecure">
      <v-switch
      :label="$t('tls.insecure')"
      hide-details
      color="primary"
      v-model="addr.insecure" />
    </v-col>
  </v-row>
  <v-row>
    <v-spacer></v-spacer>
    <v-col cols="auto" align="end" justify="center">
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('in.mdOption') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionRemark" color="primary" :label="$t('in.remark')" hide-details></v-switch>
            </v-list-item>
            <v-list-item v-if="hasTls">
              <v-switch v-model="optionTLS" color="primary" :label="$t('objects.tls')" hide-details></v-switch>
            </v-list-item>
            <v-list-item v-if="addr.tls">
              <v-switch v-model="optionSNI" color="primary" label="SNI" hide-details></v-switch>
            </v-list-item>
            <v-list-item v-if="addr.tls">
              <v-switch v-model="optionInsecure" color="primary" :label="$t('tls.insecure')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-col>
  </v-row>
</template>

<script lang="ts">
export default {
  props: ['addr', 'hasTls'],
  data() {
    return {
      menu: false
    }
  },
  computed: {
    optionTLS: {
      get(): boolean { return this.$props.addr.tls != undefined },
      set(v:boolean) { this.$props.addr.tls = v ? true : undefined; this.updateTls(v) }
    },
    optionSNI: {
      get(): boolean { return this.$props.addr.server_name != undefined },
      set(v:boolean) { this.$props.addr.server_name = v ? '' : undefined }
    },
    optionRemark: {
      get(): boolean { return this.$props.addr.remark != undefined },
      set(v:boolean) { this.$props.addr.remark = v ? '' : undefined }
    },
    optionInsecure: {
      get(): boolean { return this.$props.addr.insecure != undefined },
      set(v:boolean) { this.$props.addr.insecure = v ? false : undefined }
    }
  },
  methods: {
    updateTls(v:boolean) {
      if (!v) {
        delete this.$props.addr.insecure
        delete this.$props.addr.server_name
      }
    }
  }
}
</script>