<template>
  <RuleVue
    v-model="ruleModal.visible"
    :visible="ruleModal.visible"
    :index="ruleModal.index"
    :data="ruleModal.data"
    :clients="clients"
    :inTags="inboundTags"
    :outTags="outboundTags"
    :rsTags="rulesetTags"
    @close="closeRuleModal"
    @save="saveRuleModal"
  />
  <RulesetVue
    v-model="rulesetModal.visible"
    :visible="rulesetModal.visible"
    :index="rulesetModal.index"
    :data="rulesetModal.data"
    :outTags="outboundTags"
    @close="closeRulesetModal"
    @save="saveRulesetModal"
  />
  <v-row>
    <v-col cols="12" justify="center" align="center">
      <v-btn color="primary" @click="showRuleModal(-1)" style="margin: 0 5px;">{{ $t('rule.add') }}</v-btn>
      <v-btn color="primary" @click="showRulesetModal(-1)" style="margin: 0 5px;">{{ $t('ruleset.add') }}</v-btn>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12">{{ $t('rule.ruleset') }}</v-col>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>rulesets" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="index">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ $t('ruleset.' + item.type) }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('objects.tag') }}</v-col>
            <v-col dir="ltr">
              {{ item.tag }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('ruleset.format') }}</v-col>
            <v-col dir="ltr">
              {{ item.format }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('actions.update') }}</v-col>
            <v-col dir="ltr">
              {{ item.update_interval?? '-' }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showRulesetModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delRulesetOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delRulesetOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delRuleset(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delRulesetOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
  <v-row>
    <v-col cols="12">{{ $t('pages.rules') }}</v-col>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>rules">
      <v-card rounded="xl" elevation="5" min-width="200" :title="index">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ item.type != undefined ? $t('rule.logical') + ' (' + item.mode + ')' : $t('rule.simple') }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('objects.outbound') }}</v-col>
            <v-col dir="ltr">
              {{ item.outbound }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.rules') }}</v-col>
            <v-col dir="ltr">
              {{ item.rules ? item.rules.length : Object.keys(item).filter(r => !["rule_set_ipcidr_match_source","invert","outbound"].includes(r)).length }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('rule.invert') }}</v-col>
            <v-col dir="ltr">
              {{ $t( (item.invert?? false)? 'yes' : 'no') }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-file-edit" @click="showRuleModal(index)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
          <v-btn icon="mdi-file-remove" style="margin-inline-start:0;" color="warning" @click="delRuleOverlay[index] = true">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.del')"></v-tooltip>
          </v-btn>
          <v-overlay
            v-model="delRuleOverlay[index]"
            contained
            class="align-center justify-center"
          >
            <v-card :title="$t('actions.del')" rounded="lg">
              <v-divider></v-divider>
              <v-card-text>{{ $t('confirm') }}</v-card-text>
              <v-card-actions>
                <v-btn color="error" variant="outlined" @click="delRule(index)">{{ $t('yes') }}</v-btn>
                <v-btn color="success" variant="outlined" @click="delRuleOverlay[index] = false">{{ $t('no') }}</v-btn>
              </v-card-actions>
            </v-card>
          </v-overlay>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import Data from '@/store/modules/data'
import { computed, ref } from 'vue'
import RuleVue from '@/layouts/modals/Rule.vue'
import RulesetVue from '@/layouts/modals/Ruleset.vue'
import { Config } from '@/types/config'
import { logicalRule, ruleset } from '@/types/rules'

const appConfig = computed((): Config => {
  return <Config> Data().config
})

const clients = computed((): string[] => {
  return Data().clients.map((c:any) => c.name)
})

const route = computed((): any => {
  return appConfig.value.route
})

const rules = computed((): any[] => {
  const data = route.value
  if (!data || !('rules' in data) || !Array.isArray(data.rules)){
    return []
  }
  return data.rules
})

const rulesets = computed((): any[] => {
  const data = route.value
  if (!data || !('rule_set' in data) || !Array.isArray(data.rule_set)){
    return []
  }
  return data.rule_set
})

const rulesetTags = computed((): any[] => {
  return rulesets.value.map((rs:any) => rs.tag)
})

const outboundTags = computed((): string[] => {
  return appConfig.value.outbounds?.map((o:any) => o.tag)
})

const inboundTags = computed((): string[] => {
  return appConfig.value.inbounds?.map((i:any) => i.tag)
})

let delRuleOverlay = ref(new Array<boolean>)
let delRulesetOverlay = ref(new Array<boolean>)

const ruleModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showRuleModal = (index: number) => {
  ruleModal.value.index = index
  ruleModal.value.data = index == -1 ? '' : JSON.stringify(rules.value[index])
  ruleModal.value.visible = true
}

const closeRuleModal = () => {
  ruleModal.value.visible = false
}

const saveRuleModal = (data:logicalRule) => {
  // Logical or simple
  const ruleData = data.type == 'logical' ? data : data.rules[0]

  // New or Edit
  if (ruleModal.value.index == -1) {
    rules.value.push(ruleData)
  } else {
    rules.value[ruleModal.value.index] = ruleData
  }
  ruleModal.value.visible = false
}

const delRule = (index: number) => {
  rules.value.splice(index,1)
  delRuleOverlay.value[index] = false
}

const rulesetModal = ref({
  visible: false,
  index: -1,
  data: "",
})

const showRulesetModal = (index: number) => {
  rulesetModal.value.index = index
  rulesetModal.value.data = index == -1 ? '' : JSON.stringify(rulesets.value[index])
  rulesetModal.value.visible = true
}

const closeRulesetModal = () => {
  rulesetModal.value.visible = false
}

const saveRulesetModal = (data:ruleset) => {
  // New or Edit
  if (rulesetModal.value.index == -1) {
    rulesets.value.push(data)
  } else {
    rulesets.value[rulesetModal.value.index] = data
  }
  rulesetModal.value.visible = false
}

const delRuleset = (index: number) => {
  rulesets.value.splice(index,1)
  delRulesetOverlay.value[index] = false
}
</script>