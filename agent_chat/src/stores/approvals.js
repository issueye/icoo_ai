import { defineStore } from 'pinia'
import { mockApprovals } from '@/services/mockData'

export const useApprovalsStore = defineStore('approvals', { state: () => ({ items: mockApprovals }) })
