# TVbox-Configs

[![update](https://img.shields.io/github/actions/workflow/status/looomix/TVbox-Configs/update.yml?label=update&style=flat-square)](https://github.com/looomix/TVbox-Configs/actions/workflows/update.yml)
[![sites](https://img.shields.io/badge/sites-54-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/api/tvbox.json)
[![sources](https://img.shields.io/badge/sources-71-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/sources/sources.json)
[![links](https://img.shields.io/badge/links-25-blue?style=flat-square)](https://github.com/looomix/TVbox-Configs/blob/main/api/mirror.json)
[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&style=flat-square)](https://go.dev/)
[![last commit](https://img.shields.io/github/last-commit/looomix/TVbox-Configs?style=flat-square)](https://github.com/looomix/TVbox-Configs/commits/main)
[![Stars](https://img.shields.io/github/stars/looomix/TVbox-Configs?style=flat-square)](https://github.com/looomix/TVbox-Configs/stargazers)
[![license](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

从公开清单聚合视频接口，自动探活、去重，生成多客户端可直接导入的订阅配置。

[中文](README.md) · [English](README_EN.md) · [在线管理页](https://looomix.github.io/TVbox-Configs/) · [实时状态](api/status.json)

> 徽章数值为最近一次生成快照；实时数据以管理页与 `api/status.json` 为准。

---

## 目录

- [快速开始](#快速开始)
- [功能特性](#功能特性)
- [客户端对照](#客户端对照)
- [自己部署](#自己部署)
- [管理页](#管理页)
- [免责声明](#免责声明)

---

## 快速开始

**只需一步**：复制下面的地址，填入 TVBox / 影视仓的「设置 → 配置地址」。

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
```

打不开时按顺序换备用线路：

| 线路 | 地址 |
| :-- | :-- |
| gh-proxy | `https://gh-proxy.org/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| ghfast | `https://ghfast.top/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| Raw 直连 | `https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| Pages | `https://looomix.github.io/TVbox-Configs/api/tvbox.json` |

**影视仓提示**：① 使用支持「无 jar 接口源」的版本；② 导入后在「仓库管理」启用该仓库；③ 换地址先删旧配置并清缓存（约 3 天缓存）；④ 若 ghproxy 被拒，改用 Raw / Pages 直链；⑤ 仍失败时，浏览器打开直链全选复制 JSON，在「本地导入 / 粘贴配置」贴入。

---

## 功能特性

| 能力 | 说明 |
| :-- | :-- |
| 自动聚合 | 从公开清单与手动录入抓取接口，逐条按真实 CMS 列表返回验证 |
| 质量校验 | 探活、去重；仅验证通过的源进入订阅，保留最近 8 期历史 |
| 多端订阅 | 一处维护，同时输出 TVBox / 影视仓单仓、MoonTV 系、OuonnkiTV / KVideo / LibreTV、纯文本清单 |
| 可视化管理 | 静态管理页支持新增、批量导入、测试、修改、删除，中英文切换，状态徽章与探测摘要 |
| 云端生成 | 网页写入 `sources/sources.json` 后由 GitHub Actions 自动重新生成并发布 |
| 一次性订阅 | 源库链接抓取合并后即移除，失败项自动重试；删空全部源时队列同步清空，不会累积 |

---

## 客户端对照

| 文件 | 适用客户端 | 导入方式 |
| :-- | :-- | :-- |
| `api/tvbox.json` | TVBox / 影视仓 | 单仓配置地址 |
| `api/moontv.json` | MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus | 粘贴 JSON 内容（多数不支持 URL 订阅） |
| `api/list.json` | OuonnkiTV / KVideo / LibreTV | 数组格式源清单订阅 |
| `api/sources.txt` | 支持 `名称\|API` 的客户端 | 纯文本清单 |
| `api/tvbox.txt` | 支持多仓的 TVBox / 影视仓 | 多仓 TXT 入口 |

> `api/status.json`、`api/mirror.json`、`api/index.json`、`api/probe.json` 为内部数据，**不要**填进客户端配置地址。

---

## 自己部署

1. **Fork** 本仓库。
2. 进入 **Actions**，启用工作流。
3. **Settings → Pages**：Source 选 `Deploy from a branch`，Branch 选 `main`、`/ (root)`，保存。
4. 仓库设为 **Public**；创建 fine-grained token，仅授权本仓库 `Contents: Read and write` 与 `Actions: Read and write`，在管理页「设置令牌」中填入。
5. 在管理页新增或导入源，等待 `Update Configs` 完成。订阅链接中的用户名会自动跟随你的仓库。

---

## 管理页

`https://<用户名>.github.io/<仓库名>/`

| 区域 | 用途 |
| :-- | :-- |
| 运行状态总览 | 查看更新/探测摘要、站点与源数量、线路数、Stars、最近提交 |
| 快速导入 | 复制 TVBox / 影视仓首选单仓地址与多仓 TXT |
| 订阅地址 | 按客户端分组，每条含 5 条加速/直连线路 |
| 视频源 | 搜索、筛选、测试、编辑、删除、批量导入；删除记录阻止同一接口被重新加回 |
| 自己部署 | Fork → Actions → Pages 步骤指引 |

源库链接为一次性导入队列：云端抓取合并后即从 `sources/feeds.json` 移除，删空全部源时队列同步清空。网页写入 `sources/sources.json`，云端重新生成订阅。未设置令牌时为只读模式，可浏览与测试。

---

## 免责声明

本项目是**配置文本生成与管理工具**，仅输出供客户端读取的 JSON / TXT 文件。**本项目不存储、不缓存、不代理、不转码、不分发、不提供任何音视频内容**，也不提供播放服务。仓库中的接口地址由使用者自行录入，或由使用者自行配置的第三方公开清单抓取而来；本项目不拥有这些接口，不对其内容、版权状态、合法性、可用性、安全性或准确性进行审查、背书或保证。第三方接口可能随时变更、失效、返回错误内容或停止服务。

使用者应自行确认其录入、订阅、传播和使用的接口及内容符合所在地法律法规、平台规则与相关服务条款，并自行承担由此产生的全部责任与风险。任何因使用或无法使用本项目、因依赖仓库内配置文本、或因第三方接口的内容或失效而导致的损失、纠纷、行政处罚、第三方索赔或其他后果，均由使用者自行承担；项目作者与贡献者在法律允许的最大范围内不承担任何责任，包括但不限于直接损失、间接损失、数据丢失、设备损坏及可得利益损失。

本项目按「现状」（AS IS）提供，不作任何明示或默示担保，包括但不限于适销性、特定用途适用性、准确性、持续可用性与不侵权担保。若权利人认为某条目侵犯其权益，应向对应第三方接口的运营主体主张权利；本项目不控制、不托管该接口及其内容。禁止将本项目用于任何违法用途；如所在地法律不允许相关使用，请立即停止。

---

## License

MIT
