# 用户反馈系统 (Feedback System)

**日期**: 2026-04-14
**分支**: `feat/feedback`
**提交**: `7f8d0a39` feat(feedback): 新增用户反馈系统功能模块

---

## 概述

新增完整的用户反馈系统，支持用户提交反馈、管理员回复处理、图片上传、Markdown 内容编辑等功能。覆盖后端 API、前端用户端和管理端界面，包含完整的单元测试。

---

## 数据库变更

### 新增表

**`feedbacks`** — 反馈主表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL PK | 主键 |
| `user_id` | BIGINT FK → users | 提交用户 |
| `category` | VARCHAR(20) | 分类：bug / suggestion / complaint / other |
| `title` | VARCHAR(200) | 标题 |
| `content` | TEXT | 内容（支持 Markdown） |
| `images` | JSONB | 图片 URL 数组 |
| `contact` | VARCHAR(255) | 联系方式 |
| `priority` | VARCHAR(10) | 优先级：low / normal / high / urgent（bug→high, complaint→urgent） |
| `status` | VARCHAR(20) | 状态：pending → processing → replied → closed |
| `reply_count` | INTEGER | 回复数 |
| `last_reply_at` | TIMESTAMPTZ | 最后回复时间 |
| `last_reply_role` | VARCHAR(20) | 最后回复角色（user / admin） |
| `created_at` | TIMESTAMPTZ | 创建时间 |
| `updated_at` | TIMESTAMPTZ | 更新时间 |
| `deleted_at` | TIMESTAMPTZ | 软删除时间 |

**`feedback_replies`** — 回复表

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | BIGSERIAL PK | 主键 |
| `feedback_id` | BIGINT FK → feedbacks | 所属反馈 |
| `user_id` | BIGINT FK → users | 回复用户 |
| `role` | VARCHAR(20) | 角色：user / admin |
| `content` | TEXT | 回复内容 |
| `images` | JSONB | 图片 URL 数组 |
| `created_at` | TIMESTAMPTZ | 创建时间 |

### 索引

- `feedbacks`: user_id, status, category, priority, created_at DESC, last_reply_at DESC, deleted_at
- `feedback_replies`: feedback_id

### 迁移文件

`backend/migrations/080_add_feedbacks.sql`

---

## 后端 API

### 用户端路由 (`/api/user/feedbacks`)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/feedbacks` | 创建反馈 |
| GET | `/feedbacks` | 反馈列表（分页、筛选） |
| GET | `/feedbacks/:id` | 反馈详情（含回复列表） |
| PUT | `/feedbacks/:id` | 编辑反馈（仅 pending 状态） |
| POST | `/feedbacks/:id/replies` | 用户回复 |
| POST | `/feedbacks/upload-image` | 上传反馈图片 |

### 管理端路由 (`/api/admin/feedbacks`)

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/feedbacks` | 反馈列表（分页、筛选、排序） |
| GET | `/feedbacks/:id` | 反馈详情 |
| POST | `/feedbacks/:id/replies` | 管理员回复 |
| PUT | `/feedbacks/:id/status` | 更新状态 |
| PUT | `/feedbacks/:id/priority` | 更新优先级 |
| PUT | `/feedbacks/batch-status` | 批量更新状态（上限 50） |
| DELETE | `/feedbacks/:id` | 删除反馈（软删除） |
| POST | `/feedbacks/batch-delete` | 批量删除 |

---

## 后端架构

### 分层结构

```
domain/feedback.go          — 领域常量、模型、校验函数、错误定义
handler/dto/feedback.go     — HTTP 请求/响应 DTO 及 service→DTO 转换
handler/feedback_handler.go — 用户端 HTTP handler
handler/admin/feedback_handler.go — 管理端 HTTP handler
service/feedback_service.go — 核心业务逻辑
service/feedback.go         — service 层模型定义
repository/feedback_repo.go — Ent 数据访问
repository/feedback_cache.go — Redis 缓存（计数、列表）
```

### 核心业务规则

- **自动优先级**：根据分类自动设置优先级（bug→high, complaint→urgent, suggestion→normal, other→low）
- **状态流转**：pending → processing → replied → closed，closed 状态不可再回复
- **频率限制**：用户提交反馈有速率限制
- **软删除**：反馈使用 SoftDeleteMixin，deleted_at 标记删除
- **批量操作上限**：单次批量操作最多 50 条

---

## 前端变更

### 新增页面

**用户端：**
- `FeedbackListView.vue` — 反馈列表（筛选、分页）
- `FeedbackCreateView.vue` — 创建反馈（标题、分类、Markdown 编辑器、图片上传）
- `FeedbackDetailView.vue` — 反馈详情（含回复讨论区）
- `FeedbackEditView.vue` — 编辑反馈（仅 pending 状态可编辑）

**管理端：**
- `FeedbacksView.vue` — 反馈管理列表（筛选、排序、批量操作）
- `FeedbackDetailView.vue` — 反馈详情（状态/优先级调整、管理员回复）

### 新增组件

- `MarkdownEditorField.vue` — Markdown 编辑器字段
- `MarkdownPreview.vue` — Markdown 实时预览
- `MultiImageUpload.vue` — 多图上传组件（复用 S3 存储）

### 新增路由

| 路径 | 组件 | 权限 |
|------|------|------|
| `/feedbacks` | FeedbackListView | 登录用户 |
| `/feedbacks/new` | FeedbackCreateView | 登录用户 |
| `/feedbacks/:id` | FeedbackDetailView | 登录用户 |
| `/feedbacks/:id/edit` | FeedbackEditView | 登录用户 |
| `/admin/feedbacks` | FeedbacksView (admin) | 管理员 |
| `/admin/feedbacks/:id` | FeedbackDetailView (admin) | 管理员 |

### 新增 API 模块

- `frontend/src/api/feedbacks.ts` — 用户端反馈 API
- `frontend/src/api/admin/feedbacks.ts` — 管理端反馈 API

### 其他变更

- `AppSidebar.vue` — 侧边栏新增"反馈"入口
- `SettingsView.vue` — 管理端设置页新增反馈相关配置项
- `StatusBadge.vue` — 状态徽章组件更新，支持反馈状态显示
- `i18n/locales/zh.ts` / `en.ts` — 新增反馈模块国际化文案（~110 条）
- `utils/feedback.ts` — 反馈相关工具函数（状态颜色映射等）
- `types/index.ts` — 新增反馈相关 TypeScript 类型定义

### 依赖变更

- 新增 `marked` — Markdown 渲染

---

## 测试

### 后端

- `service/feedback_service_test.go` — 反馈服务单元测试（160 行）

### 前端

- `components/feedback/__tests__/MarkdownEditorField.spec.ts`
- `components/feedback/__tests__/MarkdownPreview.spec.ts`
- `views/user/__tests__/FeedbackCreateView.spec.ts`
- `views/user/__tests__/FeedbackDetailView.spec.ts`
- `views/user/__tests__/FeedbackEditView.spec.ts`
- `views/user/__tests__/FeedbackListView.spec.ts`

---

## 变更统计

**91 个文件变更**, +17,570 行 / -111 行

- Ent 自动生成代码（schema、client、migration）: ~8,000 行
- 后端业务逻辑: ~1,500 行
- 前端页面/组件/API: ~1,800 行
- 测试代码: ~800 行
- 国际化: ~220 行
- 其他（Wire DI、路由注册、配置）: ~200 行
