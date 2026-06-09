/**
 * Atria - 主题切换和通用交互逻辑
 */

(function() {
    'use strict';

    // ============================================
    // 主题管理
    // ============================================

    var THEME_KEY = 'atria-theme';
    var THEMES = ['light', 'dark', 'system'];

    /**
     * 获取当前保存的主题偏好
     * @returns {string} 'light' | 'dark' | 'system'
     */
    function getSavedTheme() {
        var saved = localStorage.getItem(THEME_KEY);
        if (THEMES.indexOf(saved) !== -1) {
            return saved;
        }
        return 'system';
    }

    /**
     * 保存主题偏好
     * @param {string} theme
     */
    function saveTheme(theme) {
        localStorage.setItem(THEME_KEY, theme);
    }

    /**
     * 解析 system 主题为实际的 light/dark
     * @returns {string} 'light' | 'dark'
     */
    function resolveSystemTheme() {
        if (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) {
            return 'dark';
        }
        return 'light';
    }

    /**
     * 将主题应用到 DOM
     * @param {string} theme - 'light' | 'dark' | 'system'
     */
    function applyTheme(theme) {
        var resolved = theme === 'system' ? resolveSystemTheme() : theme;
        document.documentElement.setAttribute('data-theme', resolved);

        // 更新主题切换按钮的激活状态
        var buttons = document.querySelectorAll('.theme-btn');
        for (var i = 0; i < buttons.length; i++) {
            var btn = buttons[i];
            if (btn.getAttribute('data-theme') === theme) {
                btn.classList.add('active');
            } else {
                btn.classList.remove('active');
            }
        }

        // 更新设置页面的 radio 按钮（如果存在）
        var radios = document.querySelectorAll('input[name="theme"]');
        for (var j = 0; j < radios.length; j++) {
            radios[j].checked = radios[j].value === theme;
        }
    }

    /**
     * 切换主题
     * @param {string} theme - 'light' | 'dark' | 'system'
     */
    function setTheme(theme) {
        saveTheme(theme);
        applyTheme(theme);
    }

    /**
     * 监听系统主题变化（用于 system 模式）
     */
    function watchSystemTheme() {
        if (!window.matchMedia) return;

        var mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');

        // 使用 addEventListener 或 addListener（兼容旧浏览器）
        var handler = function() {
            if (getSavedTheme() === 'system') {
                applyTheme('system');
            }
        };

        if (mediaQuery.addEventListener) {
            mediaQuery.addEventListener('change', handler);
        } else if (mediaQuery.addListener) {
            mediaQuery.addListener(handler);
        }
    }

    // ============================================
    // 初始化主题
    // ============================================

    // 应用保存的主题（页面加载时）
    var initialTheme = getSavedTheme();
    applyTheme(initialTheme);

    // 监听系统主题变化
    watchSystemTheme();

    // ============================================
    // 事件绑定
    // ============================================

    // 等待 DOM 加载完成后绑定事件
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initEventListeners);
    } else {
        initEventListeners();
    }

    function initEventListeners() {
        // 顶部栏主题切换按钮
        var themeButtons = document.querySelectorAll('.theme-btn');
        for (var i = 0; i < themeButtons.length; i++) {
            themeButtons[i].addEventListener('click', function(e) {
                var theme = this.getAttribute('data-theme');
                if (theme && THEMES.indexOf(theme) !== -1) {
                    setTheme(theme);
                }
            });
        }

        // 设置页面主题 radio 按钮
        var themeRadios = document.querySelectorAll('input[name="theme"]');
        for (var j = 0; j < themeRadios.length; j++) {
            themeRadios[j].addEventListener('change', function(e) {
                if (this.checked) {
                    setTheme(this.value);
                }
            });
        }
    }

})();
