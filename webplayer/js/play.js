// 滚轮滚动状态
var wheelState = {
    accumulatedDelta: 0,
    accumulatedDistance: 0,
    lastDirection: 0,
    scrollCount: 0,
    lastX: 0,
    lastY: 0,
    timer: null,
    baseDistance: 60,
    minDistance: 10
};

function getWheelDistance() {
    var count = Math.min(wheelState.scrollCount, 5);
    return count * 10;
}

function sendWheelScroll(direction, totalDistance, x, y) {
    if (!window.h5_lgair || !window.h5_lgair.fun_send_data_multi || !window.h5_lgair.trans_xy) {
        console.log('h5_lgair not ready');
        return;
    }
    try {
        var canvasEl = document.getElementById(window.h5_lgair.Canvas_id);
        var rect = canvasEl.getBoundingClientRect();
        var scrollDistance = direction * totalDistance;
        var lg = window.h5_lgair;
        var screenX = Math.round(x - rect.left);
        var screenY = Math.round(y - rect.top);
        var screenW = Math.round(rect.width);
        var screenH = Math.round(rect.height);
        var startPos = lg.trans_xy(x, y);
        var endPos, startY, endY;
        if (lg.be_Rotate) {
            endX = startPos.x - scrollDistance;
            endX = Math.max(0, Math.min(endX, lg.Canvas_width - 1));
            endPos = {x: endX, y: startPos.y};
        } else {
            endY = startPos.y + scrollDistance;
            endY = Math.max(0, Math.min(endY, lg.Canvas_height - 1));
            endPos = {x: startPos.x, y: endY};
        }
        var startData = {
            type: lg.SDL_EVENT_MSGTYPE_POINTSCLICK,
            data: {
                points: 1,
                action: lg.ACTION_POINTER_DOWN,
                nid: 0,
                p: [{x: startPos.x, y: startPos.y, id: 0}],
                sw: lg.Canvas_width,
                sh: lg.Canvas_height
            }
        };
        var stepCount = 6;
        var stepDelay = 10;
        var endCoord = lg.be_Rotate ? endPos.x : endPos.y;
        var startCoord = lg.be_Rotate ? startPos.x : startPos.y;
        var stepSize = (endCoord - startCoord) / stepCount;
        lg.fun_send_data_multi(startData);
        for (var i = 1; i <= stepCount; i++) {
            (function(index) {
                setTimeout(function() {
                    var currentCoord = Math.round(startCoord + stepSize * index);
                    lg.fun_send_data_multi({
                        type: lg.SDL_EVENT_MSGTYPE_POINTSCLICK,
                        data: {
                            points: 1,
                            action: lg.ACTION_MOVE,
                            nid: 0,
                            p: [{x: lg.be_Rotate ? currentCoord : startPos.x, y: lg.be_Rotate ? startPos.y : currentCoord, id: 0}],
                            sw: lg.Canvas_width,
                            sh: lg.Canvas_height
                        }
                    });
                }, index * stepDelay);
            })(i);
        }
        setTimeout(function() {
            lg.fun_send_data_multi({
                type: lg.SDL_EVENT_MSGTYPE_POINTSCLICK,
                data: {
                    points: 0,
                    action: lg.ACTION_POINTER_UP,
                    nid: 0,
                    p: [{x: endPos.x, y: endPos.y, id: 0}],
                    sw: lg.Canvas_width,
                    sh: lg.Canvas_height
                }
            });
        }, (stepCount + 1) * stepDelay);
        console.log('Wheel scroll:', {
            mouse: {x: x, y: y, screenX: screenX, screenY: screenY, screenW: screenW, screenH: screenH},
            rotate: lg.be_Rotate,
            lgCanvas: {w: lg.Canvas_width, h: lg.Canvas_height},
            start: startPos,
            end: endPos,
            distance: scrollDistance
        });
    } catch(e) {
        console.log('Wheel error:', e);
    }
}

// 右键 = 返回
document.addEventListener('contextmenu', function(e) {
    e.preventDefault();
    window.h5_lgair && window.h5_lgair.sendCmdEvent && window.h5_lgair.sendCmdEvent("goBack");
});

// 初始化滚轮处理
function initWheelHandler() {
    if (!window.h5_lgair || !window.h5_lgair.trans_xy) {
        setTimeout(initWheelHandler, 100);
        return;
    }
    wheelState.wheelHandler = function(e) {
        e.preventDefault();
        var now = Date.now();
        var delta = e.deltaY > 0 ? 1 : -1;
        wheelState.lastX = e.clientX;
        wheelState.lastY = e.clientY;
        if (wheelState.lastDirection !== delta) {
            wheelState.accumulatedDelta = 0;
            wheelState.accumulatedDistance = 0;
            wheelState.scrollCount = 0;
            wheelState.lastDirection = delta;
        }
        wheelState.accumulatedDelta += Math.abs(e.deltaY);
        wheelState.scrollCount++;
        var currentDistance = getWheelDistance();
        wheelState.accumulatedDistance += currentDistance;
        if (wheelState.timer) clearTimeout(wheelState.timer);
        wheelState.timer = setTimeout(function() {
            if (wheelState.accumulatedDelta > 0 && wheelState.lastDirection !== 0) {
                sendWheelScroll(wheelState.lastDirection, wheelState.accumulatedDistance, wheelState.lastX, wheelState.lastY);
                wheelState.accumulatedDelta = 0;
                wheelState.accumulatedDistance = 0;
                wheelState.scrollCount = 0;
            }
        }, 100);
    };
    wheelState.wheelListener = wheelState.wheelHandler;
    document.addEventListener('wheel', wheelState.wheelListener, { passive: false });
    console.log('Wheel handler initialized');
}

initWheelHandler();

// 禁用视频悬浮控件（画中画、翻译、音频增强）
(function() {
    function disableVideoOverlays(video) {
        video.disablePictureInPicture = true;
        video.disableRemotePlayback = true;
        video.controlsList && video.controlsList.add('noplaybackrate', 'nodownload', 'nofullscreen');
        video.removeAttribute('controls');
    }
    // 处理已有的 video
    document.querySelectorAll('video').forEach(disableVideoOverlays);
    // 监听动态创建的 video
    new MutationObserver(function(mutations) {
        mutations.forEach(function(m) {
            m.addedNodes.forEach(function(n) {
                if (n.tagName === 'VIDEO') disableVideoOverlays(n);
                if (n.querySelectorAll) n.querySelectorAll('video').forEach(disableVideoOverlays);
            });
        });
    }).observe(document.body, { childList: true, subtree: true });
})();

// 接收外壳 postMessage 指令（安卓按键 + 延迟查询）
window.addEventListener('message', function(e) {
    if (!e.data || !e.data.action) return;
    var action = e.data.action;
    if (action === 'getLatency') {
        // 从 WebRTC peerConnection 获取延迟
        try {
            var pc = null;
            if (window.h5_lgair) {
                pc = (window.h5_lgair.g_player && window.h5_lgair.g_player.pconnection) || window.h5_lgair.pconnection;
            }
            if (pc && pc.getStats) {
                pc.getStats().then(function(stats) {
                    var rtt = -1;
                    stats.forEach(function(report) {
                        if (report.currentRoundTripTime !== undefined && report.currentRoundTripTime > 0) {
                            rtt = Math.round(report.currentRoundTripTime * 1000);
                        }
                    });
                    if (window.parent !== window) {
                        window.parent.postMessage({ type: 'latency', rtt: rtt }, '*');
                    }
                }).catch(function() {
                    if (window.parent !== window) window.parent.postMessage({ type: 'latency', rtt: -1 }, '*');
                });
            } else {
                if (window.parent !== window) window.parent.postMessage({ type: 'latency', rtt: -1 }, '*');
            }
        } catch(ex) {
            if (window.parent !== window) window.parent.postMessage({ type: 'latency', rtt: -1 }, '*');
        }
        return;
    }
    if (window.h5_lgair && window.h5_lgair.sendCmdEvent) {
        if (action === 'goBack') window.h5_lgair.sendCmdEvent('goBack');
        else if (action === 'goHome') window.h5_lgair.sendCmdEvent('goHome');
        else if (action === 'goClean') window.h5_lgair.sendCmdEvent('goClean');
        else if (action === 'volUp') window.h5_lgair.sendCmdEvent('volUp');
        else if (action === 'volDown') window.h5_lgair.sendCmdEvent('volDown');
    }
});

// 页面清理处理器
var cleanupHandlers = {
    _registered: false,

    register: function() {
        if (this._registered) return;
        window.addEventListener('unload', this.unloadHandler);
        window.addEventListener('beforeunload', this.beforeunloadHandler);
        this._registered = true;
    },

    unregister: function() {
        if (!this._registered) return;
        window.removeEventListener('unload', this.unloadHandler);
        window.removeEventListener('beforeunload', this.beforeunloadHandler);
        this._registered = false;
    },

    unloadHandler: function() {
        console.log('[BatchControl] 页面卸载，清理资源');
        cleanupHandlers.cleanupAll();
    },

    beforeunloadHandler: function() {
        console.log('[BatchControl] 页面即将关闭，清理资源');
        cleanupHandlers.cleanupAll();
    },

    cleanupAll: function() {
        if (typeof window.h5_lgair === 'object' && window.h5_lgair && typeof window.h5_lgair.destroy === 'function') {
            window.h5_lgair.destroy();
        }
        if (typeof window.destroyWailsIPC === 'function') {
            window.destroyWailsIPC();
        }
        if (typeof LGAIR_UI !== 'undefined' && typeof LGAIR_UI.stopAllTimers === 'function') {
            LGAIR_UI.stopAllTimers();
        } else if (typeof LGAIR_UI !== 'undefined' && typeof LGAIR_UI.stopProgress === 'function') {
            LGAIR_UI.stopProgress();
        }
        if (typeof batchControlCheckTimer !== 'undefined' && batchControlCheckTimer) {
            clearInterval(batchControlCheckTimer);
            batchControlCheckTimer = null;
        }
        if (typeof BatchControl !== 'undefined' && BatchControl?.close) BatchControl.close();
        if (typeof VideoStreamProxy !== 'undefined' && VideoStreamProxy?.close) VideoStreamProxy.close();
        if (wheelState.timer) {
            clearTimeout(wheelState.timer);
            wheelState.timer = null;
        }
        if (wheelState.wheelListener) {
            document.removeEventListener('wheel', wheelState.wheelListener);
            wheelState.wheelListener = null;
        }
        if (window._h5LgairMessageHandler) {
            window.removeEventListener('message', window._h5LgairMessageHandler);
            window._h5LgairMessageHandler = null;
        }
        cleanupHandlers.unregister();
    }
};

window.globalCleanup = function() {
    cleanupHandlers.unloadHandler();
};

cleanupHandlers.register();
