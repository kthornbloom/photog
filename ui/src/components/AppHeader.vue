<script setup>
import { ref, onUnmounted } from 'vue'
import { triggerIndex, fetchIndexProgress } from '../api.js'

defineProps({
  stats: Object,
})

function formatSize(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

// Settings modal
const showSettings = ref(false)
const quickUpdateRunning = ref(false)
const quickUpdateStatus = ref('')
let pollTimer = null
let reloadTimer = null

function scheduleReload() {
  if (reloadTimer) clearTimeout(reloadTimer)
  reloadTimer = setTimeout(() => {
    window.location.reload()
  }, 5000)
}

function openSettings() {
  showSettings.value = true
}

function closeSettings() {
  showSettings.value = false
}

function onOverlayClick(e) {
  if (e.target === e.currentTarget) closeSettings()
}

async function startQuickUpdate() {
  if (quickUpdateRunning.value) return
  quickUpdateRunning.value = true
  quickUpdateStatus.value = 'Starting...'

  try {
    const res = await triggerIndex()
    if (res.status === 'already_running') {
      quickUpdateStatus.value = 'Indexing already in progress...'
    } else {
      quickUpdateStatus.value = 'Scanning for changes...'
    }
    pollQuickUpdate()
  } catch (e) {
    quickUpdateStatus.value = `Error: ${e.message}`
    quickUpdateRunning.value = false
  }
}

function pollQuickUpdate() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(async () => {
    try {
      const progress = await fetchIndexProgress()
      if (progress.running) {
        const processed = progress.processed || 0
        const total = progress.total || 0
        const skipped = progress.skipped || 0
        const newFiles = processed - skipped
        quickUpdateStatus.value = `Scanning... ${processed}/${total} checked, ${newFiles} new`
      } else {
        const processed = progress.processed || 0
        const skipped = progress.skipped || 0
        const errors = progress.errors || 0
        const newFiles = processed - skipped - errors
        quickUpdateStatus.value = newFiles > 0
          ? `Done! Found ${newFiles} new file${newFiles === 1 ? '' : 's'}. Reloading...`
          : 'Done! Library is up to date. Reloading...'
        quickUpdateRunning.value = false
        clearInterval(pollTimer)
        pollTimer = null
        scheduleReload()
      }
    } catch {
      quickUpdateStatus.value = 'Done. Reloading...'
      quickUpdateRunning.value = false
      clearInterval(pollTimer)
      pollTimer = null
      scheduleReload()
    }
  }, 1500)
}

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  if (reloadTimer) clearTimeout(reloadTimer)
})
</script>

<template>
  <header class="app-header">
    <div class="header-left">
      <img src="/logo.svg" class="logo" alt="Photog">

      <button class="settings-btn" @click="openSettings" title="Settings">
        <svg xmlns="http://www.w3.org/2000/svg" width="26" height="26" fill="none" viewBox="0 0 24 24"><path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M11.7 14c-1.077 0-1.95-.895-1.95-2s.873-2 1.95-2 1.95.895 1.95 2c0 .53-.206 1.04-.571 1.414A1.926 1.926 0 0 1 11.7 14Z" clip-rule="evenodd"/><path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M16.884 16.063v-1.342c0-.332.129-.651.358-.886l.925-.949a1.276 1.276 0 0 0 0-1.772l-.925-.949a1.27 1.27 0 0 1-.358-.886V7.936c0-.692-.547-1.253-1.222-1.253h-1.309c-.324 0-.635-.132-.864-.367l-.925-.949a1.2 1.2 0 0 0-1.728 0l-.925.949c-.23.235-.54.367-.864.367h-1.31c-.324 0-.634.132-.864.367a1.27 1.27 0 0 0-.357.887v1.342c0 .332-.129.651-.358.886l-.925.949a1.276 1.276 0 0 0 0 1.772l.925.949c.23.235.358.554.358.886v1.342c0 .692.547 1.253 1.222 1.253h1.309c.324 0 .635.132.864.367l.925.949a1.2 1.2 0 0 0 1.728 0l.925-.949c.23-.235.54-.367.864-.367h1.308c.325 0 .636-.132.865-.367a1.27 1.27 0 0 0 .358-.886Z" clip-rule="evenodd"/></svg>
      </button>
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

  <!-- Settings Modal -->
  <Teleport to="body">
    <div v-if="showSettings" class="modal-overlay" @click="onOverlayClick">
      <div class="modal-panel">
        <div class="modal-header">
          <h2 class="modal-title">Settings</h2>
          <button class="modal-close" @click="closeSettings">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>

        <div class="modal-body">
          <div class="setting-section">
            <h3 class="setting-label">Library</h3>
            <p class="setting-desc">
              Scan for new or removed photos and videos. Already-indexed files are skipped automatically.
            </p>
            <button
              class="btn-primary"
              :disabled="quickUpdateRunning"
              @click="startQuickUpdate"
            >
              <svg v-if="quickUpdateRunning" class="btn-spinner" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 2v4m0 12v4m-7.07-3.93l2.83-2.83m8.48-8.48l2.83-2.83M2 12h4m12 0h4m-3.93 7.07l-2.83-2.83M7.76 7.76 4.93 4.93"/></svg>
              <span>{{ quickUpdateRunning ? 'Refreshing...' : 'Refresh Library' }}</span>
            </button>
            <p v-if="quickUpdateStatus" class="update-status">{{ quickUpdateStatus }}</p>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
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
  width: 120px;
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

/* ---- Settings button ---- */
.settings-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: color var(--transition-fast), background var(--transition-fast);
}

.settings-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

/* ---- Modal ---- */
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 500;
  background: #000;
  display: flex;
  align-items: center;
  justify-content: center;
  animation: fadeIn 150ms ease;
}

.modal-panel {
  background: var(--bg-secondary);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  width: 380px;
  max-width: 92vw;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 16px 48px rgba(0, 0, 0, 0.6);
  animation: slideUp 200ms ease;
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--gap-lg) var(--gap-lg) var(--gap-md);
}

.modal-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.modal-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  background: transparent;
  color: var(--text-muted);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: color var(--transition-fast), background var(--transition-fast);
}

.modal-close:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.modal-body {
  padding: 0 var(--gap-lg) var(--gap-lg);
}

.setting-section {
  padding: var(--gap-md) 0;
  border-top: 1px solid var(--border);
}

.setting-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: var(--gap-sm);
}

.setting-desc {
  font-size: 0.8rem;
  color: var(--text-muted);
  line-height: 1.5;
  margin-bottom: var(--gap-md);
}

.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: var(--gap-md);
  padding: 8px 16px;
  border: none;
  border-radius: var(--radius-md);
  background: var(--accent);
  color: #fff;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: background var(--transition-fast), opacity var(--transition-fast);
}

.btn-primary:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-primary:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-spinner {
  animation: spin 1s linear infinite;
}

.update-status {
  font-size: 0.8rem;
  color: var(--text-secondary);
  margin-top: var(--gap-md);
  font-variant-numeric: tabular-nums;
}

@media (max-width: 480px) {
  .header-right {
    display: none;
  }
}
</style>
