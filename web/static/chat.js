// chat.js - 聊天页面异步交互
(function() {
    'use strict';

    function getCSRFToken() {
        var meta = document.querySelector('meta[name="csrf-token"]');
        return meta ? meta.getAttribute('content') : '';
    }

    function showError(msg) {
        var el = document.getElementById('chat-error');
        if (el) { el.textContent = msg; el.style.display = 'block'; }
    }

    function hideError() {
        var el = document.getElementById('chat-error');
        if (el) { el.style.display = 'none'; }
    }

    function setButtonLoading(loading) {
        var btn = document.getElementById('btn-send');
        if (!btn) return;
        if (loading) {
            btn.setAttribute('data-original-text', btn.textContent);
            btn.textContent = btn.getAttribute('data-loading-text') || '发送中...';
            btn.disabled = true;
        } else {
            btn.textContent = btn.getAttribute('data-default-text') || btn.getAttribute('data-original-text') || '发送';
            btn.disabled = false;
        }
    }

    function appendMessage(text) {
        var container = document.getElementById('chat-messages');
        if (!container) return;

        // 移除空状态
        var empty = container.querySelector('.chat-empty');
        if (empty) empty.remove();

        var now = new Date();
        var timeStr = String(now.getHours()).padStart(2, '0') + ':' + String(now.getMinutes()).padStart(2, '0');

        var div = document.createElement('div');
        div.className = 'chat-message chat-message-out';
        div.innerHTML = '<div class="chat-text">' + escapeHtml(text) + '</div>' +
                        '<div class="chat-msg-time">' + timeStr + '</div>';
        container.appendChild(div);
        container.scrollTop = container.scrollHeight;
    }

    function escapeHtml(str) {
        var div = document.createElement('div');
        div.appendChild(document.createTextNode(str));
        return div.innerHTML;
    }

    window.sendChatMessage = function() {
        var input = document.getElementById('chat-text');
        var text = input ? input.value.trim() : '';
        if (!text) return;
        if (typeof PEER_REF === 'undefined' || !PEER_REF) return;

        hideError();
        setButtonLoading(true);

        fetch('/api/chats/' + encodeURIComponent(PEER_REF) + '/messages', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-CSRF-Token': getCSRFToken(),
                'Accept': 'application/json'
            },
            body: JSON.stringify({ text: text })
        })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            setButtonLoading(false);
            if (data.ok) {
                appendMessage(text);
                input.value = '';
                input.focus();
            } else {
                showError(data.message || '发送失败');
            }
        })
        .catch(function() {
            setButtonLoading(false);
            showError('网络请求失败，请检查网络连接。');
        });
    };

    // 回车发送
    document.addEventListener('DOMContentLoaded', function() {
        var form = document.getElementById('chat-send-form');
        if (form) {
            form.addEventListener('keydown', function(e) {
                if (e.key === 'Enter' && !e.shiftKey) {
                    e.preventDefault();
                    sendChatMessage();
                }
            });
        }

        // 滚动到底部
        var container = document.getElementById('chat-messages');
        if (container) {
            container.scrollTop = container.scrollHeight;
        }
    });
})();
