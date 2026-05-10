import { Events } from '@wailsio/runtime'
import '@/bindings/github.com/wailsapp/wails/v3/internal/eventcreate.js'

const MAX_RECENT_EVENT_IDS = 2000

function normalizeEventPayload(input) {
  if (!input || typeof input !== 'object') return null
  const payload = { ...input }
  payload.id = typeof payload.id === 'string' ? payload.id.trim() : ''
  payload.kind = typeof payload.kind === 'string' && payload.kind.trim() ? payload.kind.trim() : 'gateway_event'
  payload.sessionId = typeof payload.sessionId === 'string' ? payload.sessionId.trim() : ''
  if (!payload.createdAt) payload.createdAt = new Date().toISOString()
  if (!payload.type && payload.safeMeta && typeof payload.safeMeta === 'object') {
    payload.type = payload.safeMeta.gatewayType || ''
  }
  return payload
}

function markEventIDSeen(cache, eventID) {
  cache.ids.add(eventID)
  cache.order.push(eventID)
  if (cache.order.length <= MAX_RECENT_EVENT_IDS) return
  const staleID = cache.order.shift()
  if (staleID) cache.ids.delete(staleID)
}

function dispatchToStores(event, stores) {
  stores.messages?.applyGatewayEvent?.(event)
  stores.runs?.applyGatewayEvent?.(event)
  stores.approvals?.applyGatewayEvent?.(event)
  stores.app?.applyGatewayEvent?.(event)
}

export function subscribeAgentEvents(stores = {}) {
  const dedupeCache = {
    ids: new Set(),
    order: [],
  }
  const unsubscribe = Events.On('agent:event', (wailsEvent) => {
    const event = normalizeEventPayload(wailsEvent?.data ?? wailsEvent)
    if (!event) return
    if (event.id && dedupeCache.ids.has(event.id)) return
    if (event.id) markEventIDSeen(dedupeCache, event.id)
    dispatchToStores(event, stores)
  })
  return () => {
    dedupeCache.ids.clear()
    dedupeCache.order.length = 0
    if (typeof unsubscribe === 'function') unsubscribe()
  }
}
