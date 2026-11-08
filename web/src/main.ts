// token first: it clears `#t=…` from the URL before the router reads the hash.
import { token, mockMode } from './lib/token'
import { createApp } from 'vue'
import App from './App.vue'
import { router } from './router'
import { setUnauthorizedHandler } from './lib/api'
import { initTheme } from './lib/theme'
import { vTip } from './lib/tooltip'
import { authProblem } from './lib/session'
import { loadAll, startLive } from './lib/store'
import './styles.css'

initTheme()
setUnauthorizedHandler(() => (authProblem.value = 'expired'))
if (!token && !mockMode) authProblem.value = 'missing'
else {
  loadAll()
  startLive()
}
createApp(App).use(router).directive('tip', vTip).mount('#app')
