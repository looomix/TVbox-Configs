# TVbox-Configs

TVBox / 影视仓 与 MoonTV / LunaTV **共用同一份已验证视频源**：每 7 天自动抓取多个公开源库、自动去重、逐条验证，只把真正能用的源写进订阅配置，并同步生成多条镜像地址。

| 产物 | 用途 |
| :--- | :--- |
| `api/tvbox.json` | TVBox / 影视仓 单仓配置（`sites` 数组） |
| `api/moontv.json` | MoonTV / LunaTV 源配置（`api_site` 对象，可直接粘贴导入） |
| `api/sources.txt` | 纯文本清单（`名称\|API\|类型\|状态`，多数客户端可一键导入） |
| `api/status.json` | 全量检测报告：状态、延迟、近 8 期趋势、抓取与去重统计 |
| `api/mirror.json` | 每个产物的 5 条镜像线路 |
| `api/index.json` | TVBox 多仓入口（主 / 备 / 加速） |
| `api/probe.json` | 网页「测试」按钮触发的单源云端检测结果 |

## 订阅地址

主线路（jsDelivr），同组还有 Pages 备用、Raw 直连、ghproxy、gh-proxy 四条，见网页或 `api/mirror.json`。

| 客户端 | 主线路 |
| :--- | :--- |
| TVBox / 影视仓 | `https://cdn.jsdelivr.net/gh/looomix/TVbox-Configs@main/api/tvbox.json` |
| MoonTV / LunaTV | `https://cdn.jsdelivr.net/gh/looomix/TVbox-Configs@main/api/moontv.json` |
| 纯文本导入 | `https://cdn.jsdelivr.net/gh/looomix/TVbox-Configs@main/api/sources.txt` |
| 多仓入口 | `https://looomix.github.io/TVbox-Configs/api/index.json` |

## 数据流

```
sources/feeds.json      远端源库（一键导入的订阅链接，支持 txt / json）
sources/sources.json    主源清单（网页新增 / 修改，手工维护）
        │
        ▼  ① 抓取 → ② 黑名单过滤 → ③ 按域名去重 → ④ 逐条验证
   只有 state=ok 的源进入产物
        │
        ▼
api/tvbox.json   api/moontv.json   api/sources.txt
api/status.json  api/mirror.json   api/index.json
```

### 抓取与去重

- `feeds.json` 里的每条链接都会在更新时被抓取，支持三种格式：纯文本 `名称|API|类型|可用率`、扁平 JSON 数组（`name` + `url`/`api`/`baseUrl`）、`api_site` 对象。
- 默认接入 9 个源库：资源族 CMS 接口清单、OuonnkiTV、KVideo、LibreTV、KatelyaTV、DecoTV、MoonTVPlus、LunaTV、CMS 状态全量清单；带 `isAdult` / `is_adult` 标记或分类为成人、以及被上游停用的条目直接丢弃。
- 抓取到的源统一清洗名称（去掉装饰符号与「资源站」后缀），标记为 `origin: feed`，网页上带「自动」标签，可单独停用或删除。
- 去重两层：先按**域名**（`http/https`、`www.`、首尾斜杠、大小写都不算差异；`…?url=` 代理按目标域名区分），再按**中文名**——`茅台 (mtzy5.com)`、`速播 (subocj.com)` 这类同站镜像域名不同、打开内容一样，一律归并为一条，保留上一轮验证可用、或手工添加、或地址更短的那条。

### 判定规则（宁缺毋滥，防误判）

1. 每个源最多请求 **3 次**：同一关键词重试两次 + 第二个关键词一次，避免单次抖动或搜索索引缺片误杀。
2. 只有 **HTTP 200 + 标准 JSON + list 非空 + 至少一条带 `http` 播放地址** 才记为 `ok`。
3. 搜索无结果但站点可访问时，再查一次「最新列表」；能返回可播放条目同样判为 `ok`（`searchable` 自动置 0）。
4. 首轮判为失效的源，会在整轮结束后用**新连接 + 双倍超时**再确认一次，确认不过才算失效。
5. `ok` 才写进 `tvbox.json` / `moontv.json` / `sources.txt`；`limited`（未验证）和 `down`（失效）只在 `status.json` 里留记录，**永远不出现在订阅链接里**。

### 7 天更新机制

- GitHub Actions 每天 16:00 UTC（北京时间 00:00）检查一次，生成器比对 `api/status.json` 的 `updated_at`，不足 168 小时就 `skip` 退出、不产生提交。
- 某次跑失败时，第二天会自动补跑；需要立刻刷新时，网页点「立即重新生成」或 Actions 手动 Run workflow（`force=true`）。
- 改周期用 `-interval=72h`，网页上的「下次更新」会跟着变。

## 网页管理

<https://looomix.github.io/TVbox-Configs/>

- **浏览 / 筛选**：按名称、地址搜索，按「可用 / 未验证 / 失效 / 停用」和分类过滤。
- **测试**：单条源点「测试」，由 `probe.yml` 在云端真实探测（采集接口几乎不带 CORS 头，浏览器直连会被拦），约 1 分钟返回状态、延迟与条数。
- **新增 / 修改 / 删除**：直接提交 `sources/sources.json` 并触发一次重新生成。
- **批量删除**：表头勾全选或多选后点「批量删除」，一次提交完成；被删掉的接口会记进 `sources/sources.json` 的 `removed`，后续抓取不会把它加回来（要恢复就重新粘一次该接口）。
- **批量导入**：`＋ 新增源` 下方一行输入框，粘贴多条接口地址（换行 / 逗号分隔）即按域名去重后入库并触发验证；粘贴源库链接（txt / `api_site` json）则接入为自动抓取源。
- 写操作需要令牌（只存浏览器 `localStorage`，只发给 `api.github.com`）；只读浏览不需要。

> 建议用 fine-grained token，仅授权本仓库 `Contents: Read and write` 与 `Actions: Read and write`。不要把令牌写进任何文件或提交。

## 本地运行

```bash
go run ./cmd/tvboxgen -feed-only      # 只抓取 + 去重，写回 sources/sources.json
go run ./cmd/tvboxgen -force          # 立即抓取、验证、重写 api/*
go run ./cmd/tvboxgen                 # 不足 7 天则跳过
go run ./cmd/tvboxgen -interval=72h -workers=16 -timeout=10s -history=8
```

## 文件格式

`sources/sources.json`

```json
{
  "version": 1,
  "updated_at": "2026-11-28",
  "sources": [
    { "id": "maoyan", "name": "猫眼", "api": "https://api.example.com/api.php/provide/vod",
      "group": "综合", "enabled": true, "origin": "manual", "added_at": "2026-11-28" }
  ]
}
```

`sources/feeds.json`

```json
{ "version": 1, "feeds": [ { "name": "资源族 CMS 接口清单", "url": "https://…/list.txt", "enabled": true } ] }
```

`sources/blocklist.json`：`domains`（按域名后缀拦截）+ `words`（按名称/地址关键词拦截），被拦截的源不会进入 `sources.json`，也不会出现在任何产物里。

## 免责声明

- 本仓库只做技术整理：清单里的接口全部来自公开互联网，视频内容、版权与可用性由原站点负责，本项目不存储、不分发任何影音文件。
- 不含成人内容，生成器会按 `blocklist.json` 与分类字段过滤。
- 接口随时可能失效或变更，配置按 7 天一次的节奏自动校正，请勿用于商业用途。

## License

[MIT](LICENSE)
