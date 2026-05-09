import { defineStore } from 'pinia'
import { mockSkills } from '@/services/mockData'

export const useSkillsStore = defineStore('skills', { state: () => ({ items: mockSkills }) })
