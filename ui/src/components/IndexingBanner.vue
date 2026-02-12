<script setup>
import { computed } from 'vue'

const props = defineProps({
  progress: Object,
  pregenProgress: Object,
})

// ---- Indexing phase ----
const indexPercent = computed(() => {
  if (!props.progress || !props.progress.total) return 0
  return Math.round((props.progress.processed / props.progress.total) * 100)
})

const indexLabel = computed(() => {
  if (!props.progress) return ''
  const p = props.progress
  if (!p.running && p.finished_at) {
    return `Indexing complete — ${p.processed.toLocaleString()} files (${p.files_per_sec.toFixed(0)} files/sec)`
  }
  return `Indexing... ${p.processed.toLocaleString()} / ${p.total.toLocaleString()} files`
})

const showIndex = computed(() => {
  if (!props.progress) return false
  return props.progress.running
})

// ---- Pregen phase ----
const pregenProcessed = computed(() => {
  if (!props.pregenProgress) return 0
  const p = props.pregenProgress
  return p.generated + p.skipped + p.errors
})

const pregenPercent = computed(() => {
  if (!props.pregenProgress || !props.pregenProgress.total) return 0
  return Math.round((pregenProcessed.value / props.pregenProgress.total) * 100)
})

const pregenEta = computed(() => {
  if (!props.pregenProgress || !props.pregenProgress.eta_seconds) return ''
  const secs = props.pregenProgress.eta_seconds
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.round(secs / 60)}m`
  const h = Math.floor(secs / 3600)
  const m = Math.round((secs % 3600) / 60)
  return `${h}h ${m}m`
})

const pregenLabel = computed(() => {
  if (!props.pregenProgress) return ''
  const p = props.pregenProgress
  if (!p.running && p.finished_at) {
    return `Thumbnails ready — ${p.generated.toLocaleString()} generated, ${p.skipped.toLocaleString()} cached`
  }
  const eta = pregenEta.value ? ` — ~${pregenEta.value} remaining` : ''
  return `Generating thumbnails... ${pregenProcessed.value.toLocaleString()} / ${p.total.toLocaleString()}${eta}`
})

const showPregen = computed(() => {
  if (!props.pregenProgress) return false
  const p = props.pregenProgress
  // Show while running, or briefly after finishing (parent will stop polling)
  return p.running || (p.finished_at && p.generated > 0)
})

const showBanner = computed(() => showIndex.value || showPregen.value)

// Determine which phase to display (indexing takes priority)
const activePhase = computed(() => {
  if (showIndex.value) return 'index'
  if (showPregen.value) return 'pregen'
  return null
})

const percent = computed(() => {
  if (activePhase.value === 'index') return indexPercent.value
  if (activePhase.value === 'pregen') return pregenPercent.value
  return 0
})

const label = computed(() => {
  if (activePhase.value === 'index') return indexLabel.value
  if (activePhase.value === 'pregen') return pregenLabel.value
  return ''
})

const isRunning = computed(() => {
  if (activePhase.value === 'index') return props.progress?.running
  if (activePhase.value === 'pregen') return props.pregenProgress?.running
  return false
})
</script>

<template>
  <div class="indexing-banner" v-if="showBanner">
    <div class="banner-content">
      <div class="spinner" v-if="isRunning"></div>
      <svg v-else class="check-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
      <span class="banner-text">{{ label }}</span>
      <span class="banner-percent" v-if="isRunning">{{ percent }}%</span>
    </div>
    <div class="progress-bar">
      <div class="progress-fill" :style="{ width: percent + '%' }"></div>
    </div>
  </div>
</template>

<style scoped>
.indexing-banner {
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  animation: slideUp var(--transition-normal);
}

.banner-content {
  display: flex;
  align-items: center;
  gap: var(--gap-md);
  padding: var(--gap-sm) var(--gap-lg);
  font-size: 0.78rem;
  color: var(--text-secondary);
}

.spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.check-icon {
  color: var(--accent);
  flex-shrink: 0;
}

.banner-text {
  flex: 1;
}

.banner-percent {
  color: var(--accent);
  font-variant-numeric: tabular-nums;
  font-weight: 500;
}

.progress-bar {
  height: 2px;
  background: var(--border);
}

.progress-fill {
  height: 100%;
  background: var(--accent);
  transition: width var(--transition-normal);
}
</style>
