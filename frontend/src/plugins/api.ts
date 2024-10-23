import axios from 'axios'

axios.defaults.headers.post['Content-Type'] = 'application/x-www-form-urlencoded; charset=UTF-8'
axios.defaults.headers.common['X-Requested-With'] = 'XMLHttpRequest'

axios.defaults.baseURL = "./"
const pendingRequests = new Map()

axios.interceptors.request.use(
    (config) => {
        // Generate a unique key for the request
        const requestKey = `${config.method}:${config.url}`
        
        // Check if there is already a pending request with the same key
        if (pendingRequests.has(requestKey)) {
            const cancelSource = pendingRequests.get(requestKey)
            cancelSource.cancel('Duplicate request cancelled')
        }
        
        // Create a new cancel token for the request
        const cancelSource = axios.CancelToken.source()
        config.cancelToken = cancelSource.token
        
        // Store the cancel token in the pending requests map
        pendingRequests.set(requestKey, cancelSource)
        
        if (config.data instanceof FormData) {
            config.headers['Content-Type'] = 'multipart/form-data'
        }
        return config
    },
    (error) => Promise.reject(error),
)

axios.interceptors.response.use(
    (response) => {
        // Remove the request from the pending requests map
        const requestKey = `${response.config.method}:${response.config.url}`
        pendingRequests.delete(requestKey)
        return response
    },
    (error) => {
        if (axios.isCancel(error)) {
            // Handle duplicate request cancellation here if needed
            console.warn(error.message)
        } else {
            // Remove the request from the pending requests map on error
            const requestKey = `${error.config.method}:${error.config.url}`
            pendingRequests.delete(requestKey)
        }
        return Promise.reject(error)
    }
)

const api = axios.create()

export default api
