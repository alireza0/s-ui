<template>
  <v-dialog transition="dialog-bottom-transition" width="800">
    <v-card class="rounded-lg">
      <v-card-title>
        {{ title }}
      </v-card-title>
      <v-divider></v-divider>
      <v-card-text style="padding: 0 16px; overflow-y: scroll;">
        <div class="code-editor">
          <div class="line-numbers">
            <span v-for="n in lineCount" :key="n">{{ n }}</span>
          </div>
          <v-textarea
            ref="textareaRef"
            v-model="content"
            @scroll="syncScroll"
            hide-details
            variant="outlined"
            bg-color="background"
            :style="{ 'font-family': 'monospace' }"
            no-resize
            auto-grow
          ></v-textarea>
        </div>
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
          @click="saveChanges"
        >
          {{ $t('actions.save') }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts">
import { useTheme } from 'vuetify'

export default {
  props: ['visible', 'data', 'title'],
  emits: ['close', 'save'],
  data() {
    return {
      content: this.$props.data,
      theme: useTheme()
    }
  },
  computed: {
    lineCount() {
      return this.content?.split('\n').length
    }
  },
  methods: {
    syncScroll() {
      const textarea = document.querySelector('textarea')
      const lineNumbers = textarea?.parentElement?.parentElement?.querySelector('.line-numbers')
      if (lineNumbers && textarea) {
        lineNumbers.scrollTop = textarea.scrollTop
      }
    },
    closeModal() {
      this.$emit('close')
    },
    saveChanges() {
      this.$emit('save', this.content)
    }
  },
  watch: {
    visible(v) {
      if (v) {
        this.content = this.$props.data
      }
    }
  }
}
</script>

<style scoped>
.code-editor {
  direction: ltr !important;
  display: flex;
  border: 1px solid v-bind('theme.current.colors["outline"]');
  border-radius: 4px;
  overflow: hidden;
  font-size: 14px; /* Consistent font size */
}

.line-numbers {
  width: 40px;
  background: v-bind('theme.current.colors["surface"]');
  text-align: right;
  padding: 12px 8px 12px 4px; /* Match textarea padding */
  line-height: 1.5; /* Match textarea line height */
  overflow-y: hidden; /* Prevent independent scrolling */
  user-select: none;
  display: flex;
  flex-direction: column;
}

.line-numbers span {
  display: block;
  line-height: 1.5; /* Match textarea line height */
  height: 1.5em; /* Ensure consistent height per line */
  font-family: monospace; /* Match textarea font */
}

/* Override Vuetify textarea styles for alignment */
:deep(.v-textarea .v-field__input) {
  padding: 12px 8px !important; /* Match line-numbers padding */
  line-height: 1.5 !important; /* Match line-numbers line height */
  font-family: monospace !important;
  white-space: pre;
  mask-image: inherit;
  font-size: 14px !important; /* Match font size */
}

/* Ensure textarea and line numbers align */
:deep(.v-textarea textarea) {
  margin-top: 0 !important; /* Remove any default margin */
  padding-top: 0 !important; /* Remove any default padding */
}
</style>