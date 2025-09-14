<template>
  <v-dialog transition="dialog-bottom-transition" width="90%" max-width="500">
    <v-card class="rounded-lg">
      <v-card-title>
        <v-row>
          <v-col>{{ $t('main.backup.title') }}</v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto">
            <v-icon icon="mdi-close" @click="control.visible = false" />
          </v-col>
        </v-row>
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text>
        <v-row>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclStats')" value="stats" hide-details></v-checkbox>
          </v-col>
          <v-col cols="auto">
            <v-checkbox v-model="exclude" :label="$t('main.backup.exclChanges')" value="changes" hide-details></v-checkbox>
          </v-col>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="backup()" hide-details>{{ $t('main.backup.backup') }}</v-btn>
          </v-col>
        </v-row>
        <v-row>
          <v-spacer></v-spacer>
          <v-col cols="auto" align-self="center">
            <v-btn color="primary" @click="restore()" hide-details>{{ $t('main.backup.restore') }}</v-btn>
          </v-col>
        </v-row>
      </v-card-text>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import HttpUtils from '@/plugins/httputil'
export default {
  props: ['control', 'visible'],
  data() {
    return {
      exclude: ["stats", "changes"],
    }
  },
  methods: {
    backup() {
      const excludeOption = this.exclude.length>0 ? '?exclude=' +this.exclude.join(',') : ''
      window.location.href = 'api/getdb' + excludeOption
    },
    restore() {
      const fileInput = document.createElement('input')
      fileInput.type = 'file'
      fileInput.accept = '.db'

      fileInput.addEventListener('change', async (event: Event) => {
        const inputElement = event.target as HTMLInputElement
        const dbFile = inputElement.files ? inputElement.files[0] : null

        if (dbFile) {
          const formData = new FormData()
          formData.append('db', dbFile)

          this.control.visible = false

          const uploadMsg = await HttpUtils.post('api/importdb', formData, {
              headers: {
                  'Content-Type': 'multipart/form-data',
              },
          })

          if (uploadMsg.success) {
            await new Promise(resolve => setTimeout(resolve, 1000))
            location.reload()
          }
        }
    })

    fileInput.click()
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.exclude = ["stats", "changes"]
      }
    },
  },
}
</script>