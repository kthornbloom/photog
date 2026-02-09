<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import AppHeader from './components/AppHeader.vue'
import Timeline from './components/Timeline.vue'
import PhotoViewer from './components/PhotoViewer.vue'
import IndexingBanner from './components/IndexingBanner.vue'
import { fetchStats, fetchIndexProgress } from './api.js'

const stats = ref(null)
const indexProgress = ref(null)
const viewerPhoto = ref(null)
const viewerPhotos = ref([])
const viewerIndex = ref(0)
const timelineRef = ref(null)

let progressInterval = null

onMounted(async () => {
  try {
    stats.value = await fetchStats()
  } catch (e) {
    console.warn('Could not load stats:', e)
  }
  pollProgress()
})

onUnmounted(() => {
  if (progressInterval) clearInterval(progressInterval)
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
        }
      } catch { /* ignore */ }
    }, 2000)
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
  <IndexingBanner :progress="indexProgress" />
  <Timeline ref="timelineRef" @open="openViewer" />
  <PhotoViewer
    v-if="viewerPhoto"
    :photo="viewerPhoto"
    :photos="viewerPhotos"
    :current-index="viewerIndex"
    @close="closeViewer"
    @navigate="navigateViewer"
  />
</template>
