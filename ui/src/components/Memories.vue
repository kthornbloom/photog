<script setup>
import { ref, onMounted } from 'vue'
import { fetchMemories, thumbUrl } from '../api.js'

const emit = defineEmits(['open'])

const photos = ref([])
const loadedIds = ref({})
const errorIds = ref({})

function yearsAgo(dateStr) {
  const taken = new Date(dateStr)
  const now = new Date()
  const years = Math.floor((now - taken) / (365.25 * 24 * 60 * 60 * 1000))
  if (years >= 1) return `${years} year${years === 1 ? '' : 's'} ago`
  const months = Math.floor((now - taken) / (30.44 * 24 * 60 * 60 * 1000))
  if (months >= 1) return `${months} month${months === 1 ? '' : 's'} ago`
  return 'Earlier this year'
}

function onImageLoad(photoId) {
  loadedIds.value = { ...loadedIds.value, [photoId]: true }
}

function onImageError(photoId) {
  errorIds.value = { ...errorIds.value, [photoId]: true }
  loadedIds.value = { ...loadedIds.value, [photoId]: true }
}

function openPhoto(photo) {
  const idx = photos.value.findIndex(p => p.id === photo.id)
  emit('open', photo, photos.value, idx >= 0 ? idx : 0)
}

function isVideo(photo) {
  return photo.type === 'video'
}

onMounted(async () => {
  try {
    const res = await fetchMemories()
    const now = Date.now()
    const fiveYearsMs = 5 * 365.25 * 24 * 60 * 60 * 1000
    // Only show memories from 5+ years ago
    photos.value = (res.photos || []).filter(p => now - new Date(p.taken_at) >= fiveYearsMs)
  } catch (e) {
    console.warn('Could not load memories:', e)
  }
})
</script>

<template>
  <section v-if="photos.length > 0" class="memories">
    <div class="memories-header">
      <h2 class="memories-label">Memories</h2>
      <span class="memories-subtitle">5+ years ago</span>
    </div>

    <div class="memories-scroll">
      <div
        v-for="photo in photos"
        :key="photo.id"
        class="memory-item"
        :class="{
          'is-video': isVideo(photo),
          'is-loading': !loadedIds[photo.id],
          'is-loaded': loadedIds[photo.id],
          'is-broken': errorIds[photo.id],
        }"
        @click="openPhoto(photo)"
      >
        <div v-if="errorIds[photo.id]" class="memory-placeholder">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="18" height="18" rx="2" />
            <circle cx="8.5" cy="8.5" r="1.5" />
            <path d="M21 15l-5-5L5 21" />
          </svg>
        </div>
        <img
          v-if="!errorIds[photo.id]"
          :src="thumbUrl(photo.id, 'sm')"
          :alt="photo.filename"
          class="memory-thumb"
          loading="lazy"
          decoding="async"
          @load="onImageLoad(photo.id)"
          @error="onImageError(photo.id)"
        />
        <div class="memory-overlay">
          <span class="memory-date">{{ yearsAgo(photo.taken_at) }}</span>
        </div>
        <div v-if="isVideo(photo) && !errorIds[photo.id]" class="memory-video-badge">
          <svg viewBox="0 0 24 24" fill="currentColor">
            <path d="M8 5v14l11-7z" />
          </svg>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.memories {
  padding: var(--gap-lg) var(--gap-lg) var(--gap-md);
  border-bottom: 1px solid var(--border);
}

.memories-header {
  display: flex;
  align-items: baseline;
  gap: var(--gap-md);
  margin-bottom: var(--gap-md);
}

.memories-label {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.memories-subtitle {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.memories-scroll {
  display: flex;
  gap: var(--gap-md);
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: var(--gap-sm);
  -webkit-overflow-scrolling: touch;
}

.memories-scroll::-webkit-scrollbar {
  height: 4px;
}

.memories-scroll::-webkit-scrollbar-thumb {
  background: var(--text-muted);
  border-radius: 2px;
}

.memory-item {
  position: relative;
  flex-shrink: 0;
  width: 120px;
  height: 120px;
  border-radius: var(--radius-md);
  overflow: hidden;
  cursor: pointer;
  background: var(--bg-surface);
}

.memory-item::after {
  content: '';
  position: absolute;
  inset: 0;
  opacity: 0;
  background: rgba(255, 255, 255, 0.06);
  transition: opacity var(--transition-fast);
}

.memory-item:hover::after {
  opacity: 1;
}

.memory-item:active {
  transform: scale(0.97);
  transition: transform 80ms ease;
}

.memory-thumb {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.memory-item.is-loading {
  background: var(--bg-surface);
  animation: pulse 1.5s ease-in-out infinite;
}

.memory-thumb {
  opacity: 0;
  transition: opacity 300ms ease;
}

.memory-item.is-loaded .memory-thumb {
  opacity: 1;
}

.memory-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: var(--text-muted);
  opacity: 0.35;
}

.memory-placeholder svg {
  width: 36%;
  height: 36%;
}

.memory-overlay {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 24px 6px 6px;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.65), transparent);
  pointer-events: none;
}

.memory-date {
  font-size: 0.65rem;
  font-weight: 600;
  color: var(--text-primary);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
}

.memory-video-badge {
  position: absolute;
  bottom: 26px;
  right: 6px;
  width: 20px;
  height: 20px;
  background: rgba(0, 0, 0, 0.65);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  pointer-events: none;
}

.memory-video-badge svg {
  width: 12px;
  height: 12px;
}

@media (min-width: 768px) {
  .memory-item {
    width: 140px;
    height: 140px;
  }
}
</style>
