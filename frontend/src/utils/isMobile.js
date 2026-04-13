// 设备检测：UA 自动识别 + localStorage 强制覆盖
const override = localStorage.getItem('force_platform')
const ua = navigator.userAgent
const mobileUA = /Android|iPhone|iPad|iPod|webOS|BlackBerry|IEMobile/i.test(ua)
export const isMobile = override === 'mobile' || (override !== 'desktop' && mobileUA)
