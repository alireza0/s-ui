<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.endpoint') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">
          <v-tabs
            v-model="tab"
            align-tabs="center"
          >
            <v-tab value="t1">{{ $t('client.basics') }}</v-tab>
            <v-tab value="t2">{{ $t('client.external') }}</v-tab>
          </v-tabs>
          <v-window v-model="tab">
            <v-window-item value="t1">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-select
                  hide-details
                  :label="$t('type')"
                  :items="Object.keys(epTypes).map((key,index) => ({title: key, value: Object.values(epTypes)[index]}))"
                  v-model="endpoint.type"
                  @update:modelValue="changeType">
                  </v-select>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="endpoint.tag" :label="$t('objects.tag')" hide-details></v-text-field>
                </v-col>
              </v-row>
              <Wireguard v-if="endpoint.type == epTypes.Wireguard" :data="endpoint" />
              <Dial :dial="endpoint" :outTags="tags" />
            </v-window-item>
            <v-window-item value="t2">
              <v-row>
                <v-col cols="12">
                  <v-text-field v-model="link" :label="$t('client.external')" hide-details />
                </v-col>
                <v-col cols="12" align="center">
                  <v-btn hide-details variant="tonal" :loading="loading" @click="linkConvert">{{ $t('submit') }}</v-btn>
                </v-col>
              </v-row>
            </v-window-item>
          </v-window>
        </v-container>
      </v-card-text>
      <v-card-actions>
        <v-spacer></v-spacer>
        <v-btn
          color="blue-darken-1"
          variant="text"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="blue-darken-1"
          variant="text"
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
import { EpTypes, createEndpoint } from '@/types/endpoints'
import RandomUtil from '@/plugins/randomUtil'
import Dial from '@/components/Dial.vue'
import Wireguard from '@/components/protocols/Wireguard.vue'
import HttpUtils from '@/plugins/httputil'
import WgUtil from '@/plugins/wgUtil'
export default {
  props: ['visible', 'data', 'id', 'tags'],
  emits: ['close', 'save'],
  data() {
    return {
      endpoint: createEndpoint("wireguard",{ "tag": "" }),
      title: "add",
      tab: "t1",
      link: "",
      loading: false,
      epTypes: EpTypes,
    }
  },
  methods: {
    updateData() {
      if (this.$props.id > 0) {
        const newData = JSON.parse(this.$props.data)
        this.endpoint = createEndpoint(newData.type, newData)
        this.title = "edit"
      }
      else {
        const port = RandomUtil.randomIntRange(10000, 60000)
        const randomIPoctet = RandomUtil.randomIntRange(1, 255)
        this.endpoint = createEndpoint("wireguard",{
          tag: "wireguard-" + RandomUtil.randomSeq(3),
          address: ['10.0.0.'+ randomIPoctet.toString() +'/32','fe80::'+ randomIPoctet.toString(16) +'/128'],
          listen_port: port,
          private_key: WgUtil.generateKeypair().privateKey,
          peers: [{
            public_key: WgUtil.generateKeypair().publicKey,
            allowed_ips: ['0.0.0.0/0', '::/0']
          }]
        })
        this.title = "add"
      }
      this.tab = "t1"
    },
    changeType() {
      // Tag change only in add endpoint
      const tag = this.$props.id > 0 ? this.endpoint.tag : this.endpoint.type + "-" + RandomUtil.randomSeq(3)
      // Use previous data
      const prevConfig = { id: this.endpoint.id, tag: tag ,listen: this.endpoint.listen, listen_port: this.endpoint.listen_port }
      this.endpoint = createEndpoint(this.endpoint.type, prevConfig)
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.endpoint)
      this.loading = false
    },
    async linkConvert() {
      if (this.link.length>0){
        this.loading = true
        const msg = await HttpUtils.post('api/linkConvert', { link: this.link })
        this.loading = false
        if (msg.success) {
          this.endpoint = msg.obj
          this.tab = "t1"
          this.link = ""
        }
      }
    }
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.updateData()
      }
    },
  },
  components: { Dial, Wireguard }
}
</script>