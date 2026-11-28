/* TVbox-Configs admin page: view, add, import, test and delete video sources.
   Static page: reads the published json, writes through the GitHub contents api
   with a token the user stores locally in this browser. */
(function () {
  'use strict';

  // The page belongs to whoever serves it: <owner>.github.io/<repo>. Forking the
  // project needs no edit, the api calls and probe polling follow the fork.
  var FALLBACK = { owner: 'looomix', repo: 'TVbox-Configs' };

  function repoContext() {
    // custom domains cannot be derived from the url: set window.TVBOX_REPO before
    // the script tag instead of editing anything else
    var forced = window.TVBOX_REPO;
    if (forced && forced.owner && forced.repo) return { owner: forced.owner, repo: forced.repo };
    var host = (location.hostname || '').toLowerCase();
    var first = (location.pathname || '').replace(/^\/+/, '').split('/')[0];
    if (/\.github\.io$/.test(host) && first) {
      return { owner: host.replace(/\.github\.io$/, ''), repo: first };
    }
    return FALLBACK;
  }

  var CTX = repoContext();
  var OWNER = CTX.owner;
  var REPO = CTX.repo;
  var BRANCH = 'main';
  var API = 'https://api.github.com/repos/' + OWNER + '/' + REPO;
  var TOKEN_KEY = 'tvbox_configs_pat';
  var KEYWORD = '庆余年';
  var HIST_MAX = 8;

  var $ = function (sel) { return document.querySelector(sel); };
  var state = {
    token: localStorage.getItem(TOKEN_KEY) || '',
    status: null,
    groups: [],
    sources: [],
    removed: [],
    feeds: [],
    filters: { q: '', state: 'all', group: 'all' },
    results: {},
    pending: {},
    selected: {},
    editingId: null
  };

  /* ---------------- helpers ---------------- */

  function esc(v) {
    return String(v == null ? '' : v).replace(/[&<>"']/g, function (c) {
      return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
    });
  }

  var toastTimer = null;
  function toast(msg) {
    var el = $('#toast');
    el.textContent = msg;
    el.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(function () { el.classList.remove('show'); }, 3000);
  }

  function fmtTime(iso) {
    if (!iso) return '—';
    var d = new Date(iso);
    if (isNaN(d.getTime())) return iso;
    var p = function (n) { return n < 10 ? '0' + n : '' + n; };
    return p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }

  function relTime(iso) {
    if (!iso) return '—';
    var diff = Date.now() - new Date(iso).getTime();
    if (isNaN(diff)) return '—';
    if (diff < 0) {
      var h = Math.round(-diff / 3600000);
      return h >= 24 ? Math.round(h / 24) + ' 天后' : h + ' 小时后';
    }
    if (diff < 3600000) return Math.max(1, Math.round(diff / 60000)) + ' 分钟前';
    if (diff < 86400000) return Math.round(diff / 3600000) + ' 小时前';
    return Math.round(diff / 86400000) + ' 天前';
  }

  function authHeaders() {
    var h = { Accept: 'application/vnd.github+json' };
    if (state.token) h.Authorization = 'token ' + state.token;
    return h;
  }

  function getJSON(path) {
    var local = path + (path.indexOf('?') >= 0 ? '&' : '?') + 't=' + Date.now();
    function once(url) {
      // 冷启动时 CDN 首个请求可能超过 10 秒，超时就放弃这次尝试，避免整页卡在「正在加载」
      var ctl = window.AbortController ? new AbortController() : null;
      var timer = setTimeout(function () { if (ctl) ctl.abort(); }, 10000);
      return fetch(url, ctl ? { cache: 'no-store', signal: ctl.signal } : { cache: 'no-store' }).then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      }).then(function (d) {
        clearTimeout(timer);
        return d;
      }, function (e) {
        clearTimeout(timer);
        throw e;
      });
    }
    function grab(url) {
      return once(url).catch(function () { return once(url); });
    }
    var raw = 'https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/' + path;
    var proxy = 'https://ghproxy.net/' + raw;
    return grab(local).catch(function () { return grab(raw).catch(function () { return grab(proxy); }); });
  }

  function withParams(api) {
    var sep = api.indexOf('?') >= 0 ? '&' : '?';
    return api + sep + 'wd=' + encodeURIComponent(KEYWORD) + '&ac=detail';
  }

  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) {
      navigator.clipboard.writeText(text).then(function () { toast('已复制'); }, function () { toast('复制失败，请手动选择'); });
      return;
    }
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand('copy'); toast('已复制'); } catch (e) { toast('复制失败'); }
    document.body.removeChild(ta);
  }

  /* ---------------- loading ---------------- */

  function loadStatus() {
    return getJSON('api/status.json').then(function (d) { state.status = d; }).catch(function () { state.status = null; });
  }

  function loadSources() {
    return getJSON('sources/sources.json').then(function (d) {
      state.sources = (d && d.sources) || [];
      state.removed = (d && d.removed) || [];
    }).catch(function () { state.sources = []; state.removed = []; });
  }

  function defaultGroups() {
    var files = [
      { file: 'api/tvbox.json', label: 'TVBox / 影视仓 单仓配置', kind: 'config' },
      { file: 'api/moontv.json', label: 'MoonTV / LunaTV / KatelyaTV / DecoTV / MoonTVPlus 视频源', kind: 'config' },
      { file: 'api/list.json', label: 'OuonnkiTV / KVideo / LibreTV 通用源清单', kind: 'config' },
      { file: 'api/sources.txt', label: '纯文本源清单（可粘贴导入）', kind: 'text' },
      { file: 'api/tvbox.txt', label: 'TVBox / 影视仓 多仓 TXT 入口', kind: 'text' }
    ];
    return files.map(function (f) {
      var raw = 'https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/' + f.file;
      var ms = [
        { name: 'ghproxy 加速', kind: 'primary', url: 'https://ghproxy.net/' + raw },
        { name: 'gh-proxy 加速', kind: 'backup', url: 'https://gh-proxy.com/' + raw },
        { name: 'Raw 直连', kind: 'backup', url: raw },
        { name: 'GitHub Pages 备用', kind: 'backup', url: 'https://' + OWNER + '.github.io/' + REPO + '/' + f.file }
      ];
      return { file: f.file, label: f.label, kind: f.kind, primary: ms[0].url, mirrors: ms };
    });
  }

  function loadMirrors() {
    function clean(groups) {
      var valid = (groups || []).filter(function (g) {
        return g.primary && g.primary.indexOf('jsdelivr') < 0 && g.mirrors && g.mirrors.length;
      });
      return valid.length ? valid : defaultGroups();
    }
    return getJSON('api/mirror.json').then(function (d) {
      state.groups = clean(d && d.files);
    }).catch(function () { state.groups = defaultGroups(); });
  }

  /* ---------------- render ---------------- */

  function statusOf(id) {
    var list = (state.status && state.status.sources) || [];
    for (var i = 0; i < list.length; i++) {
      if (list[i].id === id) return list[i];
    }
    return null;
  }

  function stateMeta(st) {
    if (!st) return { cls: 'warn', label: '待检测', rank: 3 };
    if (!st.enabled) return { cls: 'off', label: '停用', rank: 6 };
    if (st.state === 'ok') return { cls: 'ok', label: '可用', rank: 0 };
    if (st.state === 'limited') return { cls: 'warn', label: '未验证', rank: 2 };
    return { cls: 'bad', label: '失效', rank: 4 };
  }

  function groupOf(s) {
    var st = statusOf(s.id);
    return (s.group || (st && st.group) || '综合');
  }

  function spark(history) {
    var items = (history || []).slice(-HIST_MAX);
    var pad = HIST_MAX - items.length;
    var out = '';
    for (var i = 0; i < pad; i++) out += '<span title="无记录"></span>';
    items.forEach(function (c, idx) {
      var now = idx === items.length - 1 ? ' now' : '';
      out += '<span class="' + (c.ok ? 'on' : '') + now + '" title="' + esc((c.t || '').slice(0, 10)) + ' · ' + (c.ok ? '通' : '断') + ' · ' + (c.ms || 0) + 'ms"></span>';
    });
    return '<span class="spark">' + out + '</span>';
  }

  function renderStats() {
    var st = state.status;
    var srcs = st ? st.sources : [];
    var alive = 0, unverified = 0, dead = 0, off = 0;
    srcs.forEach(function (s) {
      if (!s.enabled) { off++; return; }
      if (s.state === 'ok') alive++;
      else if (s.state === 'limited') unverified++;
      else dead++;
    });
    $('#pillAlive').textContent = '可用 ' + (st ? alive : '—');
    $('#pillUnverified').textContent = '未验证 ' + (st ? unverified : '—');
    $('#pillDead').textContent = '失效 ' + (st ? dead : '—');
    $('#pillTime').textContent = '更新 ' + (st ? relTime(st.updated_at) : '—');
    $('#pillTime').title = st ? ('上次 ' + fmtTime(st.updated_at) + ' · 下次 ' + fmtTime(st.next_update_at) + ' · 停用 ' + off + ' 条') : '';
    var pill = $('#pillToken');
    pill.textContent = state.token ? '🔑 可编辑' : '🔑 只读模式';
    pill.classList.toggle('ok', !!state.token);
  }

  function renderMirrors() {
    $('#mirrorGroups').innerHTML = state.groups.map(function (g) {
      var others = (g.mirrors || []).filter(function (m) { return m.url !== g.primary; });
      var rows = others.map(function (m) {
        return '<div class="mirror">' +
          '<span class="tag ' + (m.kind === 'primary' ? 'primary' : '') + '">' + esc(m.name) + '</span>' +
          '<span class="url" title="' + esc(m.url) + '">' + esc(m.url) + '</span>' +
          '<button class="btn tiny copy-url" data-url="' + esc(m.url) + '">复制</button>' +
          '</div>';
      }).join('');
      return '<div class="mgroup">' +
        '<div class="mgroup-head">' +
        '<span class="mgroup-title">' + esc(g.label) + '</span>' +
        '<span class="badge">' + (g.kind === 'text' ? '文本' : '配置') + '</span>' +
        '<button class="btn tiny primary copy-url" data-url="' + esc(g.primary) + '">复制</button>' +
        '</div>' +
        '<div class="primary-url" title="' + esc(g.primary) + '">' + esc(g.primary) + '</div>' +
        (others.length ? '<details><summary>其他 ' + others.length + ' 条镜像</summary>' + rows + '</details>' : '') +
        '</div>';
    }).join('');
    $('#indexUrl').textContent = 'https://' + OWNER + '.github.io/' + REPO + '/api/index.json';
    var primary = $('#primaryUrl');
    if (primary) primary.textContent = state.groups[0] ? state.groups[0].primary : primary.textContent;
  }

  function renderGroups() {
    var groups = {};
    state.sources.forEach(function (s) { groups[groupOf(s)] = true; });
    var opts = ['all'].concat(Object.keys(groups).sort());
    $('#filterGroup').innerHTML = opts.map(function (g) {
      return '<option value="' + esc(g) + '">' + (g === 'all' ? '全部分类' : esc(g)) + '</option>';
    }).join('');
    $('#filterGroup').value = state.filters.group;
  }

  function visibleSources() {
    var q = state.filters.q.toLowerCase();
    return state.sources.filter(function (s) {
      var st = statusOf(s.id);
      var hay = (s.name + ' ' + s.api).toLowerCase();
      if (q && hay.indexOf(q) < 0) return false;
      if (state.filters.group !== 'all' && groupOf(s) !== state.filters.group) return false;
      var alive = st && st.enabled && st.state === 'ok';
      var unverified = st && st.enabled && st.state === 'limited';
      if (state.filters.state === 'alive' && !alive) return false;
      if (state.filters.state === 'unverified' && !unverified) return false;
      if (state.filters.state === 'down' && !(st && st.enabled && st.state === 'down')) return false;
      if (state.filters.state === 'off' && !(st && !st.enabled)) return false;
      return true;
    }).sort(function (a, b) {
      // 可达的源按延迟从低到高；连不上的沉到最后，未测过的排在它们前面
      var sa = statusOf(a.id);
      var sb = statusOf(b.id);
      var ba = latencyBucket(sa);
      var bb = latencyBucket(sb);
      if (ba !== bb) return ba - bb;
      var la = latencyOf(sa);
      var lb = latencyOf(sb);
      if (la !== lb) return la - lb;
      return a.name.localeCompare(b.name, 'zh-Hans-CN');
    });
  }

  function latencyOf(st) {
    if (st && typeof st.latency_ms === 'number' && st.latency_ms > 0) return st.latency_ms;
    return Infinity;
  }

  function latencyBucket(st) {
    if (!st || !st.enabled) return 2;
    if (st.state === 'ok' || st.state === 'limited') return 0;
    if (st.state === 'down') return 2;
    return 1;
  }

  function renderRows() {
    var list = visibleSources();
    var st = state.status;
    $('#listCount').textContent = '共 ' + state.sources.length + ' 个 · 显示 ' + list.length;
    if (!state.sources.length) {
      $('#listHint').textContent = '还没有源：点「＋ 新增源」逐条添加，或在上方整行粘贴多个接口地址批量导入。';
    } else if (st) {
      $('#listHint').textContent = '自动抓取 ' + (st.from_feed || 0) + ' 条 · 去重 ' + (st.deduped || 0) + ' 条 · 列表按延迟从低到高；只有验证通过的源才会写进订阅；「改」需要令牌。';
    } else {
      $('#listHint').textContent = '点击接口地址可复制；「改」需要令牌。';
    }
    $('#rows').innerHTML = list.map(function (s) {
      var st2 = statusOf(s.id);
      var meta = stateMeta(st2);
      var lat = st2 && st2.enabled ? (st2.latency_ms > 0 ? st2.latency_ms + ' ms' : '—') : '—';
      var res = state.results[s.id];
      var pending = state.pending[s.id];
      var stateLabel = pending ? '检测中' : (res ? (res.ok ? '可用 ' + res.vods + ' 条' : '测试失败') : meta.label);
      var stateCls = pending ? 'warn' : (res ? (res.ok ? 'ok' : 'bad') : meta.cls);
      var auto = s.origin === 'feed';
      var checked = state.selected[s.id] ? ' checked' : '';
      return '<div class="row grid-cols" data-id="' + esc(s.id) + '">' +
        '<div class="c-check"><input type="checkbox" data-sel="' + esc(s.id) + '"' + checked + ' aria-label="选择 ' + esc(s.name) + '"></div>' +
        '<div class="name-cell"><div class="n"><span class="txt" title="' + esc(s.name) + '">' + esc(s.name) + '</span>' + (auto ? '<span class="badge on">自动</span>' : '') + '</div></div>' +
        '<div class="api-cell" data-act="copy" data-url="' + esc(s.api) + '" title="点击复制：' + esc(s.api) + '">' + esc(s.api) + '</div>' +
        '<div><span class="badge">' + esc(groupOf(s)) + '</span></div>' +
        '<div class="c-state"><span class="dot ' + stateCls + '"><i></i>' + esc(stateLabel) +
          '</span></div>' +
        '<div class="c-lat">' + lat + '</div>' +
        '<div class="c-spark">' + spark(st2 && st2.history) + '</div>' +
        '<div class="ops">' +
        '<button class="btn tiny" data-act="test">测试</button>' +
        '<button class="btn tiny" data-act="edit">改</button>' +
        '<button class="btn tiny" data-act="del">删</button>' +
        '</div></div>';
    }).join('');
    syncSelection();
  }

  /* ---------------- batch delete ---------------- */

  function selectedIds() {
    var live = {};
    state.sources.forEach(function (s) { live[s.id] = true; });
    return Object.keys(state.selected).filter(function (id) {
      if (live[id]) return true;
      delete state.selected[id];
      return false;
    });
  }

  function syncSelection() {
    var n = selectedIds().length;
    var box = $('#btnBatchDel');
    if (!box) return;
    $('#selCount').textContent = n ? String(n) : '0';
    box.disabled = n === 0;
    var all = $('#checkAll');
    if (all) {
      var rows = document.querySelectorAll('#rows [data-sel]');
      var checked = document.querySelectorAll('#rows [data-sel]:checked');
      all.checked = rows.length > 0 && rows.length === checked.length;
      all.indeterminate = checked.length > 0 && checked.length < rows.length;
    }
  }

  function batchDelete() {
    var ids = selectedIds();
    if (!ids.length) return;
    if (!state.token) { openToken(); return; }
    var names = ids.map(function (id) {
      var hit = state.sources.filter(function (s) { return s.id === id; })[0];
      return hit ? hit.name : id;
    });
    if (!confirm('确定删除选中的 ' + ids.length + ' 个源？\n' + names.slice(0, 6).join('、') + (ids.length > 6 ? ' 等' : '') + '\n删除后会同步更新订阅配置。')) return;
    var drop = {};
    ids.forEach(function (id) { drop[id] = true; });
    $('#btnBatchDel').disabled = true;
    ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      doc.removed = doc.removed || [];
      state.sources.forEach(function (s) {
        if (s.id && drop[s.id]) {
          var key = apiKey(s.api);
          if (doc.removed.indexOf(key) < 0) doc.removed.push(key);
        }
      });
      doc.sources = (doc.sources || []).filter(function (s) { return !drop[s.id]; });
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/sources.json', text, res.sha, 'chore: 批量删除 ' + ids.length + ' 个视频源').then(function () {
        state.sources = doc.sources;
        state.removed = doc.removed || [];
        ids.forEach(function (id) { delete state.results[id]; delete state.pending[id]; });
        state.selected = {};
        return dispatchWorkflow();
      }).then(function (triggered) {
        renderAll();
        toast('已删除 ' + ids.length + ' 个源' + (triggered ? '，后台正在重新生成（约 2 分钟）' : '，等待下一次自动更新'));
        scheduleRefresh();
      });
    }).catch(function (err) {
      renderAll();
      toast(err.message || '批量删除失败');
    });
  }

  function renderRemoved() {
    var n = (state.removed || []).length;
    $('#removedInfo').textContent = '删除记录 ' + n + ' 条' + (n ? '（屏蔽源库重新抓回）' : '');
    $('#btnRestore').disabled = !n;
    $('#btnClearRemoved').disabled = !n;
  }

  function renderAll() {
    renderStats();
    renderMirrors();
    renderGroups();
    renderRows();
    renderRemoved();
  }

  /* ---------------- probing: cloud workflow first ---------------- */

  var PROBE_FILE = 'https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/api/probe.json';

  function deadline(ms) {
    if (typeof AbortSignal !== 'undefined' && AbortSignal.timeout) return AbortSignal.timeout(ms);
    return undefined;
  }

  function quickProbe(api) {
    var url = withParams(api);
    var t0 = performance.now();
    return fetch(url, { cache: 'no-store', signal: deadline(4000) }).then(function (r) {
      if (r.status !== 200) throw new Error('HTTP ' + r.status);
      return r.text().then(function (txt) {
        var ms = Math.round(performance.now() - t0);
        try {
          var j = JSON.parse(txt);
          return { ok: true, ms: ms, vods: (j && j.list && j.list.length) || 0, via: '直连', state: 'ok' };
        } catch (e) {
          return { ok: false, ms: ms, vods: 0, via: '直连', note: '返回内容不是标准 CMS JSON' };
        }
      });
    }).catch(function () {
      return { ok: false, ms: Math.round(performance.now() - t0), vods: 0, via: '直连', note: '跨域或网络受限' };
    });
  }

  function dispatchProbe(api) {
    return fetch(API + '/actions/workflows/probe.yml/dispatches', {
      method: 'POST',
      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders()),
      body: JSON.stringify({ ref: BRANCH, inputs: { url: api } })
    }).then(function (r) {
      if (r.status !== 204) throw new Error('云端检测无法触发（HTTP ' + r.status + '），请确认令牌含 Actions 权限');
    });
  }

  function cloudProbe(api, onProgress) {
    var started = Date.now();
    return dispatchProbe(api).then(function () {
      return new Promise(function (resolve, reject) {
        var tries = 0;
        function poll() {
          tries++;
          if (tries > 22) { reject(new Error('云端检测超时，可稍后在列表里看结果')); return; }
          setTimeout(function () {
            fetch(PROBE_FILE + '?t=' + Date.now(), { cache: 'no-store' })
              .then(function (r) { return r.ok ? r.json() : null; })
              .then(function (d) {
                if (d && d.url === api && new Date(d.checked_at).getTime() >= started - 2000) { resolve(d); return; }
                if (onProgress) onProgress('云端检测中…');
                poll();
              })
              .catch(function () { poll(); });
          }, tries === 1 ? 12000 : 6500);
        }
        poll();
      });
    });
  }

  function runProbe(api, onProgress) {
    if (state.token) {
      if (onProgress) onProgress('云端检测中…（约 1 分钟）');
      return cloudProbe(api, onProgress).then(function (d) {
        return { ok: d.state === 'ok', ms: d.latency_ms, vods: d.vods, via: '云端', state: d.state };
      }).catch(function (err) {
        if (onProgress) onProgress('云端不可用，改用本机浏览器…');
        return quickProbe(api).then(function (quick) {
          quick.note = (quick.note || '') + '；' + (err.message || err);
          return quick;
        });
      });
    }
    if (onProgress) onProgress('检测中…（未设置令牌，只能用本机浏览器）');
    return quickProbe(api).then(function (quick) {
      if (!quick.ok) quick.note = '未设置令牌，无法云端检测（多数采集接口不带 CORS 头）';
      return quick;
    });
  }

  function testRow(id, api) {
    state.pending[id] = true;
    renderRows();
    return runProbe(api).then(function (res) {
      delete state.pending[id];
      state.results[id] = res;
      renderRows();
      toast(res.ok
        ? ('可用，返回 ' + res.vods + ' 条 · ' + res.ms + ' ms · ' + res.via)
        : ('未通过验证：' + (res.note || res.via) + ' · ' + res.ms + ' ms'));
      return res;
    }, function (err) {
      delete state.pending[id];
      renderRows();
      toast('检测失败：' + (err.message || err));
    });
  }

  /* ---------------- github writes ---------------- */

  function decodeContent(b64) {
    var bin = atob(b64.replace(/\s/g, ''));
    var bytes = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return new TextDecoder('utf-8').decode(bytes);
  }

  function encodeContent(text) {
    var bytes = new TextEncoder().encode(text);
    var bin = '';
    for (var i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin);
  }

  function ghGetFile(path) {
    return fetch(API + '/contents/' + path + '?ref=' + BRANCH, { headers: authHeaders(), cache: 'no-store' })
      .then(function (r) {
        if (r.status === 401) throw new Error('令牌无效，请重新设置');
        if (r.status === 404) return { sha: '', doc: null };
        if (!r.ok) throw new Error('读取仓库失败（HTTP ' + r.status + '）');
        return r.json();
      })
      .then(function (d) {
        if (!d || !d.content) return { sha: (d && d.sha) || '', doc: null };
        var text = decodeContent(d.content);
        return { sha: d.sha, doc: JSON.parse(text) };
      });
  }

  function ghPutFile(path, text, sha, message) {
    var body = { message: message, content: encodeContent(text), branch: BRANCH };
    if (sha) body.sha = sha;
    return fetch(API + '/contents/' + path, {
      method: 'PUT',
      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders()),
      body: JSON.stringify(body)
    }).then(function (r) {
      if (r.status === 409) throw new Error('仓库刚被更新，请刷新页面后重试');
      if (!r.ok) return r.json().then(function (d) { throw new Error((d && d.message) || ('写入失败 HTTP ' + r.status)); });
      return r.json();
    });
  }

  function dispatchWorkflow() {
    if (!state.token) return Promise.resolve(false);
    var inputs = { force: 'true' };
    return fetch(API + '/actions/workflows/update.yml/dispatches', {
      method: 'POST',
      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders()),
      body: JSON.stringify({ ref: BRANCH, inputs: inputs })
    }).then(function (r) { return r.status === 204; }).catch(function () { return false; });
  }

  // clearRemoved drops the deleted-endpoint blocklist. restore also kicks off a
  // run right away, so the feeds bring their entries back in one step.
  function clearRemoved(restore) {
    if (!state.token) { openToken(); return; }
    var n = (state.removed || []).length;
    if (!n) return;
    var msg = restore
      ? '恢复被删源？会清空 ' + n + ' 条删除记录并立即重新生成，源库里的条目会回来（手动删掉的、源库也没有的例外）。'
      : '清空 ' + n + ' 条删除记录？只解除屏蔽，不立即生成，下次自动更新时才生效。';
    if (!confirm(msg)) return;
    $('#btnRestore').disabled = true;
    $('#btnClearRemoved').disabled = true;
    ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      doc.removed = [];
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/sources.json', text, res.sha, restore ? 'feat: 恢复被删源' : 'chore: 清空删除记录')
        .then(function () {
          state.removed = [];
          if (!restore) return false;
          return dispatchWorkflow();
        });
    }).then(function (triggered) {
      renderRemoved();
      toast(restore
        ? (triggered ? '已清空删除记录，正在重新抓取（约 1 分钟）' : '已清空删除记录，等下次自动更新')
        : '已清空删除记录，下次自动更新时生效');
      if (triggered) scheduleRefresh();
    }).catch(function (err) {
      renderRemoved();
      toast(err.message || '操作失败');
    });
  }

  function saveSource(payload) {
    if (!state.token) {
      openToken();
      return Promise.reject(new Error('请先设置访问令牌'));
    }
    return ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      var list = doc.sources || (doc.sources = []);
      if (doc.removed && doc.removed.length) {
        var key = apiKey(payload.api);
        doc.removed = doc.removed.filter(function (k) { return k !== key; });
        state.removed = doc.removed;
      }
      var idx = -1;
      for (var i = 0; i < list.length; i++) if (list[i].id === payload.id) { idx = i; break; }
      if (idx >= 0) list[idx] = Object.assign({}, list[idx], payload);
      else list.push(payload);
      if (!doc.version) doc.version = 1;
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var text = JSON.stringify(doc, null, 2) + '\n';
      var msg = (idx >= 0 ? 'chore: 更新视频源 ' : 'feat: 新增视频源 ') + payload.name;
      return ghPutFile('sources/sources.json', text, res.sha, msg).then(function () {
        state.sources = list;
        return dispatchWorkflow();
      }).then(function (triggered) {
        renderAll();
        toast(triggered ? '已提交，后台正在重新生成（约 2 分钟）' : '已提交，等待下一次自动更新');
        scheduleRefresh();
      });
    });
  }

  function deleteSource(id) {
    if (!state.token) { openToken(); return Promise.reject(new Error('请先设置访问令牌')); }
    return ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      var name = '';
      doc.sources = (doc.sources || []).filter(function (s) {
        if (s.id === id) { name = s.name; return false; }
        return true;
      });
      var hit = state.sources.filter(function (s) { return s.id === id; })[0];
      if (hit) {
        doc.removed = doc.removed || [];
        if (doc.removed.indexOf(apiKey(hit.api)) < 0) doc.removed.push(apiKey(hit.api));
      }
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/sources.json', text, res.sha, 'chore: 删除视频源 ' + (name || id)).then(function () {
        state.sources = doc.sources;
        state.removed = doc.removed || [];
        delete state.results[id];
        return dispatchWorkflow();
      }).then(function () {
        renderAll();
        toast('已删除并提交');
        scheduleRefresh();
      });
    });
  }

  function addInterfaces(lines) {
    if (!state.token) { openToken(); return; }
    var known = {};
    state.sources.forEach(function (s) { known[apiKey(s.api)] = true; });
    var fresh = [], dup = 0;
    lines.forEach(function (api) {
      var key = apiKey(api);
      if (known[key]) { dup++; return; }
      known[key] = true;
      fresh.push({
        id: stableId(api),
        name: hostKey(api),
        api: api,
        group: '综合',
        note: '',
        enabled: true,
        origin: 'manual',
        added_at: new Date().toISOString().slice(0, 10)
      });
    });
    if (!fresh.length) { toast(dup ? ('这些接口都已存在（' + dup + ' 个重复）') : '没有可用的接口地址'); return; }
    ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      var list = doc.sources || (doc.sources = []);
      if (doc.removed && doc.removed.length) {
        var keys = {};
        fresh.forEach(function (f) { keys[apiKey(f.api)] = true; });
        doc.removed = doc.removed.filter(function (k) { return !keys[k]; });
        state.removed = doc.removed;
      }
      var exist = {};
      list.forEach(function (s) { exist[s.id] = true; });
      fresh.forEach(function (f) {
        var id = f.id;
        while (exist[id]) id = id + '-' + Math.random().toString(36).slice(2, 4);
        exist[id] = true;
        f.id = id;
        list.push(f);
      });
      if (!doc.version) doc.version = 1;
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/sources.json', text, res.sha, 'feat: 新增 ' + fresh.length + ' 个视频接口').then(function () {
        state.sources = list;
        return dispatchWorkflow();
      }).then(function (triggered) {
        renderAll();
        toast('已加入 ' + fresh.length + ' 个接口' + (dup ? '（跳过 ' + dup + ' 个重复）' : '') +
          (triggered ? '，后台开始验证（约 2 分钟）' : '，等待下一次自动更新'));
        scheduleRefresh();
      });
    }).catch(function (err) { toast(err.message || '写入失败'); });
  }

  /* same key the generator uses, so deletions survive the next feed merge */
  // must stay in sync with gen.NormalizeAPI on the server
  // 与后端 PathKey 同口径：参数键连同它的值一起跳过，所以
  // /api.php/provide/vod、/provide/vod/from/x、/provide/vod/from/x/at/json 都算同一条
  var API_PATH_NOISE = { 'api.php': 1, 'index.php': 1, at: 1, from: 1, xml: 1, json: 1, ac: 1, wd: 1, pg: 1, p: 1, t: 1, h: 1, ids: 1, page: 1 };
  var API_PATH_PARAMS = { at: 1, from: 1, ac: 1, wd: 1, pg: 1, p: 1, t: 1, h: 1, ids: 1, page: 1 };

  function pathKey(rawPath) {
    var segs = String(rawPath || '').toLowerCase().replace(/^\/+|\/+$/g, '').split('/');
    var kept = [];
    for (var i = 0; i < segs.length; i++) {
      var seg = segs[i];
      if (!seg || API_PATH_NOISE[seg]) {
        if (API_PATH_PARAMS[seg]) i++;
        continue;
      }
      kept.push(seg);
      if (kept.length === 3) break;
    }
    return kept.join('/');
  }

  function apiKey(raw) {
    try {
      var u = new URL(raw);
      var host = u.hostname.toLowerCase().replace(/^www\./, '');
      var target = u.searchParams.get('url');
      if (target) {
        try {
          var t = new URL(target);
          var th = t.hostname.toLowerCase().replace(/^www\./, '');
          var pk = pathKey(t.pathname);
          return host + '|' + th + (pk ? '/' + pk : '');
        } catch (e) { /* keep host only */ }
      }
      var pk2 = pathKey(u.pathname);
      return pk2 ? host + '/' + pk2 : host;
    } catch (e) {
      return String(raw).toLowerCase();
    }
  }

  function hostKey(url) {
    var h = '';
    try { h = new URL(url).hostname.toLowerCase().replace(/^www\./, ''); } catch (e) { h = url.toLowerCase(); }
    return h;
  }

  function hostOf(url) {
    try { return new URL(url).hostname.replace(/^www\./, ''); } catch (e) { return url; }
  }

  // setFeeds enables all given list urls in one write and drops junk entries
  // (a feed entry must be one plain http(s) link, never a glued blob).
  function setFeeds(urls) {
    if (!state.token) { openToken(); return; }
    ghGetFile('sources/feeds.json').then(function (res) {
      var doc = res.doc || { version: 1, feeds: [] };
      var feeds = doc.feeds || (doc.feeds = []);
      var clean = feeds.filter(function (f) { return validFeedUrl(f.url); });
      urls.forEach(function (u) {
        var hit = null;
        for (var i = 0; i < clean.length; i++) if (clean[i].url === u) { hit = clean[i]; break; }
        if (hit) { hit.enabled = true; return; }
        clean.push({ name: hostOf(u), url: u, enabled: true });
      });
      doc.feeds = clean;
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/feeds.json', text, res.sha, 'feat: 导入源库 ' + urls.length + ' 个')
        .then(function () { return dispatchWorkflow(); })
        .then(function (triggered) {
          toast(triggered ? ('已导入 ' + urls.length + ' 个源库，云端正在抓取（约 1 分钟）') : '已保存，等待下一次自动更新');
          scheduleRefresh();
        });
    }).catch(function (err) {
      toast(err.message || '操作失败');
    });
  }

  function validFeedUrl(u) {
    return /^https?:\/\/[^\s,，、]+$/i.test(String(u || '').trim());
  }

  // splitUrls pulls every link out of a pasted block, even when links are glued
  // together without a separator.
  function splitUrls(raw) {
    var out = [];
    raw.split(/[\n,，、;；]+/).forEach(function (chunk) {
      chunk.split(/(?=https?:\/\/)/i).forEach(function (part) {
        part = part.trim();
        if (/^https?:\/\/\S+$/i.test(part)) out.push(part);
      });
    });
    return out;
  }


  var refreshTimer = null;
  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(function () {
      Promise.all([loadStatus(), loadSources(), loadMirrors()]).then(renderAll);
    }, 100000);
  }

  /* ---------------- modals ---------------- */

  function openModal(sel) { $(sel).classList.add('open'); }
  function closeModal(sel) { $(sel).classList.remove('open'); }

  function openToken() {
    $('#fToken').value = state.token;
    $('#tokenHint').textContent = state.token ? '当前已保存令牌，可编辑源清单和源库。' : '当前为只读模式：可浏览、可测试，不能新增或修改。';
    openModal('#modalToken');
  }

  function openEdit(source) {
    state.editingId = source ? source.id : null;
    $('#editTitle').textContent = source ? '修改视频源' : '新增视频源';
    $('#fName').value = source ? source.name : '';
    $('#fApi').value = source ? source.api : '';
    $('#fGroup').value = source ? groupOf(source) : '综合';
    $('#fEnabled').value = source && source.enabled === false ? '0' : '1';
    $('#fNote').value = source ? (source.note || '') : '';
    $('#editHint').textContent = '保存会直接提交到仓库的 sources/sources.json，并在后台触发一次检测；令牌只留在本机浏览器。';
    $('#editHint').className = 'hint';
    openModal('#modalEdit');
  }

  function slug(name, api) {
    var host = '';
    try { host = new URL(api).hostname.replace(/^www\./, ''); } catch (e) { host = ''; }
    var base = (host || name || 'source').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
    return base || 'source';
  }

  // Stable id derived from the normalized endpoint: two sources on the same
  // host but with different paths must not collide on an id.
  function stableId(api) {
    var key = apiKey(api);
    var h = 0;
    for (var i = 0; i < key.length; i++) h = ((h << 5) - h + key.charCodeAt(i)) | 0;
    var suffix = (h >>> 0).toString(36);
    return (slug('', api) + '-' + suffix).slice(0, 64);
  }

  function submitEdit() {
    var name = $('#fName').value.trim();
    var api = $('#fApi').value.trim();
    var hint = $('#editHint');
    if (!name) { hint.textContent = '请填写名称'; hint.className = 'hint err'; return; }
    if (!/^https?:\/\//i.test(api)) { hint.textContent = '接口地址需以 http(s):// 开头'; hint.className = 'hint err'; return; }
    var id = state.editingId || slug(name, api);
    var exists = state.sources.some(function (s) { return s.id === id && s.id !== state.editingId; });
    if (exists) id = id + '-' + Math.random().toString(36).slice(2, 5);
    var payload = {
      id: id,
      name: name,
      api: api,
      group: $('#fGroup').value,
      note: $('#fNote').value.trim(),
      enabled: $('#fEnabled').value === '1',
      origin: 'manual',
      added_at: new Date().toISOString().slice(0, 10)
    };
    $('#btnSave').disabled = true;
    saveSource(payload).then(function () {
      closeModal('#modalEdit');
    }).catch(function (err) {
      hint.textContent = err.message || '保存失败';
      hint.className = 'hint err';
    }).then(function () { $('#btnSave').disabled = false; });
  }

  /* ---------------- events ---------------- */

  function bind() {
    $('#q').addEventListener('input', function (e) { state.filters.q = e.target.value.trim(); renderRows(); });
    $('#filterState').addEventListener('change', function (e) { state.filters.state = e.target.value; renderRows(); });
    $('#filterGroup').addEventListener('change', function (e) { state.filters.group = e.target.value; renderRows(); });

    $('#btnAdd').addEventListener('click', function () { openEdit(null); });

    $('#checkAll').addEventListener('change', function (e) {
      var on = e.target.checked;
      document.querySelectorAll('#rows [data-sel]').forEach(function (box) {
        var id = box.getAttribute('data-sel');
        box.checked = on;
        if (on) state.selected[id] = true; else delete state.selected[id];
      });
      syncSelection();
    });

    $('#rows').addEventListener('change', function (e) {
      var box = e.target.closest('[data-sel]');
      if (!box) return;
      var id = box.getAttribute('data-sel');
      if (box.checked) state.selected[id] = true; else delete state.selected[id];
      syncSelection();
    });

    $('#btnBatchDel').addEventListener('click', batchDelete);
    var repoLink = $('#repoLink');
    if (repoLink) repoLink.href = 'https://github.com/' + OWNER + '/' + REPO;

    $('#btnRestore').addEventListener('click', function () { clearRemoved(true); });
    $('#btnClearRemoved').addEventListener('click', function () { clearRemoved(false); });

    $('#btnToken').addEventListener('click', openToken);
    $('#pillToken').addEventListener('click', openToken);

    $('#btnCopyPrimary').addEventListener('click', function () {
      var g = state.groups[0];
      copyText(g ? g.primary : ('https://ghproxy.net/https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/api/tvbox.json'));
    });

    $('#mirrorGroups').addEventListener('click', function (e) {
      var btn = e.target.closest('.copy-url');
      if (btn) copyText(btn.getAttribute('data-url'));
    });

    document.querySelector('[data-copy="index"]').addEventListener('click', function () {
      copyText($('#indexUrl').textContent);
    });

    $('#btnCopyMulti').addEventListener('click', function () {
      copyText('https://ghproxy.net/https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/api/tvbox.txt');
    });

    $('#btnFeedAdd').addEventListener('click', function () {
      var raw = $('#feedCustom').value.trim();
      if (!raw) { toast('请填入接口地址或源库链接'); return; }
      var urls = splitUrls(raw);
      if (!urls.length) { toast('没有识别到 http(s) 链接'); return; }
      var ifaces = [], feeds = [];
      urls.forEach(function (u) {
        if (/api\.php|provide\/vod|ac=detail|inc\/api|\/vod/i.test(u)) ifaces.push(u);
        else feeds.push(u);
      });
      $('#feedCustom').value = '';
      if (ifaces.length) addInterfaces(ifaces);
      if (feeds.length) setFeeds(feeds);
    });

    $('#rows').addEventListener('click', function (e) {
      var cell = e.target.closest('[data-act]');
      if (!cell) return;
      var row = e.target.closest('.row');
      var id = row && row.getAttribute('data-id');
      var act = cell.getAttribute('data-act');
      if (act === 'copy') { copyText(cell.getAttribute('data-url')); return; }
      var src = state.sources.filter(function (s) { return s.id === id; })[0];
      if (!src) return;
      if (act === 'test') {
        if (state.pending[id]) return;
        testRow(id, src.api);
      } else if (act === 'edit') {
        openEdit(src);
      } else if (act === 'del') {
        if (!state.token) { openToken(); return; }
        if (confirm('确定删除「' + src.name + '」？删除后会同步更新订阅配置。')) {
          deleteSource(id).catch(function (err) { toast(err.message || '删除失败'); });
        }
      }
    });

    $('#btnDispatch').addEventListener('click', function () {
      if (!state.token) { openToken(); return; }
      dispatchWorkflow().then(function (ok) {
        toast(ok ? '已触发后台检测，约 2 分钟后刷新' : '触发失败，请检查令牌的 Actions 权限');
        if (ok) scheduleRefresh();
      });
    });

    $('#btnTestForm').addEventListener('click', function () {
      var api = $('#fApi').value.trim();
      var hint = $('#editHint');
      if (!/^https?:\/\//i.test(api)) { hint.textContent = '接口地址需以 http(s):// 开头'; hint.className = 'hint err'; return; }
      hint.className = 'hint';
      runProbe(api, function (msg) { hint.textContent = msg; }).then(function (res) {
        hint.textContent = res.ok ? ('可用：返回 ' + res.vods + ' 条 · ' + res.ms + ' ms · ' + res.via)
          : ('未通过验证：' + (res.note || res.via) + ' · ' + res.ms + ' ms');
        hint.className = 'hint ' + (res.ok ? '' : 'err');
      });
    });

    $('#btnSave').addEventListener('click', submitEdit);

    $('#btnTokenSave').addEventListener('click', function () {
      var t = $('#fToken').value.trim();
      if (!t) { $('#tokenHint').textContent = '请填写令牌'; return; }
      state.token = t;
      localStorage.setItem(TOKEN_KEY, t);
      fetch(API + '/contents/sources/sources.json?ref=' + BRANCH, { headers: authHeaders(), cache: 'no-store' })
        .then(function (r) {
          if (r.ok) { toast('令牌已保存，可编辑'); closeModal('#modalToken'); renderStats(); }
          else { $('#tokenHint').textContent = '令牌权限不足（HTTP ' + r.status + '），需要 Contents: Read and write'; }
        })
        .catch(function () { $('#tokenHint').textContent = '校验失败，请检查网络或令牌'; });
    });

    $('#btnTokenClear').addEventListener('click', function () {
      state.token = '';
      localStorage.removeItem(TOKEN_KEY);
      $('#fToken').value = '';
      $('#tokenHint').textContent = '已清除，切换为只读模式。';
      renderStats();
    });

    document.querySelectorAll('[data-close]').forEach(function (btn) {
      btn.addEventListener('click', function () {
        closeModal('#modalEdit');
        closeModal('#modalToken');
      });
    });
    document.querySelectorAll('.modal').forEach(function (m) {
      m.addEventListener('click', function (e) { if (e.target === m) m.classList.remove('open'); });
    });
  }

  /* ---------------- boot ---------------- */

  Promise.all([loadStatus(), loadSources(), loadMirrors()]).then(function () {
    if (!state.sources.length && state.status) {
      state.sources = state.status.sources.map(function (s) {
        return { id: s.id, name: s.name, api: s.api, group: s.group, enabled: s.enabled, origin: s.origin };
      });
    }
    if (!state.status) {
      $('#pillTime').textContent = '尚无数据';
      if (!$('#rows').children.length) {
        $('#listHint').textContent = '数据没加载出来：先刷新一次页面；还是空的，去仓库 Actions 看「Update Configs」是否成功跑过。';
      }
    }
    bind();
    renderAll();
  });
})();
