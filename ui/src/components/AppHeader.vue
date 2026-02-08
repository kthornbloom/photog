<script setup>
defineProps({
  stats: Object,
})

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}
</script>

<template>
  <header class="app-header">
    <div class="header-left">
      <img src="/logo.svg" class="logo" alt="Photog">
    </div>
    <div class="header-right" v-if="stats">
      <span class="stat" v-if="stats.total_photos">
        {{ stats.total_photos.toLocaleString() }} photos
      </span>
      <span class="stat" v-if="stats.total_videos">
        {{ stats.total_videos.toLocaleString() }} videos
      </span>
      <span class="stat muted" v-if="stats.total_size">
        {{ formatSize(stats.total_size) }}
      </span>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--gap-md) var(--gap-lg);
  padding-top: calc(var(--safe-top) + var(--gap-md));
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
  z-index: 100;
  height: 55px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: var(--gap-md);
  padding: .5rem 0;
}

.logo {
  width: 80px;
  max-width: 100%;
}

.logo-icon {
  width: 22px;
  height: 22px;
  color: var(--accent);
}

.header-right {
  display: flex;
  align-items: center;
  gap: var(--gap-lg);
}

.stat {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.stat.muted {
  color: var(--text-muted);
}

@media (max-width: 480px) {
  .header-right {
    display: none;
  }
}
</style>
