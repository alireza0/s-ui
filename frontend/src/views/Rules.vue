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
      <v-btn variant="outlined" color="warning" @click="saveConfig" :loading="loading" :disabled="stateChange">
        {{ $t('actions.save') }}
      </v-btn>
    </v-col>
  </v-row>
    <v-row>
    <v-col class="v-card-subtitle" cols="12">{{ $t('basic.routing.title') }} </v-col>
    <v-col cols="12">
        <v-row>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-select
              hide-details
              :label="$t('basic.routing.defaultOut')"
              clearable
              @click:clear="delete route.final"
              :items="outboundTags"
              v-model="route.final">
            </v-select>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-text-field
              v-model="route.default_interface"
              hide-details
              clearable
              @click:clear="delete route.default_interface"
              :label="$t('basic.routing.defaultIf')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-text-field
              v-model.number="routeMark"
              hide-details
              type="number"
              min="0"
              :label="$t('basic.routing.defaultRm')"
            ></v-text-field>
          </v-col>
          <v-col cols="12" sm="6" md="3" lg="2">
            <v-switch
              v-model="route.auto_detect_interface"
              color="primary"
              :label="$t('basic.routing.autoBind')"
              hide-details>
            </v-switch>
          </v-col>
        </v-row>
      </v-col>
  </v-row>
  <v-row>
    <v-col class="v-card-subtitle" cols="12">{{ $t('rule.ruleset') }}</v-col>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>rulesets" :key="item.tag">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.tag">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ $t('ruleset.' + item.type) }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('ruleset.format') }}</v-col>
            <v-col>
              {{ item.format }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('actions.update') }}</v-col>
            <v-col>
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
    <v-col class="v-card-subtitle" cols="12">{{ $t('pages.rules') }}</v-col>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>rules"
        :key="item.id"
        :draggable="true"
        @dragstart="onDragStart(index)"
        @dragover.prevent
        @drop="onDrop(index)"
      >
      <v-card rounded="xl" elevation="5" min-width="200" :title="index+1">
        <v-card-subtitle style="margin-top: -20px;">
          <v-row>
            <v-col>{{ item.type != undefined ? $t('rule.logical') + ' (' + item.mode + ')' : $t('rule.simple') }}</v-col>
          </v-row>
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>{{ $t('admin.action') }}</v-col>
            <v-col>
              {{ item.action }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('objects.outbound') }}</v-col>
            <v-col>
              {{ item.outbound?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('pages.rules') }}</v-col>
            <v-col>
              {{ item.rules ? item.rules.length : Object.keys(item).filter(r => !actionKeys.includes(r)).length }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>{{ $t('rule.invert') }}</v-col>
            <v-col>
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
import { computed, ref, onMounted } from 'vue'
import RuleVue from '@/layouts/modals/Rule.vue'
import RulesetVue from '@/layouts/modals/Ruleset.vue'
import { Config } from '@/types/config'
import { actionKeys, ruleset } from '@/types/rules'
import { FindDiff } from '@/plugins/utils'

const oldConfig = ref({})
const loading = ref(false)

const appConfig = computed((): Config => {
  return <Config> Data().config
})

onMounted(async () => {
  oldConfig.value = JSON.parse(JSON.stringify(Data().config))
})

const routeMark = computed({
  get() { return route.value.default_mark?? 0 },
  set(v:number) { v>0 ? route.value.default_mark = v : delete appConfig.value.route.default_mark }
})

const stateChange = computed(() => {
  return FindDiff.deepCompare(appConfig.value,oldConfig.value)
})

const saveConfig = async () => {
  loading.value = true
  const success = await Data().save("config", "set", appConfig.value)
  if (success) {
    oldConfig.value = JSON.parse(JSON.stringify(Data().config))
    loading.value = false
  }
}

const clients = computed((): string[] => {
  return Data().clients.map((c:any) => c.name)
})

const route = computed((): any => {
  return appConfig.value.route?? {}
})

const rules = computed((): any[] => {
  const data = route.value
  if (!data){
    return []
  }
  if (!('rules' in data) || !Array.isArray(data.rules)) {
    data.rules = []
  }
  return data.rules
})

const rulesets = computed((): any[] => {
  const data = route.value
  if (!data){
    return []
  }
  if (!('rule_set' in data) || !Array.isArray(data.rule_set)) {
    data.rule_set = []
  }
  return data.rule_set
})

const rulesetTags = computed((): any[] => {
  return rulesets.value.map((rs:any) => rs.tag)
})

const outboundTags = computed((): string[] => {
  return [...Data().outbounds?.map((o:any) => o.tag), ...Data().endpoints?.map((e:any) => e.tag)]
})

const inboundTags = computed((): string[] => {
  return [...Data().inbounds?.map((o:any) => o.tag), ...Data().endpoints?.filter((e:any) => e.listen_port > 0).map((e:any) => e.tag)]
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

const saveRuleModal = (data:any) => {
  // New or Edit
  if (ruleModal.value.index == -1) {
    rules.value.push(data)
  } else {
    rules.value[ruleModal.value.index] = data
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

const draggedItemIndex = ref(null)

const onDragStart = (index: any) => {
  draggedItemIndex.value = index
}

const onDrop = (index: any) => {
  if (draggedItemIndex.value !== null) {
    // Swap the dragged item with the dropped one
    const draggedItem = rules.value[draggedItemIndex.value]
    rules.value.splice(draggedItemIndex.value, 1)
    rules.value.splice(index, 0, draggedItem)
    draggedItemIndex.value = null
  }
}
</script>