<template>
  <Notivue v-slot="item">
    <NotivueSwipe :item="item">
      <Notification
        :item="item"
        :theme="theme"
        :dir="direction"
        :icons="outlinedIcons"
        :hideClose="true"
        @click="item.clear"
      />
    </NotivueSwipe>
  </Notivue>
</template>

<script lang="ts" setup>
import { Notivue, Notification, NotivueSwipe, outlinedIcons, pastelTheme, darkTheme } from 'notivue'
import { computed } from 'vue'
import { useTheme } from 'vuetify'
import vuetify from '@/plugins/vuetify';

const Theme = useTheme()

const theme = computed(() =>{
  return Theme.global.name.value == "light" ? pastelTheme : darkTheme
})

const direction = computed(() => {
  return vuetify.locale.current.value == 'fa' ? 'rtl' : 'ltr'
})
</script>

<style>
:root {
  --nv-z: 10020;
}

</style>