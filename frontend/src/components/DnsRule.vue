<template>
  <v-card style="background-color: inherit;">
    <v-row>
      <v-col cols="12" v-if="optionInbound">
        <v-combobox
          v-model="rule.inbound"
          :items="inTags"
          :label="$t('pages.inbounds')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
      <v-col cols="12" v-if="optionClient">
        <v-combobox
          v-model="rule.auth_user"
          :items="clients"
          :label="$t('pages.clients')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
      <v-col cols="12" sm="6" md="4" v-if="optionIPver">
        <v-select
          hide-details
          :label="$t('rule.ipVer')"
          :items="[4,6]"
          v-model.number="rule.ip_version">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" v-if="optionProtocol">
        <v-combobox
          v-model="rule.protocol"
          :items="['http','tls', 'quic', 'stun', 'dns']"
          :label="$t('protocol')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
    </v-row>
    <v-row v-if="optionDomain">
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="domainKeys"
          @update:model-value="updateDomainOption($event)"
          v-model="domainOption">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.domain != undefined">
        <v-text-field
        :label="$t('rule.domain') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="domain"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.domain_suffix != undefined">
        <v-text-field
        :label="$t('rule.domainSufix') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="domain_suffix"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.domain_keyword != undefined">
        <v-text-field
        :label="$t('rule.domainKw') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="domain_keyword"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.domain_regex != undefined">
        <v-text-field
        :label="$t('rule.domainRgx') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="domain_regex"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionPort">
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="portKeys"
          @update:model-value="updatePortOption($event)"
          v-model="portOption">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.port != undefined">
        <v-text-field
        :label="$t('rule.port') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="port"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.port_range != undefined">
        <v-text-field
        :label="$t('rule.portRange') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="port_range"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionSrcIP">
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="srcIPKeys"
          @update:model-value="updateSrcIPOption($event)"
          v-model="srcIPOption">
        </v-select>
      </v-col>
    </v-row>
    <v-row v-if="optionSrcPort">
      <v-col cols="12" sm="6" md="4">
        <v-select
          hide-details
          :items="srcPortKeys"
          @update:model-value="updateSrcPortOption($event)"
          v-model="srcPortOption">
        </v-select>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.source_port != undefined">
        <v-text-field
        :label="$t('rule.srcPort') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="source_port"></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" v-if="rule.source_port_range != undefined">
        <v-text-field
        :label="$t('rule.srcPortRange') + ' ' + $t('commaSeparated')"
        hide-details
        v-model="source_port_range"></v-text-field>
      </v-col>
    </v-row>
    <v-row v-if="optionRuleSet">
      <v-col cols="12" sm="6">
        <v-combobox
          v-model="rule.rule_set"
          :items="ruleSets"
          :label="$t('rule.ruleset')"
          multiple
          chips
          hide-details
        ></v-combobox>
      </v-col>
    </v-row>
    <v-card-actions>
      <v-spacer></v-spacer>
      <v-menu v-model="menu" :close-on-content-click="false" location="start">
        <template v-slot:activator="{ props }">
          <v-btn v-bind="props" hide-details variant="tonal">{{ $t('rule.options') }}</v-btn>
        </template>
        <v-card>
          <v-list>
            <v-list-item>
              <v-switch v-model="optionInbound" color="primary" :label="$t('pages.inbounds')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionClient" color="primary" :label="$t('pages.clients')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionIPver" color="primary" :label="$t('rule.ipVer')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionProtocol" color="primary" :label="$t('protocol')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionDomain" color="primary" :label="$t('rule.domainRules')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionPort" color="primary" :label="$t('in.port')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionSrcIP" color="primary" :label="$t('rule.srcIpRules')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionSrcPort" color="primary" :label="$t('rule.srcPortRules')" hide-details></v-switch>
            </v-list-item>
            <v-list-item>
              <v-switch v-model="optionRuleSet" color="primary" :label="$t('rule.ruleset')" hide-details></v-switch>
            </v-list-item>
          </v-list>
        </v-card>
      </v-menu>
    </v-card-actions>
  </v-card>
</template>

<script lang="ts">
export default {
  props: ['rule', 'clients', 'inTags', 'rsTags', 'deleteable', 'ruleSets'],
  data() {
    return {
      menu: false,
      domainKeys: ['domain', 'domain_suffix', 'domain_keyword', 'domain_regex'],
      portKeys: ['port', 'port_range'],
      srcIPKeys: ['source_ip_cidr', 'source_ip_is_private'],
      srcPortKeys: ['source_port', 'source_port_range'],
      domainOption: 'domain',
      portOption: 'port',
      srcIPOption: 'source_ip_cidr',
      srcPortOption: 'source_port',
    }
  },
  methods: {
    updateDomainOption(option:string) {
      this.domainKeys.forEach(k => delete this.$props.rule[k])
      this.$props.rule[option] = []
    },
    updatePortOption(option:string) {
      this.portKeys.forEach(k => delete this.$props.rule[k])
      this.$props.rule[option] = []
    },
    updateSrcIPOption(option:string) {
      this.srcIPKeys.forEach(k => delete this.$props.rule[k])
      this.$props.rule[option] = option == 'source_ip_is_private' ? false : []
    },
    updateSrcPortOption(option:string) {
      this.srcPortKeys.forEach(k => delete this.$props.rule[k])
      this.$props.rule[option] = []
    },
  },
  computed: {
    optionInbound: {
      get() { return this.$props.rule.inbound != undefined },
      set(v:boolean) { this.$props.rule.inbound = v ? [] : undefined }
    },
    optionClient: {
      get() { return this.$props.rule.auth_user != undefined },
      set(v:boolean) { this.$props.rule.auth_user = v ? [] : undefined }
    },
    optionIPver: {
      get() { return this.$props.rule.ip_version != undefined },
      set(v:boolean) { this.$props.rule.ip_version = v ? 4 : undefined }
    },
    optionProtocol: {
      get() { return this.$props.rule.protocol != undefined },
      set(v:boolean) { this.$props.rule.protocol = v ? ['http'] : undefined }
    },
    optionDomain: {
      get() { return Object.keys(this.$props.rule).some(r => this.domainKeys.includes(r)) },
      set(v:boolean) { 
        if (v) {
          this.$props.rule.domain = []
        } else {
          this.domainKeys.forEach(k => delete this.$props.rule[k])
        }
        this.domainOption = 'domain'
      }
    },
    optionPort: {
      get() { return Object.keys(this.$props.rule).some(r => this.portKeys.includes(r)) },
      set(v:boolean) { 
        if (v) {
          this.$props.rule.port = []
        } else {
          this.portKeys.forEach(k => delete this.$props.rule[k])
        }
        this.portOption = 'port'
      }
    },
    optionSrcIP: {
      get() { return Object.keys(this.$props.rule).some(r => this.srcIPKeys.includes(r)) },
      set(v:boolean) { 
        if (v) {
          this.$props.rule.source_ip_cidr = []
        } else {
          this.srcIPKeys.forEach(k => delete this.$props.rule[k])
        }
        this.srcIPOption = 'source_ip_cidr'
      }
    },
    optionSrcPort: {
      get() { return Object.keys(this.$props.rule).some(r => this.srcPortKeys.includes(r)) },
      set(v:boolean) { 
        if (v) {
          this.$props.rule.source_port = []
        } else {
          this.srcPortKeys.forEach(k => delete this.$props.rule[k])
        }
        this.srcPortOption = 'source_port'
      }
    },
    optionRuleSet: {
      get() { return this.$props.rule.rule_set != undefined },
      set(v:boolean) { 
        if (v) {
          this.$props.rule.rule_set = []
        } else {
          delete this.$props.rule.rule_set
        }
      }
    },
    domain: {
      get() { return this.$props.rule.domain?.join(',') },
      set(v:string) { this.$props.rule.domain = v.length>0 ? v.split(',') : [] }
    },
    domain_suffix: {
      get() { return this.$props.rule.domain_suffix?.join(',') },
      set(v:string) { this.$props.rule.domain_suffix = v.length>0 ? v.split(',') : [] }
    },
    domain_keyword: {
      get() { return this.$props.rule.domain_keyword?.join(',') },
      set(v:string) { this.$props.rule.domain_keyword = v.length>0 ? v.split(',') : [] }
    },
    domain_regex: {
      get() { return this.$props.rule.domain_regex?.join(',') },
      set(v:string) { this.$props.rule.domain_regex = v.length>0 ? v.split(',') : [] }
    },
    ip_cidr: {
      get() { return this.$props.rule.ip_cidr?.join(',') },
      set(v:string) { this.$props.rule.ip_cidr = v.length>0 ? v.split(',') : [] }
    },
    port: {
      get() { return this.$props.rule.port?.join(',') },
      set(v:string) {
        if(!v.endsWith(',')) {
          this.$props.rule.port = v.length > 0 ? v.split(',').map(str => parseInt(str, 10)) : []
        }
      }
    },
    port_range: {
      get() { return this.$props.rule.port_range?.join(',') },
      set(v:string) { this.$props.rule.port_range = v.length>0 ? v.split(',') : [] }
    },
    source_ip_cidr: {
      get() { return this.$props.rule.source_ip_cidr?.join(',') },
      set(v:string) { this.$props.rule.source_ip_cidr = v.length>0 ? v.split(',') : [] }
    },
    source_port: {
      get() { return this.$props.rule.source_port?.join(',') },
      set(v:string) {
        if(!v.endsWith(',')) {
          this.$props.rule.source_port = v.length > 0 ? v.split(',').map(str => parseInt(str, 10)) : []
        }
      }
    },
    source_port_range: {
      get() { return this.$props.rule.source_port_range?.join(',') },
      set(v:string) { this.$props.rule.source_port_range = v.length>0 ? v.split(',') : [] }
    },
  },
  mounted() {
    const ruleKeys = Object.keys(this.$props.rule)
    if (this.optionDomain) {
      const enabledOption = this.domainKeys.filter(k => ruleKeys.includes(k))
      this.domainOption = enabledOption.length>0 ? enabledOption[0] : 'domain'
    }
    if (this.optionPort) {
      const enabledOption = this.portKeys.filter(k => ruleKeys.includes(k))
      this.portOption = enabledOption.length>0 ? enabledOption[0] : 'port'
    }
    if (this.optionSrcIP) {
      const enabledOption = this.srcIPKeys.filter(k => ruleKeys.includes(k))
      this.srcIPOption = enabledOption.length>0 ? enabledOption[0] : 'source_ip_cidr'
    }
    if (this.optionSrcPort) {
      const enabledOption = this.srcPortKeys.filter(k => ruleKeys.includes(k))
      this.srcPortOption = enabledOption.length>0 ? enabledOption[0] : 'source_port'
    }
  }
}
</script>