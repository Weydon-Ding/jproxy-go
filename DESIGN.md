# jproxy-go 管理控制台设计系统

## 0. 设计方向

内部管理工具，运维导向。冷灰色调 + 绿色功能色。无外部依赖，纯 HTML/CSS/JS。

## 0.1 设计债务 (已接受)

- **顶部 Tabs 而非侧边栏**：为简化实现和响应式布局，使用顶部水平 tabs。在功能增多时可能导致横向滚动。
- **无复杂表单验证**：当前使用浏览器原生验证，未实现服务端错误回显到具体字段。
- **示例/标题分页独立**：每个 domain 独立维护分页状态，未实现全局页码同步。
- **批量操作有限**：仅标题支持批量删除，规则批量操作未实现。
- **无深色模式**：当前仅支持浅色主题。

## 1. 颜色令牌

```css
--color-bg: #f5f5f5;
--color-surface: #ffffff;
--color-border: #d0d0d0;
--color-text: #1a1a1a;
--color-text-muted: #666666;
--color-primary: #2e7d32;
--color-primary-hover: #1b5e20;
--color-danger: #d32f2f;
--color-danger-hover: #b71c1c;
--color-focus: #1976d2;
```

## 2. 排版

- 字体：系统 sans-serif fallback（`system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif`）
- 代码：系统 monospace（`ui-monospace, SFMono-Regular, Menlo, Consolas, monospace`）
- 字号：14px 基础，12px 小字，16px 正文，18px/20px/24px 标题

## 3. 间距系统

4px 网格：

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-5: 20px;
--space-6: 24px;
--space-8: 32px;
```

## 4. 组件 Primitive

### 按钮

```css
.btn { padding: 8px 16px; border-radius: 4px; font-size: 14px; }
.btn-primary { background: var(--color-primary); color: white; }
.btn-danger { background: var(--color-danger); color: white; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
```

### 输入框

```css
.input { padding: 8px 12px; border: 1px solid var(--color-border); border-radius: 4px; font-size: 14px; }
.input:focus { outline: 2px solid var(--color-focus); border-color: transparent; }
```

### 面板

```css
.panel { background: var(--color-surface); border: 1px solid var(--color-border); border-radius: 4px; padding: 16px; }
```

### 表格

```css
.table { width: 100%; border-collapse: collapse; }
.table th, .table td { padding: 8px 12px; text-align: left; border-bottom: 1px solid var(--color-border); }
.table tr:hover { background: #fafafa; }
```

### 状态消息

```css
.status { padding: 8px 12px; border-radius: 4px; margin-bottom: 16px; }
.status-success { background: #e8f5e9; color: #2e7d32; }
.status-error { background: #ffebee; color: #c62828; }
```

## 5. 响应式布局

- 移动端（≤375px）：单列，堆叠导航
- 平板（≥768px）：侧边导航 + 主内容区
- 桌面（≥1280px）：固定侧边栏，宽内容区

## 6. 无障碍约束

- 所有交互元素可键盘操作（Tab 导航）
- `:focus-visible` 清晰可见（2px 蓝色 outline）
- 所有表单字段有 `<label>` 关联
- 状态消息使用 `aria-live="polite"`
- 对比度符合 WCAG AA（4.5:1）

## 7. 动画约束

- 禁用所有装饰性动画
- 仅保留必要的状态过渡（hover/focus）
- 支持 `prefers-reduced-motion`：

```css
@media (prefers-reduced-motion: reduce) {
  * { transition: none !important; animation: none !important; }
}
```

## 8. 导航结构

- Tabs/Sections 组织功能，非复杂 SPA
- 登录态：sessionStorage 仅存 token，不持久化密码/密钥
- 401/403 自动返回登录页

## 9. 敏感数据处理

- 密码输入 `type="password"`
- API Key/Secret 输入遮蔽显示
- 错误消息通用化，不泄露内部细节
