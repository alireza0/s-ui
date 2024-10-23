<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.client') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <v-container style="padding: 0;">
          <v-tabs
            v-model="tab"
            align-tabs="center"
          >
            <v-tab value="t1">{{ $t('client.basics') }}</v-tab>
            <v-tab value="t2">{{ $t('client.config') }}</v-tab>
            <v-tab value="t3">{{ $t('client.links') }}</v-tab>
          </v-tabs>
          <v-window v-model="tab">
            <v-window-item value="t1">
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-switch color="primary" v-model="client.enable" :label="$t('enable')" hide-details></v-switch>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-combobox v-model="client.group" :items="groups" :label="$t('client.group')" hide-details></v-combobox>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="client.name" :label="$t('client.name')" hide-details></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model="client.desc" :label="$t('client.desc')" hide-details></v-text-field>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="12" sm="6" md="4">
                  <v-text-field v-model.number="Volume" type="number" min="0" :label="$t('stats.volume')" suffix="GiB" hide-details></v-text-field>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <DatePick :expiry="expDate" @submit="setDate" />
                </v-col>
              </v-row>
              <v-row v-if="index != -1">
                <v-col cols="12" sm="6" md="4" class="d-flex flex-column">
                  <div class="d-flex justify-space-between align-center">
                    <div>
                      {{ $t('stats.usage') }}: {{ total }}<sup dir="ltr" v-if="percent>0">({{ percent }}%)</sup>
                    </div>
                    <v-btn density="compact" variant="text" icon="mdi-restore" @click="client.up=0;client.down=0">
                      <v-tooltip activator="parent" location="top">
                        {{ $t('reset') }}
                      </v-tooltip>
                      <v-icon />
                    </v-btn>
                  </div>
                  <v-progress-linear
                    v-model="percent"
                    :color="percentColor"
                    v-if="client.volume>0"
                    bottom
                  >
                  </v-progress-linear>
                </v-col>
                <v-col cols="12" sm="6" md="4">
                  <v-icon icon="mdi-upload" color="orange" /><span class="text-orange">{{ up }}</span>
                  / 
                  <v-icon icon="mdi-download" color="success" /><span class="text-success">{{ down }}</span>
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-combobox
                    v-model="clientInbounds"
                    :items="inboundTags"
                    :label="$t('client.inboundTags')"
                    multiple
                    chips
                    hide-details
                  ></v-combobox>
                </v-col>
              </v-row>
              <v-row>
                <v-col cols="auto">
                  <v-switch v-model="clientStats" color="primary" :label="$t('stats.enable')" hide-details></v-switch>
                </v-col>
              </v-row>
            </v-window-item>
            <v-window-item value="t2">
              <v-row v-for="(value, key) in clientConfig" :key="key">
                <v-col cols="12" md="3" align="end" align-self="center">
                    {{ key }}
                </v-col>
                <v-col>
                  <v-text-field
                    v-if="value.password != undefined"
                    label="Password"
                    v-model="value.password"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="value.uuid != undefined"
                    label="UUID"
                    v-model="value.uuid"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="value.flow != undefined"
                    label="Flow"
                    v-model="value.flow"
                    hide-details>
                  </v-text-field>
                  <v-text-field
                    v-if="value.auth_str != undefined"
                    label="Auth"
                    v-model="value.auth_str"
                    hide-details>
                  </v-text-field>
                </v-col>
              </v-row>
            </v-window-item>
            <v-window-item value="t3">
              <v-row v-for="(lnk, index) in links">
                <v-col cols="auto">{{ index + 1 }}</v-col>
                <v-col style="direction: ltr; overflow-y: hidden;">{{ lnk.uri }}</v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-btn color="primary" @click="extLinks.push({ type: 'external', uri: ''})">{{ $t('actions.add') }} {{ $t('client.external') }}</v-btn>
                </v-col>
              </v-row>
              <v-row v-for="(lnk, index) in extLinks">
                <v-col>
                  <v-text-field
                  dir="ltr"
                  :label="$t('client.external') + ' ' + (index+1)"
                  append-icon="mdi-delete"
                  @click:append="extLinks.splice(index,1)"
                  placeholder="<protocol>://<data>"
                  v-model="lnk.uri" />
                </v-col>
              </v-row>
              <v-row>
                <v-col>
                  <v-btn color="primary" @click="subLinks.push({ type: 'sub', uri: ''})">{{ $t('actions.add') }} {{ $t('client.sub') }}</v-btn>
                </v-col>
              </v-row>
              <v-row v-for="(lnk, index) in subLinks">
                <v-col>
                  <v-text-field
                  dir="ltr"
                  :label="$t('client.sub') + ' ' + (index+1)"
                  append-icon="mdi-delete"
                  @click:append="subLinks.splice(index,1)"
                  placeholder="http[s]://<domain>[:]<port>/<path>"
                  v-model="lnk.uri" />
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
          variant="outlined"
          @click="closeModal"
        >
          {{ $t('actions.close') }}
        </v-btn>
        <v-btn
          color="blue-darken-1"
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
import { Link } from '@/plugins/link'
import { createClient, randomConfigs, updateConfigs } from '@/types/clients'
import DatePick from '@/components/DateTime.vue'
import { HumanReadable } from '@/plugins/utils'

export default {
  props: ['visible', 'data', 'index', 'inboundTags', 'groups', 'stats'],
  emits: ['close', 'save'],
  data() {
    return {
      client: createClient(),
      title: "add",
      loading: false,
      clientStats: false,
      tab: "t1",
      clientConfig: <any>[],
      links: <Link[]>[],
      extLinks: <Link[]>[],
      subLinks: <Link[]>[],
    }
  },
  methods: {
    updateData() {
      if (this.$props.index != -1) {
        const newData = JSON.parse(this.$props.data)
        this.client = createClient(newData)
        this.title = "edit"
        this.clientConfig = this.client.config
      }
      else {
        this.client = createClient()
        this.title = "add"
        this.clientConfig = randomConfigs('client')
      }
      this.clientStats = this.$props.stats
      this.links = this.client.links.filter(l => l.type == 'local')
      this.extLinks = this.client.links.filter(l => l.type == 'external')
      this.subLinks = this.client.links.filter(l => l.type == 'sub')
      this.tab = "t1"
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.client.config = updateConfigs(this.clientConfig, this.client.name)
      this.client.links = [
                        ...this.links,
                        ...this.extLinks.filter(l => l.uri != ''),
                        ...this.subLinks.filter(l => l.uri != '')]
      this.$emit('save', this.client, this.clientStats)
      this.loading = false
    },
    setDate(newDate:number){
      this.client.expiry = newDate
    }
  },
  computed: {
    clientInbounds: {
      get() { return this.client.inbounds.length>0 ? this.client.inbounds.filter(i => this.inboundTags.includes(i)) : [] },
      set(newValue:string[]) { this.client.inbounds = newValue.length == 0 ?  [] : newValue }
    },
    expDate: {
      get() { return this.client.expiry},
      set(v:any) { this.client.expiry = v }
    },
    Volume: {
      get() { return this.client.volume == 0 ? 0 : (this.client.volume / (1024 ** 3)) },
      set(v:number) { this.client.volume = v > 0 ? v*(1024 ** 3) : 0 }
    },
    up() :string { return HumanReadable.sizeFormat(this.client.up) },
    down() :string { return HumanReadable.sizeFormat(this.client.down) },
    total() :string { return HumanReadable.sizeFormat(this.client.down + this.client.up) },
    percent() :number { return this.client.volume>0 ? Math.round((this.client.up + this.client.down) *100 / this.client.volume) : 0 },
    percentColor() :string { return (this.client.up+this.client.down) >= this.client.volume ? 'error' : this.percent>90 ? 'warning' : 'success' },
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.updateData()
      }
    },
  },
  components: { DatePick },
}

</script>