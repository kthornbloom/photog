<script setup>
import { ref, reactive, onMounted, onUnmounted, computed, nextTick } from 'vue'
import { fetchTimeline, thumbUrl } from '../api.js'

const emit = defineEmits(['open'])

const groups = ref([])
const allPhotos = ref([])
const loading = ref(false)
const hasMore = ref(true)
const offset = ref(0)
const totalCount = ref(0)
const PAGE_SIZE = 200

const scrollContainer = ref(null)

// Track loaded state per photo id
const loadedIds = reactive(new Set())

onMounted(() => {
  loadMore()
})

async function loadMore() {
  if (loading.value || !hasMore.value) return
  loading.value = true

  try {
    const data = await fetchTimeline(offset.value, PAGE_SIZE)
    totalCount.value = data.total_count
    hasMore.value = data.has_more

    // Merge new groups with existing ones
    for (const group of data.groups) {
      const existing = groups.value.find(g => g.date === group.date)
      if (existing) {
        const existingIds = new Set(existing.photos.map(p => p.id))
        for (const photo of group.photos) {
          if (!existingIds.has(photo.id)) {
            existing.photos.push(photo)
            existing.count = existing.photos.length
          }
        }
      } else {
        groups.value.push(group)
      }
    }

    // Build flat photo list for viewer navigation
    allPhotos.value = groups.value.flatMap(g => g.photos)

    offset.value += PAGE_SIZE
  } catch (e) {
    console.error('Failed to load timeline:', e)
  } finally {
    loading.value = false
  }
}

function onImageLoad(photoId) {
  loadedIds.add(photoId)
}

function onScroll(e) {
  const el = e.target
  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 800
  if (nearBottom && !loading.value && hasMore.value) {
    loadMore()
  }
  updateScrubberFromScroll()
}

function openPhoto(photo) {
  const idx = allPhotos.value.findIndex(p => p.id === photo.id)
  emit('open', photo, allPhotos.value, idx >= 0 ? idx : 0)
}

function isVideo(photo) {
  return photo.type === 'video'
}

// Hover video preview refs
const hoverVideoId = ref(null)

function onThumbEnter(photo) {
  if (isVideo(photo)) {
    hoverVideoId.value = photo.id
  }
}

function onThumbLeave() {
  hoverVideoId.value = null
}

// ---------------------------------------------------------------------------
// Year scrubber (right-side drag handle like Google Photos)
// ---------------------------------------------------------------------------
const scrubberYear = ref('')
const scrubberVisible = ref(false)
const scrubberDragging = ref(false)
const scrubberHandleTop = ref(0)
let scrubberHideTimer = null

// Compute the year range from loaded groups
const yearRange = computed(() => {
  if (groups.value.length === 0) return { min: new Date().getFullYear(), max: new Date().getFullYear() }
  const years = groups.value.map(g => parseInt(g.date.split('-')[0]))
  return { min: Math.min(...years), max: Math.max(...years) }
})

function updateScrubberFromScroll() {
  const el = scrollContainer.value
  if (!el || groups.value.length === 0) return

  // Show scrubber briefly
  scrubberVisible.value = true
  clearTimeout(scrubberHideTimer)
  if (!scrubberDragging.value) {
    scrubberHideTimer = setTimeout(() => { scrubberVisible.value = false }, 1500)
  }

  const scrollFraction = el.scrollTop / Math.max(1, el.scrollHeight - el.clientHeight)
  scrubberHandleTop.value = scrollFraction * 100

  // Find which group header is near the current viewport top
  const groupEls = el.querySelectorAll('.timeline-group')
  for (const groupEl of groupEls) {
    const rect = groupEl.getBoundingClientRect()
    const containerRect = el.getBoundingClientRect()
    if (rect.bottom > containerRect.top + 60) {
      const dateAttr = groupEl.dataset.date
      if (dateAttr) {
        scrubberYear.value = dateAttr.split('-')[0]
      }
      break
    }
  }
}

function onScrubberPointerDown(e) {
  e.preventDefault()
  scrubberDragging.value = true
  scrubberVisible.value = true
  clearTimeout(scrubberHideTimer)
  onScrubberMove(e)

  const onMove = (ev) => onScrubberMove(ev)
  const onUp = () => {
    scrubberDragging.value = false
    scrubberHideTimer = setTimeout(() => { scrubberVisible.value = false }, 1500)
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}

function onScrubberMove(e) {
  const el = scrollContainer.value
  if (!el) return

  const track = el.querySelector('.scrubber-track') || el.closest('.timeline')
  const rect = el.getBoundingClientRect()
  const y = Math.max(0, Math.min(e.clientY - rect.top, rect.height))
  const fraction = y / rect.height

  // Scroll to that fraction of the total scroll height
  const maxScroll = el.scrollHeight - el.clientHeight
  el.scrollTop = fraction * maxScroll

  scrubberHandleTop.value = fraction * 100

  // Determine the year at this position
  const range = yearRange.value
  const year = Math.round(range.max - fraction * (range.max - range.min))
  scrubberYear.value = String(year)
}
</script>

<template>
  <div class="timeline" ref="scrollContainer" @scroll="onScroll">
    <div v-if="groups.length === 0 && !loading" class="empty-state">
      <div class="empty-icon">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="18" height="18" rx="2" />
          <circle cx="8.5" cy="8.5" r="1.5" />
          <path d="M21 15l-5-5L5 21" />
        </svg>
      </div>
      <p class="empty-title">No photos yet</p>
      <p class="empty-subtitle">Photos will appear here once indexing completes</p>
    </div>

    <section v-for="group in groups" :key="group.date" :data-date="group.date" class="timeline-group">
      <div class="group-header">
        <h2 class="group-label">{{ group.label }}</h2>
        <span class="group-count">{{ group.count }}</span>
      </div>

      <div class="photo-grid">
        <div
          v-for="photo in group.photos"
          :key="photo.id"
          class="grid-item"
          :class="{
            'is-video': isVideo(photo),
            'is-loading': !loadedIds.has(photo.id),
            'is-loaded': loadedIds.has(photo.id),
          }"
          @click="openPhoto(photo)"
          @mouseenter="onThumbEnter(photo)"
          @mouseleave="onThumbLeave"
        >
          <img
            v-if="!isVideo(photo) || hoverVideoId !== photo.id"
            :src="thumbUrl(photo.id, 'sm')"
            :alt="photo.filename"
            class="grid-thumb"
            loading="lazy"
            decoding="async"
            @load="onImageLoad(photo.id)"
          />
          <video
            v-if="isVideo(photo) && hoverVideoId === photo.id"
            :src="`/api/media/${photo.id}`"
            class="grid-thumb"
            autoplay
            muted
            loop
            playsinline
          />
          <div class="video-badge" v-if="isVideo(photo)">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M8 5v14l11-7z" />
            </svg>
          </div>
        </div>
      </div>
    </section>

    <div v-if="loading" class="loading-indicator">
      <div class="loading-spinner"></div>
      <span>Loading photos...</span>
    </div>

    <div v-if="!hasMore && groups.length > 0" class="end-marker">
      That's everything — {{ totalCount.toLocaleString() }} items
    </div>

    <!-- Year scrubber handle -->
    <div
      class="scrubber"
      :class="{ visible: scrubberVisible || scrubberDragging, dragging: scrubberDragging }"
    >
      <div
        class="scrubber-handle"
        :style="{ top: scrubberHandleTop + '%' }"
        @pointerdown="onScrubberPointerDown"
      >
        <span class="scrubber-label" v-if="scrubberYear">{{ scrubberYear }}</span>
        <div class="scrubber-pill"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.timeline {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: var(--gap-lg);
  padding-bottom: calc(var(--safe-bottom) + var(--gap-xl));
}

/* ---- Empty State ---- */
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 60vh;
  gap: var(--gap-md);
  animation: fadeIn var(--transition-normal);
}

.empty-icon {
  width: 64px;
  height: 64px;
  color: var(--text-muted);
  opacity: 0.5;
}

.empty-icon svg {
  width: 100%;
  height: 100%;
}

.empty-title {
  font-size: 1.1rem;
  color: var(--text-secondary);
}

.empty-subtitle {
  font-size: 0.85rem;
  color: var(--text-muted);
}

/* ---- Group headers (sticky) ---- */
.timeline-group {
  margin-bottom: var(--gap-lg);
}

.group-header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: baseline;
  gap: var(--gap-md);
  padding: var(--gap-sm) var(--gap-sm);
  background: var(--bg-primary);
  /* Glass effect for sticky header */
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
}

.group-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.group-count {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}

/* ---- Photo grid (responsive masonry-like) ---- */
.photo-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(var(--thumb-min), 1fr));
  gap: var(--gap-lg);
}

.grid-item {
  position: relative;
  aspect-ratio: 1;
  overflow: hidden;
  border-radius: var(--radius-sm);
  cursor: pointer;
  background: var(--bg-surface);
}

.grid-item::after {
  content: '';
  position: absolute;
  inset: 0;
  opacity: 0;
  background: rgba(255, 255, 255, 0.06);
  transition: opacity var(--transition-fast);
}

.grid-item:hover::after {
  opacity: 1;
}

.grid-item:active {
  transform: scale(0.97);
  transition: transform 80ms ease;
}

.grid-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

/* Skeleton shimmer while image is loading */
.grid-item.is-loading {
  background: var(--bg-surface);
  animation: pulse 1.5s ease-in-out infinite;
}

.grid-thumb {
  opacity: 0;
  transition: opacity 300ms ease;
}

.grid-item.is-loaded .grid-thumb {
  opacity: 1;
}

/* Video badge */
.video-badge {
  position: absolute;
  bottom: 6px;
  right: 6px;
  width: 24px;
  height: 24px;
  background: rgba(0, 0, 0, 0.65);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  pointer-events: none;
}

.video-badge svg {
  width: 14px;
  height: 14px;
}

/* ---- Loading / End ---- */
.loading-indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: var(--gap-md);
  padding: var(--gap-xl);
  color: var(--text-muted);
  font-size: 0.85rem;
}

.loading-spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.end-marker {
  text-align: center;
  padding: var(--gap-xl);
  font-size: 0.8rem;
  color: var(--text-muted);
}

/* Responsive: bigger thumbnails on large screens */
@media (min-width: 768px) {
  .photo-grid {
    grid-template-columns: repeat(auto-fill, minmax(var(--thumb-max), 1fr));
    gap: var(--gap-sm);
  }
}

@media (min-width: 1200px) {
  .photo-grid {
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  }
}

/* ---- Year Scrubber ---- */
.scrubber {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  width: 44px;
  pointer-events: none;
  z-index: 20;
  opacity: 0;
  transition: opacity 200ms ease;
}

.scrubber.visible {
  opacity: 1;
}

.scrubber.dragging {
  pointer-events: auto;
  opacity: 1;
}

.scrubber-handle {
  position: absolute;
  right: 4px;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: grab;
  pointer-events: auto;
  touch-action: none;
  flex-direction: row-reverse;
}

.scrubber-handle:active {
  cursor: grabbing;
}

.scrubber-pill {
  width: 4px;
  height: 40px;
  background: var(--accent);
  border-radius: 2px;
  opacity: 0.8;
  transition: width 150ms ease, opacity 150ms ease;
}

.scrubber.dragging .scrubber-pill {
  width: 5px;
  opacity: 1;
}

.scrubber-label {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-primary);
  background: var(--bg-surface);
  border: 1px solid var(--border);
  padding: 4px 8px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  font-variant-numeric: tabular-nums;
  box-shadow: 0 2px 8px var(--shadow);
  opacity: 0;
  transform: translateX(8px);
  transition: opacity 150ms ease, transform 150ms ease;
}

.scrubber.visible .scrubber-label,
.scrubber.dragging .scrubber-label {
  opacity: 1;
  transform: translateX(0);
}

/* Make the timeline position: relative for the scrubber to anchor to */
.timeline {
  position: relative;
}
</style>
