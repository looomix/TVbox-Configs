# TVbox-Configs

TVBox / 影视仓 视频源配置自动生成器：每 **7 天**检测一次公开的 CMS 采集接口，只把**当前可用**的视频源写进订阅配置，并同时生成多条镜像订阅地址。

| 项目 | 说明 |
| :--- | :--- |
| 订阅配置 | `api/tvbox.json`（单仓配置，TVBox 直接粘贴） |
| 多仓入口 | `api/index.json`（内含主线路 / 备用 / 加速三条） |
| 检测报告 | `api/status.json`（每个源的状态、延迟、近 8 期趋势） |
| 单源检测 | `api/probe.json`（网页「测试」按钮触发的云端结果） |
| 镜像清单 | `api/mirror.json` |
| 网页管理 | <https://looomix.github.io/TVbox-Configs/>（新增 / 修改 / 测试 / 删除） |

## 订阅地址

复制任意一条到 TVBox「设置 → 配置地址」。主线路失效时按顺序换用下面的备用地址。

| 线路 | 地址 |
| :--- | :--- |
| jsDelivr 主线路 | `https://cdn.jsdelivr.net/gh/looomix/TVbox-Configs@main/api/tvbox.json` |
| GitHub Pages 备用 | `https://looomix.github.io/TVbox-Configs/api/tvbox.json` |
| Raw 直连 | `https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| ghproxy 加速 | `https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| gh-proxy 加速 | `https://gh-proxy.com/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |

> 这些镜像地址由生成器在每次更新时自动写进 `api/mirror.json`，网页上可一键复制。

## 工作方式

```
sources/sources.json        主源清单（人工或网页维护，唯一需要手改的文件）
        │
        ▼  tvboxgen（Go，标准库实现，无第三方依赖）
   并发探测每个接口：GET {api}?wd=庆余年&ac=detail
   ├─ 200 且返回 list  → ok      可用
   ├─ 200 但 list 为空 → limited 可访问（进配置，排在可用源之后）
   └─ 超时 / 非 200    → down    失效（不进配置，仅记录状态）
        │
        ▼
api/tvbox.json   只含 ok / limited 的源，按延迟排序
api/status.json  全量状态 + 近 8 期历史
api/mirror.json  六条镜像订阅地址
api/index.json   多仓入口（主 / 备 / 加速）
```

### 7 天更新机制

- GitHub Actions 每天 16:00 UTC（北京时间 00:00）触发一次，**不会每天都写配置**。
- 生成器读取 `api/status.json` 里的 `updated_at`，距上次成功更新不足 7 天（168 小时）就打印 `skip` 并退出，不产生任何提交。
- 好处：某次运行失败（接口全挂、Actions 排队）时，第二天会自动补跑；不需要手工干预。
- 需要立刻刷新时，在网页点「立即重新生成」，或在 Actions 页面手动 Run workflow（`force=true`）。
- 想改周期：改 `-interval`（例如 `-interval=72h`）即可，网页上的「下次更新」会跟着变。

## 网页管理

打开 <https://looomix.github.io/TVbox-Configs/> 即可：

- **浏览 / 搜索**：按名称、地址、备注筛选，按状态（可用 / 失效 / 停用）和分类过滤。
- **测试**：单条源点「测试」，由 `probe.yml` 在 GitHub Actions 里真实探测（采集接口几乎都不带 CORS 头，浏览器直连会被拦），约 1 分钟返回状态、延迟与条数，结果写进 `api/probe.json`。
- **新增 / 修改 / 删除**：走 GitHub Contents API 直接提交 `sources/sources.json`，并顺手触发一次立即检测。
- 写操作需要在页面里填入一个有该仓库写权限的令牌，令牌只存在浏览器 `localStorage`，只发给 `api.github.com`。只读浏览不需要令牌。

> 建议用 fine-grained token，仅授权本仓库 `Contents: Read and write` 与 `Actions: Read and write`。**不要把令牌写进任何文件或提交。**

## 本地运行

```bash
go run ./cmd/tvboxgen -force            # 立即检测并重写 api/*
go run ./cmd/tvboxgen                   # 不足 7 天则跳过
go run ./cmd/tvboxgen -interval=72h -workers=16 -timeout=10s
```

常用参数：`-sources`（源清单路径）、`-out`（输出目录）、`-force`、`-interval`、`-timeout`、`-workers`、`-history`（每源保留的历史点数，默认 8）、`-owner`/`-repo`/`-branch`（生成镜像地址用）。

网页本地预览：仓库根目录执行 `python3 -m http.server 8080`，访问 <http://localhost:8080/>。注意 `api/*.json` 需要先生成。

## 源清单格式

`sources/sources.json`：

```json
{
  "version": 1,
  "updated_at": "2026-11-27",
  "sources": [
    {
      "id": "maoyan",
      "name": "猫眼",
      "api": "https://api.example.com/api.php/provide/vod",
      "group": "综合",
      "note": "秒播 / 有广告",
      "enabled": true,
      "added_at": "2026-11-27"
    }
  ]
}
```

`group` 支持 `综合 / 影视 / 动漫 / 综艺 / 短剧 / 其他`，对应配置里的 `group` 分组标签。`enabled: false` 只是停用，不会从清单里消失。

## 相比原项目删掉的东西

| 原功能 | 处理 |
| :--- | :--- |
| 每日全量刷新 | 改为 7 天一次（失败次日自动补跑） |
| Python 检查脚本 + pip 依赖 | 换成 Go 单二进制，零第三方依赖 |
| NSFW / 成人频道分类与统计 | 整个删除，生成器内置关键字拦截，不会输出到任何配置 |
| Mihomo / Clash 规则生成 | 删除，与视频源无关 |
| Cloudflare Worker + KV 后端 | 删除，改由静态网页 + GitHub API 直接读写源清单 |
| README 自动生成的统计长表 | 删除，状态改由网页实时渲染，README 保持静态 |

## 免责声明

- 本仓库只做**技术整理**：清单里的接口全部来自公开互联网，视频内容、版权与可用性均由原站点负责，本项目不存储、不分发任何影音文件。
- 不含成人内容；生成器会过滤相关关键字。
- 接口随时可能失效或变更，配置按 7 天一次的节奏自动校正，请勿用于商业用途。

## License

[MIT](LICENSE)
