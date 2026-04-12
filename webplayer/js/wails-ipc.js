// Wails V3 兼容性层 - 子窗口通过 BroadcastChannel 与主窗口通信

// 创建与主窗口通信的频道
const ipcChannel = new BroadcastChannel('wails-ipc-child');

// 封装调用主窗口函数的方法
window.callMainWindow = async (funcName, args = {}) => {
    return new Promise((resolve, reject) => {
        const requestId = Date.now().toString() + Math.random().toString(36).substr(2, 9);

        ipcChannel.postMessage({
            type: 'ipc-request',
            funcName: funcName,
            args: args,
            requestId: requestId
        });

        const timeout = setTimeout(() => {
            reject(new Error('IPC timeout'));
        }, 10000);

        const handler = (event) => {
            if (event.data && event.data.type === 'ipc-response' && event.data.requestId === requestId) {
                clearTimeout(timeout);
                ipcChannel.removeEventListener('message', handler);
                if (event.data.error) {
                    reject(new Error(event.data.error));
                } else {
                    resolve(event.data.result);
                }
            }
        };

        ipcChannel.addEventListener('message', handler);
    });
};

// 便捷函数：置顶窗口
window.ToggleProjectionWindowTop = async (windowID) => {
    return window.callMainWindow('ToggleProjectionWindowTop', windowID);
};

// 便捷函数：排列窗口
window.ArrangeProjectionWindows = async (params) => {
    return window.callMainWindow('ArrangeProjectionWindows', params);
};

console.log('[Wails V3] 子窗口通信层初始化完成');

// 通知主窗口已就绪
setTimeout(() => {
    console.log('[Wails V3] 子窗口已就绪');
}, 500);
