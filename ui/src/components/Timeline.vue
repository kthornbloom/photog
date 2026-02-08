<script setup>
import { ref, reactive, onMounted, computed, nextTick } from 'vue'
import { fetchTimeline, fetchTimelineMonths, thumbUrl } from '../api.js'

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

// ---------------------------------------------------------------------------
// Month buckets — loaded once from the server for the scrubber.
// Each entry: { month: "2024-01", label: "January 2024", count: 47, cumulative_offset: 0 }
// Ordered newest-first (matches timeline order).
// ---------------------------------------------------------------------------
const monthBuckets = ref([])
const monthBucketsTotal = computed(() => {
  if (monthBuckets.value.length === 0) return 0
  const last = monthBuckets.value[monthBuckets.value.length - 1]
  return last.cumulative_offset + last.count
})

onMounted(async () => {
  // Fetch month buckets first (lightweight), then start loading timeline
  try {
    monthBuckets.value = await fetchTimelineMonths()
  } catch (e) {
    console.warn('Could not load month buckets:', e)
  }
  loadMore()
})

async function loadMore() {
  if (loading.value || !hasMore.value) return
  loading.value = true

  try {
    const data = await fetchTimeline(offset.value, PAGE_SIZE)
    totalCount.value = data.total_count
    hasMore.value = data.has_more

    mergeGroups(data.groups)

    offset.value += PAGE_SIZE
  } catch (e) {
    console.error('Failed to load timeline:', e)
  } finally {
    loading.value = false
  }
}

// Load the page of data *before* the current loaded window (for scrolling up after a scrub-jump)
async function loadBefore() {
  if (loading.value || loadedBaseOffset.value <= 0) return
  loading.value = true

  try {
    const prevOffset = Math.max(0, loadedBaseOffset.value - PAGE_SIZE)
    const count = loadedBaseOffset.value - prevOffset // might be < PAGE_SIZE near the start
    const data = await fetchTimeline(prevOffset, count)

    // Measure scroll height before inserting so we can preserve position
    const el = scrollContainer.value
    const prevScrollHeight = el ? el.scrollHeight : 0

    // Prepend groups (mergeGroups appends, so we need to insert before existing)
    prependGroups(data.groups)

    loadedBaseOffset.value = prevOffset

    // After DOM update, adjust scrollTop to keep the view stable
    await nextTick()
    if (el) {
      const addedHeight = el.scrollHeight - prevScrollHeight
      el.scrollTop += addedHeight
    }
  } catch (e) {
    console.error('Failed to load earlier timeline data:', e)
  } finally {
    loading.value = false
  }
}

function prependGroups(newGroups) {
  for (const group of newGroups) {
    const existing = groups.value.find(g => g.date === group.date)
    if (existing) {
      // Merge photos into the existing group
      const existingIds = new Set(existing.photos.map(p => p.id))
      const toAdd = group.photos.filter(p => !existingIds.has(p.id))
      if (toAdd.length) {
        existing.photos.unshift(...toAdd)
        existing.count = existing.photos.length
      }
    } else {
      // Insert in date order (these are newer, so prepend)
      groups.value.unshift(group)
    }
  }
  allPhotos.value = groups.value.flatMap(g => g.photos)
}

// Load a specific page around a target offset (for scrubber jumps)
async function loadAtOffset(targetOffset) {
  if (loading.value) return
  loading.value = true

  try {
    // Load a page centered on the target offset
    const data = await fetchTimeline(targetOffset, PAGE_SIZE)
    totalCount.value = data.total_count
    hasMore.value = (targetOffset + PAGE_SIZE) < data.total_count

    // Replace the entire timeline with data at this point
    groups.value = []
    loadedIds.clear()
    mergeGroups(data.groups)

    loadedBaseOffset.value = targetOffset
    offset.value = targetOffset + PAGE_SIZE
  } catch (e) {
    console.error('Failed to load timeline at offset:', e)
  } finally {
    loading.value = false
  }
}

function mergeGroups(newGroups) {
  for (const group of newGroups) {
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
}

function onImageLoad(photoId) {
  loadedIds.add(photoId)
}

function onScroll(e) {
  const el = e.target

  // Load more at bottom (forward in time = older photos)
  const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 800
  if (nearBottom && !loading.value && hasMore.value) {
    loadMore()
  }

  // Load more at top (backward = newer photos) when we jumped via scrubber
  const nearTop = el.scrollTop < 400
  if (nearTop && !loading.value && loadedBaseOffset.value > 0) {
    loadBefore()
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
// Year/month scrubber (right-side drag handle like Google Photos)
// Uses monthBuckets for the full date range regardless of loaded data.
// ---------------------------------------------------------------------------
const scrubberLabel = ref('')          // "Jan 2024" style label
const scrubberVisible = ref(false)
const scrubberDragging = ref(false)
const scrubberHandleTop = ref(0)       // 0-100 percentage
const scrubberTrack = ref(null)        // ref to .scrubber element
let scrubberHideTimer = null

// The global offset of the first photo currently loaded in the view.
// When we load from offset 0 sequentially this stays 0.
// After a scrub-jump via loadAtOffset it equals that target offset.
const loadedBaseOffset = ref(0)

const MONTH_NAMES = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec']

// Compute ruler tick marks from monthBuckets
// Each tick: { pct: 0-100, isYear: boolean, label: string }
const scrubberTicks = computed(() => {
  const buckets = monthBuckets.value
  if (buckets.length === 0) return []
  const total = monthBucketsTotal.value
  if (total === 0) return []
  const ticks = []
  for (const b of buckets) {
    const pct = (b.cumulative_offset / total) * 100
    const [yr, mo] = b.month.split('-')
    const isYear = mo === '01' || b === buckets[0] // first bucket always gets a year mark
    ticks.push({ pct, isYear, label: yr })
  }
  return ticks
})

function fractionFromPointer(e) {
  const track = scrubberTrack.value
  if (!track) return 0
  const rect = track.getBoundingClientRect()
  const y = Math.max(0, Math.min(e.clientY - rect.top, rect.height))
  return y / rect.height
}

function bucketFromFraction(fraction) {
  if (monthBuckets.value.length === 0) return null
  const total = monthBucketsTotal.value
  const targetPos = Math.round(fraction * total)
  let target = monthBuckets.value[0]
  for (const bucket of monthBuckets.value) {
    if (bucket.cumulative_offset <= targetPos) {
      target = bucket
    } else {
      break
    }
  }
  return target
}

function labelFromBucket(bucket) {
  if (!bucket) return ''
  const [yr, mo] = bucket.month.split('-')
  return `${MONTH_NAMES[parseInt(mo, 10) - 1]} ${yr}`
}

function showScrubberBriefly() {
  if (scrubberDragging.value) return
  scrubberVisible.value = true
  clearTimeout(scrubberHideTimer)
  scrubberHideTimer = setTimeout(() => { scrubberVisible.value = false }, 1500)
}

// ---------------------------------------------------------------------------
// Scrubber position from scroll: find the topmost visible .timeline-group
// by checking which section's bottom edge is still below the viewport top.
// Groups are few (5-15 on screen), so this is cheap on every scroll frame.
// ---------------------------------------------------------------------------

// Build a lookup from month -> tick pct for fast access
const tickPctByMonth = computed(() => {
  const map = {}
  const total = monthBucketsTotal.value
  if (total > 0) {
    for (const b of monthBuckets.value) {
      map[b.month] = (b.cumulative_offset / total) * 100
    }
  }
  return map
})

function updateScrubberFromScroll() {
  if (scrubberDragging.value) return

  const el = scrollContainer.value
  if (!el || groups.value.length === 0) return

  const containerTop = el.getBoundingClientRect().top

  // Walk through rendered group sections and find the one currently at the top.
  // Each section has data-date. We want the last section whose top is at or
  // above the viewport top (i.e. the one the user is currently scrolled into).
  const groupEls = el.querySelectorAll('.timeline-group')
  let currentDate = null

  for (const g of groupEls) {
    if (g.getBoundingClientRect().top <= containerTop + 60) {
      currentDate = g.dataset.date
    } else {
      // Past the viewport top — all subsequent are further down
      break
    }
  }

  // If nothing is above the top yet, use the first group
  if (!currentDate && groupEls.length > 0) {
    currentDate = groupEls[0].dataset.date
  }

  if (!currentDate) return

  const pct = tickPctByMonth.value[currentDate]
  if (pct == null) return

  scrubberHandleTop.value = pct
  const bucket = monthBuckets.value.find(b => b.month === currentDate)
  if (bucket) scrubberLabel.value = labelFromBucket(bucket)
  showScrubberBriefly()
}

function onScrubberPointerDown(e) {
  e.preventDefault()
  scrubberDragging.value = true
  scrubberVisible.value = true
  clearTimeout(scrubberHideTimer)
  onScrubberMove(e)

  const onMove = (ev) => onScrubberMove(ev)
  const onUp = (ev) => {
    scrubberDragging.value = false
    scrubberHideTimer = setTimeout(() => { scrubberVisible.value = false }, 1500)
    window.removeEventListener('pointermove', onMove)
    window.removeEventListener('pointerup', onUp)
    onScrubberRelease()
  }
  window.addEventListener('pointermove', onMove)
  window.addEventListener('pointerup', onUp)
}

// Track target offset during drag (we only jump on release to avoid spamming)
let pendingScrubOffset = null

function onScrubberMove(e) {
  if (monthBuckets.value.length === 0) return

  const fraction = fractionFromPointer(e)
  scrubberHandleTop.value = fraction * 100

  const bucket = bucketFromFraction(fraction)
  if (bucket) {
    scrubberLabel.value = labelFromBucket(bucket)
    pendingScrubOffset = bucket.cumulative_offset
  }
}

async function onScrubberRelease() {
  if (pendingScrubOffset === null) return

  const targetOffset = pendingScrubOffset
  pendingScrubOffset = null

  if (targetOffset >= loadedBaseOffset.value && targetOffset < offset.value) {
    // Target is within the currently loaded window — just scroll to it
    scrollToLoadedOffset(targetOffset)
  } else {
    // Target is outside loaded data — fetch from server
    await loadAtOffset(targetOffset)
    await nextTick()
    if (scrollContainer.value) {
      scrollContainer.value.scrollTop = 0
    }
  }
}

function scrollToLoadedOffset(targetOffset) {
  let targetMonth = null
  for (const bucket of monthBuckets.value) {
    if (bucket.cumulative_offset === targetOffset) {
      targetMonth = bucket.month
      break
    }
    if (bucket.cumulative_offset > targetOffset) break
    targetMonth = bucket.month
  }

  if (!targetMonth) return
  const el = scrollContainer.value
  if (!el) return

  const groupEl = el.querySelector(`.timeline-group[data-date="${targetMonth}"]`)
  if (groupEl) {
    groupEl.scrollIntoView({ behavior: 'instant', block: 'start' })
  }
}

// Allow clicking on the scrubber track itself (not just handle) to jump
function onScrubberTrackClick(e) {
  // Ignore if click was on the handle itself (it handles its own events)
  if (e.target.closest('.scrubber-handle')) return
  if (monthBuckets.value.length === 0) return

  const fraction = fractionFromPointer(e)
  scrubberHandleTop.value = fraction * 100

  const bucket = bucketFromFraction(fraction)
  if (bucket) {
    scrubberLabel.value = labelFromBucket(bucket)
    pendingScrubOffset = bucket.cumulative_offset
    onScrubberRelease()
  }
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
            v-if="hoverVideoId !== photo.id"
            :src="thumbUrl(photo.id, 'sm')"
            :alt="photo.filename"
            class="grid-thumb"
            loading="lazy"
            decoding="async"
            @load="onImageLoad(photo.id)"
            @error="onImageLoad(photo.id)"
          />
          <video
            v-if="isVideo(photo) && hoverVideoId === photo.id"
            :src="`/api/media/${photo.id}`"
            :poster="thumbUrl(photo.id, 'sm')"
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

    <!-- Year/month scrubber -->
    <div
      class="scrubber"
      ref="scrubberTrack"
      :class="{ visible: scrubberVisible || scrubberDragging, dragging: scrubberDragging }"
      @click="onScrubberTrackClick"
    >
      <!-- Ruler ticks -->
      <div class="scrubber-ruler">
        <template v-for="(tick, i) in scrubberTicks" :key="i">
          <div
            class="scrubber-tick"
            :class="{ 'is-year': tick.isYear }"
            :style="{ top: tick.pct + '%' }"
          >
            <span v-if="tick.isYear" class="scrubber-tick-label">{{ tick.label }}</span>
          </div>
        </template>
      </div>

      <!-- Drag handle -->
      <div
        class="scrubber-handle"
        :style="{ top: scrubberHandleTop + '%' }"
        @pointerdown="onScrubberPointerDown"
      >
        <!-- Tooltip bar extending left -->
        <div class="scrubber-tooltip" v-if="scrubberDragging && scrubberLabel">
          {{ scrubberLabel }}
        </div>
        <div class="scrubber-knob"></div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.timeline {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 0 var(--gap-lg);
  padding-bottom: calc(var(--safe-bottom) + var(--gap-xl));
}

.timeline::-webkit-scrollbar {
    display: none;
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

/* ---- Year/Month Scrubber ---- */
.scrubber {
  position: fixed;
  top: 95px;
  right: 0;
  bottom: 40px;
  width: 44px;
  pointer-events: none;
  z-index: 20;
  opacity: 0;
  transition: opacity 200ms ease;
}

.scrubber.visible {
  opacity: 1;
  pointer-events: auto;
}

.scrubber.dragging {
  pointer-events: auto;
  opacity: 1;
}

/* ---- Ruler ticks ---- */
.scrubber-ruler {
  position: absolute;
  top: 0;
  bottom: 0;
  right: 12px;
  width: 20px;
}

.scrubber-tick {
  position: absolute;
  right: 0;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  justify-content: flex-end;
}

/* Month dot */
.scrubber-tick::after {
  content: '';
  display: block;
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--text-muted);
  opacity: 0.5;
  flex-shrink: 0;
}

/* Year mark – longer line instead of dot */
.scrubber-tick.is-year::after {
  width: 12px;
  height: 1px;
  border-radius: 0;
  background: var(--text-secondary);
  opacity: 0.7;
}

.scrubber-tick-label {
  font-size: 0.55rem;
  font-weight: 600;
  color: var(--text-muted);
  margin-right: 4px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  letter-spacing: 0.02em;
  user-select: none;
}

/* ---- Drag handle (square knob) ---- */
.scrubber-handle {
  position: absolute;
  right: 4px;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  cursor: grab;
  pointer-events: auto;
  touch-action: none;
}

.scrubber-handle:active {
  cursor: grabbing;
}

.scrubber-knob {
  width: 18px;
  height: 18px;
  border-radius: 3px;
  background: var(--accent);
  opacity: 0.85;
  transition: opacity 150ms ease, transform 150ms ease, box-shadow 150ms ease;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.4);
}

.scrubber.dragging .scrubber-knob {
  opacity: 1;
  transform: scale(1.15);
  box-shadow: 0 2px 10px rgba(59, 130, 246, 0.5);
}

/* ---- Tooltip bar (extends left from handle) ---- */
.scrubber-tooltip {
  position: absolute;
  right: calc(100% + 8px);
  white-space: nowrap;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  background: var(--accent);
  padding: 5px 12px;
  border-radius: var(--radius-sm);
  font-variant-numeric: tabular-nums;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.5);
  user-select: none;
  animation: tooltipIn 120ms ease;
}

.scrubber-tooltip::after {
  content: '';
  position: absolute;
  right: -4px;
  top: 50%;
  transform: translateY(-50%);
  border: 4px solid transparent;
  border-left-color: var(--accent);
  border-right-width: 0;
}

@keyframes tooltipIn {
  from { opacity: 0; transform: translateX(6px); }
  to { opacity: 1; transform: translateX(0); }
}

/* Make the timeline position: relative for the scrubber to anchor to */
.timeline {
  position: relative;
}
</style>
