/**
 * API client for the Photog backend.
 */

const BASE = '/api'

async function request(path, options = {}) {
  const res = await fetch(`${BASE}${path}`, options)
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${res.status}`)
  }
  return res.json()
}

/**
 * Fetch timeline photos (paginated, grouped by month).
 */
export function fetchTimeline(offset = 0, limit = 100) {
  return request(`/timeline?offset=${offset}&limit=${limit}`)
}

/**
 * Fetch a single photo's metadata.
 */
export function fetchPhoto(id) {
  return request(`/photo/${id}`)
}

/**
 * Get library statistics.
 */
export function fetchStats() {
  return request('/stats')
}

/**
 * Trigger a re-index of the photo library.
 */
export function triggerIndex() {
  return request('/index', { method: 'POST' })
}

/**
 * Get current indexing progress.
 */
export function fetchIndexProgress() {
  return request('/index/progress')
}

/**
 * Build a thumbnail URL for a photo.
 */
export function thumbUrl(id, size = 'sm') {
  return `${BASE}/thumb/${id}/${size}`
}

/**
 * Build a full media URL for a photo/video.
 */
export function mediaUrl(id) {
  return `${BASE}/media/${id}`
}
