<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ $t('actions.' + title) + " " + $t('objects.ruleset') }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px;">
        <v-row>
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :label="$t('type')"
              :items="[{title: $t('ruleset.local'), value: 'local'},{ title: $t('ruleset.remote'), value: 'remote'}]"
              @update:model-value="updateType($event)"
              v-model="rule_set.type">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model="rule_set.tag" :label="$t('objects.tag')" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :label="$t('ruleset.format')"
              :items="['source', 'binary']"
              v-model="rule_set.format">
            </v-select>
          </v-col>
        </v-row>
        <v-row v-if="rule_set.type == 'local'">
          <v-col cols="12">
            <v-text-field v-model="rule_set.path" :label="$t('transport.path')" hide-details></v-text-field>
          </v-col>
        </v-row>
        <v-row v-else>
          <v-col cols="12">
            <v-text-field v-model="rule_set.url" label="URL" hide-details></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-select
              hide-details
              :label="$t('objects.outbound')"
              :items="outTags"
              clearable
              @click:clear="delete rule_set.download_detour"
              v-model="rule_set.download_detour">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="4">
            <v-text-field v-model.number="update_intervals" :suffix="$t('date.d')" type="number" min="0" :label="$t('ruleset.interval')" hide-details></v-text-field>
          </v-col>
        </v-row>
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
import RandomUtil from '@/plugins/randomUtil'
import { ruleset } from '@/types/rules'
export default {
  props: ['visible', 'data', 'index', 'outTags'],
  emits: ['close', 'save'],
  data() {
    return {
      title: "add",
      loading: false,
      rule_set: <ruleset>{},
    }
  },
  methods: {
    updateData() {
      if (this.$props.index != -1) {
        this.title = "edit"
        this.rule_set = <ruleset>JSON.parse(this.$props.data)
      }
      else {
        this.title = "add"
        this.rule_set = <ruleset>{type: 'local', tag: "rs-" + RandomUtil.randomSeq(3), format: 'binary'}
      }
    },
    updateType(t:string) {
      if (t == 'local') {
        delete this.rule_set.url
        delete this.rule_set.download_detour
        delete this.rule_set.update_interval
      } else {
        delete this.rule_set.path
      }
    },
    closeModal() {
      this.$emit('close')
    },
    saveChanges() {
      this.loading = true
      this.$emit('save', this.rule_set)
      this.loading = false
    }
  },
  computed: {
    update_intervals: {
      get() { return this.rule_set.update_interval != undefined ? parseInt(this.rule_set.update_interval.replace('d','')) : 0 },
      set(v:number) { this.rule_set.update_interval = v>0 ?  v + 'd' : undefined }
    },
  },
  watch: {
    visible(newValue) {
      if (newValue) {
        this.updateData()
      }
    },
  },
}
</script>