// 设备检测：UA 自动识别 + localStorage 强制覆盖
const ua = navigator.userAgent
const mobileUA = /Android|iPhone|iPad|iPod|webOS|BlackBerry|IEMobile/i.test(ua)
const override = localStorage.getItem('force_platform')
export const isMobile = override === 'mobile' || (override !== 'desktop' && mobileUA)

// 实时检测当前设备类型（每次调用都重新读取UA和localStorage）
export function checkIsMobile() {
  const currentUA = navigator.userAgent
  const isMobileUA = /Android|iPhone|iPad|iPod|webOS|BlackBerry|IEMobile/i.test(currentUA)
  const o = localStorage.getItem('force_platform')
  return o === 'mobile' || (o !== 'desktop' && isMobileUA)
}
