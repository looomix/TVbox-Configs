# TVbox-Configs

[![update](https://img.shields.io/github/actions/workflow/status/looomix/TVbox-Configs/update.yml?label=update&style=flat-square)](https://github.com/looomix/TVbox-Configs/actions/workflows/update.yml)
[![sites](https://img.shields.io/badge/sites-75-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/api/tvbox.json)
[![sources](https://img.shields.io/badge/sources-119-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/sources/sources.json)
[![links](https://img.shields.io/badge/links-25-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/api/mirror.json)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&style=flat-square)](https://go.dev/)
[![last commit](https://img.shields.io/github/last-commit/looomix/TVbox-Configs?style=flat-square)](https://github.com/looomix/TVbox-Configs/commits/main)
[![Stars](https://img.shields.io/github/stars/looomix/TVbox-Configs?style=flat-square)](https://github.com/looomix/TVbox-Configs/stargazers)
[![license](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

从公开来源自动聚合视频接口，生成 TVBox / 影视仓 / MoonTV 等客户端可直接导入的订阅配置。
每 168 小时（7 天）自动探活更新 · 实时数值见[管理页](https://looomix.github.io/TVbox-Configs/)与 [`api/status.json`](api/status.json)。
最近一次探测：75/119 个源可用（63%），剔除 34 个失效地址，过滤 23 个成人内容源。

中文 | [English](README_EN.md) | [部署](#自己部署) · [功能特性](#功能特性) · [导入地址](#导入地址) · [客户端](#其他客户端) · [免责声明](#免责声明)

> 徽章中的 sites / sources / links 为最近一次生成的快照；实时值以管理页与 `api/status.json` 为准。

## 功能特性

- Go 生成器 + 静态管理页：探活、去重、过滤后生成订阅，只有验证通过的源进入配置。
- 源来自公开清单与手动录入，逐条按真实 CMS 列表返回验证，保留最近 8 期历史。
- 默认过滤成人内容源，本地可用 `-allow-nsfw` 关闭。
- 一处维护，多端订阅：TVBox / 影视仓单仓、MoonTV 系、OuonnkiTV / KVideo / LibreTV、纯文本清单。
- 管理页支持新增、批量导入、测试、修改、删除源，数据写入 `sources/sources.json` 后由云端 Action 重新生成。
- 源库链接是一次性导入队列：云端抓取后即写入 `sources/sources.json` 并从 `sources/feeds.json` 移除，失败的重试到成功为止；删空全部源时同步清空队列，不会越积越多。
- 管理页提供中英文切换、运行状态徽章与探测摘要；订阅地址按客户端分组，每条含 5 条线路。

## 导入地址

所有源都是苹果CMS `provide/vod` 接口，统一标为 `type 1`（无需 jar）。TVBox 与影视仓同源、共用这套协议，**用同一条地址**：

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
```

按顺序替换线路：

```
https://gh-proxy.org/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://ghfast.top/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://looomix.github.io/TVbox-Configs/api/tvbox.json
```

影视仓导入注意：① 确认客户端版本支持「无 jar 的接口源」，过旧版本需升级；② 导入后在「仓库管理」里点选启用该仓库；③ 换地址先删除旧配置，并清缓存后重导入（影视仓默认有约 3 天缓存）；④ 若 ghproxy 提示地址无效，直接用 `raw.githubusercontent.com` 或 `looomix.github.io` 直链；⑤ 仍无法导入时，浏览器打开直链、**全选复制 JSON 文本**，在影视仓「本地导入 / 粘贴配置」里贴入，可完全绕过 URL 校验。支持多仓的客户端可用 `api/tvbox.txt`。

### 其他客户端

| 文件 | 适用客户端 | 用法 |
| :-- | :-- | :-- |
| `api/moontv.json` | MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus | 在「视频源配置 / 配置文件」里**粘贴 JSON 内容**；这类项目多数不支持 URL 订阅 |
| `api/list.json` | OuonnkiTV / KVideo / LibreTV | 源清单订阅地址（数组格式） |
| `api/sources.txt` | 支持粘贴 `名称\|API` 清单的客户端 | 纯文本 |
| `api/tvbox.txt` | TVBox / 影视仓多仓 TXT 入口 | 多仓导入 |

`api/status.json`、`api/mirror.json`、`api/index.json`、`api/probe.json` 是内部数据，**不要**填进客户端配置地址。填入前确认链接无首尾空格；TVBox 类客户端改地址后需删除旧配置重导入以清缓存。

## 自己部署

1. Fork 本仓库。
2. 进入 **Actions**，点绿色按钮启用工作流。
3. **Settings → Pages**：Source 选 `Deploy from a branch`，Branch 选 `main`、`/ (root)`，保存。
4. 仓库设为 **Public**；创建 fine-grained token，仅授权本仓库 `Contents: Read and write` 与 `Actions: Read and write`，在管理页「设置令牌」中填入。
5. 在管理页新增或导入源，等待 `Update Configs` 完成；订阅链接中的用户名会自动跟随你的仓库。

## 管理页

`https://<用户名>.github.io/<仓库名>/`

顶部显示运行状态徽章、更新与探测摘要，可切换中英文；按客户端分组展示订阅地址及备用线路。可新增、批量导入、测试、修改、删除源；删除记录会阻止同一接口被重新加回。源库链接为一次性导入：云端抓取合并后即从 `sources/feeds.json` 移除，删空全部源时队列同步清空，不会累积历史链接。网页写入 `sources/sources.json`，云端重新生成订阅。

## 命令行

```bash
go build -o bin/tvboxgen ./cmd/tvboxgen
./bin/tvboxgen -force                 # 立即生成
./bin/tvboxgen -feed-only             # 只合并源库，不探活
./bin/tvboxgen -probe=<接口地址>       # 测单个接口
./bin/tvboxgen -force -allow-nsfw     # 保留成人内容源
```

常用参数：`-workers`、`-timeout`、`-budget`、`-interval`、`-history`。探活以真实 CMS 列表返回为准；仅 `ok` 的源进入订阅。

## 免责声明

本项目是**配置文本生成与管理工具**，仅输出供客户端读取的 JSON / TXT 文件。**本项目不存储、不缓存、不代理、不转码、不分发、不提供任何音视频内容**，也不提供播放服务。仓库中的接口地址由使用者自行录入，或由使用者自行配置的第三方公开清单抓取而来；本项目不拥有这些接口，不对其内容、版权状态、合法性、可用性、安全性或准确性进行审查、背书或保证。第三方接口可能随时变更、失效、返回错误内容或停止服务。

使用者应自行确认其录入、订阅、传播和使用的接口及内容符合所在地法律法规、平台规则与相关服务条款，并自行承担由此产生的全部责任与风险。任何因使用或无法使用本项目、因依赖仓库内配置文本、或因第三方接口的内容或失效而导致的损失、纠纷、行政处罚、第三方索赔或其他后果，均由使用者自行承担；项目作者与贡献者在法律允许的最大范围内不承担任何责任，包括但不限于直接损失、间接损失、数据丢失、设备损坏及可得利益损失。

本项目按「现状」（AS IS）提供，不作任何明示或默示担保，包括但不限于适销性、特定用途适用性、准确性、持续可用性与不侵权担保。若权利人认为某条目侵犯其权益，应向对应第三方接口的运营主体主张权利；本项目不控制、不托管该接口及其内容。禁止将本项目用于任何违法用途；如所在地法律不允许相关使用，请立即停止。

## License

MIT
