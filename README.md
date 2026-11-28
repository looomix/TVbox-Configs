# TVbox-Configs

自动生成和管理视频源订阅：Go 生成器 + 静态管理页。源来自公开清单，逐条探活；只有验证通过且未被过滤的条目写入订阅。默认过滤成人内容源，本地可用 `-allow-nsfw` 关闭。

## 导入地址

TVBox / 影视仓的「设置 → 配置地址」只填**单仓配置**：

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
```

备用线路（按顺序试）：

```
https://gh-proxy.com/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://looomix.github.io/TVbox-Configs/api/tvbox.json
```

支持多仓的客户端可填 TXT 入口：

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.txt
```

其他客户端按各自后台要求选择：

| 文件 | 适用客户端 |
| :-- | :-- |
| `api/tvbox.json` | TVBox / 影视仓单仓配置 |
| `api/moontv.json` | MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus |
| `api/list.json` | OuonnkiTV / KVideo / LibreTV |
| `api/sources.txt` | 支持粘贴 `名称\|API` 清单的客户端 |
| `api/tvbox.txt` | TVBox / 影视仓多仓 TXT 入口 |

`api/status.json`、`api/mirror.json`、`api/index.json`、`api/probe.json` 是内部数据，**不要**填进客户端配置地址。填错文件是首页空白的最常见原因；其次检查链接首尾空格、清缓存、换 DNS。

## 自己部署

1. Fork 本仓库。
2. 进入 **Actions**，点绿色按钮启用工作流。
3. **Settings → Pages**：Source 选 `Deploy from a branch`，Branch 选 `main`、`/ (root)`，保存。
4. 仓库设为 **Public**；创建 fine-grained token，仅授权本仓库 `Contents: Read and write` 与 `Actions: Read and write`，在管理页「设置令牌」中填入。
5. 在管理页新增或导入源，等待 `Update Configs` 完成；订阅链接中的用户名会自动跟随你的仓库。

## 管理页

`https://<用户名>.github.io/<仓库名>/`

可新增、批量导入、测试、修改、删除源；删除记录会阻止源库重新加回同一接口。网页写入 `sources/sources.json`，云端重新生成订阅。

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

本项目仅为配置文本生成与管理工具，**不存储、不缓存、不代理、不转码、不分发、不提供任何音视频内容**。仓库内所有接口地址均由使用者自己录入，或由使用者自行配置的第三方公开清单抓取而来；本项目不拥有这些接口，也不对其内容、版权状态、合法性、可用性、安全性或准确性作任何审查、背书或保证。第三方接口可能随时变更、失效或停止服务。

使用者应自行确认其录入、订阅、传播和使用的接口及内容符合所在地法律法规、平台规则与相关服务条款，并自行承担由此产生的全部责任与风险。任何因使用或无法使用本项目、或因依赖仓库内配置文本而导致的损失、纠纷、行政处罚、第三方索赔或其他后果，均由使用者自行承担；项目作者与贡献者在法律允许的最大范围内不承担任何责任，包括但不限于直接损失、间接损失、数据丢失、设备损坏及可得利益损失。

本项目按「现状」（AS IS）提供，不作任何明示或默示担保，包括但不限于适销性、特定用途适用性、准确性、持续可用性与不侵权担保。若权利人认为某条目侵犯其权益，应向对应第三方接口的运营主体主张权利；本项目不控制、不托管该接口及其内容。禁止将本项目用于任何违法用途；如所在地法律不允许相关使用，请立即停止。

## License

MIT
