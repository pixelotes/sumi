// sumi web UI — directory browser + epub.js viewer with localStorage
// bookmarks. No server-side state. The only thing the server stores is
// the books themselves; reading position lives in the browser.

const KEY_TOKEN = 'sumi.jwt';
const KEY_VIEW  = 'sumi.view';
const KEY_THEME = 'sumi.reader.theme';
const KEY_FONT  = 'sumi.reader.font';      // CSS font-family string, "" = default
const KEY_SIZE  = 'sumi.reader.size';      // percentage as integer string, e.g. "120"
const KEY_BOOK_PREFIX = 'sumi.book.';      // KEY_BOOK_PREFIX + bookPath → CFI

const $ = (id) => document.getElementById(id);

const state = {
    currentPath: '',
    items: [],
    filteredItems: null,

    // Reader state.
    book: null,
    rendition: null,
    currentBookPath: null,
};

const api = {
    async login(password) {
        const res = await fetch('/api/v1/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username: 'sumi', password }),
        });
        if (!res.ok) throw new Error('Invalid credentials');
        return res.json();
    },
    async browse(path) {
        const token = sessionStorage.getItem(KEY_TOKEN);
        const url = '/api/v1/browse' + (path ? `?path=${encodeURIComponent(path)}` : '');
        const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } });
        if (res.status === 401) { logout(); throw new Error('Session expired'); }
        if (!res.ok) throw new Error('Browse failed');
        return res.json();
    },
};

function withToken(url) {
    const token = sessionStorage.getItem(KEY_TOKEN);
    const sep = url.includes('?') ? '&' : '?';
    return `${url}${sep}token=${encodeURIComponent(token)}`;
}

function toast(msg, kind = 'success') {
    const el = document.createElement('div');
    el.className = `toast toast-${kind}`;
    el.textContent = msg;
    document.body.appendChild(el);
    setTimeout(() => el.remove(), 2400);
}

function setLoading(visible, label = 'Loading...') {
    $('loading-overlay').style.display = visible ? 'flex' : 'none';
    $('loading-text').textContent = label;
}

// --- Navigation ---

async function navigate(path) {
    setLoading(true);
    try {
        const data = await api.browse(path);
        state.currentPath = path || '';
        state.items = data.items || [];
        state.filteredItems = null;
        $('search-input').value = '';
        renderBreadcrumb(data.current_folder);
        renderFolderHeader(data.current_folder);
        render();
        $('controls-bar').style.display = state.items.length > 0 ? 'flex' : 'none';
    } catch (e) {
        toast(e.message, 'error');
    } finally {
        setLoading(false);
    }
}

function renderBreadcrumb(folder) {
    const bc = $('breadcrumb');
    if (!folder) {
        bc.innerHTML = '<a onclick="window.sumiApp.goHome()">Home</a>';
        return;
    }
    const parts = [{ name: 'Home', path: '' }];
    if (folder.path) {
        const segments = folder.path.split('/').filter(Boolean);
        let acc = '';
        for (const seg of segments) {
            acc += '/' + seg;
            parts.push({ name: seg, path: acc });
        }
    }
    bc.innerHTML = parts.map((p, i) => {
        const last = i === parts.length - 1;
        const sep = i > 0 ? '<span class="sep">/</span>' : '';
        if (last) return `${sep}<span>${escapeHTML(p.name)}</span>`;
        return `${sep}<a data-path="${escapeAttr(p.path)}">${escapeHTML(p.name)}</a>`;
    }).join('');
    bc.querySelectorAll('a[data-path]').forEach(a => {
        a.addEventListener('click', () => navigate(a.getAttribute('data-path')));
    });
}

function renderFolderHeader(folder) {
    const h = $('folder-header');
    if (!folder || !state.currentPath) {
        h.innerHTML = '';
        return;
    }
    const back = folder.parent !== undefined ? folder.parent : '';
    h.innerHTML = `
        <span class="back-btn" id="back-btn" title="Back">&#8592;</span>
        <h2>${escapeHTML(folder.name)}</h2>
    `;
    $('back-btn').addEventListener('click', () => navigate(back));
}

function render() {
    const view = sessionStorage.getItem(KEY_VIEW) || 'grid';
    const grid = $('file-browser');
    grid.className = 'file-grid' + (view === 'list' ? ' list-view' : '');
    const items = state.filteredItems ?? state.items;
    if (items.length === 0) {
        grid.innerHTML = '<div class="empty-state">Empty folder</div>';
        return;
    }
    grid.innerHTML = items.map(item => {
        const cls = item.is_dir ? 'file-item dir' : 'file-item book';
        const poster = item.icon
            ? `<img src="${escapeAttr(withToken(item.icon))}" loading="lazy" alt="" onerror="this.style.display='none'">`
            : (item.is_book ? '&#128214;' : '&#128193;');
        const sub = item.is_book ? `<div class="item-sub">${displayExt(item.name)}</div>` : '';
        const progress = item.is_book ? getProgress(item.path) : 0;
        const progressBar = progress > 0
            ? `<div class="progress-bar" style="width: ${(progress * 100).toFixed(1)}%"></div>`
            : '';
        return `
            <div class="${cls}" data-path="${escapeAttr(item.path)}" data-is-dir="${item.is_dir}" data-is-book="${!!item.is_book}" data-stream="${escapeAttr(item.stream_url || '')}">
                <div class="item-poster">${poster}${progressBar}</div>
                <div class="item-info">
                    <div class="item-name">${escapeHTML(stripExt(item.name))}</div>
                    ${sub}
                </div>
            </div>`;
    }).join('');
    grid.querySelectorAll('.file-item').forEach(el => {
        el.addEventListener('click', () => {
            const path = el.getAttribute('data-path');
            const isDir = el.getAttribute('data-is-dir') === 'true';
            const isBook = el.getAttribute('data-is-book') === 'true';
            if (isDir) navigate(path);
            else if (isBook) openBook(path, el.getAttribute('data-stream'));
        });
    });
}

function applyFilter(q) {
    const needle = q.trim().toLowerCase();
    state.filteredItems = needle
        ? state.items.filter(i => i.name.toLowerCase().includes(needle))
        : null;
    render();
}

// --- Bookmarks (localStorage) ---

function bookmarkKey(bookPath) {
    return KEY_BOOK_PREFIX + bookPath;
}

function loadBookmark(bookPath) {
    const raw = localStorage.getItem(bookmarkKey(bookPath));
    if (!raw) return null;
    try { return JSON.parse(raw); } catch { return null; }
}

function saveBookmark(bookPath, cfi, percentage) {
    const data = { cfi, percentage, updatedAt: new Date().toISOString() };
    localStorage.setItem(bookmarkKey(bookPath), JSON.stringify(data));
}

function getProgress(bookPath) {
    const bm = loadBookmark(bookPath);
    return bm?.percentage || 0;
}

// --- Reader ---

async function openBook(bookPath, streamURL) {
    setLoading(true, 'Opening book...');
    try {
        // Download the full .epub as an ArrayBuffer ourselves rather than
        // letting epub.js fetch by URL: our stream endpoint is JWT-gated
        // via `?token=…`, and epub.js's URL-based loader doesn't carry
        // that query param onto its internal subresource requests
        // (chapter HTML, cover, images), so they 401. Loading from an
        // ArrayBuffer keeps the whole archive in memory — fine for the
        // LAN scale we run at — and bypasses the issue entirely.
        const url = withToken(streamURL);
        const res = await fetch(url);
        if (!res.ok) throw new Error(`stream returned ${res.status}`);
        const buf = await res.arrayBuffer();
        state.book = ePub(buf);
        state.currentBookPath = bookPath;

        await state.book.ready;

        $('reader-title').textContent = stripExt(bookPath.split('/').pop());
        $('reader-screen').classList.add('visible');
        const savedTheme = localStorage.getItem(KEY_THEME);
        if (savedTheme === 'dark') $('reader-screen').classList.add('dark');

        // Render into the viewer. "auto" lets epub.js pick a layout that
        // matches the container; "spread: none" disables side-by-side
        // pages on wide screens — easier to read on a Pi-served LAN tool.
        state.rendition = state.book.renderTo('viewer', {
            width: '100%',
            height: '100%',
            spread: 'none',
            allowScriptedContent: false,
        });
        applyReaderTheme();
        applyReaderFont();

        // Populate TOC. Cheap synchronous read; epub.js parsed it already
        // by the time `book.ready` resolved.
        renderTOC();

        // Restore or start at the beginning.
        const bm = loadBookmark(bookPath);
        if (bm?.cfi) {
            await state.rendition.display(bm.cfi);
        } else {
            await state.rendition.display();
        }

        // Generate locations for percentage display. ~1500 chars per
        // location is the epub.js default; on a Pi a one-time pass on
        // book open is fine.
        state.book.locations.generate(1500).then(() => {
            updateProgress();
        });

        state.rendition.on('relocated', (loc) => {
            const cfi = loc.start.cfi;
            const pct = state.book.locations.percentageFromCfi(cfi) || 0;
            saveBookmark(state.currentBookPath, cfi, pct);
            updateProgress();
        });
    } catch (e) {
        console.error(e);
        toast('Failed to open book: ' + e.message, 'error');
    } finally {
        setLoading(false);
    }
}

function applyReaderTheme() {
    if (!state.rendition) return;
    const dark = $('reader-screen').classList.contains('dark');
    state.rendition.themes.register('light', {
        body: { color: '#222', background: '#ece6da' },
        'a': { color: '#5a4a8a' },
    });
    state.rendition.themes.register('dark', {
        body: { color: '#ddd', background: '#1a1a1a' },
        'a': { color: '#b9a0e8' },
    });
    state.rendition.themes.select(dark ? 'dark' : 'light');
}

// --- Reader preferences (font family + size, persisted in localStorage) ---

function applyReaderFont() {
    if (!state.rendition) return;
    const font = localStorage.getItem(KEY_FONT) || '';
    const size = parseInt(localStorage.getItem(KEY_SIZE) || '100', 10);
    if (font) state.rendition.themes.font(font);
    else state.rendition.themes.font('inherit'); // let epub pick
    state.rendition.themes.fontSize(size + '%');

    // Reflect current state in the panel UI.
    $('font-size-display').textContent = size + '%';
    $('font-family-row').querySelectorAll('button').forEach(b => {
        b.classList.toggle('active', (b.dataset.font || '') === font);
    });
}

function setFont(family) {
    localStorage.setItem(KEY_FONT, family);
    applyReaderFont();
}

function fontSizeDelta(delta) {
    const current = parseInt(localStorage.getItem(KEY_SIZE) || '100', 10);
    // Clamp to a sane range — below 70% lines get unreadable, above 200%
    // a single page barely holds one paragraph.
    const next = Math.max(70, Math.min(200, current + delta));
    localStorage.setItem(KEY_SIZE, String(next));
    applyReaderFont();
}

// --- Settings panel toggle ---

function toggleSettings() {
    $('settings-panel').classList.toggle('visible');
}

// --- Table of contents ---

function renderTOC() {
    const root = $('toc-list');
    root.innerHTML = '';
    if (!state.book || !state.book.navigation || !state.book.navigation.toc) {
        root.innerHTML = '<div style="opacity:0.5">No table of contents</div>';
        return;
    }
    const ul = buildTocList(state.book.navigation.toc);
    root.appendChild(ul);
}

function buildTocList(items) {
    const ul = document.createElement('ul');
    for (const item of items) {
        const li = document.createElement('li');
        const a = document.createElement('a');
        a.textContent = item.label.trim();
        a.dataset.href = item.href;
        a.addEventListener('click', (e) => {
            e.preventDefault();
            if (state.rendition) state.rendition.display(item.href);
            // Auto-close on chapter jump — wider reading area is more important
            // than keeping the panel open after a navigation.
            $('settings-panel').classList.remove('visible');
        });
        li.appendChild(a);
        if (item.subitems && item.subitems.length > 0) {
            li.appendChild(buildTocList(item.subitems));
        }
        ul.appendChild(li);
    }
    return ul;
}

// Wire up font-family buttons once at startup; they don't change content.
function wireFontFamilyButtons() {
    $('font-family-row').querySelectorAll('button').forEach(b => {
        b.addEventListener('click', () => setFont(b.dataset.font || ''));
    });
}

function updateProgress() {
    if (!state.book || !state.book.locations || state.book.locations.length() === 0) {
        $('reader-progress').textContent = '';
        return;
    }
    const bm = loadBookmark(state.currentBookPath);
    if (!bm) { $('reader-progress').textContent = '0%'; return; }
    $('reader-progress').textContent = `${Math.round(bm.percentage * 100)}%`;
}

async function closeReader() {
    if (state.rendition) state.rendition.destroy();
    if (state.book) state.book.destroy();
    state.rendition = null;
    state.book = null;
    state.currentBookPath = null;
    $('reader-screen').classList.remove('visible');
    // Re-render so any progress bars reflect the latest position.
    render();
}

function toggleReaderTheme() {
    const screen = $('reader-screen');
    screen.classList.toggle('dark');
    localStorage.setItem(KEY_THEME, screen.classList.contains('dark') ? 'dark' : 'light');
    applyReaderTheme();
}

function prevPage() {
    if (state.rendition) state.rendition.prev();
}

function nextPage() {
    if (state.rendition) state.rendition.next();
}

// --- Helpers ---

function stripExt(name) {
    const idx = name.lastIndexOf('.');
    return idx > 0 ? name.slice(0, idx) : name;
}
function displayExt(name) {
    const idx = name.lastIndexOf('.');
    return idx > 0 ? name.slice(idx + 1).toUpperCase() : '';
}
function escapeHTML(s) {
    return String(s).replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
}
function escapeAttr(s) {
    return escapeHTML(s).replaceAll('"', '&quot;');
}

function logout() {
    sessionStorage.clear();
    if (state.rendition) state.rendition.destroy();
    if (state.book) state.book.destroy();
    $('reader-screen').classList.remove('visible');
    $('app-container').style.display = 'none';
    $('login-screen').style.display = 'flex';
}

function showApp() {
    $('login-screen').style.display = 'none';
    $('app-container').style.display = 'block';
    navigate('');
}

window.sumiApp = {
    goHome: () => navigate(''),
    setView: (v) => {
        sessionStorage.setItem(KEY_VIEW, v);
        $('view-grid').classList.toggle('active', v === 'grid');
        $('view-list').classList.toggle('active', v === 'list');
        render();
    },
    closeReader,
    toggleReaderTheme,
    toggleSettings,
    fontSizeDelta,
    prevPage,
    nextPage,
    logout,
};

document.addEventListener('keydown', (e) => {
    if (!$('reader-screen').classList.contains('visible')) return;
    if (e.key === 'ArrowLeft' || e.key === 'PageUp') prevPage();
    else if (e.key === 'ArrowRight' || e.key === 'PageDown' || e.key === ' ') nextPage();
    else if (e.key === 'Escape') closeReader();
});

document.addEventListener('DOMContentLoaded', () => {
    const savedView = sessionStorage.getItem(KEY_VIEW);
    if (savedView) {
        $('view-grid').classList.toggle('active', savedView === 'grid');
        $('view-list').classList.toggle('active', savedView === 'list');
    }

    wireFontFamilyButtons();

    if (sessionStorage.getItem(KEY_TOKEN)) {
        showApp();
    }

    $('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const password = $('password-input').value;
        try {
            const res = await api.login(password);
            sessionStorage.setItem(KEY_TOKEN, res.token);
            $('login-error').textContent = '';
            showApp();
        } catch (err) {
            $('login-error').textContent = err.message;
        }
    });

    $('search-input').addEventListener('input', (e) => applyFilter(e.target.value));
});
