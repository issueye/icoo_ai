import { defineStore } from 'pinia'
import { mockAuditEvents } from '@/services/mockData'

export const useAuditStore = defineStore('audit', { state: () => ({ items: mockAuditEvents }) })
