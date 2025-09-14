<template>
  <v-card subtitle="Tor">
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.executable_path" :label="$t('types.tor.execPath')" hide-details></v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="data.data_directory" :label="$t('types.tor.dataDir')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <v-row>
      <v-col cols="12" sm="6" md="4">
        <v-text-field v-model="extra_args" :label="$t('types.tor.extArgs') + ' ' + $t('commaSeparated')" hide-details></v-text-field>
      </v-col>
    </v-row>
    <div class="v-card-subtitle" style="margin: 10px;">Torrc
      <v-chip color="primary" density="compact" variant="elevated" @click="add_torrc_option"><v-icon icon="mdi-plus" /></v-chip>
    </div>
    <v-row v-for="(torrc, index) in torrc_options">
      <v-col cols="auto" align-self="center" justify-self="center">
        <v-icon @click="del_torrc_option(index)" color="error" icon="mdi-delete" />
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('objects.key')"
          hide-details
          @input="update_key(index,$event.target.value)"
          v-model="torrc.name">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('objects.value')"
          hide-details
          @input="update_value(index,$event.target.value)"
          v-model="torrc.value">
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">
type torrc_option = {
  name: string
  value: string
}
export default {
  props: ['data'],
  data() {
    return {}
  },
    methods: {
    add_torrc_option() {
      this.torrc_options = [...this.torrc_options, {name: "", value: ""}]
    },
    del_torrc_option(i:number) {
      let h = this.torrc_options
      h.splice(i,1)
      this.torrc_options = h
    },
    update_key(i:number,k:string) {
      let h = this.torrc_options
      h[i].name = k
      this.torrc_options = h
    },
    update_value(i:number,v:string) {
      let h = this.torrc_options
      h[i].value = v
      this.torrc_options = h
    },
  },
  computed: {
    torrc_options: {
      get() :torrc_option[] {
        let options: torrc_option[] = []
        const h = this.$props.data.torrc
        if (h) {
          Object.keys(h).forEach(key => {
            if (Array.isArray(h[key])){
              h[key].forEach((v:string) => options.push({ name: key, value: v }))
            } else {
              options.push({ name: key, value: h[key] })
            }
          })
        }
        return options
      },
      set(v:torrc_option[]) {
        if (v.length>0) {
          let torrc:any = {}
          v.forEach((h:torrc_option) => {
            if (torrc[h.name]) {
              if (Array.isArray(torrc[h.name])) {
                torrc[h.name].push(h.value)
              } else {
                torrc[h.name] = [torrc[h.name], h.value]
              }
            } else {
              torrc[h.name] = h.value
            }
          })
          this.$props.data.torrc = torrc
        } else {
          this.$props.data.torrc = undefined
        }
      }
    },
    extra_args: {
      get() { return this.$props.data.extra_args?.join(',') },
      set(v:string) { this.$props.data.extra_args = v.length > 0 ? v.split(',') : undefined }
    },
  },
}
</script>