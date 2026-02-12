/**
 * Mock API server for frontend development.
 * Generates fake photo data with placeholder images so you can
 * preview the full UI without the Go backend running.
 *
 * Usage: node mock-server.js
 * Runs on port 3001 (Vite proxies /api/* here)
 */

import http from 'node:http'

const PORT = 3001

// ---------------------------------------------------------------------------
// Generate demo photo library
// ---------------------------------------------------------------------------

// Picsum categories for variety
function picsum(id, w, h) {
  return `https://picsum.photos/id/${id}/${w}/${h}`
}

function generatePhotos(count = 300) {
  const photos = []
  const now = new Date()

  for (let i = 1; i <= count; i++) {
    // Spread photos across the last 15 years so memories (5+ years) have data
    const daysAgo = Math.floor(Math.random() * 5475) // ~15 years
    const date = new Date(now)
    date.setDate(date.getDate() - daysAgo)
    date.setHours(Math.floor(Math.random() * 14) + 7) // 7am-9pm
    date.setMinutes(Math.floor(Math.random() * 60))

    const isVideo = Math.random() < 0.08 // 8% videos
    const w = [3024, 4032, 2048, 3840, 1920][i % 5]
    const h = [4032, 3024, 1536, 2160, 1080][i % 5]

    photos.push({
      id: i,
      path: `/photos/demo/IMG_${String(i).padStart(4, '0')}.${isVideo ? 'mp4' : 'jpg'}`,
      filename: `IMG_${String(i).padStart(4, '0')}.${isVideo ? 'mp4' : 'jpg'}`,
      taken_at: date.toISOString(),
      width: w,
      height: h,
      orientation: 1,
      type: isVideo ? 'video' : 'image',
      file_size: isVideo ? 15_000_000 + Math.random() * 50_000_000 : 2_000_000 + Math.random() * 8_000_000,
      duration: isVideo ? 5 + Math.random() * 55 : 0,
      indexed_at: new Date().toISOString(),
    })
  }

  // Sort newest first
  photos.sort((a, b) => new Date(b.taken_at) - new Date(a.taken_at))
  return photos
}

const ALL_PHOTOS = generatePhotos(300)

// ---------------------------------------------------------------------------
// API handlers
// ---------------------------------------------------------------------------

function getTimeline(offset = 0, limit = 100) {
  const slice = ALL_PHOTOS.slice(offset, offset + limit)

  // Group by month
  const groupMap = new Map()
  const groupOrder = []

  for (const photo of slice) {
    const d = new Date(photo.taken_at)
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
    const label = d.toLocaleDateString('en-US', { month: 'long', year: 'numeric' })

    if (!groupMap.has(key)) {
      groupMap.set(key, { date: key, label, count: 0, photos: [] })
      groupOrder.push(key)
    }
    const g = groupMap.get(key)
    g.photos.push(photo)
    g.count++
  }

  return {
    groups: groupOrder.map(k => groupMap.get(k)),
    total_count: ALL_PHOTOS.length,
    has_more: offset + limit < ALL_PHOTOS.length,
  }
}

function getStats() {
  const images = ALL_PHOTOS.filter(p => p.type === 'image')
  const videos = ALL_PHOTOS.filter(p => p.type === 'video')
  return {
    total_photos: images.length,
    total_videos: videos.length,
    total_size: ALL_PHOTOS.reduce((s, p) => s + p.file_size, 0),
    oldest_date: ALL_PHOTOS[ALL_PHOTOS.length - 1]?.taken_at || '',
    newest_date: ALL_PHOTOS[0]?.taken_at || '',
  }
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

const server = http.createServer((req, res) => {
  // CORS
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type')
  if (req.method === 'OPTIONS') { res.writeHead(200); res.end(); return }

  const url = new URL(req.url, `http://localhost:${PORT}`)
  const path = url.pathname

  // JSON helper
  function json(data, statusCode) {
    if (statusCode) {
      res.writeHead(statusCode, { 'Content-Type': 'application/json' })
    } else {
      res.setHeader('Content-Type', 'application/json')
    }
    res.end(JSON.stringify(data))
  }

  // Route: /api/timeline/months (must be before /api/timeline)
  if (path === '/api/timeline/months') {
    const monthMap = new Map()
    const monthOrder = []
    for (const p of ALL_PHOTOS) {
      const d = new Date(p.taken_at)
      const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      if (!monthMap.has(key)) {
        monthMap.set(key, 0)
        monthOrder.push(key)
      }
      monthMap.set(key, monthMap.get(key) + 1)
    }
    let cumulative = 0
    const buckets = monthOrder.map(key => {
      const count = monthMap.get(key)
      const [y, m] = key.split('-')
      const label = new Date(parseInt(y), parseInt(m) - 1).toLocaleDateString('en-US', { month: 'long', year: 'numeric' })
      const bucket = { month: key, label, count, cumulative_offset: cumulative }
      cumulative += count
      return bucket
    })
    return json(buckets)
  }

  // Route: /api/timeline
  if (path === '/api/timeline') {
    const offset = parseInt(url.searchParams.get('offset') || '0')
    const limit = parseInt(url.searchParams.get('limit') || '100')
    return json(getTimeline(offset, limit))
  }

  // Route: /api/stats
  if (path === '/api/stats') {
    return json(getStats())
  }

  // Route: /api/photo/:id
  if (path.startsWith('/api/photo/')) {
    const id = parseInt(path.split('/').pop())
    const photo = ALL_PHOTOS.find(p => p.id === id)
    if (photo) return json(photo)
    return json({ error: 'Not found' }, 404)
  }

  // Route: /api/thumb/:id/:size — redirect to picsum placeholder
  if (path.startsWith('/api/thumb/')) {
    const parts = path.replace('/api/thumb/', '').split('/')
    const id = parseInt(parts[0])
    const size = parts[1] || 'sm'
    const dim = size === 'lg' ? 1200 : size === 'md' ? 600 : 250
    // Use photo ID as picsum seed for consistent images
    const picsumId = ((id * 7 + 13) % 200) + 10 // map to picsum range
    res.writeHead(302, { Location: picsum(picsumId, dim, dim) })
    return res.end()
  }

  // Route: /api/media/:id — redirect to a larger picsum image
  if (path.startsWith('/api/media/')) {
    const id = parseInt(path.split('/').pop())
    const photo = ALL_PHOTOS.find(p => p.id === id)
    if (!photo) { return json({ error: 'Not found' }, 404) }

    if (photo.type === 'video') {
      // Redirect to a small sample video
      res.writeHead(302, { Location: 'https://www.w3schools.com/html/mov_bbb.mp4' })
      return res.end()
    }

    const picsumId = ((id * 7 + 13) % 200) + 10
    res.writeHead(302, { Location: picsum(picsumId, 1200, 900) })
    return res.end()
  }

  // Route: /api/years — year summary for scrubber
  if (path === '/api/years') {
    const yearMap = new Map()
    for (const p of ALL_PHOTOS) {
      const y = new Date(p.taken_at).getFullYear()
      yearMap.set(y, (yearMap.get(y) || 0) + 1)
    }
    const years = [...yearMap.entries()]
      .map(([year, count]) => ({ year, count }))
      .sort((a, b) => b.year - a.year)
    return json({ years, oldest_year: years[years.length - 1]?.year, newest_year: years[0]?.year })
  }

  // Route: /api/memories — photos from 5-year intervals (5, 10, 15 years ago)
  if (path === '/api/memories') {
    const now = new Date()
    const currentYear = now.getFullYear()
    const memoryPhotos = []
    // Match the Go backend: strict 5-year intervals only
    for (let i = 1; i <= 5; i++) {
      const targetYear = currentYear - (i * 5)
      const matches = ALL_PHOTOS.filter(p => {
        const d = new Date(p.taken_at)
        return d.getFullYear() === targetYear && p.type === 'image'
      })
      if (matches.length > 0) {
        // Pick one random photo per interval (same as Go backend)
        const pick = matches[Math.floor(Math.random() * matches.length)]
        memoryPhotos.push(pick)
      }
    }
    return json({ photos: memoryPhotos })
  }

  // Route: /api/pregen/progress
  if (path === '/api/pregen/progress') {
    return json({
      running: false,
      total: ALL_PHOTOS.length,
      generated: ALL_PHOTOS.length,
      skipped: 0,
      errors: 0,
      eta_seconds: 0,
    })
  }

  // Route: /api/index/progress
  if (path === '/api/index/progress') {
    return json({
      running: false,
      total: ALL_PHOTOS.length,
      processed: ALL_PHOTOS.length,
      skipped: 0,
      errors: 0,
      started_at: new Date(Date.now() - 5000).toISOString(),
      finished_at: new Date().toISOString(),
      files_per_sec: 1250,
    })
  }

  // Route: POST /api/index
  if (path === '/api/index' && req.method === 'POST') {
    return json({ status: 'complete' })
  }

  json({ error: 'Not found' }, 404)
})

server.listen(PORT, () => {
  console.log(`\n  Mock API server running at http://localhost:${PORT}`)
  console.log(`  ${ALL_PHOTOS.length} demo photos generated (${ALL_PHOTOS.filter(p => p.type === 'video').length} videos)`)
  console.log(`  Thumbnails served via picsum.photos placeholders\n`)
})
