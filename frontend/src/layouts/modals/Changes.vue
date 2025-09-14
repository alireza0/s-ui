<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="800" :loading="loading">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('admin.changes') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close-box" @click="$emit('close')" /></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="12" sm="4" md="3">
            <v-select
            hide-details
            :label="$t('admin.actor')"
            :items="['', 'DepleteJob', ...admins]"
            v-model="user"
            @update:model-value="loadData">
            </v-select>
          </v-col>
          <v-col cols="12" sm="4" md="3">
            <v-select
            hide-details
            :label="$t('admin.key')"
            :items="['', 'inbounds', 'outbounds', 'clients', 'route', 'tls', 'experimental']"
            v-model="key"
            @update:model-value="loadData">
            </v-select>
          </v-col>
          <v-col cols="6" sm="4" md="3">
            <v-select
            hide-details
            :label="$t('count')"
            :items="[10,20,30,50,100]"
            v-model.number="chngCount"
            @update:model-value="loadData">
            </v-select>
          </v-col>
          <v-col cols="auto" align="center" justify="center">
            <v-btn
              icon="mdi-refresh"
              variant="tonal"
              :loading="loading"
              @click="loadData">
              <v-icon />
            </v-btn>
          </v-col>
        </v-row>
        <v-data-table
          :headers="changesHeaders"
          :items="changes"
          item-value="id"
          density="compact"
          show-expand
          items-per-page="10"
        >
          <template v-slot:item.dateTime="{ value }">
            <v-chip variant="text" dir="ltr" density="compact">
              {{ dateFormatted(value) }}
            </v-chip>
          </template>
          <template v-slot:item.action="{ value }">
            <v-chip density="compact">
              {{ $t('actions.' + value) }}
            </v-chip>
          </template>
          <template v-slot:expanded-row="{ columns, item }">
            <tr>
              <td :colspan="columns.length">
                <v-card dir="ltr" v-if="item.index>0">Index: {{ item.index }}</v-card>
                <v-card style="background-color: background" dir="ltr"><pre>{{ item.obj }}</pre></v-card>
              </td>
            </tr>
          </template>
        </v-data-table>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'

export default {
  props: ['admins', 'actor', 'visible'],
  data() {
    return {
      loading: false,
      changes: <any[]>[],
      user: '',
      key: '',
      chngCount: 10,
      expanded: [],
      changesHeaders: [
        { title: 'ID', key: 'id' },
        { title: i18n.global.t('admin.date') + '-' + i18n.global.t('admin.time'), key: 'dateTime' },
        { title: i18n.global.t('admin.actor'), key: 'Actor' },
        { title: i18n.global.t('admin.key'), key: 'key' },
        { title: i18n.global.t('admin.action'), key: 'action' },
      ],
    }
  },
  methods: {
    async loadData() {
      this.loading = true
      const data = await HttpUtils.get('api/changes',{ a: this.user, k: this.key, c: this.chngCount })
      if (data.success) {
        this.changes = data.obj?? []
        this.loading = false
      }
    },
    dateFormatted(dt: number): string {
      const date = new Date(dt*1000)
      return date.toLocaleString(this.locale)
    },
  },
  computed: {
    locale() {
      const l = i18n.global.locale.value
      switch (l) {
        case "zhHans":
          return "zh-cn"
        case "zhHant":
          return "zh-tw"
        default:
          return l
      }
    },
  },
  watch: {
    visible(newValue) {
      this.changes = []
      this.user = this.$props.actor
      this.key = ''
      this.chngCount = 10
      if (newValue) {
        this.loadData()
      }
    },
  },
}
</script>