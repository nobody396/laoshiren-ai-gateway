# laoshirenai 项目 DragonCode 字样扫描报告

生成日期：2026-05-16
扫描目录：/Users/fanenda/Documents/Codex/2026-05-16/nobody396-laoshiren-ai-gateway-https-github
扫描关键词：dragon code、DragonCode、Dragon Code、bozhouDev/DragonCode-sub2api、github.com/bozhouDev

## 结论

当前网站页面上直接显示的文字中，未确认看到 dragon code；但线上 dashboard 当前加载的前端 JS 包里确实存在 DragonCode / Dragon Code / bozhouDev/DragonCode-sub2api。普通用户不一定能直接看到，但打开开发者工具、点击相关菜单链接、或进入 Key Usage / Admin Settings 等页面时可能暴露。

本地源码扫描显示，全仓库约 870 个文件命中相关字样。这个数字主要来自 Go module/import 路径和 Ent 生成代码，不代表 870 个用户可见风险。真正必须优先改的是前端布局、页面链接、设置页链接和打包进浏览器的兼容品牌列表。

## 需要重点处理的位置

| 风险 | 位置 | 出现位置/用途 | 是否有必要修改 |
|---|---|---|---|
| 高 | frontend/src/components/layout/AppHeader.vue:143 | 登录后布局/用户菜单里的 GitHub 链接 | 会加载到 dashboard 当前页面；用户点开菜单或开发者工具可见。必须修改或移除。 |
| 高 | frontend/src/views/KeyUsageView.vue:378 | Key Usage 页面 GitHub 链接 | 公开或半公开页面资源里有上游仓库链接。必须修改或移除。 |
| 高 | frontend/src/views/admin/SettingsView.vue:1915 | 管理员设置页付款 API 文档 raw GitHub 链接 | 管理员可见，且直接指向上游 DragonCode 仓库。必须改成本项目文档或移除。 |
| 中 | frontend/src/stores/app.ts:18 | LEGACY_SITE_NAMES 包含 DragonCode / Dragon Code | 普通页面不直接显示，但已打包进线上 index JS，开发者工具可搜到。建议修改，避免前端包暴露。 |
| 中 | Dockerfile:87,89 / Dockerfile.goreleaser:15,17 / deploy/Dockerfile:76,78 | 镜像 maintainer/source label | 不会在网页显示，但镜像元数据、容器检查可能看到。建议改为 laoshirenai。 |
| 中 | README.md / README_CN.md / deploy/README.md / deploy/DOCKER.md / deploy/install.sh / deploy/docker-deploy.sh / deploy/*.yml | 安装命令、镜像名、文档链接、GitHub Releases | 如果要交付给用户或公开仓库，必须修改。仅内部自用可后置。 |
| 低 | backend/go.mod:1 和 backend/internal/** / backend/ent/** 等约 870 个文件 | Go module path 和 import 路径 | 网页不直接显示；大范围改名风险高，需要 go.mod、所有 import、生成代码、CI、Docker 同步改。建议单独规划，不和前端品牌清理混在一起。 |
| 低 | SECURITY_FIX_*.md / SECURITY_AUDIT_HANDOFF_*.md | 安全修复过程文档里的旧路径 | 不属于运行代码，打包时已排除。若要公开仓库，可删除或归档。 |

## 线上检查结果

| 线上位置 | 检查结果 |
|---|---|
| https://laoshirenai.com/dashboard | 当前 Chrome 页面标题是“仪表盘 - 老实人AI”；屏幕显示文字未确认看到 dragon code。 |
| /assets/AppLayout.vue_vue_type_script_setup_true_lang-UKPu377U.js | 包含 https://github.com/bozhouDev/DragonCode-sub2api；对应 AppHeader.vue。 |
| /assets/index-Bgy4E-3i.js | 包含 ["Sub2API", "Dragon", "DragonCode", "Dragon Code"]；对应 stores/app.ts。 |
| /assets/KeyUsageView-DmVHGhGA.js | 包含 bozhouDev/DragonCode-sub2api；对应 KeyUsageView.vue。 |
| /assets/SettingsView-BNr5Vj3Z.js | 包含 raw.githubusercontent.com/bozhouDev/DragonCode-sub2api；对应 SettingsView.vue。 |

## 建议修改顺序

| 优先级 | 范围 | 建议 |
|---|---|---|
| 第一优先级 | 前端可见/可加载位置 | 改 AppHeader.vue、KeyUsageView.vue、SettingsView.vue、stores/app.ts；重新 build 并部署。 |
| 第二优先级 | 文档与部署配置 | 改 README、deploy 文档、Docker labels、image/repo 地址；避免交付包或公开仓库暴露。 |
| 第三优先级 | Go module/import 全量改名 | 只有在准备长期维护私有品牌仓库时再做。需要统一改 go.mod、imports、Ent 生成代码、CI、发布脚本并跑完整测试。 |

## 具体建议

1. 立即修改 frontend/src/components/layout/AppHeader.vue:143，把 GitHub 链接换成你们自己的仓库、官网文档，或直接移除该菜单项。
2. 修改 frontend/src/views/KeyUsageView.vue:378，把 GitHub 地址换成 laoshirenai 自己的文档入口，或删除外链按钮。
3. 修改 frontend/src/views/admin/SettingsView.vue:1915，把 raw.githubusercontent.com/bozhouDev/DragonCode-sub2api 的文档链接换成站内文档或 laoshirenai 仓库地址。
4. 修改 frontend/src/stores/app.ts:18，移除 DragonCode / Dragon Code 字符串；如果仍要保留旧品牌兼容，建议改成不含旧品牌明文的后端配置兜底逻辑。
5. 前端清理后必须重新构建和部署，并确认线上 assets 文件名变化后，再扫一次 /dashboard、/key-usage、/admin/settings 相关资源。
6. 暂时不建议马上全量重命名 backend/go.mod 和所有 Go import。那是大范围工程变更，容易影响构建、测试、镜像发布和生成代码。可以作为第二阶段品牌彻底清理任务。

## 备注

deploy 和 README 中的 DragonCode 链接虽然不一定在网站前台显示，但如果你们要把 zip、仓库、部署文档发给别人，也应该改。之前生成给桌面的交付 zip 已经排除了安全修复过程文档，但 README/deploy 文档仍属于项目必要文件，若要对外交付建议先做品牌清理后重新打包。
