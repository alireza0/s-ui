<template>
  <Line v-if="loaded" :data="data" :options="<any>options" />
</template>

<script lang="ts">
import { ref } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Filler,
} from 'chart.js'
import { HumanReadable } from '@/plugins/utils'
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Filler
)
ChartJS.defaults.font.family = 'Vazirmatn'
export default {
  components: {
    Line
  },
  props: ['tilesData','type'],
  data() {
    return {
      loaded: false,
      labels: new Array(20).fill(''),
      oldValues: <any>{},
      options1: {
        animation: false,
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        plugins: {
          tooltip: {
            enabled: false
          },
          legend: {
              display: false,
          }
        },
        scales: {
          y: {
            min: 0,
            max: 100,
            grid: {
              color: () => { return this.$vuetify.theme.current.colors.secondary },
            },
            beginAtZero: true,
            ticks: {
                beginAtZero: true,
                steps: 10,
                stepValue: 5,
                max: 100
            }
          }
        }
      },
      optionsNet: {
        animation: false,
        responsive: true,
        maintainAspectRatio: false,
        interaction: {
          intersect: false,
          mode: 'index',
        },
        plugins: {
          tooltip: {
            enabled: false
          },
          legend: {
              display: false,
          }
        },
        scales: {
          y: {
            grid: {
              color: () => { return this.$vuetify.theme.current.colors.secondary },
            },
            beginAtZero: true,
            ticks: {
              callback: (label:any, index: number) => { return parseInt(label).toString() },
              count: 10
            }
          }
        }
      },
      data: ref(<any>{})
    }
  },
  computed: {
    options() {
      switch (this.$props.type){
        case "h-net":
          this.optionsNet.scales.y.ticks.callback = (label:any, index: number) => {
            return label == 0 ? "0" : HumanReadable.sizeFormat(label,0)
          }
          return this.optionsNet
        case "hp-net":
          this.optionsNet.scales.y.ticks.callback = (label:any, index: number) => {
            return label == 0 ? "0" : HumanReadable.packetFormat(label,0)
          }
          return this.optionsNet
      }
      return this.options1
    }
  },
  methods: {
    updateData1(value1: number) {
      const newData = <number[]>[]
      if (this.data.datasets){
        newData.push(...this.data.datasets[0].data,value1)
      }
      if (newData.length>20) newData.shift()
      this.data = {
        labels: this.labels,
        datasets: [
          {
            label: '',
            backgroundColor: 'rgba(255, 165, 0, 0.2)',
            borderColor: 'rgba(255, 165, 0,0.8)',
            fill: true,
            data: newData
          }
        ],
      }
      this.loaded = true
    },
    updateData2(value1: number, value2:number) {
      const newData1 = <number[]>[]
      const newData2 = <number[]>[]
      if (this.data.datasets){
        newData1.push(...this.data.datasets[0].data,value1)
        newData2.push(...this.data.datasets[1].data,value2)
      }
      if (newData1.length>20) newData1.shift()
      if (newData2.length>20) newData2.shift()
      this.data = {
        labels: this.labels,
        datasets: [
          {
            label: '',
            backgroundColor: 'rgba(255, 165, 0, 0.2)',
            borderColor: 'rgba(255, 165, 0,0.8)',
            fill: true,
            data: newData1
          },
          {
            label: '',
            backgroundColor: 'rgba(0, 128, 0, 0.1)',
            borderColor: 'rgba(0, 128, 0,0.8)',
            fill: true,
            data: newData2
          }
        ],
      }
      this.loaded = true
    }
  },
  watch: {
    tilesData(v:any) {
      switch (this.$props.type) {
        case 'h-cpu':
          this.updateData1(v.cpu)
          break
        case 'h-mem':
          this.updateData1(v.mem.current*100/v.mem.total)
          break
        case 'h-net':
          if (this.oldValues.sent) {
            const downSpeed = (v.net.recv-this.oldValues.recv)/2  // Each 2 sec
            const upSpeed = (v.net.sent-this.oldValues.sent)/2  // Each 2 sec
            this.updateData2(upSpeed,downSpeed)
          }
          this.oldValues = v.net
          break
        case 'hp-net':
          if (this.oldValues.psent) {
            const downSpeed = (v.net.precv-this.oldValues.precv)/2  // Each 2 sec
            const upSpeed = (v.net.psent-this.oldValues.psent)/2  // Each 2 sec
            this.updateData2(upSpeed,downSpeed)
          }
          this.oldValues = v.net
          break
      }
    }
  }
}
</script>