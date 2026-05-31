'use strict';

/* ── Theme manager ─────────────────────────────────────────────────────────── */
const ThemeManager = (() => {
  const THEMES = ['dark', 'light', 'auto'];
  const KEY = 'cassonic_theme';

  function apply(theme) {
    document.documentElement.setAttribute('data-theme', theme);
  }

  function current() {
    return localStorage.getItem(KEY) || 'dark';
  }

  function cycle() {
    const next = THEMES[(THEMES.indexOf(current()) + 1) % THEMES.length];
    localStorage.setItem(KEY, next);
    apply(next);
    updateToggleBtns(next);
  }

  function updateToggleBtns(theme) {
    document.querySelectorAll('[data-theme-toggle]').forEach(btn => {
      btn.setAttribute('title', 'Theme: ' + theme);
    });
  }

  function init() {
    apply(current());
    document.querySelectorAll('[data-theme-toggle]').forEach(btn => {
      btn.addEventListener('click', cycle);
    });
    updateToggleBtns(current());
  }

  return { init, current, cycle };
})();

/* ── Toast notifications ─────────────────────────────────────────────────── */
const Toast = (() => {
  function ensure() {
    let c = document.getElementById('toast-container');
    if (!c) {
      c = document.createElement('div');
      c.id = 'toast-container';
      c.className = 'toast-container';
      document.body.appendChild(c);
    }
    return c;
  }

  function show(message, type = 'info', duration = 3500) {
    const container = ensure();
    const el = document.createElement('div');
    el.className = 'toast ' + type;
    el.textContent = message;
    container.appendChild(el);
    setTimeout(() => {
      el.style.transition = 'opacity 0.3s ease';
      el.style.opacity = '0';
      setTimeout(() => el.remove(), 300);
    }, duration);
  }

  return { show };
})();

/* ── API client ──────────────────────────────────────────────────────────── */
class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function api(method, path, body) {
  const token = localStorage.getItem('cassonic_token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;

  const opts = { method, headers };
  if (body !== undefined) opts.body = JSON.stringify(body);

  const res = await fetch(path, opts);
  const ct = res.headers.get('content-type') || '';
  const data = ct.includes('application/json') ? await res.json() : await res.text();

  if (!res.ok) {
    const msg = (data && data.error && data.error.message) || (typeof data === 'string' ? data : 'Request failed');
    throw new ApiError(res.status, msg);
  }
  return data;
}

/* ── Audio Player ─────────────────────────────────────────────────────────── */
class CassonicPlayer {
  constructor() {
    this.audio = new Audio();
    this.queue = [];
    this.queueIndex = -1;
    this.shuffle = localStorage.getItem('cassonic_shuffle') === 'true';
    this.repeat = localStorage.getItem('cassonic_repeat') || 'none';
    this.volume = parseFloat(localStorage.getItem('cassonic_volume') || '1');
    this.audio.volume = this.volume;
    this._shuffleOrder = [];

    this._bindAudio();
    this._bindKeyboard();
    this._bindUI();
    this._restoreState();
  }

  _bindAudio() {
    this.audio.addEventListener('timeupdate', () => this._updateProgress());
    this.audio.addEventListener('ended', () => this._onEnded());
    this.audio.addEventListener('play', () => this._onStateChange());
    this.audio.addEventListener('pause', () => this._onStateChange());
    this.audio.addEventListener('error', () => {
      Toast.show('Playback error — skipping track', 'error');
      this.next();
    });
    this.audio.addEventListener('loadedmetadata', () => this._updateDuration());
  }

  _bindKeyboard() {
    document.addEventListener('keydown', e => {
      const tag = e.target.tagName.toLowerCase();
      if (tag === 'input' || tag === 'textarea' || tag === 'select') return;

      switch (e.key) {
        case ' ':
          e.preventDefault();
          this.togglePlay();
          break;
        case 'ArrowLeft':
          if (e.shiftKey) { e.preventDefault(); this.prev(); }
          else { e.preventDefault(); this.seek(Math.max(0, this.audio.currentTime - 5)); }
          break;
        case 'ArrowRight':
          if (e.shiftKey) { e.preventDefault(); this.next(); }
          else { e.preventDefault(); this.seek(Math.min(this.audio.duration || 0, this.audio.currentTime + 5)); }
          break;
        case 'm':
          e.preventDefault();
          this._toggleMute();
          break;
        case 'f':
          e.preventDefault();
          window.location.href = '/player';
          break;
      }
    });
  }

  _bindUI() {
    const q = id => document.getElementById(id);

    const playBtn = q('player-play');
    const prevBtn = q('player-prev');
    const nextBtn = q('player-next');
    const shuffleBtn = q('player-shuffle');
    const repeatBtn = q('player-repeat');
    const progress = q('player-progress');
    const volume = q('player-volume');
    const muteBtn = q('player-mute');

    if (playBtn) playBtn.addEventListener('click', () => this.togglePlay());
    if (prevBtn) prevBtn.addEventListener('click', () => this.prev());
    if (nextBtn) nextBtn.addEventListener('click', () => this.next());
    if (shuffleBtn) shuffleBtn.addEventListener('click', () => this._toggleShuffle());
    if (repeatBtn) repeatBtn.addEventListener('click', () => this._cycleRepeat());
    if (muteBtn) muteBtn.addEventListener('click', () => this._toggleMute());

    if (progress) {
      progress.addEventListener('input', e => {
        const pct = parseFloat(e.target.value);
        if (this.audio.duration) {
          this.seek((pct / 100) * this.audio.duration);
        }
      });
    }

    if (volume) {
      volume.value = this.volume * 100;
      volume.addEventListener('input', e => {
        this.setVolume(parseFloat(e.target.value) / 100);
      });
    }

    this._updateShuffleBtn();
    this._updateRepeatBtn();
  }

  _restoreState() {
    const saved = sessionStorage.getItem('cassonic_queue');
    if (!saved) return;
    try {
      const state = JSON.parse(saved);
      this.queue = state.queue || [];
      this.queueIndex = state.index >= 0 ? state.index : -1;
      if (this.queueIndex >= 0 && this.queue[this.queueIndex]) {
        this._loadTrack(this.queue[this.queueIndex], false);
      }
    } catch (_) { /* ignore corrupt state */ }
  }

  _saveState() {
    if (!this.queue.length) return;
    sessionStorage.setItem('cassonic_queue', JSON.stringify({
      queue: this.queue,
      index: this.queueIndex,
    }));
  }

  play(trackId) {
    const idx = this.queue.findIndex(t => t.id === trackId);
    if (idx >= 0) {
      this.queueIndex = idx;
      this._loadTrack(this.queue[idx], true);
    }
  }

  playTrack(track) {
    this.queue = [track];
    this.queueIndex = 0;
    this._shuffleOrder = [0];
    this._loadTrack(track, true);
  }

  playAll(tracks, startIndex = 0) {
    this.queue = tracks;
    this.queueIndex = startIndex;
    this._buildShuffleOrder();
    if (tracks[startIndex]) this._loadTrack(tracks[startIndex], true);
  }

  addToQueue(track) {
    this.queue.push(track);
    this._buildShuffleOrder();
    this._saveState();
    Toast.show(track.title + ' added to queue', 'info', 2000);
  }

  _loadTrack(track, autoplay) {
    const streamUrl = `/api/v1/stream/${track.id}?format=mp3&maxBitRate=320`;
    this.audio.src = streamUrl;
    if (autoplay) {
      this.audio.play().catch(() => {});
    }
    this._updateTrackUI(track);
    this._saveState();
    this._emitEvent('trackchange', { track });
  }

  togglePlay() {
    if (this.audio.paused) {
      if (!this.audio.src && this.queue.length) {
        this.queueIndex = 0;
        this._loadTrack(this.queue[0], true);
      } else {
        this.audio.play().catch(() => {});
      }
    } else {
      this.audio.pause();
    }
  }

  next() {
    if (!this.queue.length) return;
    let nextIdx;
    if (this.shuffle) {
      const pos = this._shuffleOrder.indexOf(this.queueIndex);
      const nextPos = (pos + 1) % this._shuffleOrder.length;
      nextIdx = this._shuffleOrder[nextPos];
    } else {
      nextIdx = this.queueIndex + 1;
    }

    if (nextIdx >= this.queue.length) {
      if (this.repeat === 'all') {
        nextIdx = 0;
      } else {
        this.audio.pause();
        return;
      }
    }

    this.queueIndex = nextIdx;
    this._loadTrack(this.queue[this.queueIndex], true);
  }

  prev() {
    if (this.audio.currentTime > 3) {
      this.audio.currentTime = 0;
      return;
    }
    if (!this.queue.length) return;

    let prevIdx;
    if (this.shuffle) {
      const pos = this._shuffleOrder.indexOf(this.queueIndex);
      prevIdx = this._shuffleOrder[(pos - 1 + this._shuffleOrder.length) % this._shuffleOrder.length];
    } else {
      prevIdx = this.queueIndex - 1;
    }

    if (prevIdx < 0) {
      if (this.repeat === 'all') prevIdx = this.queue.length - 1;
      else { this.audio.currentTime = 0; return; }
    }

    this.queueIndex = prevIdx;
    this._loadTrack(this.queue[this.queueIndex], true);
  }

  seek(seconds) {
    if (this.audio.duration) {
      this.audio.currentTime = Math.max(0, Math.min(seconds, this.audio.duration));
    }
  }

  setVolume(v) {
    this.volume = Math.max(0, Math.min(1, v));
    this.audio.volume = this.volume;
    this.audio.muted = false;
    localStorage.setItem('cassonic_volume', String(this.volume));
    const volInput = document.getElementById('player-volume');
    if (volInput) volInput.value = this.volume * 100;
  }

  _toggleMute() {
    this.audio.muted = !this.audio.muted;
  }

  _toggleShuffle() {
    this.shuffle = !this.shuffle;
    localStorage.setItem('cassonic_shuffle', String(this.shuffle));
    this._buildShuffleOrder();
    this._updateShuffleBtn();
  }

  _cycleRepeat() {
    const modes = ['none', 'all', 'one'];
    this.repeat = modes[(modes.indexOf(this.repeat) + 1) % modes.length];
    localStorage.setItem('cassonic_repeat', this.repeat);
    this._updateRepeatBtn();
    this.audio.loop = this.repeat === 'one';
  }

  _buildShuffleOrder() {
    const n = this.queue.length;
    const arr = Array.from({ length: n }, (_, i) => i);
    if (this.shuffle) {
      for (let i = n - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [arr[i], arr[j]] = [arr[j], arr[i]];
      }
    }
    this._shuffleOrder = arr;
  }

  _onEnded() {
    if (this.repeat === 'one') {
      this.audio.play().catch(() => {});
    } else {
      this.next();
    }
  }

  _onStateChange() {
    const btn = document.getElementById('player-play');
    if (!btn) return;
    const paused = this.audio.paused;
    btn.setAttribute('aria-label', paused ? 'Play' : 'Pause');
    const svg = btn.querySelector('svg use, svg');
    if (svg) {
      btn.innerHTML = paused ? ICONS.play : ICONS.pause;
    }
    this._emitEvent('statechange', { paused });
  }

  _updateProgress() {
    const current = this.audio.currentTime;
    const duration = this.audio.duration || 0;
    const pct = duration ? (current / duration) * 100 : 0;

    const progress = document.getElementById('player-progress');
    if (progress) {
      progress.value = pct;
      progress.style.setProperty('--progress', pct.toFixed(1) + '%');
    }

    const elapsed = document.getElementById('player-elapsed');
    if (elapsed) elapsed.textContent = formatTime(current);
  }

  _updateDuration() {
    const duration = this.audio.duration || 0;
    const el = document.getElementById('player-duration');
    if (el) el.textContent = formatTime(duration);
  }

  _updateTrackUI(track) {
    const titleEl = document.getElementById('player-title');
    const artistEl = document.getElementById('player-artist');
    const thumbEl = document.getElementById('player-thumb');

    if (titleEl) titleEl.textContent = track.title || '';
    if (artistEl) artistEl.textContent = track.artist || '';

    if (thumbEl) {
      if (track.coverArtId) {
        thumbEl.innerHTML = `<img src="/api/v1/cover/${track.coverArtId}?size=64" alt="" loading="lazy">`;
      } else {
        thumbEl.innerHTML = `<div class="player-thumb-placeholder">${ICONS.music}</div>`;
      }
    }

    document.title = (track.title ? track.title + ' — ' : '') + 'cassonic';
    this._highlightCurrentSong(track.id);
  }

  _highlightCurrentSong(id) {
    document.querySelectorAll('[data-song-id]').forEach(row => {
      row.classList.toggle('now-playing', row.dataset.songId === String(id));
    });
  }

  _updateShuffleBtn() {
    const btn = document.getElementById('player-shuffle');
    if (btn) btn.classList.toggle('active', this.shuffle);
  }

  _updateRepeatBtn() {
    const btn = document.getElementById('player-repeat');
    if (!btn) return;
    btn.classList.toggle('active', this.repeat !== 'none');
    btn.title = 'Repeat: ' + this.repeat;
  }

  _emitEvent(name, detail) {
    document.dispatchEvent(new CustomEvent('cassonic:' + name, { detail }));
  }
}

/* ── Icon snippets used by the player ─────────────────────────────────────── */
const ICONS = {
  play: `<svg viewBox="0 0 24 24" fill="currentColor"><polygon points="5,3 19,12 5,21"/></svg>`,
  pause: `<svg viewBox="0 0 24 24" fill="currentColor"><rect x="6" y="4" width="4" height="16"/><rect x="14" y="4" width="4" height="16"/></svg>`,
  music: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/></svg>`,
};

/* ── Format helpers ────────────────────────────────────────────────────────── */
function formatTime(secs) {
  if (!isFinite(secs) || secs < 0) return '0:00';
  const h = Math.floor(secs / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const s = Math.floor(secs % 60);
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
  return `${m}:${String(s).padStart(2, '0')}`;
}

/* ── SSE now-playing ────────────────────────────────────────────────────────── */
function initNowPlayingSSE() {
  if (!window.EventSource) return;
  const token = localStorage.getItem('cassonic_token');
  if (!token) return;

  const src = new EventSource(`/api/v1/nowplaying/stream?token=${encodeURIComponent(token)}`);
  src.addEventListener('message', e => {
    try {
      const data = JSON.parse(e.data);
      if (data && data.user_id) {
        document.querySelectorAll('[data-song-id="' + data.song_id + '"]').forEach(el => {
          el.classList.add('now-playing');
        });
      }
    } catch (_) {}
  });
  src.onerror = () => src.close();
}

/* ── Star toggle ────────────────────────────────────────────────────────────── */
function initStarButtons() {
  document.addEventListener('click', async e => {
    const btn = e.target.closest('.star-btn');
    if (!btn) return;
    const songId = btn.dataset.songId;
    if (!songId) return;

    const starred = btn.classList.contains('starred');
    btn.classList.toggle('starred', !starred);

    try {
      if (starred) {
        await api('DELETE', `/api/v1/stars/songs/${songId}`);
      } else {
        await api('POST', '/api/v1/stars', { song_id: parseInt(songId, 10) });
      }
    } catch (err) {
      btn.classList.toggle('starred', starred);
      Toast.show('Failed to update star: ' + err.message, 'error');
    }
  });
}

/* ── Play buttons ────────────────────────────────────────────────────────────── */
function collectTracksFromPage() {
  const rows = document.querySelectorAll('[data-song-id]');
  return Array.from(rows).map(row => ({
    id: parseInt(row.dataset.songId, 10),
    title: row.dataset.songTitle || '',
    artist: row.dataset.songArtist || '',
    coverArtId: row.dataset.coverArtId || '',
  }));
}

function initPlayButtons() {
  document.addEventListener('click', e => {
    const btn = e.target.closest('[data-play-song]');
    if (!btn) return;
    const songId = parseInt(btn.dataset.playSong, 10);
    const tracks = collectTracksFromPage();
    const idx = tracks.findIndex(t => t.id === songId);
    if (idx >= 0 && window.player) window.player.playAll(tracks, idx);
  });

  document.addEventListener('click', e => {
    const btn = e.target.closest('[data-queue-song]');
    if (!btn) return;
    const songId = parseInt(btn.dataset.queueSong, 10);
    const title = btn.dataset.songTitle || '';
    const artist = btn.dataset.songArtist || '';
    const coverArtId = btn.dataset.coverArtId || '';
    if (window.player) window.player.addToQueue({ id: songId, title, artist, coverArtId });
  });

  document.addEventListener('click', e => {
    const btn = e.target.closest('[data-play-album]');
    if (!btn) return;
    const albumId = parseInt(btn.dataset.playAlbum, 10);
    if (!albumId) return;
    api('GET', `/api/v1/albums/${albumId}/songs`).then(data => {
      const tracks = (data.data || []).map(s => ({
        id: s.id,
        title: s.title,
        artist: s.artist_name,
        coverArtId: s.cover_art_id || '',
      }));
      if (window.player && tracks.length) window.player.playAll(tracks, 0);
    }).catch(err => Toast.show('Failed to load album: ' + err.message, 'error'));
  });
}

/* ── Debounce ──────────────────────────────────────────────────────────────── */
function debounce(fn, ms) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

/* ── Search ─────────────────────────────────────────────────────────────────── */
function initSearch() {
  const input = document.getElementById('search-input');
  if (!input) return;

  const form = input.closest('form');
  if (form) {
    const debouncedSubmit = debounce(() => form.submit(), 300);
    input.addEventListener('input', debouncedSubmit);
  }
}

/* ── Tabs ─────────────────────────────────────────────────────────────────── */
function initTabs() {
  document.querySelectorAll('.tabs').forEach(tabGroup => {
    const btns = tabGroup.querySelectorAll('.tab-btn');
    const panelIds = Array.from(btns).map(b => b.dataset.tab);

    btns.forEach((btn, i) => {
      btn.addEventListener('click', () => {
        btns.forEach(b => b.classList.remove('active'));
        btn.classList.add('active');

        panelIds.forEach(id => {
          const panel = document.getElementById(id);
          if (panel) panel.classList.remove('active');
        });

        const panel = document.getElementById(panelIds[i]);
        if (panel) panel.classList.add('active');
      });
    });
  });
}

/* ── Settings nav ─────────────────────────────────────────────────────────── */
function initSettingsNav() {
  const navBtns = document.querySelectorAll('.settings-nav-btn');
  if (!navBtns.length) return;

  navBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      navBtns.forEach(b => b.classList.remove('active'));
      btn.classList.add('active');

      document.querySelectorAll('.settings-panel').forEach(p => p.classList.remove('active'));
      const panel = document.getElementById(btn.dataset.panel);
      if (panel) panel.classList.add('active');
    });
  });
}

/* ── Upload zone ─────────────────────────────────────────────────────────── */
function initUploadZone() {
  const zone = document.getElementById('upload-zone');
  if (!zone) return;

  const fileInput = document.getElementById('upload-file-input');
  const fileList = document.getElementById('upload-file-list');
  const librarySelect = document.getElementById('upload-library');

  zone.addEventListener('click', () => fileInput && fileInput.click());

  zone.addEventListener('dragover', e => {
    e.preventDefault();
    zone.classList.add('dragover');
  });

  zone.addEventListener('dragleave', () => zone.classList.remove('dragover'));

  zone.addEventListener('drop', e => {
    e.preventDefault();
    zone.classList.remove('dragover');
    if (e.dataTransfer.files.length) uploadFiles(e.dataTransfer.files);
  });

  if (fileInput) {
    fileInput.addEventListener('change', () => {
      if (fileInput.files.length) uploadFiles(fileInput.files);
    });
  }

  function uploadFiles(files) {
    const libraryId = librarySelect ? librarySelect.value : '';
    Array.from(files).forEach(file => uploadFile(file, libraryId));
  }

  function uploadFile(file, libraryId) {
    const item = document.createElement('div');
    item.className = 'upload-file';
    item.innerHTML = `
      <div class="upload-file-name">${escapeHtml(file.name)}</div>
      <div class="upload-progress"><div class="upload-progress-bar" style="width:0%"></div></div>
    `;
    if (fileList) fileList.appendChild(item);

    const bar = item.querySelector('.upload-progress-bar');
    const formData = new FormData();
    formData.append('file', file);
    if (libraryId) formData.append('library_id', libraryId);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/api/v1/upload');
    const token = localStorage.getItem('cassonic_token');
    if (token) xhr.setRequestHeader('Authorization', 'Bearer ' + token);

    xhr.upload.addEventListener('progress', e => {
      if (e.lengthComputable) bar.style.width = ((e.loaded / e.total) * 100).toFixed(1) + '%';
    });

    xhr.addEventListener('load', () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        bar.style.width = '100%';
        bar.style.background = 'var(--success)';
        Toast.show(file.name + ' uploaded', 'success');
      } else {
        bar.style.background = 'var(--danger)';
        Toast.show('Upload failed: ' + file.name, 'error');
      }
    });

    xhr.addEventListener('error', () => {
      bar.style.background = 'var(--danger)';
      Toast.show('Upload error: ' + file.name, 'error');
    });

    xhr.send(formData);
  }
}

/* ── MusicBrainz lookup ─────────────────────────────────────────────────────── */
function initMusicBrainzLookup() {
  const btn = document.getElementById('mb-lookup-btn');
  if (!btn) return;

  btn.addEventListener('click', async () => {
    const title = document.getElementById('tag-title');
    const artist = document.getElementById('tag-artist');
    if (!title || !artist) return;

    const q = `${artist.value} ${title.value}`.trim();
    if (!q) { Toast.show('Enter artist and title first', 'info'); return; }

    btn.disabled = true;
    try {
      const data = await api('GET', `/api/v1/musicbrainz/search?q=${encodeURIComponent(q)}`);
      if (data.data && data.data.mb_track_id) {
        const d = data.data;
        if (d.mb_track_id) document.getElementById('tag-mb-track-id').value = d.mb_track_id;
        if (d.mb_album_id) document.getElementById('tag-mb-album-id').value = d.mb_album_id;
        if (d.mb_artist_id) document.getElementById('tag-mb-artist-id').value = d.mb_artist_id;
        Toast.show('MusicBrainz IDs populated', 'success');
      } else {
        Toast.show('No MusicBrainz match found', 'info');
      }
    } catch (err) {
      Toast.show('MusicBrainz lookup failed: ' + err.message, 'error');
    } finally {
      btn.disabled = false;
    }
  });
}

/* ── Scan library ────────────────────────────────────────────────────────────── */
function initScanButtons() {
  document.querySelectorAll('[data-scan-library]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const id = btn.dataset.scanLibrary;
      btn.disabled = true;
      try {
        await api('POST', `/api/v1/library/${id}/scan`);
        Toast.show('Library scan started', 'success');
      } catch (err) {
        Toast.show('Scan failed: ' + err.message, 'error');
        btn.disabled = false;
      }
    });
  });
}

/* ── Icecast mount control ────────────────────────────────────────────────── */
function initIcecastControls() {
  document.querySelectorAll('[data-mount-start]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const mountId = btn.dataset.mountStart;
      try {
        await api('POST', `/api/v1/icecast/mounts/${mountId}/start`);
        Toast.show('Stream starting', 'success');
        setTimeout(() => location.reload(), 1500);
      } catch (err) {
        Toast.show('Failed to start: ' + err.message, 'error');
      }
    });
  });

  document.querySelectorAll('[data-mount-stop]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const mountId = btn.dataset.mountStop;
      try {
        await api('POST', `/api/v1/icecast/mounts/${mountId}/stop`);
        Toast.show('Stream stopped', 'info');
        setTimeout(() => location.reload(), 1500);
      } catch (err) {
        Toast.show('Failed to stop: ' + err.message, 'error');
      }
    });
  });
}

/* ── Logout form ─────────────────────────────────────────────────────────────── */
function initLogout() {
  document.querySelectorAll('[data-logout]').forEach(el => {
    el.addEventListener('click', e => {
      e.preventDefault();
      const form = document.createElement('form');
      form.method = 'POST';
      form.action = '/logout';
      document.body.appendChild(form);
      form.submit();
    });
  });
}

/* ── HTML escape ─────────────────────────────────────────────────────────── */
function escapeHtml(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

/* ── Infinite scroll ─────────────────────────────────────────────────────── */
function initInfiniteScroll() {
  const sentinel = document.getElementById('scroll-sentinel');
  if (!sentinel) return;

  const observer = new IntersectionObserver(entries => {
    if (!entries[0].isIntersecting) return;
    const nextUrl = sentinel.dataset.nextUrl;
    if (!nextUrl) return;
    sentinel.dataset.nextUrl = '';

    fetch(nextUrl, { headers: { 'X-Requested-With': 'XMLHttpRequest' } })
      .then(r => r.text())
      .then(html => {
        const parser = new DOMParser();
        const doc = parser.parseFromString(html, 'text/html');
        const newItems = doc.querySelectorAll('[data-grid-item]');
        const newSentinel = doc.getElementById('scroll-sentinel');
        const grid = document.querySelector('[data-grid]');

        if (grid) {
          newItems.forEach(item => grid.appendChild(item));
        }

        if (newSentinel && newSentinel.dataset.nextUrl) {
          sentinel.dataset.nextUrl = newSentinel.dataset.nextUrl;
        }
      })
      .catch(() => {});
  }, { rootMargin: '200px' });

  observer.observe(sentinel);
}

/* ── Init ────────────────────────────────────────────────────────────────────── */
document.addEventListener('DOMContentLoaded', () => {
  ThemeManager.init();

  if (document.getElementById('player-play')) {
    window.player = new CassonicPlayer();
  }

  initNowPlayingSSE();
  initStarButtons();
  initPlayButtons();
  initSearch();
  initTabs();
  initSettingsNav();
  initUploadZone();
  initMusicBrainzLookup();
  initScanButtons();
  initIcecastControls();
  initLogout();
  initInfiniteScroll();

  const navLinks = document.querySelectorAll('.sidebar-nav a, .nav-tab a');
  navLinks.forEach(link => {
    if (link.href === location.href || link.pathname === location.pathname) {
      link.classList.add('active');
    }
  });
});
