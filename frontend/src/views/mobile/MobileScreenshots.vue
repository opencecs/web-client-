<template>
  <div class="mobile-screenshots">
    <van-nav-bar title="实时截图" left-arrow @click-left="$router.back()" :border="false">
      <template #right>
        <van-icon name="replay" size="20" @click="refresh" />
      </template>
    </van-nav-bar>

    <div class="screenshot-grid">
      <div v-for="c in filteredContainers" :key="c.name" class="screenshot-item"
        @click="goProjection(c)">
        <div class="screenshot-preview">
          <img v-if="screenshots[c.indexNum]" :src="screenshots[c.indexNum]" class="screenshot-img" />
          <div v-else class="screenshot-placeholder">
            <span>{{ c.indexNum }}</span>
          </div>
        </div>
        <div class="screenshot-name">{{ device.displayName(c.name) }}</div>
      </div>
    </div>

    <van-empty v-if="filteredContainers.length === 0" description="暂无运行中的容器" />
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showToast } from 'vant'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()

const screenshots = computed(() => device.screenshots || {})
const filteredContainers = computed(() =>
  device.containers.filter(c => auth.canSlot(c.indexNum) && c.status === 'running')
)

function goProjection(c) {
  if (!auth.can('projection')) { showToast('无投屏权限'); return }
  router.push(`/m/android/projection/${c.name}`)
}

function refresh() {
  device.refreshContainers()
  showToast('已刷新')
}
</script>

<style scoped>
.mobile-screenshots {
  background: #0a0a0a;
  min-height: 100vh;
}

.screenshot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 10px;
  padding: 12px;
}

.screenshot-item {
  text-align: center;
}

.screenshot-preview {
  aspect-ratio: 9/16;
  border-radius: 8px;
  overflow: hidden;
  background: #141414;
  border: 1px solid #2a2a2a;
}
.screenshot-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.screenshot-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  font-weight: 700;
  color: #555;
}
.screenshot-name {
  font-size: 11px;
  color: #999;
  margin-top: 4px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
