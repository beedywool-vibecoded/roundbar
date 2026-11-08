// Why the API can't be used: no token in the URL, or the server refused it.
import { ref } from 'vue'

export const authProblem = ref<'' | 'missing' | 'expired'>('')
