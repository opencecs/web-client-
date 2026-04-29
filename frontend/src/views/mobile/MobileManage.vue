<template>
  <div class="mobile-manage">
    <van-nav-bar title="管理" :border="false" />

    <div class="manage-grid">
      <van-cell-group inset>
        <van-cell title="镜像管理" icon="apps-o" is-link to="/m/images"
          label="在线镜像拉取与本地管理" v-if="auth.can('image_view')" />
        <van-cell title="虚拟网卡" icon="cluster-o" is-link to="/m/network"
          label="网络桥接与网段管理" v-if="auth.can('network_bridge')" />
        <van-cell title="VPC 管理" icon="shield-o" is-link to="/m/vpc"
          label="VPC 分组、域名过滤与容器规则" v-if="auth.can('vpc_manage')" />
      </van-cell-group>

      <van-cell-group inset style="margin-top: 12px" v-if="auth.can('backup_manage') || auth.isAdmin">
        <van-cell title="备份管理" icon="description" is-link to="/m/backup"
          label="容器备份查看与删除" v-if="auth.can('backup_manage')" />
        <van-cell title="设备管理" icon="desktop-o" is-link to="/m/device"
          label="SDK 升级、面板更新、网络设置" v-if="auth.isAdmin" />
        <van-cell title="用户管理" icon="friends-o" is-link to="/m/users"
          label="用户创建、权限配置" v-if="auth.isAdmin" />
      </van-cell-group>

      <van-empty v-if="!hasAnyPermission" description="暂无管理权限" />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useAuthStore } from '../../stores/auth.js'

const auth = useAuthStore()

const hasAnyPermission = computed(() =>
  auth.can('image_view') || auth.can('network_bridge') || auth.can('vpc_manage') || auth.can('backup_manage') || auth.isAdmin
)
</script>

<style scoped>
.mobile-manage {
  background: #0a0a0a;
  min-height: 100vh;
}

.manage-grid {
  padding: 12px 0;
}
</style>
