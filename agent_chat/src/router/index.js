import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/chats/sess_main_20260509_001' },
  { path: '/chats/:sessionId', name: 'chat', component: { template: '<div />' } },
  { path: '/skills', name: 'skills', component: { template: '<div />' } },
  { path: '/audit', name: 'audit', component: { template: '<div />' } },
  { path: '/settings', name: 'settings', component: { template: '<div />' } },
]

export default createRouter({ history: createWebHashHistory(), routes })
