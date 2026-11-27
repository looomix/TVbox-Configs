/* TVbox-Configs admin page: view, add, edit, test and delete video sources.
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
    mirrors: [],
    indexUrl: '',
    sources: [],
    filters: { q: '', state: 'all', group: 'all' },
    results: {},
    editingId: null,
    busy: false
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
    toastTimer = setTimeout(function () { el.classList.remove('show'); }, 2600);
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
    return getJSON('api/status.json').then(function (d) {
      state.status = d;
    }).catch(function () { state.status = null; });
  }

  function loadSources() {
    return getJSON('sources/sources.json').then(function (d) {
      state.sources = (d && d.sources) || [];
    }).catch(function () { state.sources = []; });
  }

  function defaultMirrors() {
    var raw = 'https://raw.githubusercontent.com/' + OWNER + '/' + REPO + '/' + BRANCH + '/api/tvbox.json';
    return [
      { name: 'jsDelivr 主线路', kind: 'primary', url: 'https://cdn.jsdelivr.net/gh/' + OWNER + '/' + REPO + '@' + BRANCH + '/api/tvbox.json' },
      { name: 'GitHub Pages 备用', kind: 'backup', url: 'https://' + OWNER + '.github.io/' + REPO + '/api/tvbox.json' },
      { name: 'Raw 直连', kind: 'backup', url: raw },
      { name: 'Statically CDN', kind: 'backup', url: 'https://cdn.statically.io/gh/' + OWNER + '/' + REPO + '/' + BRANCH + '/api/tvbox.json' },
      { name: 'ghproxy 加速', kind: 'proxy', url: 'https://ghproxy.net/' + raw },
      { name: 'gh-proxy 加速', kind: 'proxy', url: 'https://gh-proxy.com/' + raw }
    ];
  }

  function loadMirrors() {
    return getJSON('api/mirror.json').then(function (d) {
      state.mirrors = (d && d.mirrors) || defaultMirrors();
    }).catch(function () { state.mirrors = defaultMirrors(); });
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
    if (st.state === 'limited') return { cls: 'warn', label: '可访问', rank: 2 };
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
    var alive = 0, dead = 0, off = 0;
    srcs.forEach(function (s) {
      if (!s.enabled) off++;
      else if (s.state === 'ok' || s.state === 'limited') alive++;
      else dead++;
    });
    $('#pillAlive').textContent = '可用 ' + (st ? alive : '—');
    $('#pillDead').textContent = '失效 ' + (st ? dead : '—');
    $('#pillTime').textContent = '更新 ' + (st ? relTime(st.updated_at) : '—');
    $('#pillTime').title = st ? '上次 ' + fmtTime(st.updated_at) + ' · 下次 ' + fmtTime(st.next_update_at) : '';
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
    $('#mirrors').innerHTML = state.mirrors.map(function (m) {
      return '<div class="mirror">' +
        '<span class="tag ' + (m.kind === 'proxy' ? 'proxy' : '') + '">' + esc(m.name) + '</span>' +
        '<span class="url" title="' + esc(m.url) + '">' + esc(m.url) + '</span>' +
        '<button class="btn tiny copy-url" data-url="' + esc(m.url) + '">复制</button>' +
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
      var meta = stateMeta(st);
      var hay = (s.name + ' ' + s.api + ' ' + (s.note || '')).toLowerCase();
      if (q && hay.indexOf(q) < 0) return false;
      if (state.filters.group !== 'all' && groupOf(s) !== state.filters.group) return false;
      if (state.filters.state === 'alive' && !(st && st.enabled && (st.state === 'ok' || st.state === 'limited'))) return false;
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
    } else {
      $('#listHint').textContent = '点击接口地址可复制；「改」需要令牌；修改后会自动提交并触发一次重新检测。';
    }
    $('#rows').innerHTML = list.map(function (s) {
      var st2 = statusOf(s.id);
      var meta = stateMeta(st2);
      var lat = st2 && st2.enabled ? (st2.latency_ms > 0 ? st2.latency_ms + ' ms' : '—') : '—';
      var res = state.results[s.id];
      var stateLabel = res ? (res.ok ? '测试 ' + res.vods + ' 条' : '测试失败') : meta.label;
      var stateCls = res ? (res.ok ? 'ok' : 'bad') : meta.cls;
      return '<div class="row grid-cols" data-id="' + esc(s.id) + '">' +
        '<div class="name-cell"><div class="n">' + esc(s.name) + '</div>' +
        '<div class="note" title="' + esc(s.note || '') + '">' + (esc(s.note || '') || '—') + '</div></div>' +
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

  /* ---------------- probing ---------------- */

  function probe(api) {
    var url = withParams(api);
    var t0 = performance.now();
    var attempts = [
      function (u) { return u; },
      function (u) { return 'https://api.allorigins.win/raw?url=' + encodeURIComponent(u); },
      function (u) { return 'https://api.codetabs.com/v1/proxy?quest=' + encodeURIComponent(u); }
    ];
    var names = ['直连', '代理 A', '代理 B'];

    function attempt(i) {
      if (i >= attempts.length) {
        return Promise.resolve({ ok: false, ms: Math.round(performance.now() - t0), vods: 0, via: '全部通道失败' });
      }
      var opts = { cache: 'no-store' };
      if (typeof AbortSignal !== 'undefined' && AbortSignal.timeout) opts.signal = AbortSignal.timeout(15000);
      return fetch(attempts[i](url), opts).then(function (r) {
        return r.text().then(function (txt) {
          var ms = Math.round(performance.now() - t0);
          if (r.status !== 200) return attempt(i + 1);
          try {
            var j = JSON.parse(txt);
            var vods = (j && j.list && j.list.length) || 0;
            return { ok: vods > 0, ms: ms, vods: vods, via: names[i], reachable: true };
          } catch (e) {
            return { ok: false, ms: ms, vods: 0, via: names[i], reachable: true, note: '非标准 JSON' };
          }
        });
      }).catch(function () { return attempt(i + 1); });
    }
    return attempt(0);
  }

  function testRow(id, api) {
    return probe(api).then(function (res) {
      state.results[id] = res;
      renderRows();
      toast(res.ok ? ('可用，返回 ' + res.vods + ' 条 · ' + res.ms + ' ms（' + res.via + '）')
        : ('不可用：' + (res.note || res.via) + ' · ' + res.ms + ' ms'));
      return res;
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

  function ghGetSourceFile() {
    return fetch(API + '/contents/sources/sources.json?ref=' + BRANCH, { headers: authHeaders(), cache: 'no-store' })
      .then(function (r) {
        if (r.status === 401) throw new Error('令牌无效，请重新设置');
        if (!r.ok) throw new Error('读取仓库失败（HTTP ' + r.status + '）');
        return r.json();
      })
      .then(function (d) {
        return { sha: d.sha, text: decodeContent(d.content), doc: JSON.parse(decodeContent(d.content)) };
      });
  }

  function ghPutSourceFile(text, sha, message) {
    return fetch(API + '/contents/sources/sources.json', {
      method: 'PUT',
      headers: Object.assign({ 'Content-Type': 'application/json' }, authHeaders()),
      body: JSON.stringify({ message: message, content: encodeContent(text), sha: sha, branch: BRANCH })
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
    return ghGetSourceFile().then(function (res) {
      var doc = res.doc || {};
      var list = doc.sources || (doc.sources = []);
      var idx = -1;
      for (var i = 0; i < list.length; i++) if (list[i].id === payload.id) { idx = i; break; }
      if (idx >= 0) list[idx] = Object.assign({}, list[idx], payload);
      else list.push(payload);
      if (!doc.version) doc.version = 1;
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var body = JSON.stringify(doc, null, 2) + '\n';
      var msg = (idx >= 0 ? 'chore: 更新视频源 ' : 'feat: 新增视频源 ') + payload.name;
      return ghPutSourceFile(body, res.sha, msg).then(function () {
        state.sources = list;
        return dispatchWorkflow();
      }).then(function (triggered) {
        renderAll();
        toast(triggered ? '已提交，后台正在重新生成（约 1 分钟）' : '已提交，等待下一次自动更新');
        scheduleRefresh();
      });
    });
  }

  function deleteSource(id) {
    if (!state.token) { openToken(); return Promise.reject(new Error('请先设置访问令牌')); }
    return ghGetSourceFile().then(function (res) {
      var doc = res.doc || {};
      var name = '';
      doc.sources = (doc.sources || []).filter(function (s) {
        if (s.id === id) { name = s.name; return false; }
        return true;
      });
      doc.updated_at = new Date().toISOString().slice(0, 10);
      var body = JSON.stringify(doc, null, 2) + '\n';
      return ghPutSourceFile(body, res.sha, 'chore: 删除视频源 ' + (name || id)).then(function () {
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

  var refreshTimer = null;
  function scheduleRefresh() {
    clearTimeout(refreshTimer);
    refreshTimer = setTimeout(function () {
      Promise.all([loadStatus(), loadSources(), loadMirrors()]).then(renderAll);
    }, 75000);
  }

  /* ---------------- modals ---------------- */

  function openModal(sel) { $(sel).classList.add('open'); }
  function closeModal(sel) { $(sel).classList.remove('open'); }

  function openToken() {
    $('#fToken').value = state.token;
    $('#tokenHint').textContent = state.token ? '当前已保存令牌，可编辑源清单。' : '当前为只读模式：可浏览、可测试，不能新增或修改。';
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
    $('#editHint').textContent = '保存会直接提交到仓库的 sources/sources.json，并在后台触发一次检测。';
    $('#editHint').className = 'hint';
    openModal('#modalEdit');
  }

  function slug(name, api) {
    var host = '';
    try { host = new URL(api).hostname; } catch (e) { host = ''; }
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
      added_at: (new Date().toISOString().slice(0, 10))
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
      var primary = state.mirrors.filter(function (m) { return m.kind === 'primary'; })[0] || state.mirrors[0];
      if (primary) copyText(primary.url);
    });

    $('#mirrors').addEventListener('click', function (e) {
      var btn = e.target.closest('.copy-url');
      if (btn) copyText(btn.getAttribute('data-url'));
    });

    $('#indexUrl').parentNode.querySelector('[data-copy]').addEventListener('click', function () {
      copyText($('#indexUrl').textContent);
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
        if (state.busy) return;
        state.busy = true;
        testRow(id, src.api).then(function () { state.busy = false; });
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
        toast(ok ? '已触发后台检测，约 1 分钟后刷新' : '触发失败，请检查令牌的 Actions 权限');
        if (ok) scheduleRefresh();
      });
    });

    $('#btnTestForm').addEventListener('click', function () {
      var api = $('#fApi').value.trim();
      var hint = $('#editHint');
      if (!/^https?:\/\//i.test(api)) { hint.textContent = '接口地址需以 http(s):// 开头'; hint.className = 'hint err'; return; }
      hint.textContent = '测试中…';
      hint.className = 'hint';
      probe(api).then(function (res) {
        hint.textContent = res.ok ? ('可用：返回 ' + res.vods + ' 条 · ' + res.ms + ' ms（' + res.via + '）')
          : ('不可用：' + (res.note || res.via) + ' · ' + res.ms + ' ms');
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
        return { id: s.id, name: s.name, api: s.api, group: s.group, note: s.note || '', enabled: s.enabled };
      });
    }
    if (!state.status) $('#pillTime').textContent = '尚无数据';
    if (state.status && state.mirrors.length === 0) state.mirrors = defaultMirrors();
    bind();
    renderAll();
  });
})();
