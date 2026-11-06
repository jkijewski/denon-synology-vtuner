'use strict';
// All URLs are relative, so the page works both behind the DSM CGI and on the server's own /admin/ path.
const $ = (s, el = document) => el.querySelector(s);
let cat = null;        // working copy of the catalog
let dirty = false;
let selected = null;   // selected folder id
let editing = null;    // station being edited (null = new)
let device = 'default'; // selected receiver
let devices = [];
let catVersion = null; // version of the catalog in cat, so a save can't overwrite a newer one
const dq = () => 'device=' + encodeURIComponent(device);

function status(msg) { $('#status').textContent = msg || ''; }
function setDirty(v) { dirty = v; $('#save').disabled = !v; $('#revert').disabled = !v; status(v ? 'Unsaved changes' : ''); }

async function api(path, opts = {}) {
  const r = await fetch(path, opts);
  const v = r.headers.get('x-catalog-version');
  if (v) catVersion = v; // only catalog answers carry one
  const ct = r.headers.get('content-type') || '';
  const body = ct.includes('json') ? await r.json() : await r.text();
  if (!r.ok) {
    const e = new Error(body && body.error ? body.error : (r.status + ' ' + r.statusText));
    e.status = r.status;
    throw e;
  }
  return body;
}

function walk(folders, fn, parent = null) {
  for (const f of folders || []) { fn(f, parent); walk(f.folders, fn, f); }
}
function findFolder(id) { let hit = null, par = null; walk(cat.folders, (f, p) => { if (f.id === id) { hit = f; par = p; } }); return [hit, par]; }
function station(id) { return cat.stations.find(s => s.id === id); }
let tmpId = -1; // new entries get negative ids until save() gives them real ones
function newId() { return tmpId--; }

async function load() {
  try { cat = await api('api/catalog?' + dq()); setDirty(false); render(); }
  catch (e) { status('Could not load: ' + e.message); }
}

function render() {
  renderDirResults();
  if (selected !== null && !findFolder(selected)[0]) selected = null;
  if (selected === null && cat.folders.length) selected = cat.folders[0].id;
  renderTree(); renderFolder(); renderOrphans();
}

function renderTree() {
  const build = (folders) => folders.map(f => {
    const li = document.createElement('li');
    const row = document.createElement('div');
    row.className = f.id === selected ? 'sel' : '';
    row.innerHTML = '<button type="button" class="name"></button><span class="count"></span>';
    row.firstChild.textContent = f.name;
    if (f.id === selected) row.firstChild.setAttribute('aria-current', 'true');
    if (f.filter) {
      const badge = document.createElement('span');
      badge.className = 'badge virtual'; badge.textContent = '\u{1F50D} virtual'; badge.title = 'Virtual folder: lists public directory stations that match its filters';
      row.querySelector('.count').append(badge);
      row.classList.add('virtual');
    } else row.querySelector('.count').textContent = (f.stations || []).length + (f.folders || []).length;
    row.onclick = () => { selected = f.id; render(); };
    const up = button('▲', 'Move up', () => move(siblings(f), f, -1)); up.className = 'icon';
    const dn = button('▼', 'Move down', () => move(siblings(f), f, 1)); dn.className = 'icon';
    row.append(up, dn);
    li.append(row);
    if (f.folders && f.folders.length) { const ul = document.createElement('ul'); ul.append(...build(f.folders)); li.append(ul); }
    return li;
  });
  $('#tree').replaceChildren(...build(cat.folders));
}

function siblings(f) { const [, p] = findFolder(f.id); return p ? p.folders : cat.folders; }
function move(arr, item, d) {
  const i = arr.indexOf(item), j = i + d;
  if (j < 0 || j >= arr.length) return;
  [arr[i], arr[j]] = [arr[j], arr[i]];
  setDirty(true); render();
}

function button(text, title, fn, cls) {
  const b = document.createElement('button');
  b.type = 'button'; b.textContent = text; b.title = title; b.setAttribute('aria-label', title); if (cls) b.className = cls;
  b.onclick = (e) => { e.stopPropagation(); fn(); };
  return b;
}

function renderFolder() {
  const [f] = findFolder(selected);
  $('#folder-title').textContent = f ? f.name : 'No folder yet';
  $('#rename-folder').disabled = $('#delete-folder').disabled = !f;
  const virtual = !!(f && f.filter);
  $('#add-subfolder').disabled = $('#add-station').disabled = !f || virtual;
  $('#edit-filter').hidden = $('#virtual-view').hidden = !virtual;
  $('#stations-view').hidden = virtual;
  renderVirtual(f);
  const tb = $('#stations tbody');
  tb.replaceChildren();
  $('#empty-hint').textContent = f ? ((f.stations || []).length ? '' : 'No stations in this folder.') : 'Add a folder first.';
  if (!f) return;
  f.stations = f.stations || [];
  for (const id of f.stations) {
    const s = station(id); if (!s) continue;
    const tr = document.createElement('tr');
    tr.innerHTML = '<td></td><td></td><td></td><td class="actions"></td>';
    tr.children[0].textContent = s.name;
    tr.children[1].textContent = (s.format || 'MP3') + (s.bitrate ? ' ' + s.bitrate + 'k' : '');
    tr.children[2].textContent = s.relay ? 'on' : '';
    const sel = document.createElement('select');
    sel.title = 'Copy to folder';
    sel.innerHTML = '<option value="">Copy to…</option>';
    walk(cat.folders, (g) => { if (g.id !== f.id && !g.filter) { const o = document.createElement('option'); o.value = g.id; o.textContent = g.name; sel.append(o); } });
    sel.onchange = () => { const [g] = findFolder(Number(sel.value)); if (g) { g.stations = g.stations || []; if (!g.stations.includes(id)) g.stations.push(id); setDirty(true); render(); } };
    tr.lastChild.append(
      button('▲', 'Move up', () => move(f.stations, id, -1), 'icon'),
      button('▼', 'Move down', () => move(f.stations, id, 1), 'icon'),
      button('Edit', 'Edit station', () => openStation(s)),
      sel,
      button('Remove', 'Remove from this folder', () => { f.stations = f.stations.filter(x => x !== id); setDirty(true); render(); }, 'danger'));
    tb.append(tr);
  }
}

function renderOrphans() {
  const used = new Set(); walk(cat.folders, f => (f.stations || []).forEach(id => used.add(id)));
  const items = cat.stations.filter(s => !used.has(s.id)).map(s => {
    const li = document.createElement('li');
    const span = document.createElement('span'); span.textContent = s.name;
    li.append(span,
      button('Add to this folder', 'Add to the selected folder', () => { const [f] = findFolder(selected); if (f && !f.filter) { f.stations = f.stations || []; f.stations.push(s.id); setDirty(true); render(); } }),
      button('Delete', 'Delete the station', () => { if (confirm('Delete "' + s.name + '"?')) { cat.stations = cat.stations.filter(x => x !== s); setDirty(true); render(); } }, 'danger'));
    return li;
  });
  $('#orphans').replaceChildren(...items);
  if (!items.length) $('#orphans').innerHTML = '<li class="hint">None</li>';
}

// ---- virtual folders
const orderNames = { votes: 'Most votes', clicktrend: 'Trending', bitrate: 'Highest bitrate', name: 'Name, A to Z' };
// The stations a filter lists on the receiver, whatever the state of the page.
const previewStations = (filter) => api('api/directory/preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(filter) });
function stationRow(s) {
  const li = document.createElement('li');
  const name = document.createElement('span'); name.textContent = s.name;
  const meta = document.createElement('span'); meta.className = 'meta';
  meta.textContent = [s.country, (s.codec || '') + (s.bitrate ? ' ' + s.bitrate + 'k' : '')].filter(Boolean).join(' · ');
  li.append(name, meta);
  return li;
}
function countryName(code) {
  const o = [...$('#dir-country').options].find(x => x.value === code);
  return o ? o.textContent.replace(/ \(\d+\)$/, '') : code;
}
function filterText(f) {
  const parts = [];
  if (f.country) parts.push('Country: ' + countryName(f.country));
  if (f.genres && f.genres.length) parts.push((f.genres.length > 1 ? 'Genres: ' : 'Genre: ') + f.genres.join(' or '));
  if (f.format) parts.push('Format: ' + f.format);
  if (f.language) parts.push('Language: ' + f.language);
  if (f.minBitrate) parts.push('At least ' + f.minBitrate + ' kbit/s');
  if (f.name) parts.push('Name contains: ' + f.name);
  if (f.order) parts.push('Sorted by: ' + (orderNames[f.order] || f.order));
  if (f.limit) parts.push('At most ' + f.limit + ' stations');
  return parts.join(' · ');
}
let previewKey = null; // the filter the preview shows, so re-drawing the page doesn't ask again
function renderVirtual(f) {
  if (!f || !f.filter) { previewKey = null; return; }
  $('#filter-summary').textContent = filterText(f.filter);
  const key = JSON.stringify(f.filter);
  if (key === previewKey) return;
  previewKey = key;
  $('#virtual-preview').replaceChildren();
  $('#virtual-preview-status').textContent = 'Loading matching stations…';
  previewStations(f.filter).then(list => {
    if (previewKey !== key) return;
    $('#virtual-preview-status').textContent = previewSummary(list);
    $('#virtual-preview').replaceChildren(...list.slice(0, 20).map(stationRow));
  }).catch(e => { if (previewKey === key) $('#virtual-preview-status').textContent = e.message; });
}

function previewSummary(list) {
  if (!list.length) return 'No stations match these filters now.';
  return list.length > 20 ? list.length + ' matching stations now. The first 20:' : list.length + ' matching stations now:';
}

const maxGenres = 10; // same as maxFilterGenres in the catalog
let filterGenres = []; // genres picked in the virtual folder form, in the order picked
function genreChoices() {
  const known = [...$('#dir-tag').options].filter(o => o.value).map(o => [o.value, o.textContent]);
  // A saved genre that isn't in the list stays selectable, so that opening the form doesn't lose it.
  const extra = filterGenres.filter(g => !known.some(([v]) => v === g)).map(g => [g, g]);
  return [...extra, ...known];
}
function renderPickedGenres() {
  $('#filter-genres-picked').replaceChildren(...filterGenres.map(g => {
    const b = document.createElement('button');
    b.type = 'button'; b.className = 'badge in'; b.textContent = g + ' ×';
    b.setAttribute('aria-label', 'Remove genre ' + g);
    b.onclick = () => { filterGenres = filterGenres.filter(x => x !== g); renderPickedGenres(); renderGenreList(); };
    return b;
  }));
}
function renderGenreList() {
  const q = $('#filter-genre-search').value.trim().toLowerCase();
  $('#filter-genre-list').replaceChildren(...genreChoices().filter(([v]) => v.toLowerCase().includes(q)).map(([v, text]) => {
    const label = document.createElement('label'); label.className = 'check';
    const box = document.createElement('input'); box.type = 'checkbox'; box.checked = filterGenres.includes(v);
    box.onchange = () => {
      filterGenres = box.checked ? [...filterGenres, v] : filterGenres.filter(x => x !== v);
      renderPickedGenres();
    };
    label.append(box, text);
    return label;
  }));
}
$('#filter-genre-search').addEventListener('input', renderGenreList);
// Enter in this box must not submit the form.
$('#filter-genre-search').addEventListener('keydown', (e) => { if (e.key === 'Enter') e.preventDefault(); });

let filterTarget = null; // folder being edited (null = new)
let filterOnSave = null; // runs after a new virtual folder is added
async function openFilterDialog(folder, preset, onSave) {
  await loadDirLists();
  filterTarget = folder; filterOnSave = onSave || null;
  const el = $('#filter-form').elements;
  const copy = (from, to) => to.replaceChildren(...[...from.options].map(o => new Option(o.textContent, o.value)));
  copy($('#dir-country'), el.country);
  el.language.replaceChildren(new Option('Any', ''), ...dirLanguages.map(l => new Option(l.name + ' (' + l.stationcount + ')', l.name)));
  const v = (folder ? folder.filter : preset) || {};
  for (const [sel, val] of [[el.country, v.country], [el.language, v.language]]) {
    // Keep a saved value that isn't in the list, so that opening the dialog doesn't lose it.
    if (val && ![...sel.options].some(o => o.value === val)) sel.append(new Option(val, val));
    sel.value = val || '';
  }
  filterGenres = [...(v.genres || [])];
  $('#filter-genre-search').value = '';
  renderPickedGenres(); renderGenreList();
  el.format.value = v.format || ''; el.name.value = v.name || '';
  el.minBitrate.value = v.minBitrate || ''; el.order.value = v.order || ''; el.limit.value = v.limit || '';
  el.folder.value = folder ? folder.name : (v.folder || '');
  // A new virtual folder can go at the top level or into a plain folder.
  $('#filter-parent-label').hidden = !!folder;
  el.parent.replaceChildren(new Option('Top level', ''));
  const addParents = (folders, depth) => (folders || []).forEach(g => {
    if (g.filter) return;
    el.parent.append(new Option('\u00a0\u00a0'.repeat(depth) + g.name, g.id));
    addParents(g.folders, depth + 1);
  });
  addParents(cat.folders, 0);
  $('#filter-error').textContent = '';
  $('#filter-preview-status').textContent = ''; $('#filter-preview').replaceChildren();
  $('#filter-dialog-title').textContent = folder ? 'Edit virtual folder' : 'Add virtual folder';
  $('#filter-dialog').showModal();
}
function filterFromForm() {
  const el = $('#filter-form').elements;
  const filter = {};
  for (const k of ['country', 'language', 'format', 'order']) if (el[k].value) filter[k] = el[k].value;
  if (filterGenres.length) filter.genres = [...filterGenres];
  if (el.name.value.trim()) filter.name = el.name.value.trim();
  for (const k of ['minBitrate', 'limit']) if (Number(el[k].value) > 0) filter[k] = Math.floor(Number(el[k].value));
  return filter;
}
$('#filter-form').addEventListener('submit', (e) => {
  if (e.submitter && e.submitter.value !== 'ok') return;
  const el = $('#filter-form').elements;
  let err = '';
  if (!el.folder.value.trim()) err = 'Give the folder a name.';
  else if (!Object.keys(filterFromForm()).length) err = 'Set at least one filter.';
  else if (filterGenres.length > maxGenres) err = 'Choose at most ' + maxGenres + ' genres.';
  if (err) { e.preventDefault(); $('#filter-error').textContent = err; }
});
let filterPreviewRun = 0; // only the latest click shows its answer
$('#filter-preview-button').onclick = async () => {
  const run = ++filterPreviewRun;
  const out = $('#filter-preview-status'), filter = filterFromForm();
  $('#filter-error').textContent = ''; $('#filter-preview').replaceChildren();
  if (!Object.keys(filter).length) { out.textContent = ''; $('#filter-error').textContent = 'Set at least one filter.'; return; }
  out.textContent = 'Loading matching stations…';
  try {
    const list = await previewStations(filter);
    if (run !== filterPreviewRun) return;
    out.textContent = previewSummary(list);
    $('#filter-preview').replaceChildren(...list.slice(0, 20).map(stationRow));
  } catch (e) {
    if (run !== filterPreviewRun) return;
    out.textContent = ''; $('#filter-error').textContent = e.message;
  }
};
$('#filter-dialog').addEventListener('close', async () => {
  if ($('#filter-dialog').returnValue !== 'ok') return;
  const el = $('#filter-form').elements;
  const filter = filterFromForm();
  const name = el.folder.value.trim();
  if (filterTarget) { filterTarget.name = name; filterTarget.filter = filter; }
  else {
    const f = { id: newId(), name, folders: [], stations: [], filter };
    const [parent] = el.parent.value ? findFolder(Number(el.parent.value)) : [null];
    if (parent) { parent.folders = parent.folders || []; parent.folders.push(f); } else cat.folders.push(f);
    selected = f.id;
  }
  setDirty(true); render();
  if (filterOnSave) await filterOnSave();
});
$('#add-virtual').onclick = () => openFilterDialog(null);
$('#edit-filter').onclick = () => { const [f] = findFolder(selected); if (f) openFilterDialog(f); };

function addFolder(parent) {
  const name = prompt('Folder name'); if (!name || !name.trim()) return;
  const f = { id: newId(), name: name.trim(), folders: [], stations: [] };
  if (parent) { parent.folders = parent.folders || []; parent.folders.push(f); } else cat.folders.push(f);
  selected = f.id; setDirty(true); render();
}

$('#add-folder').onclick = () => addFolder(null);
$('#add-subfolder').onclick = () => addFolder(findFolder(selected)[0]);
$('#rename-folder').onclick = () => {
  const [f] = findFolder(selected); if (!f) return;
  const name = prompt('Folder name', f.name); if (!name || !name.trim()) return;
  f.name = name.trim(); setDirty(true); render();
};
$('#delete-folder').onclick = () => {
  const [f] = findFolder(selected); if (!f) return;
  const n = (f.stations || []).length + (f.folders || []).length;
  if (n && !confirm('"' + f.name + '" is not empty. Delete it and its sub-folders? Stations stay in the list of stations not in any folder.')) return;
  const arr = siblings(f); arr.splice(arr.indexOf(f), 1);
  selected = null; setDirty(true); render();
};

function openStation(s) {
  editing = s;
  const form = $('#station-form');
  form.reset();
  $('#station-dialog-title').textContent = s ? 'Edit station' : 'Add station';
  $('#test-result').textContent = ''; $('#test-result').className = '';
  if (s) for (const k of ['name', 'url', 'description', 'format', 'bitrate', 'location', 'logo']) form.elements[k].value = s[k] || '';
  form.elements.relay.checked = !!(s && s.relay);
  $('#station-dialog').showModal();
}
$('#add-station').onclick = () => openStation(null);
$('#station-dialog').addEventListener('close', () => {
  if ($('#station-dialog').returnValue !== 'ok') return;
  const el = $('#station-form').elements;
  const v = {
    name: el.name.value.trim(), url: el.url.value.trim(), description: el.description.value.trim(),
    format: el.format.value, bitrate: Number(el.bitrate.value) || 0, location: el.location.value.trim(),
    logo: el.logo.value.trim(), relay: el.relay.checked,
  };
  if (editing) Object.assign(editing, v);
  else {
    const s = Object.assign({ id: newId() }, v);
    cat.stations.push(s);
    const [f] = findFolder(selected); if (f) { f.stations = f.stations || []; f.stations.push(s.id); }
  }
  setDirty(true); render();
});

$('#test-url').onclick = async () => {
  const out = $('#test-result'); const url = $('#station-form').elements.url.value.trim();
  if (!url) return;
  out.className = ''; out.textContent = 'Testing…';
  try {
    const p = await api('api/test-url', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ url }) });
    if (!p.reachable) { out.className = 'err'; out.textContent = 'Not reachable: ' + (p.error || ''); return; }
    let msg = 'Reachable (' + (p.contentType || 'unknown type') + ').';
    if (p.httpsOnly) msg += ' HTTPS only.';
    if (p.suggestRelay) { msg += ' Relay recommended.'; $('#station-form').elements.relay.checked = true; }
    if (p.warning) msg += ' ' + p.warning;
    out.className = p.warning ? 'warn' : 'ok'; out.textContent = msg;
  } catch (e) { out.className = 'err'; out.textContent = e.message; }
};

// force: save even if the list changed on the server since it was loaded (overwrites that change).
async function save(force = false) {
  // Assign real ids: the server only assigns ids for id 0, so give new entries ids above nextId here.
  let next = cat.nextId;
  const remap = new Map();
  for (const s of cat.stations) if (s.id < 0) { remap.set(s.id, next); s.id = next++; }
  walk(cat.folders, f => { if (f.id < 0) { if (selected === f.id) selected = next; f.id = next++; } f.stations = (f.stations || []).map(id => remap.get(id) || id); });
  cat.nextId = next;
  const headers = { 'Content-Type': 'application/json' };
  if (catVersion && !force) headers['X-Catalog-Version'] = catVersion;
  cat = await api('api/catalog?' + dq(), { method: 'PUT', headers, body: JSON.stringify(cat) });
  setDirty(false); render(); status('Saved');
}
$('#save').onclick = () => save().catch(e => {
  if (e.status === 412 && confirm('This list was changed somewhere else since you opened it (another window, or an import). ' +
      'Save your version anyway? That replaces the other changes.\n\nCancel keeps your changes on screen, unsaved.')) {
    return save(true).catch(e2 => status('Not saved: ' + e2.message));
  }
  status('Not saved: ' + e.message);
});
// ---- receivers
async function loadDevices() {
  devices = await api('api/devices');
  if (!devices.find(d => d.key === device)) device = 'default';
  const sel = $('#device'); sel.replaceChildren();
  const copy = $('#copy-device'); copy.replaceChildren(copy.firstElementChild || new Option('Copy list from…', ''));
  copy.firstElementChild.textContent = 'Copy list from…';
  for (const d of devices) {
    const label = d.name + (d.lastIp ? ' (' + d.lastIp + ')' : '');
    sel.append(new Option(label, d.key, false, d.key === device));
    if (d.key !== device) copy.append(new Option(label, d.key));
  }
  $('#delete-device').disabled = device === 'default';
}
$('#device').onchange = async () => {
  if (dirty && !confirm('Discard unsaved changes?')) { $('#device').value = device; return; }
  device = $('#device').value; selected = null; await loadDevices(); load();
};
$('#rename-device').onclick = async () => {
  const d = devices.find(x => x.key === device); const name = prompt('Receiver name', d ? d.name : '');
  if (!name) return;
  try { await api('api/devices/' + device, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name }) }); loadDevices(); }
  catch (e) { status(e.message); }
};
$('#copy-device').onchange = async () => {
  const from = $('#copy-device').value; $('#copy-device').value = ''; if (!from) return;
  const nameOf = (k) => (devices.find(x => x.key === k) || { name: k }).name;
  if (!confirm('Replace the list of "' + nameOf(device) + '" with a copy of the list of "' + nameOf(from) + '"?\n\n'
    + 'All folders and stations of "' + nameOf(device) + '" are replaced. "' + nameOf(from) + '" is not changed.')) return;
  try { cat = await api('api/devices/' + device + '/copy?from=' + encodeURIComponent(from), { method: 'POST', headers: { 'Content-Type': 'application/json' } }); setDirty(false); render(); status('Copied the list of "' + nameOf(from) + '"'); }
  catch (e) { status(e.message); }
};
$('#delete-device').onclick = async () => {
  if (!confirm('Forget this receiver and delete its list? If it connects again it starts from the default list.')) return;
  try { await api('api/devices/' + device, { method: 'DELETE' }); device = 'default'; await loadDevices(); load(); }
  catch (e) { status(e.message); }
};

// ---- public directory
let dirLoaded = false;
let dirLanguages = [];
async function loadDirLists() {
  if (dirLoaded) return;
  try {
    const [countries, tags, languages] = await Promise.all([api('api/directory/countries'), api('api/directory/tags'), api('api/directory/languages')]);
    dirLanguages = languages;
    for (const c of countries) $('#dir-country').append(new Option(c.name + ' (' + c.stationcount + ')', c.iso_3166_1));
    for (const t of tags) $('#dir-tag').append(new Option(t.name + ' (' + t.stationcount + ')', t.name));
    dirLoaded = true;
    if (cat) renderFolder(); // country names for the filter summary
  } catch (e) { status(e.message); }
}
$('#dir-q').addEventListener('keydown', (e) => { if (e.key === 'Enter') $('#dir-search').click(); });
let dirResults = null; // last directory search, re-drawn when the receiver's list changes

$('#dir-search').onclick = async () => {
  const p = new URLSearchParams();
  if ($('#dir-country').value) p.set('country', $('#dir-country').value);
  if ($('#dir-tag').value) p.set('tag', $('#dir-tag').value);
  if ($('#dir-q').value.trim()) p.set('q', $('#dir-q').value.trim());
  if ($('#dir-codec').value) {
    if (!p.toString()) { status('Choose a country, a genre or a name as well'); return; }
    p.set('codec', $('#dir-codec').value);
  }
  status('Searching…');
  try {
    dirResults = await api('api/directory/stations?' + p);
    status(dirResults.length + ' stations');
    renderDirResults();
  } catch (e) { status(e.message); }
};

$('#dir-save-virtual').onclick = () => {
  const tag = $('#dir-tag').value;
  const preset = { country: $('#dir-country').value, genres: tag ? [tag] : [], format: $('#dir-codec').value, name: $('#dir-q').value.trim() };
  if (!preset.country && !tag && !preset.name) { status('Choose a country, a genre or a name first'); return; }
  const wasDirty = dirty;
  openFilterDialog(null, preset, async () => {
    // With other unsaved edits open, leave the decision to save them to the user, like adding a station.
    if (wasDirty) { status('Added the virtual folder. Save changes to keep it.'); return; }
    try { await save(); status('Saved the virtual folder. Find it under My stations.'); }
    catch (e) { setDirty(true); status('Not saved: ' + e.message); }
  });
};

// The station in the receiver's list that matches a directory entry: same directory id, or same stream URL.
function ownStation(d) {
  const url = d.url_resolved || d.url;
  return cat.stations.find(x => x.source === 'radio-browser' && x.sourceId === d.stationuuid) ||
    cat.stations.find(x => x.url === url || (d.url && x.url === d.url));
}

// Folder paths ("By country › Poland") that hold station id.
function foldersHolding(id) {
  const out = [];
  const go = (folders, path) => (folders || []).forEach(f => {
    const p = path.concat(f.name);
    if ((f.stations || []).includes(id)) out.push(p.join(' › '));
    go(f.folders, p);
  });
  go(cat.folders, []);
  return out;
}

function renderDirResults() {
  if (!dirResults) return;
  $('#dir-results tbody').replaceChildren(...dirResults.map(s => {
    const tr = document.createElement('tr');
    const nameTd = document.createElement('td');
    const name = document.createElement('div'); name.textContent = s.name || '';
    nameTd.append(name);
    const own = ownStation(s);
    const where = own ? foldersHolding(own.id) : [];
    if (where.length) {
      const b = document.createElement('div'); b.className = 'badges';
      b.title = 'Already in these folders of this receiver';
      for (const w of where) { const x = document.createElement('span'); x.className = 'badge in'; x.textContent = w; b.append(x); }
      nameTd.append(b);
    }
    tr.append(nameTd);
    for (const v of [s.country, (s.codec || '') + (s.bitrate ? ' ' + s.bitrate + 'k' : ''), (s.tags || '').replace(/,/g, ', ')]) { const td = document.createElement('td'); td.textContent = v || ''; tr.append(td); }
    const td = document.createElement('td'); td.className = 'actions';
    const add = button(where.length ? 'Add to another folder' : 'Add', 'Choose a folder for this station', () => openFolderPicker(td, add, s), where.length ? '' : 'primary');
    td.append(add);
    tr.append(td);
    return tr;
  }));
}

// Folder picker shown in a result row after clicking Add. Only one is open at a time.
let closePicker = null;
function openFolderPicker(td, addBtn, d) {
  if (closePicker) closePicker();
  let anyFolder = false; walk(cat.folders, f => { if (!f.filter) anyFolder = true; });
  if (!anyFolder) { status('Create a folder under My stations first'); return; }
  const existing = ownStation(d);
  const sel = document.createElement('select');
  sel.className = 'folder-picker';
  sel.setAttribute('aria-label', 'Folder for ' + d.name);
  sel.append(new Option('Choose folder…', ''));
  const addOptions = (folders, depth) => (folders || []).forEach(f => {
    if (f.filter) return; // a virtual folder lists its own matches
    const there = existing && (f.stations || []).includes(existing.id);
    const o = new Option('\u00a0\u00a0'.repeat(depth) + f.name + (there ? ' (already there)' : ''), f.id);
    o.disabled = !!there;
    sel.append(o);
    addOptions(f.folders, depth + 1);
  });
  addOptions(cat.folders, 0);
  const cancel = button('Cancel', 'Close without adding', () => closePicker());
  addBtn.hidden = true;
  td.append(sel, cancel);
  closePicker = () => { sel.remove(); cancel.remove(); addBtn.hidden = false; closePicker = null; };
  sel.onchange = async () => {
    const id = Number(sel.value);
    if (!id) return;
    closePicker();
    await addFromDirectory(d, id);
  };
  sel.focus();
}

async function addFromDirectory(d, folderId) {
  const [f] = findFolder(folderId);
  if (!f) { status('That folder no longer exists'); return; }
  let s = ownStation(d);
  if (!s) {
    const url = d.url_resolved || d.url;
    s = { id: newId(), name: d.name, url, description: (d.tags || '').slice(0, 300), format: (d.codec || 'MP3').toUpperCase(),
      bitrate: d.bitrate || 0, location: d.country || '', logo: (d.favicon || '').length <= 4096 ? (d.favicon || '') : '', relay: url.startsWith('https://'),
      source: 'radio-browser', sourceId: d.stationuuid };
    cat.stations.push(s);
  }
  f.stations = f.stations || [];
  if (f.stations.includes(s.id)) { status('Already in ' + f.name); return; }
  f.stations.push(s.id);
  // With other unsaved edits open, leave the decision to save them to the user.
  if (dirty) { setDirty(true); render(); status('Added "' + d.name + '" to ' + f.name + '. Save changes to keep it.'); return; }
  try { await save(); status('Added "' + d.name + '" to ' + f.name); }
  catch (e) { setDirty(true); render(); status('Not saved: ' + e.message); }
}

$('#revert').onclick = () => { if (confirm('Discard unsaved changes?')) load(); };
window.addEventListener('beforeunload', (e) => { if (dirty) { e.preventDefault(); e.returnValue = ''; } });

async function loadRequests() {
  try {
    const rows = await api('api/requests');
    $('#requests tbody').replaceChildren(...rows.map(r => {
      const tr = document.createElement('tr');
      for (const v of [new Date(r.time).toLocaleString(), r.client, r.host, r.path, r.query, r.note || '']) { const td = document.createElement('td'); td.textContent = v; tr.append(td); }
      return tr;
    }));
  } catch (e) { status(e.message); }
}
async function loadLog() { try { $('#log').textContent = await api('api/log'); } catch (e) { status(e.message); } }
$('#refresh-requests').onclick = loadRequests;
$('#refresh-log').onclick = loadLog;

$('#export').onclick = () => { location.href = 'api/export?' + dq(); };
$('#import').onclick = async () => {
  const file = $('#import-file').files[0]; if (!file) { status('Choose a file first'); return; }
  const mode = document.querySelector('input[name=mode]:checked').value;
  if (dirty && !confirm('Importing discards your unsaved changes. Continue?')) return;
  if (mode === 'replace' && !confirm('Replace the whole station list?')) return;
  try { cat = await api('api/import?mode=' + mode + '&' + dq(), { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: await file.text() }); setDirty(false); render(); status('Imported'); }
  catch (e) { status('Import failed: ' + e.message); }
};

for (const b of document.querySelectorAll('nav button')) b.onclick = () => {
  for (const x of document.querySelectorAll('nav button')) x.classList.toggle('active', x === b);
  for (const s of document.querySelectorAll('main > section')) s.hidden = s.id !== 'tab-' + b.dataset.tab;
  if (b.dataset.tab === 'requests') loadRequests();
  if (b.dataset.tab === 'log') loadLog();
  if (b.dataset.tab === 'directory') loadDirLists();
};

loadDevices().then(load).catch(e => status('Could not load: ' + e.message));
