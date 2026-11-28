# TVbox-Configs

[中文](README.md) | English | [Deploy](#deploy) · [Features](#features) · [Import URL](#import-url) · [Clients](#other-clients) · [Disclaimer](#disclaimer)

Aggregates video API endpoints from public sources into subscription configs that TVBox, YSC, MoonTV and other clients can import directly. A Go generator probes each endpoint, deduplicates and filters entries, and only verified sources reach the subscription.

Auto-refresh every 168 hours (7 days). Live numbers are shown on the [admin page](https://looomix.github.io/TVbox-Configs/) and in [`api/status.json`](api/status.json). Latest probe: 75/119 sources alive (63%), 34 dead endpoints removed, 23 adult sources filtered.

> Badge numbers (sites / sources / links) are a snapshot from the latest generation. See the admin page and `api/status.json` for live values.

## Features

- Go generator + static admin page: probes, deduplicates and filters sources, keeping the last 8 check results.
- Sources come from public feeds and manual entries, verified against real CMS list responses.
- Adult content sources are filtered by default; use `-allow-nsfw` locally to keep them.
- Maintain once, subscribe everywhere: TVBox / YSC single repo, MoonTV family, OuonnkiTV / KVideo / LibreTV, plain-text list.
- Admin page supports adding, bulk importing, testing, editing and deleting sources. Writes go to `sources/sources.json`, then the cloud Action regenerates the subscriptions.
- Feed URLs are a one-shot import queue: the cloud fetches them, merges the entries into `sources/sources.json` and removes them from `sources/feeds.json`; failed feeds are retried until they succeed. Deleting all sources also clears the queue, so the file never accumulates.
- Admin page includes a language switcher, status badges, probe summary, and subscription groups with 5 routes each.

## Import URL

All sources are Apple CMS `provide/vod` endpoints marked `type 1` (no jar required). TVBox and YSC share the same protocol, so use the same URL:

```
https://ghproxy.net/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
```

Backup routes, in order:

```
https://gh-proxy.org/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://ghfast.top/https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://raw.githubusercontent.com/looomix/TVbox-Configs/main/api/tvbox.json
https://looomix.github.io/TVbox-Configs/api/tvbox.json
```

YSC notes: ① use a version that supports jar-free endpoints; ② after import, select the repository in "Repository Management"; ③ delete the old config and clear cache before re-importing (YSC caches for about 3 days); ④ if a proxy URL is rejected, use the `raw.githubusercontent.com` or `looomix.github.io` direct link; ⑤ as a last resort, open the direct link in a browser, copy the whole JSON text and paste it into YSC's "local import / paste config". Clients with multi-repo support can use `api/tvbox.txt`.

### Other clients

| File | Clients | Usage |
| :-- | :-- | :-- |
| `api/moontv.json` | MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus | Paste the JSON content into "Video Source Config"; most of these projects do not support URL subscriptions |
| `api/list.json` | OuonnkiTV / KVideo / LibreTV | Source list subscription URL (JSON array) |
| `api/sources.txt` | Clients accepting a plain `name\|API` list | Plain text |
| `api/tvbox.txt` | TVBox / YSC multi-repo TXT index | Multi-repo import |

`api/status.json`, `api/mirror.json`, `api/index.json` and `api/probe.json` are internal data — **do not** paste them into a client's config URL. Check for leading/trailing spaces, and delete the old config before re-importing to clear client cache.

## Deploy

1. Fork this repository.
2. Open **Actions** and enable the workflows.
3. **Settings → Pages**: Source = `Deploy from a branch`, Branch = `main`, `/ (root)`, then save.
4. Make the repository **Public**. Create a fine-grained token with `Contents: Read and write` and `Actions: Read and write` for this repository, then enter it in the admin page's "Set token" dialog.
5. Add or import sources in the admin page and wait for `Update Configs` to finish. Subscription URLs follow your own repository name automatically.

## Admin page

`https://<username>.github.io/<repo>/`

The top section shows status badges, update and probe summaries, plus a language switcher. Subscription URLs are grouped by client with backup routes. You can add, bulk-import, test, edit and delete sources; deleted endpoints are blocked from being re-added. Feed URLs are one-shot: the cloud fetches and merges them, then removes them from `sources/feeds.json`; deleting all sources clears the queue too, so links never accumulate. Changes are written to `sources/sources.json` and regenerated in the cloud.

## Command line

```bash
go build -o bin/tvboxgen ./cmd/tvboxgen
./bin/tvboxgen -force                 # generate immediately
./bin/tvboxgen -feed-only             # merge feeds only, no probing
./bin/tvboxgen -probe=<endpoint>      # test a single endpoint
./bin/tvboxgen -force -allow-nsfw     # keep adult sources
```

Common flags: `-workers`, `-timeout`, `-budget`, `-interval`, `-history`. Probing relies on real CMS list responses; only `ok` sources enter the subscriptions.

## Disclaimer

This project is a **config text generation and management tool**. It only outputs JSON / TXT files for clients to read. **This project does not store, cache, proxy, transcode, distribute or provide any audio or video content, and offers no playback service.** Endpoints in this repository are entered by the user or fetched from third-party public feeds configured by the user. The project does not own these endpoints and makes no review, endorsement or warranty regarding their content, copyright status, legality, availability, security or accuracy. Third-party endpoints may change, fail, return incorrect content or stop serving at any time.

Users are solely responsible for ensuring that the endpoints and content they enter, subscribe to, distribute and use comply with local laws, platform rules and applicable terms of service, and bear all resulting responsibilities and risks. Any loss, dispute, administrative penalty, third-party claim or other consequence arising from the use or inability to use this project, from reliance on the config text in this repository, or from third-party endpoint content or failures, is borne solely by the user. To the maximum extent permitted by law, the authors and contributors accept no liability, including but not limited to direct or indirect loss, data loss, device damage and loss of anticipated profits.

This project is provided **AS IS**, without any express or implied warranty, including but not limited to merchantability, fitness for a particular purpose, accuracy, continuous availability and non-infringement. If a rights holder believes an entry infringes their rights, the claim should be directed to the operator of the corresponding third-party endpoint; this project does not control or host that endpoint or its content. Do not use this project for any unlawful purpose. If such use is not permitted where you live, stop using it immediately.

## License

MIT
