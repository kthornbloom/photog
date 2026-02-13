<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { mediaUrl, thumbUrl } from '../api.js'

const props = defineProps({
  photo: Object,
  photos: Array,
  currentIndex: Number,
})

const emit = defineEmits(['close', 'navigate'])

const containerRef = ref(null)
const closing = ref(false)

function closeViewer() {
  closing.value = true
  setTimeout(() => {
    emit('close')
  }, 250)
}

// ---------------------------------------------------------------
// Keyed-Slide Approach
//
// Render a small window of slides around the active photo. Each
// slide is keyed by PHOTO ID so Vue never reuses a DOM element
// for a different photo — meaning img src is NEVER mutated on a
// live element. Navigation simply changes the track translateX.
// Vue adds/removes slides at the edges, but slides currently in
// view are untouched. Zero flicker, fully interruptible.
// ---------------------------------------------------------------

const WINDOW = 2 // render current ± 2

// The photo index we're currently showing / animating to.
// This is the single source of truth for navigation.
const activeIndex = ref(0)

// Whether CSS transition is active (off during drag)
const animating = ref(false)

// Drag state
const dragOffset = ref(0)
const dragging = ref(false)

// Zoom/pan
const scale = ref(1)
const zoomX = ref(0)
const zoomY = ref(0)
const isZoomed = computed(() => scale.value > 1.05)

// Media error tracking
const mediaError = ref(false)

// Background crossfade — two permanent layers that alternate
// Layer A and B each hold a background-image. We fade between them
// by toggling which one is "on top" (opacity 1) vs behind (opacity 0).
const bgLayerA = ref('')
const bgLayerB = ref('')
const bgShowA = ref(true)  // true = A is visible on top, false = B is on top
let bgCurrent = 'a'        // which layer currently holds the active image

// Pointer tracking
let touchStartX = 0
let touchStartY = 0
let touchStartDist = 0
let touchStartScale = 1
let isPinching = false
let isZoomDragging = false
let swipeLocked = false

// ---- Helpers ----

function isVideoAtIndex(i) {
  return props.photos?.[i]?.type === 'video'
}

function srcForIndex(i) {
  if (i < 0 || i >= (props.photos?.length ?? 0)) return ''
  return mediaUrl(props.photos[i].id)
}

function previewSrcForIndex(i) {
  if (i < 0 || i >= (props.photos?.length ?? 0)) return ''
  if (isVideoAtIndex(i)) return thumbUrl(props.photos[i].id, 'lg')
  return mediaUrl(props.photos[i].id)
}

function bgForIndex(i) {
  if (i < 0 || i >= (props.photos?.length ?? 0)) return ''
  return thumbUrl(props.photos[i].id, 'lg')
}

// ---- Computed slide window ----
// Returns an array of { index, id, src, isCenter } for indices around activeIndex.
// Keyed by photo id — Vue will create a fresh <img> for each photo and never
// change its src. When activeIndex changes, slides at the edges are added/removed
// but the center one (and its neighbors) are left completely alone in the DOM.

const visibleSlides = computed(() => {
  const photos = props.photos
  if (!photos?.length) return []
  const center = activeIndex.value
  const slides = []
  for (let i = center - WINDOW; i <= center + WINDOW; i++) {
    if (i < 0 || i >= photos.length) continue
    slides.push({
      index: i,
      id: photos[i].id,
      // Always use previewSrcForIndex for the <img> — this returns the full
      // mediaUrl for images, or a thumbnail for videos (since videos can't
      // render in <img>). This means the img src NEVER changes when a slide
      // transitions between center and side positions. Videos get a separate
      // <video> element layered on top.
      src: previewSrcForIndex(i),
      isCenter: i === center,
      isVideo: photos[i].type === 'video',
    })
  }
  return slides
})

// ---- Computed display info ----

const hasPrev = computed(() => activeIndex.value > 0)
const hasNext = computed(() => activeIndex.value < (props.photos?.length ?? 0) - 1)

const isVideo = computed(() => isVideoAtIndex(activeIndex.value))

const displayPhoto = computed(() => props.photos?.[activeIndex.value])

const photoDate = computed(() => {
  if (!displayPhoto.value?.taken_at) return ''
  const d = new Date(displayPhoto.value.taken_at)
  return d.toLocaleDateString(undefined, {
    weekday: 'long',
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
})

const thumbSrc = computed(() => {
  if (!displayPhoto.value) return ''
  return thumbUrl(displayPhoto.value.id, 'lg')
})

// ---- Track positioning ----
// The track holds ALL slide positions via left: index * 100%.
// We slide it with translateX to show the active one.
// During drag, we add the pixel drag offset.

const trackStyle = computed(() => {
  const base = -(activeIndex.value * 100)
  if (dragging.value) {
    // Convert drag pixel offset to a percentage of viewport for smooth combination
    // We can't mix % and px in a single translateX, so use calc()
    return {
      transform: `translateX(calc(${base}% + ${dragOffset.value}px))`,
      transition: 'none',
    }
  }
  return {
    transform: `translateX(${base}%)`,
    transition: animating.value
      ? 'transform 280ms cubic-bezier(0.25, 0.46, 0.45, 0.94)'
      : 'none',
  }
})

// ---- Initialize ----

let initialized = false
watch(() => props.currentIndex, (idx) => {
  if (!initialized) {
    initialized = true
    activeIndex.value = idx
    // Set initial background on layer A directly (no crossfade needed)
    const bg = bgForIndex(idx)
    bgLayerA.value = bg
    bgShowA.value = true
    bgCurrent = 'a'
  }
}, { immediate: true })

function updateBackground(idx) {
  const newBg = bgForIndex(idx)
  if (!newBg) return

  // Write the new image to the HIDDEN layer, then flip visibility.
  // The CSS transition on opacity handles the crossfade.
  // No setTimeout, no race conditions, works perfectly during rapid swiping.
  if (bgCurrent === 'a') {
    // A is currently visible — load new image into B, then show B
    bgLayerB.value = newBg
    bgShowA.value = false
    bgCurrent = 'b'
  } else {
    // B is currently visible — load new image into A, then show A
    bgLayerA.value = newBg
    bgShowA.value = true
    bgCurrent = 'a'
  }
}

// ---- Zoom ----

function resetZoom() {
  scale.value = 1
  zoomX.value = 0
  zoomY.value = 0
}

function toggleZoom(e) {
  if (isVideo.value) return
  if (isZoomed.value) {
    resetZoom()
  } else {
    scale.value = 2.5
    if (containerRef.value) {
      const rect = containerRef.value.getBoundingClientRect()
      zoomX.value = (rect.width / 2 - e.clientX) * 0.6
      zoomY.value = (rect.height / 2 - e.clientY) * 0.6
    }
  }
}

// ---- Navigation ----

function goTo(newIndex) {
  const clamped = Math.max(0, Math.min((props.photos?.length ?? 1) - 1, newIndex))
  if (clamped === activeIndex.value) return

  mediaError.value = false
  animating.value = true
  activeIndex.value = clamped
  resetZoom()
  updateBackground(clamped)
  emit('navigate', clamped)
}

function onTransitionEnd(e) {
  if (e.target !== e.currentTarget) return
  if (e.propertyName !== 'transform') return
  animating.value = false
}

function goPrev() {
  if (hasPrev.value) goTo(activeIndex.value - 1)
}

function goNext() {
  if (hasNext.value) goTo(activeIndex.value + 1)
}

function onKeyDown(e) {
  switch (e.key) {
    case 'Escape': closeViewer(); break
    case 'ArrowLeft': goPrev(); break
    case 'ArrowRight': goNext(); break
  }
}

// ---- Media error ----

function onMediaError() {
  mediaError.value = true
}

// ---- Pointer / swipe handling ----

function onPointerDown(e) {
  if (e.pointerType === 'touch' && !e.isPrimary) return

  touchStartX = e.clientX
  touchStartY = e.clientY
  swipeLocked = false
  isPinching = false

  if (isZoomed.value) {
    isZoomDragging = true
    return
  }

  // If mid-animation, just kill it — the track is already at the right
  // final position, we just stop the transition immediately.
  if (animating.value) {
    animating.value = false
  }

  dragging.value = true
  dragOffset.value = 0
  containerRef.value?.setPointerCapture(e.pointerId)
}

function onPointerMove(e) {
  if (isPinching) return

  if (isZoomDragging && isZoomed.value) {
    zoomX.value += e.clientX - touchStartX
    zoomY.value += e.clientY - touchStartY
    touchStartX = e.clientX
    touchStartY = e.clientY
    return
  }

  if (!dragging.value) return

  const dx = e.clientX - touchStartX
  const dy = e.clientY - touchStartY

  if (!swipeLocked && (Math.abs(dx) > 8 || Math.abs(dy) > 8)) {
    swipeLocked = true
    if (Math.abs(dy) > Math.abs(dx)) {
      dragging.value = false
      return
    }
  }

  if (swipeLocked) {
    let x = dx
    // Rubber-band at edges
    if ((!hasPrev.value && dx > 0) || (!hasNext.value && dx < 0)) {
      x = dx * 0.25
    }
    dragOffset.value = x
  }
}

function onPointerUp(e) {
  if (isZoomDragging) { isZoomDragging = false; return }
  if (!dragging.value) return

  const dx = dragOffset.value
  const vw = window.innerWidth
  const threshold = vw * 0.15

  // End drag mode first — this stops the pixel-based positioning
  dragging.value = false
  dragOffset.value = 0

  if (dx > threshold && hasPrev.value) {
    // Navigate to previous — the track animates from current position
    mediaError.value = false
    animating.value = true
    activeIndex.value = activeIndex.value - 1
    resetZoom()
    updateBackground(activeIndex.value)
    emit('navigate', activeIndex.value)
  } else if (dx < -threshold && hasNext.value) {
    // Navigate to next
    mediaError.value = false
    animating.value = true
    activeIndex.value = activeIndex.value + 1
    resetZoom()
    updateBackground(activeIndex.value)
    emit('navigate', activeIndex.value)
  } else {
    // Snap back — animate from wherever drag left us back to center
    animating.value = true
    // animating gets cleared by transitionend, or by timeout as fallback
    setTimeout(() => { animating.value = false }, 300)
  }
}

// ---- Pinch-to-zoom ----

function onTouchStart(e) {
  if (e.touches.length === 2) {
    isPinching = true
    dragging.value = false
    dragOffset.value = 0
    touchStartDist = getTouchDist(e)
    touchStartScale = scale.value
  }
}

function onTouchMove(e) {
  if (e.touches.length === 2 && isPinching) {
    e.preventDefault()
    scale.value = Math.max(0.5, Math.min(5, touchStartScale * (getTouchDist(e) / touchStartDist)))
  }
}

function onTouchEnd(e) {
  if (isPinching && e.touches.length < 2) {
    isPinching = false
    if (scale.value < 1) { scale.value = 1; zoomX.value = 0; zoomY.value = 0 }
  }
}

function getTouchDist(e) {
  const dx = e.touches[0].clientX - e.touches[1].clientX
  const dy = e.touches[0].clientY - e.touches[1].clientY
  return Math.sqrt(dx * dx + dy * dy)
}

// Mouse wheel zoom
function onWheel(e) {
  if (isVideo.value) return
  e.preventDefault()
  const delta = e.deltaY > 0 ? 0.9 : 1.1
  scale.value = Math.max(0.5, Math.min(5, scale.value * delta))
  if (scale.value <= 1) { zoomX.value = 0; zoomY.value = 0 }
}

// ---- Download ----

function downloadCurrent() {
  const photo = displayPhoto.value
  if (!photo) return
  const url = mediaUrl(photo.id)
  const a = document.createElement('a')
  a.href = url
  a.download = photo.filename || 'download'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

// ---- Share (Web Share API) ----

const canShare = ref(false)
const sharing = ref(false)

function checkShareSupport() {
  canShare.value = !!navigator.share && !!navigator.canShare
}

async function shareCurrent() {
  const photo = displayPhoto.value
  if (!photo || sharing.value) return
  sharing.value = true

  try {
    const url = mediaUrl(photo.id)
    const response = await fetch(url)
    const blob = await response.blob()
    const ext = photo.filename?.split('.').pop() || (isVideo.value ? 'mp4' : 'jpg')
    const mime = blob.type || (isVideo.value ? 'video/mp4' : 'image/jpeg')
    const file = new File([blob], photo.filename || `photo.${ext}`, { type: mime })

    if (navigator.canShare && navigator.canShare({ files: [file] })) {
      await navigator.share({
        files: [file],
        title: photo.filename,
      })
    } else {
      await navigator.share({
        title: photo.filename,
        url: window.location.origin + url,
      })
    }
  } catch (e) {
    if (e.name !== 'AbortError') {
      console.warn('Share failed:', e)
    }
  } finally {
    sharing.value = false
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeyDown)
  checkShareSupport()
})
onUnmounted(() => document.removeEventListener('keydown', onKeyDown))
</script>

<template>
  <div class="viewer-overlay" :class="{ closing }" @click.self="closeViewer">
    <!-- Fuzzy background — two permanent layers that crossfade via opacity -->
    <div class="fuzzy-bg-layer">
      <div
        class="fuzzy-background"
        :class="{ 'bg-visible': bgShowA }"
        :style="{ backgroundImage: bgLayerA ? `url(${bgLayerA})` : 'none' }"
      ></div>
      <div
        class="fuzzy-background"
        :class="{ 'bg-visible': !bgShowA }"
        :style="{ backgroundImage: bgLayerB ? `url(${bgLayerB})` : 'none' }"
      ></div>
    </div>

    <!-- Top bar -->
    <div class="viewer-top">
      <button class="viewer-btn" @click="closeViewer" title="Close">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
      <div class="viewer-info">
        <span class="viewer-filename">{{ displayPhoto?.filename }}</span>
        <span class="viewer-date">{{ photoDate }}</span>
      </div>
      <div class="viewer-spacer"></div>
      <div class="viewer-actions">
        <button v-if="canShare" class="viewer-btn" @click.stop="shareCurrent" :disabled="sharing" title="Share">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
            <polyline points="16 6 12 2 8 6" />
            <line x1="12" y1="2" x2="12" y2="15" />
          </svg>
        </button>
        <button class="viewer-btn" @click.stop="downloadCurrent" title="Download">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
            <polyline points="7 10 12 15 17 10" />
            <line x1="12" y1="15" x2="12" y2="3" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Navigation arrows -->
    <button v-if="hasPrev" class="nav-btn nav-prev" @click.stop="goPrev" title="Previous">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <path d="M15 18l-6-6 6-6" />
      </svg>
    </button>
    <button v-if="hasNext" class="nav-btn nav-next" @click.stop="goNext" title="Next">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
        <path d="M9 18l6-6-6-6" />
      </svg>
    </button>

    <!-- Slide track — keyed by photo ID, no src mutation ever -->
    <div
      class="viewer-media"
      ref="containerRef"
      @dblclick="toggleZoom"
      @pointerdown.prevent="onPointerDown"
      @pointermove="onPointerMove"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @touchstart="onTouchStart"
      @touchmove="onTouchMove"
      @touchend="onTouchEnd"
      @wheel.prevent="onWheel"
    >
      <div
        class="slide-track"
        :style="trackStyle"
        @transitionend="onTransitionEnd"
      >
        <div
          v-for="slide in visibleSlides"
          :key="slide.id"
          class="slide-panel"
          :style="{ left: (slide.index * 100) + '%' }"
        >
          <!--
            Every slide always renders the same <img> element — keyed by
            photo ID, so Vue never swaps it out or changes its src.
            The center slide gets zoom styling; side slides get none.
            Video gets a <video> element OVER the img (img acts as poster).
            Error placeholder only shows on center slide.
          -->

          <!-- Error placeholder (center only) -->
          <div v-if="slide.isCenter && mediaError" class="viewer-placeholder">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
              <rect x="3" y="3" width="18" height="18" rx="2" />
              <circle cx="8.5" cy="8.5" r="1.5" />
              <path d="M21 15l-5-5L5 21" />
              <line x1="3" y1="3" x2="21" y2="21" />
            </svg>
            <span class="viewer-placeholder-label">File not found</span>
          </div>

          <!-- The image — always present, never conditionally destroyed -->
          <img
            v-show="!(slide.isCenter && mediaError) && !(slide.isCenter && slide.isVideo)"
            :src="slide.src"
            :alt="slide.isCenter ? displayPhoto?.filename : undefined"
            class="viewer-img"
            :style="slide.isCenter && isZoomed ? {
              transform: `translate(${zoomX}px, ${zoomY}px) scale(${scale})`,
              cursor: 'grab',
            } : (slide.isCenter ? { cursor: 'zoom-in' } : {})"
            draggable="false"
            @error="(e) => slide.isCenter ? onMediaError() : e.target.style.display='none'"
          />

          <!-- Video overlay (center only, replaces img visually) -->
          <video
            v-if="slide.isCenter && slide.isVideo && !mediaError"
            :src="srcForIndex(slide.index)"
            :poster="thumbSrc"
            class="viewer-video"
            controls
            autoplay
            playsinline
            @error="onMediaError"
          />
        </div>
      </div>
    </div>

    <!-- Bottom bar: counter -->
    <div class="viewer-bottom">
      <span class="viewer-counter">
        {{ activeIndex + 1 }} / {{ photos?.length ?? 0 }}
      </span>
    </div>
  </div>
</template>

<style scoped>
.viewer-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: var(--bg-overlay);
  display: flex;
  flex-direction: column;
  animation: viewerOpen 200ms ease;
  user-select: none;
  -webkit-user-select: none;
  overflow: hidden;
  transition: opacity 250ms ease, transform 250ms ease;
}

.viewer-overlay.closing {
  opacity: 0;
  transform: scale(0.95);
  pointer-events: none;
}

@keyframes viewerOpen {
  from { opacity: 0; transform: scale(1.03); }
  to { opacity: 1; transform: scale(1); }
}

/* ---- Fuzzy background with crossfade ---- */
.fuzzy-bg-layer {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  background: #000;
}

.fuzzy-background {
  position: absolute;
  inset: -60px;
  background-size: cover;
  background-position: center;
  filter: blur(30px) brightness(0.3) saturate(1.2);
  opacity: 0;
  transition: opacity 350ms ease;
}

.fuzzy-background.bg-visible {
  opacity: 1;
}

/* ---- Top bar ---- */
.viewer-top {
  display: flex;
  align-items: center;
  gap: var(--gap-md);
  padding: var(--gap-md) var(--gap-lg);
  padding-top: calc(var(--safe-top) + var(--gap-md));
  background: linear-gradient(to bottom, rgba(0,0,0,0.6), transparent);
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  z-index: 10;
}

.viewer-btn {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255,255,255,0.1);
  border: none;
  border-radius: 50%;
  color: white;
  cursor: pointer;
  transition: background var(--transition-fast);
  flex-shrink: 0;
}

.viewer-btn:hover {
  background: rgba(255,255,255,0.2);
}

.viewer-btn svg {
  width: 20px;
  height: 20px;
}

.viewer-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.viewer-filename {
  font-size: 0.85rem;
  color: white;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.viewer-date {
  font-size: 0.75rem;
  color: rgba(255,255,255,0.6);
}

.viewer-spacer {
  flex: 1;
}

.viewer-actions {
  display: flex;
  align-items: center;
  gap: var(--gap-md);
}

.viewer-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* ---- Nav arrows ---- */
.nav-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0,0,0,0.4);
  border: none;
  border-radius: 50%;
  color: white;
  cursor: pointer;
  transition: background var(--transition-fast), opacity var(--transition-fast);
  opacity: 0.7;
}

.nav-btn:hover {
  background: rgba(0,0,0,0.7);
  opacity: 1;
}

.nav-btn svg {
  width: 24px;
  height: 24px;
}

.nav-prev {
  left: var(--gap-lg);
}

.nav-next {
  right: var(--gap-lg);
}

/* ---- Slide track (keyed slides, no src mutation) ---- */
.viewer-media {
  flex: 1;
  position: relative;
  z-index: 1;
  overflow: hidden;
  touch-action: none;
}

.slide-track {
  position: absolute;
  inset: 0;
  will-change: transform;
}

.slide-panel {
  position: absolute;
  top: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.viewer-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  transition: transform 200ms ease, box-shadow 200ms ease;
  will-change: transform;
  pointer-events: none;
}

.slide-panel:nth-child(3) .viewer-img {
  box-shadow: 0 30px 100px #000;
}

.viewer-video {
  max-width: 100%;
  max-height: 100%;
  outline: none;
  border-radius: var(--radius-md);
}

/* ---- Missing / broken media placeholder ---- */
.viewer-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  color: var(--text-muted);
  opacity: 0.45;
  animation: fadeIn 300ms ease;
}

.viewer-placeholder svg {
  width: 72px;
  height: 72px;
}

.viewer-placeholder-label {
  font-size: 0.85rem;
  font-weight: 500;
  letter-spacing: 0.02em;
}

/* ---- Bottom bar ---- */
.viewer-bottom {
  display: flex;
  justify-content: center;
  padding: var(--gap-md);
  padding-bottom: calc(var(--safe-bottom) + var(--gap-md));
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 10;
  background: linear-gradient(to top, rgba(0,0,0,0.4), transparent);
  pointer-events: none;
}

.viewer-counter {
  font-size: 0.8rem;
  color: rgba(255,255,255,0.6);
  font-variant-numeric: tabular-nums;
}

/* ---- Mobile ---- */
@media (max-width: 768px) {
  .nav-btn { display: none; }
}
@media (hover: none) {
  .nav-btn { display: none; }
}
</style>
