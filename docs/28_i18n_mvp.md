# 前端国际化 MVP

## 实现方案

轻量级自定义 i18n composable，不引入 vue-i18n 依赖。

```
frontend/src/i18n/
  index.ts              ← useI18n() composable
  locales/
    en.ts               ← English (base/reference, 341 keys)
    zh-CN.ts            ← 简体中文
    zh-TW.ts            ← 繁體中文
    ja.ts               ← 日本語
    ko.ts               ← 한국어
    de.ts               ← Deutsch
    fr.ts               ← Français
    es.ts               ← Español
    pt-BR.ts            ← Português
    ru.ts               ← Русский
```

## 支持的语言

| 代码 | 语言 |
|------|------|
| zh-CN | 简体中文 |
| zh-TW | 繁體中文 |
| en | English |
| ja | 日本語 |
| ko | 한국어 |
| de | Deutsch |
| fr | Français |
| es | Español |
| pt-BR | Português |
| ru | Русский |

## 浏览器语言检测与 fallback

1. 读取 `localStorage.getItem('atria_locale')`
2. 如果有值且有效 → 使用
3. 否则检查 `navigator.languages` / `navigator.language`
4. 精确匹配（如 `zh-CN`）→ 使用
5. 前缀匹配（如 `zh-HK` → `zh-TW`，`pt` → `pt-BR`）→ 使用
6. 都不匹配 → fallback 到 `en`

## 用户手动切换

- Topbar 右侧 🌐 图标，点击展开语言下拉菜单
- 选择后立即更新页面（无需刷新）
- 写入 `localStorage` key: `atria_locale`
- 后续访问优先使用 localStorage 值

## API

```typescript
const { t, locale, setLocale, locales } = useI18n()

t('nav.dashboard')    // 返回当前语言的翻译
locale.value           // 当前语言代码
setLocale('en')        // 切换语言并持久化
locales                // 可用语言列表 [{code, label}]
```

## 翻译 key 结构

采用扁平 key 结构，按模块前缀分组（共 341 key/locale）：

- `nav.*` — 导航
- `common.*` — 通用操作
- `dashboard.*` — 仪表盘
- `chat.*` — 聊天（含 composer、灯箱、会话搜索、服务消息）
- `media.*` — 媒体类型与操作
- `contacts.*` — 联系人
- `audit.*` — 审计日志
- `event.*` — 审计事件类型
- `risk.*` — 风险等级
- `settings.*` — 设置（密码、API Key、代理）
- `maintenance.*` — 数据维护
- `accounts.*` — 账号会话（含 runtime 状态）
- `search.*` — 搜索
- `login.*` — 登录流程
- `accountDetail.*` — 账号详情
- `peerType.*` — peer 类型标签
- `lightbox.*` — 图片灯箱工具栏

## 已接入 i18n 的页面

| 页面 | 覆盖范围 |
|------|---------|
| Sidebar | 导航标签、分区标题 |
| Topbar | 账号切换、主题、语言切换、退出确认 |
| Dashboard | 全部文案、审计事件类型标签 |
| Chat | 标题、空态、状态标签、composer、消息气泡、服务消息、会话搜索、图片灯箱 |
| Contacts | 标题、搜索、空态、badge |
| Search | 标题、placeholder、结果、分页、错误提示 |
| Audit | 筛选、表头、分页、空态 |
| Maintenance | 全部文案、媒体缓存、清理操作 |
| Accounts | 全部文案、状态标签、操作按钮、runtime 标签 |
| Settings | 全部文案（密码、API Key、代理） |
| Login | 全部文案（手机号、验证码、2FA） |
| Account Detail | 全部文案、危险操作 |

## 如何新增语言

1. 在 `frontend/src/i18n/locales/` 下新建 `{code}.ts`
2. 导出 `Record<string, string>`，包含 en.ts 中的所有 key
3. 在 `frontend/src/i18n/index.ts` 中 import 并加入 `messages` 和 `locales`
4. 运行 `npm run typecheck` 确认无 TypeScript 错误

## 暂不覆盖

- 后端错误消息翻译
- Telegram 原始内容翻译
- 用户消息/联系人名称翻译
- 服务器端语言偏好
- 复数规则
- 远程语言包
