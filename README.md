# TVbox-Configs

一个**容器型**视频源配置仓库：自带生成器、网页管理台和 9 种客户端订阅产物，**默认不包含任何视频源**。放什么源、怎么用，由使用者自己决定。

- 一次生成，多客户端可用：TVBox / 影视仓、MoonTV / LunaTV、KatelyaTV、DecoTV、MoonTVPlus、OuonnkiTV、KVideo、LibreTV、纯文本清单
- 网页即可新增、导入、测试、修改、单条删除、批量删除
- 网页写入落到 `sources/sources.json`，云端 Action 重新生成订阅
- 7 天一次自动检测，单次运行硬上限 45 秒

## 目录

```
cmd/tvboxgen        生成器入口（Go，无第三方依赖）
internal/probe      接口探活（CMS 搜索 + 最新列表，最多三次请求定结论）
internal/gen        产物构造、接口键（域名 + 路径前缀）与中文名去重
internal/feed       源库拉取（纯文本 / json / api_site 三种格式）
internal/mirror     各产物的多镜像地址
sources/            源清单与源库清单（默认全空）
api/                生成后的产物（勿手改）
index.html          管理页
```

## 使用

1. Fork 或克隆本仓库，在 **Settings → Pages** 选择 `main` / 根目录。
2. 打开 `https://<用户名>.github.io/<仓库名>/`，点右上角「设置令牌」填入 PAT（fine-grained，勾选本仓库的 **Contents: Read and write** 与 **Actions: Read and write**）。
3. 在「＋ 新增源」逐条添加，或在下方整行粘贴多个接口地址（换行 / 逗号分隔）批量导入。
4. 保存后自动触发 `Update Configs`，约 1 分钟产物更新，订阅地址始终不变。

订阅地址形如 `https://cdn.jsdelivr.net/gh/<用户名>/<仓库名>@main/api/tvbox.json`，9 个产物各配 5 条镜像，页面上一键复制。

| 产物 | 用途 |
| :--- | :--- |
| `api/tvbox.json` | TVBox / 影视仓 单仓配置（`sites` 数组） |
| `api/moontv.json` | MoonTV / LunaTV 源配置（`api_site` 对象） |
| `api/katelyatv.json` | KatelyaTV 源配置（同 MoonTV 结构） |
| `api/decotv.json` | DecoTV 源配置（同 MoonTV 结构） |
| `api/moontvplus.json` | MoonTVPlus 源配置（同 MoonTV 结构） |
| `api/ouonnkitv.json` | OuonnkiTV 源清单（`id / name / url / detailUrl / timeout / isEnabled`） |
| `api/kvideo.json` | KVideo 源清单（`id / name / baseUrl / group / enabled`） |
| `api/libretv.json` | LibreTV 源清单（`name / url`） |
| `api/sources.txt` | 纯文本清单（`名称\|API\|类型\|状态`） |
| `api/status.json` | 全量检测报告：状态、延迟、近 8 期趋势、去重统计 |
| `api/mirror.json` | 每个产物的 5 条镜像线路 |
| `api/index.json` | TVBox 多仓入口（主 / 备 / 加速） |
| `api/probe.json` | 网页「测试」按钮的单源云端检测结果 |

## 命令行

```bash
go build -o bin/tvboxgen ./cmd/tvboxgen
./bin/tvboxgen -force             # 立即生成一次
./bin/tvboxgen                    # 距上次不足 7 天则跳过
./bin/tvboxgen -feed-only         # 只把源库合并进 sources.json，不探活
./bin/tvboxgen -probe=<接口地址>   # 只测一个接口
```

常用参数：`-workers`（并发，默认 24）、`-timeout`（单请求，默认 8s）、`-budget`（整轮上限，默认 50s）、`-interval`（更新间隔，默认 168h）、`-history`（每源保留的检查点）。

探活判定：先按两个关键词搜索，再取最新列表；任一成功且返回真实条目即 `ok`，能连但返回空则 `limited`，连不上才 `down`。**只有 `ok` 的源会写进订阅产物**，`limited` 与 `down` 只留在 `api/status.json`。

## 免责声明

本项目仅提供配置生成与管理工具，**不存储、不转码、不代理、不分发任何音视频内容**。

- 所有接口地址由仓库使用者自行录入，或从其自行配置的第三方清单抓取。本项目不筛选、不背书，也不保证任何第三方接口的合法性、可用性、安全性或内容合规性；该第三方接口的运营主体、内容版权及由此产生的法律责任，均与本项目及作者无关。
- 使用者应确保其录入、传播与使用的接口及内容符合所在地法律法规，并符合其所用软件的许可条款。因使用本项目产生的版权纠纷、行政处罚或其他法律后果，由使用者自行承担。
- 本项目按「现状」（AS IS）提供，不作任何明示或默示担保，包括但不限于适销性、特定用途适用性与不侵权担保。作者不对因使用或无法使用本项目造成的任何直接或间接损失承担责任。
- 仓库内产物仅为结构化配置文本，不含任何媒体文件；本项目不承载、不缓存第三方内容。如权利人认为某条目侵权，应针对该第三方接口主张权利。
- 禁止将本项目用于任何违法用途；如所在地法律不允许相关使用，请立即停止。

## English

**TVbox-Configs** is a container-style repository for video source configs: a Go generator, a static web console, and nine subscription artifacts (TVBox, MoonTV / LunaTV, KatelyaTV, DecoTV, MoonTVPlus, OuonnkiTV, KVideo, LibreTV, plus a plain text list). **It ships with no sources** — what you put in is entirely up to you.

- **Setup**: fork the repo, enable GitHub Pages (`main` / root), open the site, and paste a fine-grained PAT with `Contents: read & write` and `Actions: read & write` scoped to this repository.
- **Manage**: add sources one by one, or paste many interface URLs at once (newline / comma separated); test, edit, delete a single row, or delete a multi-selected batch. Every write lands in `sources/sources.json` and triggers one cloud run.
- **Generate**: `go build -o bin/tvboxgen ./cmd/tvboxgen && ./bin/tvboxgen -force`. Each run is hard-capped (`-budget`, default 50s). Only endpoints that answer with a real vod list (`ok`) reach the artifacts; the rest stay in `api/status.json`.
- **Dedupe**: by endpoint key (host + first meaningful path segments, so two stations on one host stay apart) and by normalized Chinese display name, so mirrors of the same station collapse into one entry.
- **Disclaimer**: this project only generates and manages configuration files. It does **not** host, proxy, transcode, or distribute any media. All endpoints are supplied by the repository's own users or fetched from lists they configure; the project does not screen or endorse them and gives no warranty as to their legality, availability, security, or content rights. Users are solely responsible for complying with the laws of their jurisdiction and the license terms of the software they use. Provided "as is", without any express or implied warranty, including merchantability, fitness for a particular purpose, and non-infringement; the author accepts no liability for any direct or indirect damages arising from use of, or inability to use, this project.

## License

MIT
