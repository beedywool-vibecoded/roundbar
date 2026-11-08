import { createRouter, createWebHashHistory } from 'vue-router'
import { config, dirty } from './lib/store'

// Hash history: the Go server only has to serve index.html at `/`, and the token arrives
// in the hash anyway (lib/token.ts clears it first).
export const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'profiles', component: () => import('./views/ProfilesView.vue') },
    { path: '/profiles/:pid', name: 'profile', component: () => import('./views/ProfileView.vue'), props: true },
    {
      path: '/profiles/:pid/pages/:page',
      name: 'page',
      component: () => import('./views/PageEditorView.vue'),
      props: true,
    },
    { path: '/appearance', name: 'appearance', component: () => import('./views/AppearanceView.vue') },
    { path: '/device', name: 'device', component: () => import('./views/DeviceView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior: () => ({ top: 0 }),
})

// Closing the tab with unsaved edits asks first. Moving between views keeps them.
window.addEventListener('beforeunload', (e) => {
  if (config.value && dirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
})
