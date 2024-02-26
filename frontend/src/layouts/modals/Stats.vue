<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg" :loading="loading" color="background">
      <v-card-title>
        <v-row>
          <v-col>
            {{ $t('stats.graphTitle') + " - " + $t('objects.' + resource) + " : " + tag }}
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto"><v-icon icon="mdi-close" @click="$emit('close')"></v-icon></v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px;">
        <v-container id="container">
          <v-alert :text="$t('noData')" type="warning" variant="outlined" v-if="alert"></v-alert>
          <Line v-if="loaded" :data="usage" :options="<any>options" />
        </v-container>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { i18n } from '@/locales'
import HttpUtils from '@/plugins/httputil'
import { HumanReadable } from '@/plugins/utils'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { ref } from 'vue'
import { Line } from 'vue-chartjs'
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)
ChartJS.defaults.font.family = 'Vazirmatn'
export default {
  components: {
    Line
  },
  props: ['visible','resource','tag'],
  data() {
    return {
      loading: false,
      loaded: false,
      alert: false,
      intervalId: <any>0,
      options: {
        responsive: true,
        maintainAspectRatio: true,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        plugins: {
          tooltip: {
            callbacks: {
              text: (ctx:any) => {
                const {axis = 'xy', intersect, mode} = ctx.chart.options.interaction;
                return 'Mode: ' + mode + ', axis: ' + axis + ', intersect: ' + intersect;
              },
              footer: (items:any[]) => {
                return HumanReadable.sizeFormat(items.reduce((acc, c) => acc + c.raw, 0))
              }
            }
          }
        },
        scales: {
          y: {
            grid: {
              color: () => { return this.$vuetify.theme.current.colors.secondary },
            },
            beginAtZero: true,
            ticks: {
              callback: function(label:any, index: number) {
                return label == 0 ? 0 : HumanReadable.sizeFormat(label,0)
              },
              count: 10
            }
          }
        }
      },
      usage: ref(<any>{}),
    }
  },
  methods: {
    async loadData(limit: number) {
      this.loading = true
      const data = await HttpUtils.get('api/stats', { resource: this.resource, tag: this.tag, limit: limit })
      if (data.success && data.obj) {
        const obj = <any[]>data.obj
        const l = String(i18n.global.locale) == 'fa' ? "fa-IR" : "en-US"
        const oneStep = limit * 3600 * 1000 / 360 // Each 10 sec
        const now = new Date().getTime()
        const steps = <number[]>[]
        for (let i = 360; i >= 0; i--) {
          steps.push(now - (oneStep * i))
        }
        const labels = <string[]>[]
        const uplinkData = <number[]>[]
        const downlinkData = <number[]>[]
        for (let i = 1; i<360; i++) {
          labels.push(this.genLable(steps[i],l))
          let upSum:number
          let downSum:number
          const upTraffics = obj.filter(o => o.direction && o.dateTime*1000 < steps[i] && o.dateTime*1000 > steps[i-1]).map((o:any) => o.traffic)
          upSum = upTraffics.length>0 ? upTraffics.reduce(u => u) : null
          const downTraffics = obj.filter(o => !o.direction && o.dateTime*1000 < steps[i] && o.dateTime*1000 > steps[i-1]).map((o:any) => o.traffic)
          downSum = downTraffics.length>0 ? downTraffics.reduce(d => d) : null
          uplinkData.push(upSum)
          downlinkData.push(downSum)
        }
        this.usage = {
          labels: labels,
          datasets: [
            {
              label: i18n.global.t('stats.upload'),
              backgroundColor: 'rgba(255, 165, 0, 0.4)',
              borderColor: 'rgba(255, 165, 0)',
              fill: true,
              data: uplinkData
            },
            {
              label: i18n.global.t('stats.download'),
              backgroundColor: 'rgba(0, 128, 0, 0.2)',
              borderColor: 'rgba(0, 128, 0)',
              fill: true,
              data: downlinkData
            }
          ],
        }
        this.loaded = true
      } else {
        this.alert = true
      }
      this.loading = false
    },
    genLable(step:number, locale: string) {
      return new Date(step).toLocaleString(locale,{
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      })
    },
  },
  watch: {
    visible(v) {
      if (v) {
        const limit = 1
        this.loadData(limit)
        this.intervalId = setInterval(() => {
          this.loadData(limit)
        }, 10000)
      } else {
        this.loaded = false
        this.alert = false
        this.usage.labels = []
        if (this.usage.datasets) {
          this.usage.datasets[0].data = []
          this.usage.datasets[1].data = []
        }
        if (this.intervalId && this.intervalId != 0) {
          clearInterval(this.intervalId)
        }
      }
    }
  }
}
</script>