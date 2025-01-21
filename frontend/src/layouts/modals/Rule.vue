<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.rule') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px;">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="logical" :label="$t('rule.logical')" hide-details></v-switch>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" v-if="logical" justify="center" align="center">
            <v-btn color="primary" @click="ruleData.rules.push(<rule>{})" hide-details>{{ $t('actions.add') + " " + $t('objects.rule') }}</v-btn>
          </v-col>
        </v-row>
        <v-card style="background-color: inherit; margin-bottom: 5px;" v-for="(r, index) in ruleData.rules" v-if="ruleData.type == 'logical'">
          <v-card-subtitle>{{ $t('objects.rule') + ' ' + (index+1) }}
            <v-icon @click="ruleData.rules.splice(index,1)" icon="mdi-delete" v-if="ruleData.rules.length>1" />
          </v-card-subtitle>
          <v-card-text style="padding: 0;">
            <RuleOptions
              :rule="r"
              :clients="clients"
              :inTags="inTags"
              :rsTags="rsTags" />
          </v-card-text>
        </v-card>
        <RuleOptions
          v-else
          :rule="ruleData.rules[0]"
          :clients="clients"
          :inTags="inTags"
          :rsTags="rsTags" />
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
              v-model="ruleData.action"
              :items="actions"
              :label="$t('admin.action')"
              hide-details
            ></v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4" v-if="logical">
            <v-combobox
              v-model="ruleData.mode"
              :items="['and', 'or']"
              :label="$t('rule.mode')"
              hide-details
            ></v-combobox>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-switch color="primary" v-model="ruleData.invert" :label="$t('rule.invert')" hide-details></v-switch>
          </v-col>
        </v-row>
        <v-card subtitle="Route" v-if="ruleData.action == 'route'">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="ruleData.outbound"
                :items="outTags"
                :label="$t('objects.outbound')"
                hide-details
              ></v-select>
            </v-col>
          </v-row>
        </v-card>
        <v-card subtitle="Route Option" v-if="ruleData.action == 'route-options'">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="ruleData.override_address" :label="$t('types.direct.overrideAddr')" hide-details></v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field
                v-model.number="ruleData.override_port"
                type="number"
                min="0"
                max="65534"
                :label="$t('types.direct.overridePort')"
                hide-details>
              </v-text-field>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-switch v-model="ruleData.udp_disable_domain_unmapping" :label="$t('rule.udpDisableDomainUnmapping')" hide-details></v-switch>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-switch v-model="ruleData.udp_connect" :label="$t('rule.udpConnect')" hide-details></v-switch>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="ruleData.udp_timeout" :label="$t('rule.udpTimeout')" hide-details></v-text-field>
            </v-col>
          </v-row>
        </v-card>
        <v-card subtitle="Reject" v-if="ruleData.action == 'reject'">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="ruleData.method"
                :items="[{ title: 'Default', value: 'default' },{ title: 'Drop', value: 'drop'}]"
                :label="$t('rule.method')"
                clearable
                @click:clear="delete ruleData.method"
                hide-details>
            </v-select>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-switch v-model="ruleData.no_drop" :label="$t('rule.noDrop')" hide-details></v-switch>
            </v-col>
          </v-row>
        </v-card>
        <v-card subtitle="Sniff" v-if="ruleData.action == 'sniff'">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="ruleData.sniffer"
                :items="sniffers"
                :label="$t('rule.sniffer')"
                multiple
                chips
                hide-details>
              </v-select>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="ruleData.timeout" :label="$t('rule.timeout')" hide-details></v-text-field>
            </v-col>
          </v-row>
        </v-card>
        <v-card subtitle="Resolve" v-if="ruleData.action == 'resolve'">
          <v-row>
            <v-col cols="12" sm="6" md="4">
              <v-select
                v-model="ruleData.strategy"
                :items="strategies"
                :label="$t('rule.strategy')"
                clearable
                @click:clear="delete ruleData.strategy"
                hide-details>
              </v-select>
            </v-col>
            <v-col cols="12" sm="6" md="4">
              <v-text-field v-model="ruleData.server" :label="$t('basic.dns.server')" hide-details></v-text-field>
            </v-col>
          </v-row>
        </v-card>
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
import { logicalRule, rule, actionKeys } from '@/types/rules'
import RuleOptions from '@/components/Rule.vue'
export default {
  props: ['visible', 'data', 'index', 'clients', 'inTags', 'outTags', 'rsTags'],
  emits: ['close', 'save'],
  data() {
    return {
      title: 'add',
      loading: false,
      ruleData: <any>{
        type: 'logical',
        mode: 'and',
        rules: <rule[]>[{}],
        invert: false,
        action: 'route',
        outbound: 'direct',
      },
      actions: [
        { title: 'Route', value: 'route'},
        { title: 'Route Options', value: 'route-options'},
        { title: 'Reject', value: 'reject'},
        { title: 'Hijack DNS', value: 'hijack-dns'},
        { title: 'Sniff', value: 'sniff'},
        { title: 'Resolve', value: 'resolve'}
      ],
      sniffers: [
        { title: 'HTTP', value: 'http' },
        { title: 'TLS', value: 'tls' },
        { title: 'QUIC', value: 'quic' },
        { title: 'STUN', value: 'stun' },
        { title: 'DNS', value: 'dns' },
        { title: 'BitTorrent', value: 'bittorrent' },
        { title: 'DTLS', value: 'dtls' },
        { title: 'SSH', value: 'ssh' },
        { title: 'RDP', value: 'rdp' },
      ],
      strategies: [
        { title: 'Prefer IPv4', value: 'prefer_ipv4' },
        { title: 'Prefer IPv6', value: 'prefer_ipv6' },
        { title: 'IPv4 Only', value: 'ipv4_only' },
        { title: 'IPv6 Only', value: 'ipv6_only' },
      ]
    }
  },
  methods: {
    updateData() {
      if (this.$props.index != -1) {
        const newData = JSON.parse(this.$props.data)
        if (newData.type) {
          this.ruleData = newData
        } else {
          this.ruleData = {
            type: 'simple',
            mode: 'and',
            rules: <rule[]>[{}],
          }
          Object.keys(newData).forEach(key => {
            if (actionKeys.includes(key)) {
              this.ruleData[key] = newData[key]
            } else {
              this.ruleData.rules[0][key] = newData[key]
            }
          })
        }
        this.title = 'edit'
      }
      else {
        this.ruleData = <logicalRule>{
            type: 'simple',
            mode: 'and',
            rules: <rule[]>[{}],
            invert: false,
            action: 'route',
            outbound: this.$props.outTags[0]?? 'direct',
          }
        this.title = 'add'
      }
    },
    closeModal() {
      this.updateData() // reset
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      let newRule = <any>{
        action: this.ruleData.action,
        invert: this.ruleData.invert? this.ruleData.invert : undefined,
      }

      // Filter action data
      switch (newRule.action){
        case 'route':
          newRule.outbound = this.ruleData.outbound
          break
        case 'route-options':
          newRule.override_address = this.ruleData.override_address?.length > 0 ? this.ruleData.override_address : undefined
          newRule.override_port = this.ruleData?.override_port > 0 ? this.ruleData.override_port : undefined
          newRule.network_strategy = this.ruleData.network_strategy?.length > 0 ? this.ruleData.network_strategy : undefined
          newRule.fallback_delay = this.ruleData.fallback_delay?.length > 0 ? this.ruleData.fallback_delay : undefined
          newRule.udp_disable_domain_unmapping = this.ruleData.udp_disable_domain_unmapping? true : undefined
          newRule.udp_connect = this.ruleData.udp_connect? true : undefined
          newRule.udp_timeout = this.ruleData.udp_timeout?.length > 0 ? this.ruleData.udp_timeout : undefined
          break
        case 'reject':
          newRule.method = this.ruleData.method?.length > 0 ? this.ruleData.method : undefined
          newRule.no_drop = this.ruleData.no_drop? true : undefined
          break
        case 'sniff':
          newRule.sniffer = this.ruleData.sniffer?.length > 0 ? this.ruleData.sniffer : undefined
          newRule.timeout = this.ruleData.timeout?.length > 0 ? this.ruleData.timeout : undefined
          break
        case 'resolve':
          newRule.strategy = this.ruleData.strategy?.length > 0 ? this.ruleData.strategy : undefined
          newRule.server = this.ruleData.server?.length > 0 ? this.ruleData.server : undefined
          break
      }

      // Add rules
      if (this.ruleData.type == 'simple'){
        newRule = { ...this.ruleData.rules[0], ...newRule }
      } else {
        newRule.type = 'logical'
        newRule.mode = this.ruleData.mode
        newRule.rules = this.ruleData.rules
      }
      this.$emit('save', newRule)
      this.loading = false
    },
    deleteRule(index:number) {
      this.ruleData.rules.splice(index,1)
    }
  },
  computed: {
    logical: {
      get() { return this.ruleData.type == 'logical' },
      set(v:boolean) {
        this.ruleData.type = v? 'logical' : 'simple'
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
  components: { RuleOptions }
}

</script>