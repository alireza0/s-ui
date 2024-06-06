<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="1200" :loading="loading">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ (logType == 's-ui'? "S-UI" : "Sing-Box") + " logs" }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
            hide-details
            label="Level"
            :items="logLevels"
            v-model="logLevel"
            @update:model-value="loadData">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
            hide-details
            label="Count"
            :items="[10,20,30,50,100]"
            v-model.number="logCount"
            @update:model-value="loadData">
            </v-select>
          </v-col>
          <v-col cols="auto" align="center" justify="center">
            <v-btn
              icon="mdi-refresh"
              variant="tonal"
              :loading="loading"
              @click="loadData">
              <v-icon  />
            </v-btn>
          </v-col>
        </v-row>
        <v-card style="background-color: background" dir="ltr" v-html="lines.join('<br />')"></v-card>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="blue-darken-1"
          variant="outlined"
          @click="$emit('close')"
        >
          {{ $t('actions.close') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil';

export default {
  props: ['logType', 'visible'],
  data() {
    return {
      loading: false,
      lines: [],
      logLevel: 'info',
      logLevels: [
        { title: 'DEBUG', value: 'debug' },
        { title: 'INFO', value: 'info' },
        { title: 'WARNING', value: 'warn' },
        { title: 'ERROR', value: 'error' },
      ],
      logCount: 10,
    }
  },
  methods: {
    async loadData() {
      this.loading = true
      const data = await HttpUtils.get('api/logs',{ s: this.$props.logType, c: this.logCount, l: this.logLevel })
      if (data.success) {
        this.lines = data.obj?? []
        this.loading = false
      }
    }
  },
  watch: {
    visible(newValue) {
      this.lines = []
      this.logLevel = 'info'
      this.logCount = 10
      if (newValue) {
        this.loadData()
      }
    },
  },
}
</script>