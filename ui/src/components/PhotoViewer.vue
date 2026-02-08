<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { mediaUrl, thumbUrl } from '../api.js'

const props = defineProps({
  photo: Object,
  photos: Array,
  currentIndex: Number,
})

const emit = defineEmits(['close', 'navigate'])

const containerRef = ref(null)

// ---------------------------------------------------------------
// Internal display state — decoupled from props during animation
// We control exactly what's shown in each slide panel so that
// prop changes never cause a flash mid-transition.
// ---------------------------------------------------------------
const displayIndex = ref(0)         // the index currently centered
const isAnimating = ref(false)      // true while a slide transition is playing

// The three image sources rendered in the slide panels
const prevSrc = ref('')
const currSrc = ref('')
const nextSrc = ref('')

// Slide track
const dragX = ref(0)
const dragging = ref(false)
const slideTransition = ref(false)

// Zoom/pan
const scale = ref(1)
const zoomX = ref(0)
const zoomY = ref(0)
const isZoomed = computed(() => scale.value > 1.05)

// Background crossfade
const bgSrc = ref('')
const bgNextSrc = ref('')
const bgFading = ref(false)

// Pointer tracking
let touchStartX = 0
let touchStartY = 0
let touchStartDist = 0
let touchStartScale = 1
let isPinching = false
let isZoomDragging = false
let swipeLocked = false

// ---- Helpers ----

function srcForIndex(i) {
  if (i < 0 || i >= (props.photos?.length ?? 0)) return ''
  return mediaUrl(props.photos[i].id)
}

function bgForIndex(i) {
  if (i < 0 || i >= (props.photos?.length ?? 0)) return ''
  return thumbUrl(props.photos[i].id, 'lg')
}

function updateSlides(idx) {
  displayIndex.value = idx
  prevSrc.value = srcForIndex(idx - 1)
  currSrc.value = srcForIndex(idx)
  nextSrc.value = srcForIndex(idx + 1)
}

const hasPrev = computed(() => displayIndex.value > 0)
const hasNext = computed(() => displayIndex.value < (props.photos?.length ?? 0) - 1)

const isVideo = computed(() => {
  const i = displayIndex.value
  return props.photos?.[i]?.type === 'video'
})

const displayPhoto = computed(() => props.photos?.[displayIndex.value])

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

// ---- Initialize once from props ----
// We only use the prop for the *initial* index. After that, the viewer
// manages displayIndex internally via commitNav(). We do NOT watch
// props.currentIndex for changes — that would re-trigger updateSlides
// after our own emit('navigate') round-trips through the parent,
// causing a flash of the old image.

let initialized = false
watch(() => props.currentIndex, (idx) => {
  if (!initialized) {
    initialized = true
    updateSlides(idx)
    updateBackground(idx)
  }
}, { immediate: true })

function updateBackground(idx) {
  const newBg = bgForIndex(idx)
  if (bgSrc.value && bgSrc.value !== newBg) {
    bgNextSrc.value = newBg
    bgFading.value = true
    setTimeout(() => {
      bgSrc.value = newBg
      bgFading.value = false
      bgNextSrc.value = ''
    }, 200)
  } else {
    bgSrc.value = newBg
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

function commitNav(newIndex) {
  // Called AFTER the slide animation has finished.
  // The track is currently translated off-screen, showing the next/prev panel.

  // Step 1: Copy the incoming image src into the CENTER panel.
  //         This is the image the user is already looking at (in the next or prev slot).
  //         Visually nothing changes because the track is still off-screen.
  currSrc.value = srcForIndex(newIndex)
  displayIndex.value = newIndex

  // Step 2: Kill the transition and snap the track back to center.
  //         The center panel now shows the correct (new) image, so no flash.
  setTimeout(() => {
    slideTransition.value = false
    dragX.value = 0
    console.log('position reset');


      prevSrc.value = srcForIndex(newIndex - 1)
      nextSrc.value = srcForIndex(newIndex + 1)
      isAnimating.value = false
      console.log('images reset');
    },195);

  //updateBackground(newIndex)
  resetZoom()
  emit('navigate', newIndex)
}

function animateNav(direction) {
  // direction: -1 = go next (slide left), 1 = go prev (slide right)
  const newIndex = displayIndex.value - direction
  if (newIndex < 0 || newIndex >= props.photos.length) return
  if (isAnimating.value) return

  isAnimating.value = true
  slideTransition.value = true

  nextTick(() => {
    dragX.value = direction * window.innerWidth
  })

  setTimeout(() => {
    commitNav(newIndex)
  }, 300)
}

function goPrev() {
  if (hasPrev.value) animateNav(1)
}

function goNext() {
  if (hasNext.value) animateNav(-1)
}

function onKeyDown(e) {
  if (isAnimating.value) return
  switch (e.key) {
    case 'Escape': emit('close'); break
    case 'ArrowLeft': goPrev(); break
    case 'ArrowRight': goNext(); break
  }
}

// ---- Pointer / swipe handling ----

function onPointerDown(e) {
  if (isAnimating.value) return
  if (e.pointerType === 'touch' && !e.isPrimary) return

  touchStartX = e.clientX
  touchStartY = e.clientY
  swipeLocked = false
  isPinching = false

  if (isZoomed.value) {
    isZoomDragging = true
    return
  }

  dragging.value = true
  containerRef.value?.setPointerCapture(e.pointerId)
}

function onPointerMove(e) {
  if (isAnimating.value || isPinching) return

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
    if ((!hasPrev.value && dx > 0) || (!hasNext.value && dx < 0)) {
      x = dx * 0.25
    }
    dragX.value = x
  }
}

function onPointerUp(e) {
  if (isZoomDragging) { isZoomDragging = false; return }
  if (!dragging.value) return
  dragging.value = false

  const dx = dragX.value
  const vw = window.innerWidth
  const threshold = vw * 0.15

  if (dx > threshold && hasPrev.value) {
    isAnimating.value = true
    slideTransition.value = true
    dragX.value = vw
    setTimeout(() => commitNav(displayIndex.value - 1), 250)
  } else if (dx < -threshold && hasNext.value) {
    isAnimating.value = true
    slideTransition.value = true
    dragX.value = -vw
    setTimeout(() => commitNav(displayIndex.value + 1), 250)
  } else {
    // Snap back
    slideTransition.value = true
    dragX.value = 0
    setTimeout(() => { slideTransition.value = false }, 250)
  }
}

// ---- Pinch-to-zoom ----

function onTouchStart(e) {
  if (e.touches.length === 2) {
    isPinching = true
    dragging.value = false
    dragX.value = 0
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

onMounted(() => document.addEventListener('keydown', onKeyDown))
onUnmounted(() => document.removeEventListener('keydown', onKeyDown))
</script>

<template>
  <div class="viewer-overlay" @click.self="$emit('close')">
    <!-- Fuzzy background with crossfade -->
    <div class="fuzzy-bg-layer">
      <div
        class="fuzzy-background"
        :style="{ backgroundImage: `url(${bgSrc})` }"
      ></div>
      <div
        v-if="bgFading && bgNextSrc"
        class="fuzzy-background fuzzy-bg-enter"
        :style="{ backgroundImage: `url(${bgNextSrc})` }"
      ></div>
    </div>

    <!-- Top bar -->
    <div class="viewer-top">
      <button class="viewer-btn" @click="$emit('close')" title="Close">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      </button>
      <div class="viewer-info">
        <span class="viewer-filename">{{ displayPhoto?.filename }}</span>
        <span class="viewer-date">{{ photoDate }}</span>
      </div>
      <div class="viewer-spacer"></div>
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

    <!-- Slide track: prev + current + next -->
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
        :class="{ 'slide-animate': slideTransition }"
        :style="{ transform: `translateX(${dragX}px)` }"
      >
        <!-- Previous image (off-screen left) -->
        <div class="slide-panel slide-prev" v-if="prevSrc">
          <img :src="prevSrc" class="viewer-img" draggable="false" />
        </div>

        <!-- Current image -->
        <div class="slide-panel slide-current">
          <img
            v-if="!isVideo"
            :src="currSrc"
            :alt="displayPhoto?.filename"
            class="viewer-img"
            :style="isZoomed ? {
              transform: `translate(${zoomX}px, ${zoomY}px) scale(${scale})`,
              cursor: 'grab',
            } : {
              cursor: 'zoom-in',
            }"
            draggable="false"
          />
          <video
            v-else
            :src="currSrc"
            :poster="thumbSrc"
            class="viewer-video"
            controls
            autoplay
            playsinline
          />
        </div>

        <!-- Next image (off-screen right) -->
        <div class="slide-panel slide-next" v-if="nextSrc">
          <img :src="nextSrc" class="viewer-img" draggable="false" />
        </div>
      </div>
    </div>

    <!-- Bottom bar: counter -->
    <div class="viewer-bottom">
      <span class="viewer-counter">
        {{ displayIndex + 1 }} / {{ photos?.length ?? 0 }}
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
  animation: fadeIn 150ms ease;
  user-select: none;
  -webkit-user-select: none;
  overflow: hidden;
}

/* ---- Fuzzy background with crossfade ---- */
.fuzzy-bg-layer {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
}

.fuzzy-background {
  position: absolute;
  inset: -60px;
  background-size: cover;
  background-position: center;
  filter: blur(30px) brightness(0.3) saturate(1.2);
  transition: opacity 200ms ease;
}

.fuzzy-bg-enter {
  animation: bgFadeIn 200ms ease forwards;
}

@keyframes bgFadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
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

/* ---- Slide track ---- */
.viewer-media {
  flex: 1;
  position: relative;
  z-index: 1;
  overflow: hidden;
  touch-action: none;
}

.slide-track {
  display: flex;
  align-items: center;
  height: 100%;
  will-change: transform;
}

.slide-track.slide-animate {
  transition: transform 100ms cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.slide-panel {
  flex: 0 0 100%;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.slide-prev {
  margin-left: -100%;
}

.viewer-img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  transition: transform 200ms ease;
  will-change: transform;
  pointer-events: none;
}

.viewer-video {
  max-width: 100%;
  max-height: 100%;
  outline: none;
  border-radius: var(--radius-md);
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
