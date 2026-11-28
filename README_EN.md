# TVbox-Configs

Aggregates video API endpoints from public lists, probes and deduplicates them, and generates subscriptions that TVBox, YSC, MoonTV and other clients can import directly.

[中文](README.md) · English · [Live admin page](https://looomix.github.io/TVbox-Configs/) · [Live status](api/status.json)

> Badge numbers are a snapshot from the latest generation. See the admin page and `api/status.json` for live values.

---

## Contents

- [Quick start](#quick-start)
- [Features](#features)
- [Verdict rules](#verdict-rules)
- [Subscription URLs](#subscription-urls)
- [Client matrix](#client-matrix)
- [Self-host](#self-host)
- [Admin page](#admin-page)
- [Command line](#command-line)
- [File map](#file-map)
- [Disclaimer](#disclaimer)

---

## Quick start

**One step**: copy the URL below into "Settings → Config URL" in TVBox / YSC.

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
```

If it fails, try the backups in order:

| Route | URL |
| :-- | :-- |
| gh-proxy | `https://gh-proxy.org/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| ghfast | `https://ghfast.top/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| Raw direct | `https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json` |
| Pages | `https://looomix.github.io/TVbox-Configs/api/tvbox.json` |

**YSC tips**: ① use a version supporting jar-free endpoints; ② enable the repository in "Repository Management" after import; ③ delete the old config and clear cache before re-importing (~3-day cache); ④ if a proxy URL is rejected, use the Raw / Pages direct link; ⑤ as a last resort, open the direct link in a browser, copy the whole JSON and paste it into "local import / paste config".

---

## Features

| Capability | Details |
| :-- | :-- |
| Auto aggregation | Fetches endpoints from public lists and manual entries, verifying each against a real CMS list response |
| Quality filtering | Probes, deduplicates and filters adult sources; only verified ones are published, with the last 8 check results kept |
| Multi-client | One source list feeds TVBox / YSC, MoonTV family, OuonnkiTV / KVideo / LibreTV and plain-text output |
| Visual admin | Static page for add, bulk import, test, edit and delete, with language switcher, badges and probe summary |
| Cloud generation | Page writes `sources/sources.json`; GitHub Actions regenerates and publishes automatically |
| One-shot feeds | Feed URLs are removed after being fetched and merged; failures are retried; deleting all sources clears the queue |

---

## Verdict rules

Probes run on GitHub Actions (an overseas datacenter), which is not your local network. The verdict is therefore conservative: **a source is only marked dead when the site explicitly refuses to serve it; network-level failures never count as dead.**

| State | Condition | Published |
| :-- | :-- | :-- |
| Alive | A valid vod list with at least one playable link | Yes |
| Unverified | Host reachable, but no usable list: not an API, empty list, no playable link, 403/rate limit, DNS/TLS/timeout/connection error | No |
| Dead | The site explicitly refuses: HTTP 404/410, or a persistent 5xx | No |

Each endpoint is tried with `?ac=detail&pg=1`, a keyword search, `?ac=videolist&pg=1`, a forced JSON request and the bare address; both JSON and Apple CMS XML are understood; certificate errors get one insecure retry; proxy endpoints (`?url=…`) receive the parameters inside the target URL.

**Unverified does not mean dead**: geo blocking, cross-border latency and broken certificates can stop the probe while the source still works for you. Hover the state badge to see the exact reason.

---

## Subscription URLs

All sources are Apple CMS `provide/vod` endpoints marked `type 1` (no jar plugin). TVBox and YSC share the same URL. Multi-repo clients can use `api/tvbox.txt`; the full entry index is `api/index.json`.

### Client matrix

| File | Clients | How to import |
| :-- | :-- | :-- |
| `api/tvbox.json` | TVBox / YSC | Single-repo config URL |
| `api/moontv.json` | MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus | Paste JSON (most do not support URL subscription) |
| `api/list.json` | OuonnkiTV / KVideo / LibreTV | JSON-array source list URL |
| `api/sources.txt` | Clients accepting a plain `name\|API` list | Plain text |
| `api/tvbox.txt` | Multi-repo TVBox / YSC | Multi-repo TXT index |

> `api/status.json`, `api/mirror.json`, `api/index.json` and `api/probe.json` are internal data — **do not** paste them into a client's config URL. Check for leading/trailing spaces, and delete the old config before re-importing to clear cache.

---

## Self-host

1. **Fork** this repository.
2. Open **Actions** and enable the workflows.
3. **Settings → Pages**: Source = `Deploy from a branch`, Branch = `main`, `/ (root)`, then save.
4. Make the repo **Public**. Create a fine-grained token with `Contents: Read and write` and `Actions: Read and write`, then enter it in the admin page's "Set token".
5. Add or import sources in the admin page and wait for `Update Configs`. Subscription URLs follow your own repository name automatically.

---

## Admin page

`https://<username>.github.io/<repo>/`

| Area | Purpose |
| :-- | :-- |
| System overview | Update/probe summary, site & source counts, route count, Stars, latest commit |
| Quick import | Copy the primary TVBox / YSC single-repo URL and multi-repo TXT |
| Subscription URLs | Grouped by client, 5 accelerated/direct routes each |
| Sources | Search, filter, test, edit, delete, bulk import; deleted endpoints stay blocked |
| Self-host | Fork → Actions → Pages checklist |

Feed URLs are a one-shot import queue: the cloud fetches and merges them, then removes them from `sources/feeds.json`; deleting all sources clears the queue too. The page writes `sources/sources.json` and the cloud regenerates subscriptions. Without a token the page is read-only for browsing and testing.

---

## Command line

```bash
go build -o bin/tvboxgen ./cmd/tvboxgen

./bin/tvboxgen -force                 # generate immediately (ignore the 7-day interval)
./bin/tvboxgen -feed-only             # merge feeds only, no probing
./bin/tvboxgen -probe=<endpoint>      # test a single endpoint
./bin/tvboxgen -force -allow-nsfw     # keep adult sources
```

Common flags: `-workers` (parallel probes), `-timeout` (per-request timeout), `-budget` (total run cap), `-interval` (minimum update gap), `-history` (check points kept). Probing relies on real CMS list responses; only `ok` sources enter subscriptions.

---

## File map

| Path | Purpose |
| :-- | :-- |
| `sources/sources.json` | Master source list (includes the deletion blocklist) |
| `sources/feeds.json` | One-shot feed import queue, cleared after fetching |
| `api/tvbox.json` / `moontv.json` / `list.json` | Per-client subscription configs |
| `api/sources.txt` / `tvbox.txt` | Plain-text list / multi-repo index |
| `api/status.json` / `mirror.json` / `index.json` / `probe.json` | Status, route index and probe result (internal) |
| `index.html` + `assets/` | Static admin page (HTML / styles / logic / i18n dictionary) |
| `cmd/tvboxgen` + `internal/` | Go generator and internal packages |
| `.github/workflows/` | Cloud generation and single-source probe workflows |

---

## Disclaimer

This project is a **config text generation and management tool**. It only outputs JSON / TXT files for clients to read. **This project does not store, cache, proxy, transcode, distribute or provide any audio or video content, and offers no playback service.** Endpoints in this repository are entered by the user or fetched from third-party public lists configured by the user. The project does not own these endpoints and makes no review, endorsement or warranty regarding their content, copyright status, legality, availability, security or accuracy. Third-party endpoints may change, fail, return incorrect content or stop serving at any time.

Users are solely responsible for ensuring that the endpoints and content they enter, subscribe to, distribute and use comply with local laws, platform rules and applicable terms of service, and bear all resulting responsibilities and risks. Any loss, dispute, administrative penalty, third-party claim or other consequence arising from the use or inability to use this project, from reliance on the config text in this repository, or from third-party endpoint content or failures, is borne solely by the user. To the maximum extent permitted by law, the authors and contributors accept no liability, including but not limited to direct or indirect loss, data loss, device damage and loss of anticipated profits.

This project is provided **AS IS**, without any express or implied warranty, including but not limited to merchantability, fitness for a particular purpose, accuracy, continuous availability and non-infringement. If a rights holder believes an entry infringes their rights, the claim should be directed to the operator of the corresponding third-party endpoint; this project does not control or host that endpoint or its content. Do not use this project for any unlawful purpose. If such use is not permitted where you live, stop using it immediately.

---

## License

MIT
