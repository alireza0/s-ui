<template>
    <v-snackbar
    v-model="sb.showMsg"
    location="top"
    :color="snackbar.color"
    :timeout="snackbar.timeout">
      {{ snackbar.message }}
    </v-snackbar>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import Message from '@/store/modules/message'

const sb = Message()

const snackbar = ref(sb.snackbar)
</script>
