<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import AppHeader from './components/AppHeader.vue'
import Timeline from './components/Timeline.vue'
import PhotoViewer from './components/PhotoViewer.vue'
import IndexingBanner from './components/IndexingBanner.vue'
import Memories from './components/Memories.vue'
import { fetchStats, fetchIndexProgress, fetchPregenProgress } from './api.js'

const stats = ref(null)
const indexProgress = ref(null)
const pregenProgress = ref(null)
const viewerPhoto = ref(null)
const viewerPhotos = ref([])
const viewerIndex = ref(0)
const timelineRef = ref(null)

let progressInterval = null
let pregenInterval = null

onMounted(async () => {
  try {
    stats.value = await fetchStats()
  } catch (e) {
    console.warn('Could not load stats:', e)
  }
  pollProgress()
  pollPregen()
})

onUnmounted(() => {
  if (progressInterval) clearInterval(progressInterval)
  if (pregenInterval) clearInterval(pregenInterval)
})

async function pollProgress() {
  try {
    indexProgress.value = await fetchIndexProgress()
  } catch { /* ignore */ }

  if (indexProgress.value?.running) {
    progressInterval = setInterval(async () => {
      try {
        indexProgress.value = await fetchIndexProgress()
        if (!indexProgress.value.running) {
          clearInterval(progressInterval)
          progressInterval = null
          stats.value = await fetchStats()
          // Indexing just finished — pregen will start soon, begin polling it
          pollPregen()
        }
      } catch { /* ignore */ }
    }, 2000)
  }
}

async function pollPregen() {
  try {
    pregenProgress.value = await fetchPregenProgress()
  } catch { /* ignore */ }

  // If already polling, don't start another interval
  if (pregenInterval) return

  if (pregenProgress.value?.running) {
    pregenInterval = setInterval(async () => {
      try {
        pregenProgress.value = await fetchPregenProgress()
        if (!pregenProgress.value.running) {
          clearInterval(pregenInterval)
          pregenInterval = null
        }
      } catch { /* ignore */ }
    }, 3000)
  }
}

function openViewer(photo, photos, index) {
  viewerPhoto.value = photo
  viewerPhotos.value = photos
  viewerIndex.value = index
}

function closeViewer() {
  viewerPhoto.value = null
}

function navigateViewer(newIndex) {
  if (newIndex >= 0 && newIndex < viewerPhotos.value.length) {
    viewerIndex.value = newIndex
    viewerPhoto.value = viewerPhotos.value[newIndex]
    // Keep the grid scrolled to the current photo so it's visible when the viewer closes
    timelineRef.value?.scrollToPhoto(viewerPhotos.value[newIndex].id)
  }
}
</script>

<template>
  <AppHeader :stats="stats" />
  <IndexingBanner :progress="indexProgress" :pregen-progress="pregenProgress" />
  <Timeline ref="timelineRef" @open="openViewer">
    <Memories @open="openViewer" />
  </Timeline>
  <PhotoViewer
    v-if="viewerPhoto"
    :photo="viewerPhoto"
    :photos="viewerPhotos"
    :current-index="viewerIndex"
    @close="closeViewer"
    @navigate="navigateViewer"
  />
</template>
