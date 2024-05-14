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
            <v-btn color="primary" @click="ruleData.rules.push({})" hide-details>{{ $t('actions.add') + " " + $t('objects.rule') }}</v-btn>
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
            <v-combobox
              v-model="ruleData.outbound"
              :items="outTags"
              :label="$t('objects.outbound')"
              hide-details
            ></v-combobox>
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
import { logicalRule, rule } from '@/types/rules'
import RuleOptions from '@/components/Rule.vue'
export default {
  props: ['visible', 'data', 'index', 'clients', 'inTags', 'outTags', 'rsTags'],
  emits: ['close', 'save'],
  data() {
    return {
      title: 'add',
      loading: false,
      ruleData: <logicalRule>{
        type: 'logical',
        mode: 'and',
        rules: <rule[]>[{}],
        invert: false,
        outbound: 'direct',
      }
    }
  },
  methods: {
    updateData() {
      if (this.$props.index != -1) {
        const newData = JSON.parse(this.$props.data)
        if (newData.type) {
          this.ruleData = newData
        } else {
          this.ruleData = <logicalRule>{
            type: 'simple',
            mode: 'and',
            rules: <rule[]>[{...newData}],
            invert: newData.invert,
            outbound: newData.outbound,
          }
        }
        this.title = 'edit'
      }
      else {
        this.ruleData = <logicalRule>{
            type: 'simple',
            mode: 'and',
            rules: <rule[]>[{}],
            invert: false,
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
      if (this.ruleData.type == 'simple'){
        this.ruleData.rules[0].outbound = this.ruleData.outbound
        this.ruleData.rules[0].invert = this.ruleData.invert
      }
      this.$emit('save', this.ruleData)
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
        if (v) {
          this.ruleData.type = 'logical'
          this.ruleData.outbound = this.ruleData.rules[0].outbound?? this.$props.outTags[0]
          delete this.ruleData.rules[0].outbound
        } else {
          this.ruleData.type = 'simple'
          this.ruleData.rules[0].outbound = this.ruleData.outbound
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
  components: { RuleOptions }
}

</script>