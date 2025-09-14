<template>
  <v-card>
    <v-card-subtitle>
      {{ $t('objects.headers') }}
      <v-chip color="primary" density="compact" variant="elevated" @click="add_header">
      <v-icon icon="mdi-plus" />
      </v-chip>
    </v-card-subtitle>
    <v-row v-for="(header, index) in hdrs">
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('objects.key')"
          hide-details
          @input="update_key(index,$event.target.value)"
          v-model="header.name">
        </v-text-field>
      </v-col>
      <v-col cols="12" sm="6" md="4">
        <v-text-field
          :label="$t('objects.value')"
          hide-details
          @input="update_value(index,$event.target.value)"
          v-model="header.value">
          <template v-slot:append>
            <v-icon @click="del_header(index)" color="error" icon="mdi-delete" />
          </template>
        </v-text-field>
      </v-col>
    </v-row>
  </v-card>
</template>

<script lang="ts">

type Header = {
  name: string
  value: string
}
export default {
  props: ['data'],
  data() {
    return {}
  },
  methods: {
    add_header() {
      this.hdrs = [...this.hdrs, {name: "Host", value: ""}]
    },
    del_header(i:number) {
      let h = this.hdrs
      h.splice(i,1)
      this.hdrs = h
    },
    update_key(i:number,k:string) {
      let h = this.hdrs
      h[i].name = k
      this.hdrs = h
    },
    update_value(i:number,v:string) {
      let h = this.hdrs
      h[i].value = v
      this.hdrs = h
    },
  },
  computed: {
    hdrs: {
      get() :Header[] {
        let headers: Header[] = []
        const h = this.$props.data.headers
        if (h) {
          Object.keys(h).forEach(key => {
            if (Array.isArray(h[key])){
              h[key].forEach((v:string) => headers.push({ name: key, value: v }))
            } else {
              headers.push({ name: key, value: h[key] })
            }
          })
        }
        return headers
      },
      set(v:Header[]) {
        if (v.length>0) {
          let headers:any = {}
          v.forEach((h:Header) => {
            if (headers[h.name]) {
              if (Array.isArray(headers[h.name])) {
                headers[h.name].push(h.value)
              } else {
                headers[h.name] = [headers[h.name], h.value]
              }
            } else {
              headers[h.name] = h.value
            }
          })
          this.$props.data.headers = headers
        } else {
          this.$props.data.headers = undefined
        }
      }
    }
  }
}
</script>