<template>
  <AdminModal 
    v-model="editModal.visible"
    :visible="editModal.visible"
    :user="editModal.user"
    @close="closeEditModal"
    @save="saveEditModal"
  />
  <v-row>
    <v-col cols="12" sm="4" md="3" lg="2" v-for="(item, index) in <any[]>users" :key="item.id">
      <v-card rounded="xl" elevation="5" min-width="200" :title="item.username">
        <v-card-subtitle>
          Last Login
        </v-card-subtitle>
        <v-card-text>
          <v-row>
            <v-col>Date</v-col>
            <v-col dir="ltr">
              {{ item.lastLogin.split(" ")[0]?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>Time</v-col>
            <v-col dir="ltr">
              {{ item.lastLogin.split(" ")[1]?? '-' }}
            </v-col>
          </v-row>
          <v-row>
            <v-col>IP</v-col>
            <v-col dir="ltr">
              {{ item.lastLogin.split(" ")[2]?? '-' }}
            </v-col>
          </v-row>
        </v-card-text>
        <v-divider></v-divider>
        <v-card-actions style="padding: 0;">
          <v-btn icon="mdi-account-edit" @click="showEditModal(item)">
            <v-icon />
            <v-tooltip activator="parent" location="top" :text="$t('actions.edit')"></v-tooltip>
          </v-btn>
        </v-card-actions>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import AdminModal from '@/layouts/modals/Admin.vue';
import HttpUtils from '@/plugins/httputil';
import { Ref, ref, inject, onMounted } from 'vue';

const loading:Ref = inject('loading')?? ref(false)

const users = ref([])

onMounted(async () => {loadData()})

const loadData = async () => {
  loading.value = true
  const msg = await HttpUtils.get('api/users')
  loading.value = false
  if (msg.success) {
    users.value = msg.obj
  }
}

const editModal = ref({
  visible: false,
  user: {},
})

const showEditModal = (user: any) => {
  editModal.value.user = user
  editModal.value.visible = true
}
const closeEditModal = () => {
  editModal.value.visible = false
}
const saveEditModal = async (data:any) => {
  loading.value=true
  const response = await HttpUtils.post('api/changePass',data)
  if(response.success){
    setTimeout(() => {
      loading.value=false
      editModal.value.visible = false
    }, 500)
  } else {
    loading.value=false
  }
}
</script>