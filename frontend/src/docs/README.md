# 文档管理指南

## 添加新文档

两步完成：

### 1. 创建 Markdown 文件

在 `src/docs/content/` 目录下新建 `.md` 文件，文件名即为 URL slug。

例如：创建 `src/docs/content/Gemini CLI快速开始指南.md`，访问路径为 `/docs/Gemini CLI快速开始指南`

### 2. 在配置中注册

编辑 `src/docs/config.ts`，在对应分组的 `items` 数组中加一行：

```typescript
{ title: '显示名称', slug: '文件名（不含.md）' },
```

## 当前配置结构

```typescript
export const docsConfig: DocsConfig = [
  {
    title: '分组名称',        // 侧边栏分类标题
    collapsed: false,         // 可选，默认展开。设为 true 则默认折叠
    items: [
      { title: '显示名称', slug: '对应content目录下的md文件名' },
    ],
  },
]
```

## 示例：新增一篇 Gemini CLI 教程

1. 创建文件 `src/docs/content/Gemini CLI快速开始指南.md`
2. 编辑 `src/docs/config.ts`：

```typescript
{
  title: '快速接入',
  items: [
    { title: 'Node.js环境安装指南', slug: 'Node.js环境安装指南' },
    { title: 'Claude Code快速开始指南', slug: 'Claude Code快速开始指南' },
    { title: 'Codex快速开始指南', slug: 'Codex快速开始指南' },
    { title: 'Gemini CLI快速开始指南', slug: 'Gemini CLI快速开始指南' },  // 新增
  ],
},
```

## 新增分组

在 `docsConfig` 数组中添加一个新对象即可：

```typescript
{
  title: '新分组名',
  collapsed: true,  // 默认折叠
  items: [
    { title: '文章标题', slug: '文件名' },
  ],
},
```

## 默认首页

`config.ts` 中 `defaultSlug` 控制访问 `/docs` 时默认显示的文档，当前为 `introduction`。
