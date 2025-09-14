<template>
  <v-text-field
    id="expiry"
    :label="$t('date.expiry')"
    v-model="dateFormatted"
    prepend-inner-icon="mdi-calendar"
    readonly
    hide-details
  ></v-text-field>
  <DatePicker
    v-model="Input"
    @input="Input=$event"
    :locale="locale"
    element="expiry"
    compact-time
    type="datetime">
      <template v-slot:next-month>
        <v-icon icon="mdi-chevron-right" />
      </template>
      <template v-slot:prev-month>
        <v-icon icon="mdi-chevron-left" />
      </template>
      <template #submit-btn="{ submit, canSubmit  }">
        <v-btn
          :disabled="!canSubmit"
          @click="submit"
        >{{ $t('submit') }}</v-btn>
      </template>
      <template #cancel-btn="{ vm }">
        <v-btn
          @click="reset(vm)"
        >{{ $t('reset') }}</v-btn>
      </template>
      <template #now-btn="{ goToday }">
        <v-btn
          @click="goToday"
        >{{ $t('now') }}</v-btn>
      </template>
    </DatePicker>
</template>

<script lang="ts">
import DatePicker from 'vue3-persian-datetime-picker'
import { i18n } from '@/locales'
import 'moment/locale/vi'
import 'moment/locale/zh-cn'
import 'moment/locale/zh-tw'

export default {
  props: ['expiry'],
  emits: ['submit'],
  data() {
    return {
      menu: false,
      input: new Date(),
    }
  },
  components: { DatePicker },
  computed: {
    locale() {
      const l = i18n.global.locale.value
      switch (l) {
        case "zhHans":
          return "zh-cn"
        case "zhHant":
          return "zh-tw"
        default:
          return l
      }
    },
    dateFormatted() {
      if (this.expDate == 0) return i18n.global.t('unlimited')
      const date = new Date(this.expDate*1000)
      return date.toLocaleString(this.locale)
    },
    expDate() {
      return parseInt(this.expiry?? 0)
    },
    Input: {
      get() { return this.expDate == 0 ? new Date() : new Date(this.expDate*1000) },
      set(v:string) {
        this.input = new Date(v)
        this.submit()
      }
    }
  },
  methods: {
    updateInput(v:Date) {
      this.input = v
    },
    setNow() {
      this.input = new Date()
    },
    submit() {
      this.$emit('submit',Math.floor(this.input.getTime()/1000))
    },
    reset(vm:any) {
      this.$emit('submit',0)
      this.input = new Date()
      vm.visible = false
    }
  },
  watch: {
    menu(v) {
      if (v) {
        this.input = this.expiry == 0 ? new Date() : new Date(this.expDate*1000)
      }
    }
  }
};
</script>

<style>
.vpd-addon-list,
.vpd-addon-list-item {
  background-color: rgb(var(--v-theme-background)) !important;
  border-color: rgb(var(--v-theme-background)) !important;
}
.vpd-content {
  background-color: rgb(var(--v-theme-background)) !important;
}
.vpd-addon-list-item.vpd-selected,
.vpd-addon-list-item:hover {
  background-color: rgb(var(--v-theme-primary)) !important;
}
.vpd-close-addon {
  color: rgb(var(--v-theme-on-surface)) !important;
  background-color: transparent;
}
.vpd-controls {
  overflow-x: hidden;
}
.vpd-month-label {
  width: auto;
}
.vpd-actions button:hover {
  background-color: transparent;
}
.vpd-wrapper[data-type=datetime].vpd-compact-time .vpd-time {
  border-top: 0;
}
.vpd-time .vpd-time-h .vpd-counter-item,
.vpd-time .vpd-time-m .vpd-counter-item {
  vertical-align: top;
}
</style>