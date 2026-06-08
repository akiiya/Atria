// login.js - 账号登录异步流程
(function() {
    'use strict';

    // 读取 CSRF token
    function getCSRFToken() {
        var meta = document.querySelector('meta[name="csrf-token"]');
        return meta ? meta.getAttribute('content') : '';
    }

    // 显示错误
    function showError(elementId, message) {
        var el = document.getElementById(elementId);
        if (el) {
            el.textContent = message;
            el.style.display = 'block';
        }
    }

    // 隐藏错误
    function hideError(elementId) {
        var el = document.getElementById(elementId);
        if (el) {
            el.style.display = 'none';
        }
    }

    // 显示成功
    function showSuccess(elementId, message) {
        var el = document.getElementById(elementId);
        if (el) {
            el.textContent = message;
            el.style.display = 'block';
        }
    }

    // 切换步骤
    function showStep(stepId) {
        var steps = ['step-phone', 'step-code', 'step-password'];
        for (var i = 0; i < steps.length; i++) {
            var el = document.getElementById(steps[i]);
            if (el) {
                el.style.display = steps[i] === stepId ? 'block' : 'none';
            }
        }
    }

    // 设置按钮 loading 状态
    function setButtonLoading(btnId, loading) {
        var btn = document.getElementById(btnId);
        if (!btn) return;

        if (loading) {
            btn.setAttribute('data-original-text', btn.textContent);
            btn.textContent = btn.getAttribute('data-loading-text') || '处理中...';
            btn.disabled = true;
            btn.classList.add('is-loading');
        } else {
            btn.textContent = btn.getAttribute('data-default-text') || btn.getAttribute('data-original-text') || '提交';
            btn.disabled = false;
            btn.classList.remove('is-loading');
        }
    }

    // 异步请求封装
    function postJSON(url, formData, timeout) {
        timeout = timeout || 45000;
        return new Promise(function(resolve, reject) {
            var controller = new AbortController();
            var timer = setTimeout(function() {
                controller.abort();
            }, timeout);

            fetch(url, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': getCSRFToken(),
                    'Accept': 'application/json'
                },
                body: formData,
                signal: controller.signal
            })
            .then(function(response) {
                clearTimeout(timer);
                return response.json();
            })
            .then(function(data) {
                resolve(data);
            })
            .catch(function(err) {
                clearTimeout(timer);
                if (err.name === 'AbortError') {
                    reject({ ok: false, message: '请求超时，请检查网络或代理配置。' });
                } else {
                    reject({ ok: false, message: '网络请求失败，请检查网络连接。' });
                }
            });
        });
    }

    // Step 1: 发送验证码
    window.handleLoginStart = function() {
        var phoneInput = document.getElementById('phone');
        var phone = phoneInput ? phoneInput.value.trim() : '';

        if (!phone || !/^\+[0-9]{7,19}$/.test(phone)) {
            showError('phone-error', '请输入完整的国际手机号，例如 +8613800000000。');
            return;
        }

        hideError('phone-error');
        setButtonLoading('btn-start', true);

        var formData = new FormData();
        formData.append('phone', phone);

        postJSON('/api/accounts/login/start', formData)
            .then(function(data) {
                setButtonLoading('btn-start', false);

                if (data.ok) {
                    // 成功：进入验证码步骤
                    document.getElementById('code-flow-id').value = data.flow_id;
                    hideError('phone-error');
                    showStep('step-code');
                    showSuccess('code-success', data.message || '验证码已发送。');
                    var codeInput = document.getElementById('code');
                    if (codeInput) codeInput.focus();
                } else {
                    showError('phone-error', data.message || '操作失败，请稍后重试。');
                }
            })
            .catch(function(err) {
                setButtonLoading('btn-start', false);
                showError('phone-error', err.message || '网络请求失败，请检查网络连接。');
            });
    };

    // Step 2: 提交验证码
    window.handleLoginCode = function() {
        var flowIdInput = document.getElementById('code-flow-id');
        var codeInput = document.getElementById('code');
        var flowId = flowIdInput ? flowIdInput.value : '';
        var code = codeInput ? codeInput.value.trim() : '';

        if (!flowId) {
            showError('code-error', '登录流程已过期，请重新开始。');
            return;
        }

        if (!code) {
            showError('code-error', '验证码不能为空。');
            return;
        }

        hideError('code-error');
        hideError('code-success');
        setButtonLoading('btn-code', true);

        var formData = new FormData();
        formData.append('flow_id', flowId);
        formData.append('code', code);

        postJSON('/api/accounts/login/code', formData)
            .then(function(data) {
                setButtonLoading('btn-code', false);

                if (data.ok) {
                    if (data.next === 'password') {
                        // 需要 2FA
                        document.getElementById('password-flow-id').value = data.flow_id || flowId;
                        hideError('code-error');
                        showStep('step-password');
                        var pwdInput = document.getElementById('password');
                        if (pwdInput) pwdInput.focus();
                    } else if (data.next === 'done') {
                        // 登录完成
                        window.location.href = data.redirect || '/accounts';
                    }
                } else {
                    showError('code-error', data.message || '验证码提交失败。');
                }
            })
            .catch(function(err) {
                setButtonLoading('btn-code', false);
                showError('code-error', err.message || '网络请求失败，请检查网络连接。');
            });
    };

    // Step 3: 提交 2FA 密码
    window.handleLoginPassword = function() {
        var flowIdInput = document.getElementById('password-flow-id');
        var passwordInput = document.getElementById('password');
        var flowId = flowIdInput ? flowIdInput.value : '';
        var password = passwordInput ? passwordInput.value : '';

        if (!flowId) {
            showError('password-error', '登录流程已过期，请重新开始。');
            return;
        }

        if (!password) {
            showError('password-error', '密码不能为空。');
            return;
        }

        hideError('password-error');
        setButtonLoading('btn-password', true);

        var formData = new FormData();
        formData.append('flow_id', flowId);
        formData.append('password', password);

        postJSON('/api/accounts/login/password', formData)
            .then(function(data) {
                setButtonLoading('btn-password', false);

                if (data.ok && data.next === 'done') {
                    window.location.href = data.redirect || '/accounts';
                } else {
                    showError('password-error', data.message || '密码提交失败。');
                }
            })
            .catch(function(err) {
                setButtonLoading('btn-password', false);
                showError('password-error', err.message || '网络请求失败，请检查网络连接。');
            });
    };

    // 取消/重新开始
    window.handleLoginCancel = function() {
        var flowIdInput = document.getElementById('code-flow-id') || document.getElementById('password-flow-id');
        var flowId = flowIdInput ? flowIdInput.value : '';

        // 通知后端清理 flow
        if (flowId) {
            var formData = new FormData();
            formData.append('flow_id', flowId);
            fetch('/api/accounts/login/cancel', {
                method: 'POST',
                headers: { 'X-CSRF-Token': getCSRFToken() },
                body: formData
            }).catch(function() {});
        }

        // 清理页面状态
        var codeInput = document.getElementById('code');
        var passwordInput = document.getElementById('password');
        if (codeInput) codeInput.value = '';
        if (passwordInput) passwordInput.value = '';

        hideError('phone-error');
        hideError('code-error');
        hideError('code-success');
        hideError('password-error');

        showStep('step-phone');
    };

    // 表单回车提交
    document.addEventListener('DOMContentLoaded', function() {
        var phoneForm = document.getElementById('phone-form');
        var codeForm = document.getElementById('code-form');
        var passwordForm = document.getElementById('password-form');

        if (phoneForm) {
            phoneForm.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') { e.preventDefault(); handleLoginStart(); }
            });
        }
        if (codeForm) {
            codeForm.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') { e.preventDefault(); handleLoginCode(); }
            });
        }
        if (passwordForm) {
            passwordForm.addEventListener('keydown', function(e) {
                if (e.key === 'Enter') { e.preventDefault(); handleLoginPassword(); }
            });
        }
    });
})();
