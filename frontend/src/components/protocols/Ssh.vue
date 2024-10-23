<template>
  <v-card subtitle="SSH">
    <template v-if="optionKey">
        <v-row>
          <v-col cols="auto">
            <v-btn-toggle v-model="usePath"
            class="rounded-xl"
            density="compact"
            variant="outlined"
            shaped
            mandatory>
              <v-btn
                @click="data.private_key=undefined; data.private_key_path=''"
              >{{ $t('tls.usePath') }}</v-btn>
              <v-btn
                @click="data.private_key_path=undefined; data.private_key=''"
              >{{ $t('tls.useText') }}</v-btn>
            </v-btn-toggle>
          </v-col>
        </v-row>
        <v-row v-if="usePath == 0">
          <v-col cols="12" sm="6">
            <v-text-field
              :label="$t('tls.keyPath')"
              hide-details
              v-model="data.private_key_path">
            </v-text-field>
          </v-col>
        </v-row>
        <v-row v-else>
          <v-col cols="12" sm="6">
            <v-textarea
              :label="$t('tls.key')"
              hide-details
              v-model="data.private_key">
            </v-textarea>
          </v-col>
        </v-row>
        <v-row>
          <v-col cols="12" sm="6">
            <v-text-field
              :label="$t('types.ssh.passphrase')"
              hide-details
              v-model="data.private_key_passphrase">
            </v-text-field>
          </v-col>
        </v-row>
      </template>
      <template v-else>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="data.user" :label="$t('types.un')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="data.password" :label="$t('types.pw')" hide-details></v-text-field>
          </v-col>
        </v-row>
      </template>
    <v-row v-if="optionHostKey">
      <v-col cols="12" sm="6">
        <v-textarea
          :label="$t('types.ssh.hostKey')"
          hide-details
          v-model="host_key">
        </v-textarea>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4" v-if="data.host_key_algorithms != undefined">
        <v-text-field v-model="algorithms" :label="$t('types.ssh.algorithm') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="data.client_version != undefined">
        <v-text-field v-model="data.client_version" :label="$t('types.ssh.clientVer')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
          <template v-slot:activator="{ props }">
            <v-btn v-bind="props" hide-details variant="tonal">{{ $t('types.ssh.options') }}</v-btn>
          </template>
          <v-card>
            <v-list>
              <v-list-item>
                <v-switch v-model="optionKey" color="primary" label="SSH Key" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionHostKey" color="primary" :label="$t('types.ssh.hostKey')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionAlgorithms" color="primary" :label="$t('types.ssh.algorithm')" hide-details></v-switch>
              </v-list-item>
              <v-list-item>
                <v-switch v-model="optionVer" color="primary" :label="$t('types.ssh.clientVer')" hide-details></v-switch>
              </v-list-item>
            </v-list>
          </v-card>
        </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">

export default {
  props: ['data'],
  data() {
    return {
      menu: false,
      usePath: 0,
    }
  },
  computed: {
    optionKey: {
      get(): boolean { return this.data.private_key != undefined || this.data.private_key_path != undefined },
      set(v:boolean) {
        this.usePath = 0
        if (v) {
          this.$props.data.private_key_path = ""
          delete this.$props.data.user
          delete this.$props.data.password
        } else {
          delete this.$props.data.private_key_path
          delete this.$props.data.private_key
          delete this.$props.data.private_key_passphrase
        }
      }
    },
    optionHostKey: {
      get(): boolean { return this.data.host_key != undefined },
      set(v:boolean) { this.data.host_key = v ? '' : undefined }
    },
    optionAlgorithms: {
      get(): boolean { return this.data.host_key_algorithms != undefined },
      set(v:boolean) { this.data.host_key_algorithms = v ? [] : undefined }
    },
    optionVer: {
      get(): boolean { return this.data.client_version != undefined },
      set(v:boolean) { this.data.client_version = v ? 'SSH-2.0-OpenSSH_7.4p1' : undefined }
    },
    host_key: {
      get(): string { return this.$props.data.host_key ? this.$props.data.host_key.join('\n') : '' },
      set(v:string) { this.$props.data.host_key = v.split('\n') }
    },
    algorithms: {
      get() { return this.$props.data.host_key_algorithms ? this.$props.data.host_key_algorithms.join(',') : '' },
      set(v:string) { this.$props.data.host_key_algorithms = v.length > 0 ? v.split(',') : undefined }
    },
  },
}
</script>