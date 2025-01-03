<template>
  <v-app-bar :elevation="5">
    <v-icon v-if="isMobile" icon="mdi-menu" @click="$emit('toggleDrawer')" />
    <span v-else style="width: 24px"></span>
    <v-app-bar-title :text="$t(<string>route.name)" class="align-center text-center " />
    <v-icon icon="mdi-theme-light-dark" @click="toggleTheme()" style="margin: 0 10px;"></v-icon>
  </v-app-bar>
</template>

<script lang="ts" setup>
import { ref } from "vue"
import { useTheme } from "vuetify"
import { useRoute } from "vue-router";

defineProps(['isMobile'])

const route = useRoute();
const theme = useTheme()
const darkMode = ref(localStorage.getItem('theme') == "dark")

const toggleTheme = () => {
  darkMode.value = !darkMode.value
  theme.global.name.value = darkMode.value ? "dark" : "light"
  localStorage.setItem('theme', theme.global.name.value)
}
</script>
