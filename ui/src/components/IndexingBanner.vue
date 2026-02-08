<script setup>
import { computed } from 'vue'

const props = defineProps({
  progress: Object,
})

const percent = computed(() => {
  if (!props.progress || !props.progress.total) return 0
  return Math.round((props.progress.processed / props.progress.total) * 100)
})

const label = computed(() => {
  if (!props.progress) return ''
  const p = props.progress
  if (!p.running && p.finished_at) {
    return `Indexing complete — ${p.processed} files (${p.files_per_sec.toFixed(0)} files/sec)`
  }
  return `Indexing... ${p.processed.toLocaleString()} / ${p.total.toLocaleString()} files`
})
</script>

<template>
  <div class="indexing-banner" v-if="progress && (progress.running || (progress.finished_at && percent < 100))">
    <div class="banner-content">
      <div class="spinner" v-if="progress.running"></div>
      <span class="banner-text">{{ label }}</span>
      <span class="banner-percent">{{ percent }}%</span>
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
