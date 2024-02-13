import { defineStore } from 'pinia'

const Message = defineStore('msg', {
  state: () => ({
    showMsg: false,
    snackbar: {
      message: '',
      timeout: 5000,
      color: '',
    }
  }),
  actions: {
    showMessage(message:string, color='success',timeout=5000) {
      this.snackbar.message = message
      this.snackbar.color = color
      this.snackbar.timeout = timeout
      this.showMsg = true
    }
  },
})

export default Message