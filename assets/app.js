/* TVbox-Configs admin page: view, add, import, test and delete video sources.
   Static page: reads the published json, writes through the GitHub contents api
   with a token the user stores locally in this browser. */
(function () {
  'use strict';

  var OWNER = 'looomix';
  var REPO = 'TVbox-Configs';
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
    feeds: [],
    filters: { q: '', state: 'all', group: 'all' },
    results: {},
    pending: {},
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
    var remote = 'https://cdn.jsdelivr.net/gh/' + OWNER + '/' + REPO + '@' + BRANCH + '/' + path;
    function grab(url) {
      return fetch(url, { cache: 'no-store' }).then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      });
    }
    return grab(local).catch(function () { return grab(remote); });
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
    return getJSON('sources/sources.json').then(function (d) { state.sources = (d && d.sources) || []; }).catch(function () { state.sources = []; });
  }

  function defaultGroups() {
    var files = [
      { file: 'api/tvbox.json', label: 'TVBox / 影视仓 单仓配置', kind: 'config' },
      { file: 'api/moontv.json', label: 'MoonTV / LunaTV 视频源', kind: 'config' },
      { file: 'api/sources.txt', label: '纯文本源清单（可粘贴导入）', kind: 'text' }
    ];
    return files.map(function (f) {
      var raw = 'https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/' + f.file;
      var del = 'https://cdn.jsdelivr.net/gh/' + OWNER + '/' + REPO + '@' + BRANCH + '/' + f.file;
      return {
        file: f.file,
        label: f.label,
        kind: f.kind,
        primary: del,
        mirrors: [
          { name: 'jsDelivr 主线路', kind: 'primary', url: del },
          { name: 'GitHub Pages 备用', kind: 'backup', url: 'https://' + OWNER + '.github.io/' + REPO + '/' + f.file },
          { name: 'Raw 直连', kind: 'backup', url: raw },
          { name: 'ghproxy 加速', kind: 'proxy', url: 'https://ghproxy.net/' + raw },
          { name: 'gh-proxy 加速', kind: 'proxy', url: 'https://gh-proxy.com/' + raw }
        ]
      };
    });
  }

  function loadMirrors() {
    return getJSON('api/mirror.json').then(function (d) {
      state.groups = (d && d.files && d.files.length) ? d.files : defaultGroups();
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
    if (state.token) {
      pill.textContent = '🔑 可编辑';
      pill.style.color = '#17a673';
    } else {
      pill.textContent = '🔑 只读模式';
      pill.style.color = '';
    }
  }

  function renderMirrors() {
    $('#mirrorGroups').innerHTML = state.groups.map(function (g) {
      var others = (g.mirrors || []).filter(function (m) { return m.url !== g.primary; });
      var rows = (g.mirrors || []).map(function (m) {
        return '<div class="mirror">' +
          '<span class="tag ' + (m.kind === 'proxy' ? 'proxy' : '') + '">' + esc(m.name) + '</span>' +
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
      var ra = stateMeta(statusOf(a.id)).rank;
      var rb = stateMeta(statusOf(b.id)).rank;
      if (ra !== rb) return ra - rb;
      return a.name.localeCompare(b.name, 'zh-Hans-CN');
    });
  }

  function renderRows() {
    var list = visibleSources();
    var st = state.status;
    $('#listCount').textContent = '共 ' + state.sources.length + ' 个 · 显示 ' + list.length;
    if (!state.sources.length) {
      $('#listHint').textContent = st ? '未能读取源清单，仅展示检测结果。' : '还没有数据，等待首次生成或点击「新增源」。';
    } else if (st) {
      $('#listHint').textContent = '自动抓取 ' + (st.from_feed || 0) + ' 条 · 去重 ' + (st.deduped || 0) + ' 条 · 只有验证通过的源才会写进订阅；「改」需要令牌。';
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
      return '<div class="row grid-cols" data-id="' + esc(s.id) + '">' +
        '<div class="name-cell"><div class="n"><span class="txt" title="' + esc(s.name) + '">' + esc(s.name) + '</span>' + (auto ? '<span class="badge on">自动</span>' : '') + '</div></div>' +
        '<div class="api-cell" data-act="copy" data-url="' + esc(s.api) + '" title="点击复制：' + esc(s.api) + '">' + esc(s.api) + '</div>' +
        '<div><span class="badge">' + esc(groupOf(s)) + '</span></div>' +
        '<div class="c-state"><span class="dot ' + stateCls + '"><i></i>' + esc(stateLabel) + '</span></div>' +
        '<div class="c-lat">' + lat + '</div>' +
        '<div class="c-spark">' + spark(st2 && st2.history) + '</div>' +
        '<div class="ops">' +
        '<button class="btn tiny" data-act="test">测试</button>' +
        '<button class="btn tiny" data-act="edit">改</button>' +
        '<button class="btn tiny" data-act="del">删</button>' +
        '</div></div>';
    }).join('');
  }

  function renderAll() {
    renderStats();
    renderMirrors();
    renderGroups();
    renderRows();
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
                if (d && d.url === api && new Date(d.checked_at).getTime() >= started - 20000) { resolve(d); return; }
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
    return fetch(API + '/actions/workflows/update.yml/dispatches', {
      method: 'POST',
      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders()),
      body: JSON.stringify({ ref: BRANCH, inputs: { force: 'true' } })
    }).then(function (r) { return r.status === 204; }).catch(function () { return false; });
  }

  function saveSource(payload) {
    if (!state.token) {
      openToken();
      return Promise.reject(new Error('请先设置访问令牌'));
    }
    return ghGetFile('sources/sources.json').then(function (res) {
      var doc = res.doc || {};
      var list = doc.sources || (doc.sources = []);
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
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/sources.json', text, res.sha, 'chore: 删除视频源 ' + (name || id)).then(function () {
        state.sources = doc.sources;
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
    state.sources.forEach(function (s) { known[hostKey(s.api)] = true; });
    var fresh = [], dup = 0;
    lines.forEach(function (api) {
      var key = hostKey(api);
      if (known[key]) { dup++; return; }
      known[key] = true;
      fresh.push({
        id: slug(key, api),
        name: key,
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

  function hostKey(url) {
    var h = '';
    try { h = new URL(url).hostname.toLowerCase().replace(/^www\./, ''); } catch (e) { h = url.toLowerCase(); }
    return h;
  }

  function hostOf(url) {
    try { return new URL(url).hostname.replace(/^www\./, ''); } catch (e) { return url; }
  }

  function toggleFeed(url, on, name) {
    if (!state.token) { openToken(); return; }
    ghGetFile('sources/feeds.json').then(function (res) {
      var doc = res.doc || { version: 1, feeds: [] };
      var feeds = doc.feeds || (doc.feeds = []);
      var idx = -1;
      for (var i = 0; i < feeds.length; i++) if (feeds[i].url === url) { idx = i; break; }
      if (idx >= 0) feeds[idx].enabled = on;
      else feeds.push({ name: name || hostOf(url), url: url, enabled: on });
      var text = JSON.stringify(doc, null, 2) + '\n';
      return ghPutFile('sources/feeds.json', text, res.sha, (on ? 'feat: 导入源库 ' : 'chore: 停用源库 ') + (name || hostOf(url)))
        .then(function () {
          return dispatchWorkflow();
        }).then(function (triggered) {
          toast(triggered ? ('已' + (on ? '导入' : '停用') + '，云端正在抓取并验证（约 2 分钟）') : '已保存，等待下一次自动更新');
          scheduleRefresh();
        });
    }).catch(function (err) {
      toast(err.message || '操作失败');
    });
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
    $('#btnToken').addEventListener('click', openToken);
    $('#pillToken').addEventListener('click', openToken);

    $('#btnCopyPrimary').addEventListener('click', function () {
      var g = state.groups[0];
      if (g) copyText(g.primary);
    });

    $('#mirrorGroups').addEventListener('click', function (e) {
      var btn = e.target.closest('.copy-url');
      if (btn) copyText(btn.getAttribute('data-url'));
    });

    document.querySelector('[data-copy="index"]').addEventListener('click', function () {
      copyText($('#indexUrl').textContent);
    });

    $('#btnFeedAdd').addEventListener('click', function () {
      var raw = $('#feedCustom').value.trim();
      if (!raw) { toast('请填入接口地址或源库链接'); return; }
      var parts = raw.split(/[\n,，、]+/).map(function (x) { return x.trim(); }).filter(Boolean);
      var looksInterface = parts.length > 1 || parts.some(function (u) {
        return /api\.php|provide\/vod|ac=detail|inc\/api/i.test(u);
      });
      if (looksInterface) {
        var bad = parts.filter(function (u) { return !/^https?:\/\//i.test(u); });
        if (bad.length) { toast('有 ' + bad.length + ' 条不是 http(s) 地址，已忽略'); }
        addInterfaces(parts.filter(function (u) { return /^https?:\/\//i.test(u); }));
      } else {
        if (!/^https?:\/\//i.test(parts[0])) { toast('请填写 http(s) 开头的接口或链接'); return; }
        toggleFeed(parts[0], true, hostOf(parts[0]));
      }
      $('#feedCustom').value = '';
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
    if (!state.status) $('#pillTime').textContent = '尚无数据';
    bind();
    renderAll();
  });
})();
