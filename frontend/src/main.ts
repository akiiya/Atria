import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { VueQueryPlugin } from '@tanstack/vue-query'
import App from './App.vue'
import router from './router'
import './styles/variables.css'
import './styles/base.css'
import './styles/shell.css'
import './styles/chat.css'

// Canonicalize URL: redirect /app/<path> to /app/#/<path>
// This handles cases where user lands on a history-style URL
// before the Go redirect takes effect.
const loc = window.location
const appBase = '/app/'
if (loc.pathname.startsWith(appBase) && loc.pathname !== appBase && loc.pathname !== appBase.slice(0, -1)) {
  // User is at /app/accounts or /app/chats/u_123 etc.
  // Redirect to /app/#/accounts or /app/#/chats/u_123
  const subPath = loc.pathname
    .slice(appBase.length)
    .replace(/\/+$/, '')
    .replace(/^app\/+/, '')
  if (subPath) {
    window.location.replace(appBase + '#/' + subPath + loc.search)
    // Stop further execution
    throw new Error('Redirecting to canonical hash URL')
  }
}

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(VueQueryPlugin)
app.mount('#app')
