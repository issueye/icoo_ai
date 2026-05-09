import { defineStore } from 'pinia'
import { mockRuns } from '@/services/mockData'

export const useRunsStore = defineStore('runs', { state: () => ({ items: mockRuns }) })
