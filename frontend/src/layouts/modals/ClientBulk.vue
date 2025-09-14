<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.addbulk') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model.number="count" type="number" min="1" max="100" :label="$t('count')" hide-details></v-text-field>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" sm="8">
              <v-combobox
                chips
                multiple
                v-model="bulkData.name"
                :items="patterns"
                :label="$t('client.name')"
                hide-details>
              </v-combobox>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" sm="8">
              <v-combobox
                chips
                multiple
                v-model="bulkData.desc"
                :items="patterns"
                :label="$t('client.desc')"
                hide-details>
              </v-combobox>
            </v-col>
          </v-row>
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-combobox v-model="bulkData.group" :items="groups" :label="$t('client.group')" hide-details></v-combobox>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model.number="bulkData.Volume" type="number" min="0" :label="$t('stats.volume')" suffix="GiB" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <DatePick :expiry="bulkData.expiry" @submit="setDate" />
            </v-col>
          </v-row>
          <v-row>
            <v-col>
              <v-combobox
                v-model="bulkData.clientInbounds"
                :items="inboundTags"
                :label="$t('client.inboundTags')"
                :return-object="false"
                multiple
                chips
                hide-details
              ></v-combobox>
            </v-col>
          </v-row>
        </v-container>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="primary"
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="primary"
          variant="tonal"
          :loading="loading"
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import DatePick from '@/components/DateTime.vue'
import { push } from 'notivue'
import RandomUtil from '@/plugins/randomUtil'
import { Client, createClient, randomConfigs } from '@/types/clients'
import { i18n } from '@/locales'
import Data from '@/store/modules/data'

export default {
  props: ['visible', 'inboundTags', 'groups'],
  emits: ['close'],
  data() {
    return {
      count: 1,
      clients: <Client[]>[],
      bulkData: {
        name: <any[]>[],
        desc: <any[]>[],
        group: '',
        clientInbounds: [],
        expiry: 0,
        Volume: 0,
      },
      patterns: [
        { title: i18n.global.t("bulk.random"), value: "random" },
        { title: i18n.global.t("bulk.order"), value: "order" },
      ],
      loading: false,
    }
  },
  methods: {
    resetData() {
      this.count = 1,
      this.clients = [],
      this.bulkData = {
        name: [this.patterns[1], "-", this.patterns[0]],
        desc: [],
        group: '',
        clientInbounds: [],
        expiry: 0,
        Volume: 0,
      }
    },
    closeModal() {
      this.$emit('close')
    },
    async saveChanges() {
      if (!this.$props.visible) return
      if (this.bulkData.name.findIndex(n => typeof(n) == 'object') == -1) {
        push.error(i18n.global.t('error.dplData'))
        return
      }
      this.clients = []
      this.loading = true
      for(let i=0;i<this.count;i++){
        const name = this.genByPattern(this.bulkData.name, i)
        this.clients.push(createClient({
          enable: true,
          name: name,
          config: randomConfigs(name),
          inbounds: this.bulkData.clientInbounds,
          links: [],
          volume: this.bulkData.Volume*(1024 ** 3),
          expiry: this.bulkData.expiry,
          up: 0,
          down: 0,
          desc: this.genByPattern(this.bulkData.desc, i),
          group: this.bulkData.group
        }))
      }
      // Check duplicate names
      const isDuplicateName = Data().checkBulkClientNames(this.clients.map(c => c.name))
      if (isDuplicateName) return
      const success = await Data().save("clients", "addbulk", this.clients)
      if (success) this.closeModal()
      this.loading = false
    },
    genByPattern(pattern: any[], order :number){
      if (pattern.length == 0) return RandomUtil.randomSeq(8)
      let result = ''
      pattern.forEach(p => {
        switch(typeof p){
          case 'object':
            switch(p.value){
              case "random":
                result += RandomUtil.randomSeq(8)
                break
              case "order":
                result += order+1
            }
            break
          default:
            result += p
        }
      })
      return result
    },
    setDate(v:number){
      this.bulkData.expiry = v
    }
  },
  computed: {},
  watch: {
    visible(newValue) {
      if (newValue) {
        this.resetData()
      }
    },
  },
  components: { DatePick },
}

</script>