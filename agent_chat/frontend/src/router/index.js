import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  { path: '/', redirect: '/chats' },
  { path: '/chats', name: 'chats', component: { template: '<div />' } },
  { path: '/chats/:sessionId', name: 'chat', component: { template: '<div />' } },
  { path: '/skills', name: 'skills', component: { template: '<div />' } },
  { path: '/mcp', name: 'mcp', component: { template: '<div />' } },
  { path: '/schedule', name: 'schedule', component: { template: '<div />' } },
  { path: '/audit', name: 'audit', component: { template: '<div />' } },
  { path: '/channels', name: 'channels', component: { template: '<div />' } },
  { path: '/settings', name: 'settings', component: { template: '<div />' } },
]

export default createRouter({ history: createWebHashHistory(), routes })
