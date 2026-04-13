// 移动端路由定义 — 统一 /m/ 前缀
export const mobileRoutes = [
  {
    path: '/m/login',
    name: 'MobileLogin',
    component: () => import('../views/mobile/MobileLogin.vue'),
    meta: { guest: true, mobile: true }
  },
  {
    path: '/m',
    name: 'MobileDashboard',
    component: () => import('../views/mobile/MobileDashboard.vue'),
    meta: { mobile: true, tabbar: true }
  },
  {
    path: '/m/android',
    name: 'MobileAndroid',
    component: () => import('../views/mobile/MobileAndroid.vue'),
    meta: { mobile: true, tabbar: true }
  },
  {
    path: '/m/android/container/:name',
    name: 'MobileContainerDetail',
    component: () => import('../views/mobile/MobileContainerDetail.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/android/projection/:name',
    name: 'MobileProjection',
    component: () => import('../views/mobile/MobileProjection.vue'),
    meta: { mobile: true, fullscreen: true }
  },
  {
    path: '/m/android/create',
    name: 'MobileCreateContainer',
    component: () => import('../views/mobile/MobileCreateContainer.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/screenshots',
    name: 'MobileScreenshots',
    component: () => import('../views/mobile/MobileScreenshots.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/manage',
    name: 'MobileManage',
    component: () => import('../views/mobile/MobileManage.vue'),
    meta: { mobile: true, tabbar: true }
  },
  {
    path: '/m/images',
    name: 'MobileImages',
    component: () => import('../views/mobile/MobileImages.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/network',
    name: 'MobileNetwork',
    component: () => import('../views/mobile/MobileNetwork.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/vpc',
    name: 'MobileVpc',
    component: () => import('../views/mobile/MobileVpc.vue'),
    meta: { mobile: true }
  },
  {
    path: '/m/device',
    name: 'MobileDevice',
    component: () => import('../views/mobile/MobileDevice.vue'),
    meta: { mobile: true, admin: true }
  },
  {
    path: '/m/users',
    name: 'MobileUsers',
    component: () => import('../views/mobile/MobileUsers.vue'),
    meta: { mobile: true, admin: true }
  },
  {
    path: '/m/profile',
    name: 'MobileProfile',
    component: () => import('../views/mobile/MobileProfile.vue'),
    meta: { mobile: true, tabbar: true }
  },
]
