// SDK API 封装 - 通过后端代理调用设备 SDK API
import api from './index.js'

const sdk = {
  // ========== 容器管理 ==========
  // 创建容器（快速，镜像必须已存在）
  createContainer(data) {
    return api.post('/sdk/android', data, { timeout: 120000 })
  },
  // 执行命令
  execCommand(name, command) {
    return api.post('/sdk/android/exec', { name, command })
  },
  // 批量切换镜像
  batchChangeImage(containerNames, image) {
    return api.post('/sdk/android/change-image', { containerNames, image })
  },
  // 查询任务进度
  getTaskStatus(taskId) {
    return api.get('/sdk/android/task-status', { params: { taskId } })
  },
  // 导出容器
  exportContainer(name) {
    return api.post('/sdk/android/export', { name })
  },

  // ========== 镜像管理 ==========
  // 获取本地镜像列表
  listImages(imageName) {
    return api.get('/sdk/android/image', { params: imageName ? { imageName } : {} })
  },
  // 删除本地镜像
  deleteImage(image) {
    return api.delete('/sdk/android/image', { params: { image } })
  },
  // 拉取镜像
  pullImage(imageUrl) {
    return api.post('/sdk/android/pullImage', { imageUrl })
  },
  // 清理未使用镜像
  pruneImages() {
    return api.post('/sdk/android/pruneImages')
  },
  // 获取镜像压缩包列表
  listImageTars() {
    return api.get('/sdk/android/imageTar')
  },
  // 删除镜像压缩包
  deleteImageTar(name) {
    return api.delete('/sdk/android/imageTar', { params: { name } })
  },

  // ========== 机型 ==========
  // 获取在线机型列表
  getPhoneModels() {
    return api.get('/sdk/android/phoneModel')
  },
  // 获取国家代码列表
  getCountryCodes() {
    return api.get('/sdk/android/countryCode')
  },
  // 获取本地机型列表（已废弃）
  // getLocalModels() { ... }

  // ========== 机型备份 ==========
  listModelBackups() {
    return api.get('/sdk/android/backup/model')
  },
  createModelBackup(name, suffix) {
    return api.post('/sdk/android/backup/model', { name, suffix })
  },
  deleteModelBackup(name) {
    return api.delete('/sdk/android/backup/model', { params: { name } })
  },

  // ========== 云机备份 ==========
  listBackups() {
    return api.get('/sdk/backup')
  },
  deleteBackup(name) {
    return api.delete('/sdk/backup', { params: { name } })
  },
  getBackupDownloadUrl(name) {
    return `/api/sdk/backup/download?name=${encodeURIComponent(name)}`
  },

  // ========== 网络管理 ==========
  // MytBridge
  listBridges() {
    return api.get('/sdk/mytBridge')
  },
  createBridge(customName, cidr) {
    return api.post('/sdk/mytBridge', { customName, cidr })
  },
  updateBridge(name, newCidr) {
    return api.put('/sdk/mytBridge', { name, newCidr })
  },
  deleteBridge(name) {
    return api.delete('/sdk/mytBridge', { params: { name } })
  },

  // VPC 分组
  listVpcGroups(alias) {
    return api.get('/sdk/mytVpc/group', { params: alias ? { alias } : {} })
  },
  createVpcGroup(data) {
    return api.post('/sdk/mytVpc/group', data)
  },
  deleteVpcGroup(id) {
    return api.delete('/sdk/mytVpc/group', { params: { id } })
  },
  // VPC 节点规则
  listContainerRules() {
    return api.get('/sdk/mytVpc/containerRule')
  },
  addVpcRule(name, vpcID) {
    return api.post('/sdk/mytVpc/addRule', { name, vpcID })
  },
  removeVpcRule(name) {
    return api.post('/sdk/mytVpc/delRule', { name })
  },
  testVpcNode(address) {
    return api.get('/sdk/mytVpc/test', { params: { address } })
  },

  // ========== 设备信息 ==========
  getDeviceInfo() {
    return api.get('/sdk/info/device')
  },
  getVersionInfo() {
    return api.get('/sdk/info')
  },
  // 获取在线镜像列表（安卓14/16 ALL版，已按设备型号筛选）
  getMirrorList() {
    return api.get('/device/mirrors')
  },
}

export default sdk
