// Conduit Dashboard Engine (Vanilla JS, 100% Offline)
(function () {
  'use strict';

  // Application State
  const state = {
    downloads: [],
    filter: 'all',
    search: '',
    isWebSocket: false,
    pollTimer: null,
    loaded: false,
    loadFailed: false,
  };

  // DOM Elements
  const elements = {
    connectionStatus: document.getElementById('connectionStatus'),
    metricActiveCount: document.getElementById('metricActiveCount'),
    metricTotalSpeed: document.getElementById('metricTotalSpeed'),
    metricCompleteCount: document.getElementById('metricCompleteCount'),
    countAll: document.getElementById('countAll'),
    countActive: document.getElementById('countActive'),
    countComplete: document.getElementById('countComplete'),
    countPaused: document.getElementById('countPaused'),
    countError: document.getElementById('countError'),
    searchInput: document.getElementById('searchInput'),
    tabBtns: document.querySelectorAll('.tab-btn'),
    transferTableBody: document.getElementById('transferTableBody'),
    emptyState: document.getElementById('emptyState'),
    emptyTitle: document.getElementById('emptyTitle'),
    emptyDesc: document.getElementById('emptyDesc'),
    emptyActionBtn: document.getElementById('emptyActionBtn'),
    transferTable: document.getElementById('transferTable'),
    queueSummary: document.getElementById('queueSummary'),
    emptyAddBtn: document.getElementById('emptyAddBtn'),
    batchActions: document.querySelector('.batch-actions'),
    // Batch Actions
    pauseAllBtn: document.getElementById('pauseAllBtn'),
    resumeAllBtn: document.getElementById('resumeAllBtn'),
    clearCompletedBtn: document.getElementById('clearCompletedBtn'),
    // Add Download Modal
    addModal: document.getElementById('addModal'),
    openAddModalBtn: document.getElementById('openAddModalBtn'),
    closeAddModalBtn: document.getElementById('closeAddModalBtn'),
    cancelAddBtn: document.getElementById('cancelAddBtn'),
    addDownloadForm: document.getElementById('addDownloadForm'),
    downloadUrl: document.getElementById('downloadUrl'),
    downloadFilename: document.getElementById('downloadFilename'),
    downloadUrlError: document.getElementById('downloadUrlError'),
    submitAddBtn: document.getElementById('submitAddBtn'),
    addDestination: document.getElementById('addDestination'),
    // Connection Banner
    connectionBanner: document.getElementById('connectionBanner'),
    retryConnectionBtn: document.getElementById('retryConnectionBtn'),
    appLayout: document.querySelector('.app-layout'),
    // Speed Limiter Quick Selector
    speedLimiterSelect: document.getElementById('speedLimiterSelect'),
    // Move Folder Modal
    moveModal: document.getElementById('moveModal'),
    closeMoveModalBtn: document.getElementById('closeMoveModalBtn'),
    cancelMoveBtn: document.getElementById('cancelMoveBtn'),
    confirmMoveBtn: document.getElementById('confirmMoveBtn'),
    moveFolderForm: document.getElementById('moveFolderForm'),
    moveTargetName: document.getElementById('moveTargetName'),
    moveTargetDir: document.getElementById('moveTargetDir'),
    // Delete Confirmation Modal
    deleteModal: document.getElementById('deleteModal'),
    closeDeleteModalBtn: document.getElementById('closeDeleteModalBtn'),
    cancelDeleteBtn: document.getElementById('cancelDeleteBtn'),
    confirmDeleteBtn: document.getElementById('confirmDeleteBtn'),
    deleteTargetName: document.getElementById('deleteTargetName'),
    deleteTargetPath: document.getElementById('deleteTargetPath'),
    deleteFileFromDisk: document.getElementById('deleteFileFromDisk'),
    deleteFileHint: document.getElementById('deleteFileHint'),
    deleteNote: document.getElementById('deleteNote'),
    // Clear Finished Modal
    clearModal: document.getElementById('clearModal'),
    closeClearModalBtn: document.getElementById('closeClearModalBtn'),
    cancelClearBtn: document.getElementById('cancelClearBtn'),
    confirmClearBtn: document.getElementById('confirmClearBtn'),
    clearSummary: document.getElementById('clearSummary'),
    clearCompletedOnly: document.getElementById('clearCompletedOnly'),
    // Settings Drawer
    settingsDrawer: document.getElementById('settingsDrawer'),
    openSettingsBtn: document.getElementById('openSettingsBtn'),
    closeSettingsBtn: document.getElementById('closeSettingsBtn'),
    settingsForm: document.getElementById('settingsForm'),
    cfgDownloadDir: document.getElementById('cfgDownloadDir'),
    cfgSpeedLimit: document.getElementById('cfgSpeedLimit'),
    cfgMaxConnections: document.getElementById('cfgMaxConnections'),
    maxConnValue: document.getElementById('maxConnValue'),
    cfgMaxConcurrent: document.getElementById('cfgMaxConcurrent'),
    cfgAutoStart: document.getElementById('cfgAutoStart'),
    cfgShowOnStart: document.getElementById('cfgShowOnStart'),
    cfgShowOnComplete: document.getElementById('cfgShowOnComplete'),
    cfgShowOnError: document.getElementById('cfgShowOnError'),
    cfgAutoCheckUpdates: document.getElementById('cfgAutoCheckUpdates'),
    cfgAutoInstallUpdates: document.getElementById('cfgAutoInstallUpdates'),
    updateSummary: document.getElementById('updateSummary'),
    updateActionBtn: document.getElementById('updateActionBtn'),
    updateDetail: document.getElementById('updateDetail'),
    appVersion: document.getElementById('appVersion'),
    extensionGuideURL: document.getElementById('extensionGuideURL'),
    openExtensionGuideBtn: document.getElementById('openExtensionGuideBtn'),
    emptyExtensionLink: document.getElementById('emptyExtensionLink'),
    toastContainer: document.getElementById('toastContainer'),
  };

  const shellMode = new URLSearchParams(window.location.search).get('shell') === 'app'
    || window.sessionStorage.getItem('conduitShell') === '1';
  if (new URLSearchParams(window.location.search).get('shell') === 'app') {
    window.sessionStorage.setItem('conduitShell', '1');
  }

  // State caches & focus management
  const renderedRows = new Map();
  let pendingDeleteGid = null;
  let pendingMoveGid = null;
  let lastActiveElement = null;
  let isAddingDownload = false;

  // ---- Dialog layer: focus trap, background inert, one top layer ----
  const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
  const openLayers = [];

  // Cleanup that must run whenever a layer closes, by any route.
  const layerCleanup = {
    addModal: function () {
      if (elements.addDownloadForm) elements.addDownloadForm.reset();
      clearUrlError();
    },
    deleteModal: function () {
      pendingDeleteGid = null;
    },
    clearModal: function () {
      if (elements.confirmClearBtn) elements.confirmClearBtn.disabled = false;
    },
    moveModal: function () {
      pendingMoveGid = null;
    },
  };

  function focusablesIn(container) {
    return Array.from(container.querySelectorAll(FOCUSABLE))
      .filter(el => el.offsetParent !== null || el === document.activeElement);
  }

  function openLayer(layer, focusTarget) {
    if (!layer) return;
    if (!openLayers.includes(layer)) {
      openLayers.push(layer);
    }
    layer.classList.remove('hidden');
    // Focus the layer before the background becomes inert or hidden to AT.
    const target = focusTarget || focusablesIn(layer)[0] || layer;
    if (target && typeof target.focus === 'function') {
      target.focus();
    }
    setBackgroundInert(true);
  }

  function closeLayer(layer) {
    if (!layer) return;
    layer.classList.add('hidden');
    const cleanup = layerCleanup[layer.id];
    if (cleanup) cleanup();
    const idx = openLayers.indexOf(layer);
    if (idx !== -1) openLayers.splice(idx, 1);
    if (openLayers.length === 0) {
      setBackgroundInert(false);
      if (lastActiveElement && typeof lastActiveElement.focus === 'function') {
        lastActiveElement.focus();
      }
      lastActiveElement = null;
    } else {
      const top = openLayers[openLayers.length - 1];
      const target = focusablesIn(top)[0];
      if (target) target.focus();
    }
  }

  function closeTopLayer() {
    if (openLayers.length === 0) return false;
    closeLayer(openLayers[openLayers.length - 1]);
    return true;
  }

  function setBackgroundInert(isInert) {
    const shell = document.querySelector('.app-layout');
    if (!shell) return;
    if (isInert) {
      shell.setAttribute('inert', '');
      shell.setAttribute('aria-hidden', 'true');
    } else {
      shell.removeAttribute('inert');
      shell.removeAttribute('aria-hidden');
    }
  }

  function trapTab(e, layer) {
    if (e.key !== 'Tab') return;
    const items = focusablesIn(layer);
    if (items.length === 0) {
      e.preventDefault();
      return;
    }
    const first = items[0];
    const last = items[items.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  // Every layer gets Tab trapping, and a backdrop click closes only itself.
  ['addModal', 'deleteModal', 'clearModal', 'moveModal', 'settingsDrawer'].forEach(id => {
    const layer = document.getElementById(id);
    if (!layer) return;
    layer.addEventListener('keydown', (e) => trapTab(e, layer));
    layer.addEventListener('click', (e) => {
      if (e.target === layer) {
        closeLayer(layer);
      }
    });
  });

  // 1. Connection Management (WebSocket with Exponential Backoff Fallback)
  let ws = null;
  let wsReconnectTimer = null;
  let reconnectDelay = 1000;
  const maxReconnectDelay = 10000;

  function initConnection() {
    connectWebSocket();
  }

  function connectWebSocket() {
    if (wsReconnectTimer) {
      clearTimeout(wsReconnectTimer);
      wsReconnectTimer = null;
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/ws`;

    let socket;
    try {
      socket = new WebSocket(wsUrl);
      ws = socket;
    } catch (e) {
      ws = null;
      handleDisconnection();
      return;
    }

    // An old socket's close event can arrive after a reconnect created a new
    // one. Ignore handlers that no longer belong to the current socket.
    socket.onopen = function () {
      if (ws !== socket) return;
      if (wsReconnectTimer) {
        clearTimeout(wsReconnectTimer);
        wsReconnectTimer = null;
      }
      state.isWebSocket = true;
      reconnectDelay = 1000;
      if (shellMode) {
        refreshEngineStatus();
      } else {
        setConnectionStatus('online', 'Connected');
      }
      stopPolling();
    };

    socket.onmessage = function (event) {
      if (ws !== socket) return;
      let msg;
      try {
        msg = JSON.parse(event.data);
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
        return;
      }
      if (msg.type === 'downloads') {
        updateDownloads(msg.downloads);
      } else if (msg.type === 'event' && Array.isArray(msg.events)) {
        // The daemon just brought this window forward; say why.
        announceDownloadEvents(msg.events);
      } else if (msg.type === 'update' && msg.update) {
        // The daemon pushes the update status whenever it changes.
        onUpdateStatus(msg.update);
      } else if (msg.type === 'config' && msg.config) {
        // Another window or tab changed settings (or this one did).
        applySettings(msg.config, { keepOpenForm: true });
      }
    };

    socket.onclose = function () {
      if (ws !== socket) return;
      ws = null;
      handleDisconnection();
    };

    socket.onerror = function () {
      if (ws !== socket) return;
      handleDisconnection();
    };
  }

  function handleDisconnection() {
    // A socket that fails before it opens never set isWebSocket. The HTTP
    // fallback must start anyway, in shell mode too, so the queue is never
    // empty for no reason.
    state.isWebSocket = false;
    if (shellMode) {
      startPolling();
      refreshEngineStatus();
    } else {
      setConnectionStatus('reconnecting', 'Reconnecting...');
      startPolling();
    }
    if (!wsReconnectTimer) {
      const jitter = Math.random() * 300;
      const delay = Math.min(reconnectDelay + jitter, maxReconnectDelay);
      wsReconnectTimer = setTimeout(() => {
        wsReconnectTimer = null;
        connectWebSocket();
      }, delay);
      reconnectDelay = Math.min(reconnectDelay * 1.5, maxReconnectDelay);
    }
  }

  function startPolling() {
    if (state.pollTimer) return;
    fetchDownloads();
    state.pollTimer = setInterval(fetchDownloads, 1000);
  }

  function stopPolling() {
    if (state.pollTimer) {
      clearInterval(state.pollTimer);
      state.pollTimer = null;
    }
  }

  async function fetchDownloads() {
    try {
      const res = await fetch('/api/downloads');
      if (res.ok) {
        const items = await res.json();
        updateDownloads(items);
        if (!state.isWebSocket && !shellMode) {
          setConnectionStatus('online', 'Connected (HTTP)');
        }
        if (shellMode) {
          refreshEngineStatus();
        }
      } else if (shellMode) {
        markLoadFailed();
        refreshEngineStatus();
      } else {
        markLoadFailed();
        setConnectionStatus('offline', 'Disconnected');
      }
    } catch (e) {
      markLoadFailed();
      if (shellMode) {
        refreshEngineStatus();
      } else {
        setConnectionStatus('offline', 'Offline');
      }
    }
  }

  async function refreshEngineStatus() {
    if (!shellMode || !elements.connectionStatus) return;
    try {
      const res = await fetch('/api/health');
      if (!res.ok) {
        setConnectionStatus('offline', 'Engine offline');
        return;
      }
      const data = await res.json();
      if (data.engine === 'ready') {
        setConnectionStatus('online', 'Ready');
      } else if (data.engine === 'starting') {
        setConnectionStatus('reconnecting', 'Starting');
      } else if (data.engine === 'missing') {
        // The engine is not installed, so retrying will not help until it is.
        setConnectionStatus('offline', 'Engine not installed');
        if (elements.connectionStatus) elements.connectionStatus.title = data.engineHint || '';
      } else {
        setConnectionStatus('offline', 'Engine offline');
      }
    } catch (e) {
      setConnectionStatus('offline', 'Engine offline');
    }
  }

  async function openExtensionGuide() {
    try {
      const res = await fetch('/api/extension/open-guide', { method: 'POST' });
      if (!res.ok) {
        showToast('Could not open a browser. Use the extension guide address in Settings.', 'error');
      }
    } catch (e) {
      showToast('Could not open a browser. Use the extension guide address in Settings.', 'error');
    }
  }

  async function redirectIfSetupNeeded() {
    try {
      const res = await fetch('/api/config');
      if (!res.ok) return false;
      const cfg = await res.json();
      if (cfg.setupComplete === false) {
        const query = shellMode ? '?shell=app' : '';
        window.location.replace('/setup' + query);
        return true;
      }
    } catch (e) {
      return false;
    }
    return false;
  }

  function setConnectionStatus(status, text) {
    if (!elements.connectionStatus) return;
    elements.connectionStatus.className = `status-indicator ${status}`;
    const textEl = elements.connectionStatus.querySelector('.status-text');
    if (textEl) textEl.textContent = text;

    if (elements.connectionBanner) {
      const isDown = status === 'offline';
      elements.connectionBanner.classList.toggle('hidden', !isDown);
    }
  }

  // 2. State & UI Rendering
  function updateDownloads(items) {
    state.downloads = (Array.isArray(items) ? items : []).filter(d => d.status !== 'removed');
    state.loaded = true;
    state.loadFailed = false;
    updateMetrics();
    renderTable();
  }

  function markLoadFailed() {
    if (state.loaded) return;
    state.loadFailed = true;
    renderTable();
  }

  function updateMetrics() {
    let activeCount = 0;
    let completeCount = 0;
    let pausedCount = 0;
    let errorCount = 0;
    let totalSpeed = 0;

    state.downloads.forEach(d => {
      if (d.status === 'active') {
        activeCount++;
        totalSpeed += d.downloadSpeed || 0;
      } else if (d.status === 'complete') {
        completeCount++;
      } else if (d.status === 'paused') {
        pausedCount++;
      } else if (d.status === 'error') {
        errorCount++;
      }
    });

    if (elements.metricActiveCount) elements.metricActiveCount.textContent = activeCount;
    if (elements.metricTotalSpeed) {
      elements.metricTotalSpeed.textContent = formatSpeed(totalSpeed);
      elements.metricTotalSpeed.classList.toggle('active-speed', totalSpeed > 0);
    }
    if (elements.metricCompleteCount) elements.metricCompleteCount.textContent = completeCount;

    if (elements.countAll) elements.countAll.textContent = state.downloads.length;
    if (elements.countActive) elements.countActive.textContent = activeCount;
    if (elements.countComplete) elements.countComplete.textContent = completeCount;
    if (elements.countPaused) elements.countPaused.textContent = pausedCount;
    if (elements.countError) elements.countError.textContent = errorCount;

    // A batch button that cannot act stays disabled, so no click lands on nothing.
    const finishedTotal = completeCount + errorCount;
    if (elements.pauseAllBtn) {
      elements.pauseAllBtn.disabled = activeCount === 0;
      elements.pauseAllBtn.title = activeCount === 0
        ? 'No active downloads to pause'
        : `Pause all ${activeCount} active ${activeCount === 1 ? 'transfer' : 'transfers'}`;
    }
    if (elements.resumeAllBtn) {
      elements.resumeAllBtn.disabled = pausedCount === 0;
      elements.resumeAllBtn.title = pausedCount === 0
        ? 'No paused downloads to resume'
        : `Resume all ${pausedCount} paused ${pausedCount === 1 ? 'transfer' : 'transfers'}`;
    }
    if (elements.clearCompletedBtn) {
      elements.clearCompletedBtn.disabled = finishedTotal === 0;
      elements.clearCompletedBtn.title = finishedTotal === 0
        ? 'No finished downloads to clear'
        : `Remove ${finishedTotal} finished ${finishedTotal === 1 ? 'download' : 'downloads'} from the list`;
    }
  }

  function renderTable() {
    if (!state.loaded) {
      elements.transferTableBody.innerHTML = '';
      renderedRows.clear();
      if (elements.transferTable) elements.transferTable.classList.add('hidden');
      if (elements.emptyState) {
        elements.emptyState.classList.remove('hidden');
        if (elements.emptyTitle) {
          elements.emptyTitle.textContent = state.loadFailed ? 'Waiting for Conduit' : 'Loading downloads';
        }
        if (elements.emptyDesc) {
          elements.emptyDesc.textContent = state.loadFailed
            ? 'The queue is not available now. Conduit retries automatically.'
            : 'Reading the queue from Conduit.';
        }
        if (elements.emptyAddBtn) elements.emptyAddBtn.classList.add('hidden');
        if (elements.emptyExtensionLink) elements.emptyExtensionLink.classList.add('hidden');
        if (elements.emptyActionBtn) elements.emptyActionBtn.classList.add('hidden');
      }
      updateQueueSummary(0);
      return;
    }

    const query = state.search.trim().toLowerCase();
    const filtered = state.downloads.filter(item => {
      // Status filter
      if (state.filter === 'active' && item.status !== 'active') return false;
      if (state.filter === 'complete' && item.status !== 'complete') return false;
      if (state.filter === 'paused' && item.status !== 'paused') return false;
      if (state.filter === 'error' && item.status !== 'error') return false;

      // Search matches the visible name and the visible folder.
      if (query) {
        const haystack = `${item.filename || ''} ${item.dir || ''}`.toLowerCase();
        if (!haystack.includes(query)) return false;
      }
      return true;
    });

    updateQueueSummary(filtered.length);

    if (filtered.length === 0) {
      elements.transferTableBody.innerHTML = '';
      renderedRows.clear();
      if (elements.transferTable) elements.transferTable.classList.add('hidden');
      if (elements.emptyState) {
        elements.emptyState.classList.remove('hidden');
        const noDownloads = state.downloads.length === 0;
        const hasQuery = query.length > 0;
        const hasFilter = state.filter !== 'all';
        if (elements.emptyTitle) {
          elements.emptyTitle.textContent = noDownloads
            ? 'No downloads yet'
            : 'No matching downloads';
        }
        if (elements.emptyDesc) {
          elements.emptyDesc.textContent = noDownloads
            ? 'Add a link to start. Finished files go to your download folder.'
            : hasQuery
              ? `No file name or folder contains "${state.search.trim()}".`
              : `There are no ${filterLabel(state.filter)} downloads.`;
        }
        if (elements.emptyAddBtn) elements.emptyAddBtn.classList.toggle('hidden', !noDownloads);
        if (elements.emptyExtensionLink) elements.emptyExtensionLink.classList.toggle('hidden', !noDownloads);
        if (elements.emptyActionBtn) elements.emptyActionBtn.classList.toggle('hidden', noDownloads || (!hasQuery && !hasFilter));
      }
      return;
    }

    if (elements.emptyState) elements.emptyState.classList.add('hidden');
    if (elements.transferTable) elements.transferTable.classList.remove('hidden');

    const currentGids = new Set();

    filtered.forEach((item, index) => {
      currentGids.add(item.gid);

      const percent = item.totalLength > 0
        ? Math.min(100, Math.round((item.completedLength / item.totalLength) * 100))
        : (item.status === 'complete' ? 100 : 0);

      const speedStr = item.status === 'active' ? formatSpeed(item.downloadSpeed) : '--';
      const etaStr = item.status === 'active' ? formatETA(item.totalLength, item.completedLength, item.downloadSpeed) : '--';
      const badgeClass = getBadgeClass(item.status);
      const conns = Number(item.connections) || 16;
      const connsBadgeHtml = item.status === 'active'
        ? `<span class="conns-badge" title="${conns} parallel connections">${conns} connections</span>`
        : '';
      const errorHtml = (item.status === 'error' && item.errorMessage)
        ? `<span class="file-error"><svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>${escapeHtml(item.errorMessage)}</span>`
        : '';

      let row = renderedRows.get(item.gid);

      if (row) {
        // Reconcile in place without DOM re-creation
        const fileNameEl = row.querySelector('.file-name');
        if (fileNameEl && fileNameEl.textContent !== item.filename) {
          fileNameEl.textContent = item.filename;
          fileNameEl.title = item.filename;
        }

        const filePathEl = row.querySelector('.file-path');
        if (filePathEl && filePathEl.textContent !== (item.dir || '')) {
          filePathEl.textContent = item.dir || '';
        }

        const errorEl = row.querySelector('.file-error-container');
        if (errorEl && row._lastError !== (item.errorMessage || '')) {
          errorEl.innerHTML = errorHtml;
          row._lastError = item.errorMessage || '';
        }

        const barWrapper = row.querySelector('.progress-bar-bg');
        if (barWrapper) {
          barWrapper.setAttribute('aria-valuenow', percent);
        }

        const fill = row.querySelector('.progress-bar-fill');
        if (fill) {
          fill.style.transform = `scaleX(${percent / 100})`;
          const expectedFillClass = `progress-bar-fill ${item.status}`;
          if (fill.className !== expectedFillClass) {
            fill.className = expectedFillClass;
          }
        }

        const percentEl = row.querySelector('.progress-percent');
        if (percentEl && percentEl.textContent !== `${percent}%`) {
          percentEl.textContent = `${percent}%`;
        }

        const connsWrap = row.querySelector('.conns-badge-wrap');
        if (connsWrap && row._lastConnsBadge !== connsBadgeHtml) {
          connsWrap.innerHTML = connsBadgeHtml;
          row._lastConnsBadge = connsBadgeHtml;
        }

        const bytesEl = row.querySelector('.progress-bytes');
        const bytesText = `${formatBytes(item.completedLength)} / ${formatBytes(item.totalLength)}`;
        if (bytesEl && bytesEl.textContent !== bytesText) {
          bytesEl.textContent = bytesText;
        }

        const speedCell = row.querySelector('.col-speed');
        if (speedCell) {
          if (speedCell.textContent !== speedStr) {
            speedCell.textContent = speedStr;
          }
          if (item.status === 'active') {
            speedCell.classList.add('active');
          } else {
            speedCell.classList.remove('active');
          }
        }

        const etaCell = row.querySelector('.col-eta');
        if (etaCell && etaCell.textContent !== etaStr) {
          etaCell.textContent = etaStr;
        }

        const badge = row.querySelector('.col-status .badge');
        if (badge) {
          const expectedBadgeClass = `badge ${badgeClass}`;
          if (badge.className !== expectedBadgeClass) {
            badge.className = expectedBadgeClass;
          }
          const label = statusLabel(item.status);
          if (badge.textContent !== label) {
            badge.textContent = label;
          }
        }

        if (row._lastStatus !== item.status) {
          const actionGroup = row.querySelector('.action-btn-group');
          if (actionGroup) {
            actionGroup.innerHTML = renderActionButtons(item);
          }
          row._lastStatus = item.status;
        }
      } else {
        // Create new row
        row = document.createElement('tr');
        row.dataset.gid = item.gid;
        row._lastStatus = item.status;
        row._lastError = item.errorMessage || '';
        row._lastConnsBadge = connsBadgeHtml;

        row.innerHTML = `
          <td class="col-file">
            <div class="file-info">
              <span class="file-name" title="${escapeHtml(item.filename)}">${escapeHtml(item.filename)}</span>
              <span class="file-path" title="${escapeHtml(item.dir || '')}">${escapeHtml(item.dir || '')}</span>
              <div class="file-error-container">${errorHtml}</div>
            </div>
          </td>
          <td class="col-progress">
            <div class="progress-wrapper">
              <div class="progress-bar-bg" role="progressbar" aria-valuenow="${percent}" aria-valuemin="0" aria-valuemax="100" aria-label="Download progress for ${escapeHtml(item.filename)}">
                <div class="progress-bar-fill ${escapeHtml(item.status)}" style="transform: scaleX(${percent / 100});"></div>
              </div>
              <div class="progress-stats">
                <span class="progress-percent-wrap">
                  <span class="progress-percent">${percent}%</span>
                  <span class="conns-badge-wrap">${connsBadgeHtml}</span>
                </span>
                <span class="progress-bytes">${formatBytes(item.completedLength)} / ${formatBytes(item.totalLength)}</span>
              </div>
            </div>
          </td>
          <td class="col-speed ${item.status === 'active' ? 'active' : ''}">${speedStr}</td>
          <td class="col-eta">${etaStr}</td>
          <td class="col-status">
            <span class="badge ${badgeClass}">${escapeHtml(statusLabel(item.status))}</span>
          </td>
          <td class="col-actions">
            <div class="action-btn-group">
              ${renderActionButtons(item)}
            </div>
          </td>
        `;

        renderedRows.set(item.gid, row);
      }

      // Reorder if necessary
      if (elements.transferTableBody.children[index] !== row) {
        elements.transferTableBody.insertBefore(row, elements.transferTableBody.children[index] || null);
      }
    });

    // Remove defunct rows
    for (const [gid, row] of renderedRows.entries()) {
      if (!currentGids.has(gid)) {
        row.remove();
        renderedRows.delete(gid);
      }
    }
  }

  function filterLabel(filter) {
    const labels = { active: 'active', complete: 'completed', paused: 'paused', error: 'failed' };
    return labels[filter] || '';
  }

  function updateQueueSummary(visibleCount) {
    if (!elements.queueSummary) return;
    const total = state.downloads.length;
    let text;
    if (!state.loaded) {
      text = state.loadFailed ? 'Queue unavailable' : 'Loading...';
    } else if (total > 0) {
      if (visibleCount !== total) {
        text = `${visibleCount} of ${total} ${total === 1 ? 'download' : 'downloads'}`;
      } else {
        const active = state.downloads.filter(d => d.status === 'active').length;
        text = `${total} ${total === 1 ? 'download' : 'downloads'}`;
        if (active > 0) text += ' \u00B7 ' + active + ' active';
      }
    } else {
      text = 'No downloads';
    }
    if (elements.queueSummary.textContent !== text) {
      elements.queueSummary.textContent = text;
    }
    // Batch controls act on existing downloads only. Hide them on first run.
    if (elements.batchActions) {
      elements.batchActions.classList.toggle('hidden', total === 0);
    }
  }

  function renderActionButtons(item) {
    if (item.status === 'removed') {
      return '';
    }
    const filenameLabel = escapeHtml(item.filename || 'download');
    let mainAction = '';
    if (item.status === 'active') {
      mainAction = `
        <button class="btn-icon-only action-pause" data-gid="${escapeHtml(item.gid)}" title="Pause download" aria-label="Pause ${filenameLabel}">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <rect x="6" y="4" width="4" height="16"></rect>
            <rect x="14" y="4" width="4" height="16"></rect>
          </svg>
        </button>
      `;
    } else if (item.status === 'paused' || item.status === 'waiting') {
      mainAction = `
        <button class="btn-icon-only action-resume" data-gid="${escapeHtml(item.gid)}" title="Resume download" aria-label="Resume ${filenameLabel}">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <polygon points="5 3 19 12 5 21 5 3"></polygon>
          </svg>
        </button>
      `;
    } else if (item.status === 'error') {
      mainAction = `
        <button class="btn-icon-only action-retry" data-gid="${escapeHtml(item.gid)}" title="Retry download" aria-label="Retry ${filenameLabel}">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <polyline points="23 4 23 10 17 10"></polyline>
            <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"></path>
          </svg>
        </button>
      `;
    }

    let reorderActions = '';
    if (item.status === 'waiting') {
      reorderActions = `
        <button class="btn-icon-only action-move-up" data-gid="${escapeHtml(item.gid)}" title="Move up in queue" aria-label="Move ${filenameLabel} up in queue">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <polyline points="18 15 12 9 6 15"></polyline>
          </svg>
        </button>
        <button class="btn-icon-only action-move-down" data-gid="${escapeHtml(item.gid)}" title="Move down in queue" aria-label="Move ${filenameLabel} down in queue">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </button>
      `;
    }

    let moveFolderAction = '';
    if (item.status !== 'complete') {
      moveFolderAction = `
        <button class="btn-icon-only action-move-dir" data-gid="${escapeHtml(item.gid)}" title="Change download folder" aria-label="Change download folder for ${filenameLabel}">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
            <polyline points="12 11 12 17"></polyline>
            <polyline points="9 14 12 11 15 14"></polyline>
          </svg>
        </button>
      `;
    }

    // A waiting row has no file to open yet, so it offers no Open action.
    const openAction = item.status === 'waiting' ? '' : `
      <button class="btn-icon-only action-open" data-gid="${escapeHtml(item.gid)}" title="Open in file manager" aria-label="Open ${filenameLabel} in file manager">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
        </svg>
      </button>
    `;

    const openFileAction = item.status !== 'complete' ? '' : `
      <button class="btn-icon-only action-open-file" data-gid="${escapeHtml(item.gid)}" title="Open file" aria-label="Open file ${filenameLabel}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
          <polyline points="14 2 14 8 20 8"></polyline>
        </svg>
      </button>
    `;

    const copyLinkAction = !item.url ? '' : `
      <button class="btn-icon-only action-copy-link" data-gid="${escapeHtml(item.gid)}" title="Copy link" aria-label="Copy link for ${filenameLabel}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
          <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
        </svg>
      </button>
    `;

    return `
      ${reorderActions}
      ${mainAction}
      ${moveFolderAction}
      ${openFileAction}
      ${openAction}
      ${copyLinkAction}
      <button class="btn-icon-only action-delete" data-gid="${escapeHtml(item.gid)}" title="Remove download" aria-label="Remove ${filenameLabel}">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
          <polyline points="3 6 5 6 21 6"></polyline>
          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
        </svg>
      </button>
    `;
  }

  function getBadgeClass(status) {
    switch (status) {
      case 'active': return 'badge-active';
      case 'complete': return 'badge-complete';
      case 'paused': return 'badge-paused';
      case 'error': return 'badge-error';
      default: return 'badge-waiting';
    }
  }

  // The badge must use the same word as the filter tab beside it.
  const STATUS_LABELS = {
    active: 'Active',
    waiting: 'Waiting',
    paused: 'Paused',
    complete: 'Completed',
    error: 'Failed',
    removed: 'Removed',
  };

  function statusLabel(status) {
    return STATUS_LABELS[status] || status;
  }

  // 3. User Actions & Handlers
  async function performAction(gid, action, triggerBtn) {
    if (triggerBtn) triggerBtn.disabled = true;
    try {
      const res = await fetch(`/api/downloads/${encodeURIComponent(gid)}/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: action })
      });
      if (res.ok) {
        const actionLabels = {
          pause: 'Download paused.',
          resume: 'Download resumed.',
          retry: 'Download restarted.',
          open_folder: 'Opened the download folder.',
          open_file: 'Opening the file.',
        };
        showToast(actionLabels[action] || 'Action complete.', 'success');
        if (!state.isWebSocket) fetchDownloads();
      } else {
        const err = await res.json().catch(() => ({}));
        showToast(err.error || 'The engine could not complete this action.', 'error');
      }
    } catch (e) {
      console.error('Download action failed:', e);
      showToast('Could not reach Conduit. Check that the app is running.', 'error');
    } finally {
      if (triggerBtn) triggerBtn.disabled = false;
    }
  }

  async function performDeleteAction(gid, deleteFile, triggerBtn) {
    if (triggerBtn) triggerBtn.disabled = true;
    try {
      const res = await fetch(`/api/downloads/${encodeURIComponent(gid)}/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: 'remove', deleteFile: deleteFile })
      });
      if (res.ok) {
        showToast(deleteFile ? 'Download and file deleted.' : 'Download removed from the list.', 'success');
        // Optimistically remove from state so the row disappears immediately
        state.downloads = state.downloads.filter(d => d.gid !== gid);
        updateMetrics();
        renderTable();
        if (!state.isWebSocket) fetchDownloads();
      } else {
        const err = await res.json().catch(() => ({}));
        showToast(err.error || 'Could not remove this download.', 'error');
      }
    } catch (e) {
      console.error('Remove failed:', e);
      showToast('Could not reach Conduit. The download was not removed.', 'error');
    } finally {
      if (triggerBtn) triggerBtn.disabled = false;
    }
  }

  // Delete Confirmation Modal Handling
  function updateDeleteConfirmLabel() {
    if (!elements.confirmDeleteBtn) return;
    const deleteFile = elements.deleteFileFromDisk ? elements.deleteFileFromDisk.checked : false;
    elements.confirmDeleteBtn.textContent = deleteFile ? 'Delete file and remove' : 'Remove from list';
    if (elements.deleteFileHint) {
      elements.deleteFileHint.textContent = deleteFile
        ? 'The file is deleted from disk. Conduit cannot restore it.'
        : 'The file stays on disk. You can add it again from the same link.';
    }
    if (elements.deleteNote) {
      elements.deleteNote.textContent = deleteFile
        ? 'This deletes the file. You cannot undo this.'
        : 'Conduit removes only this entry. The file is not touched.';
    }
  }

  function openDeleteModal(gid) {
    lastActiveElement = document.activeElement;
    const item = state.downloads.find(d => d.gid === gid);
    pendingDeleteGid = gid;
    if (elements.deleteTargetName) {
      elements.deleteTargetName.textContent = item ? (item.filename || gid) : gid;
    }
    if (elements.deleteTargetPath) {
      elements.deleteTargetPath.textContent = item && item.dir ? item.dir : '';
    }
    if (elements.deleteFileFromDisk) {
      elements.deleteFileFromDisk.checked = false;
    }
    updateDeleteConfirmLabel();
    openLayer(elements.deleteModal, elements.cancelDeleteBtn);
  }

  function closeDeleteModal() {
    closeLayer(elements.deleteModal);
  }

  if (elements.deleteFileFromDisk) {
    elements.deleteFileFromDisk.addEventListener('change', updateDeleteConfirmLabel);
  }

  if (elements.closeDeleteModalBtn) {
    elements.closeDeleteModalBtn.addEventListener('click', closeDeleteModal);
  }
  if (elements.cancelDeleteBtn) {
    elements.cancelDeleteBtn.addEventListener('click', closeDeleteModal);
  }
  if (elements.confirmDeleteBtn) {
    elements.confirmDeleteBtn.addEventListener('click', async () => {
      if (!pendingDeleteGid) return;
      const gid = pendingDeleteGid;
      const deleteFile = elements.deleteFileFromDisk ? elements.deleteFileFromDisk.checked : false;
      const confirmBtn = elements.confirmDeleteBtn;
      closeDeleteModal();
      await performDeleteAction(gid, deleteFile, confirmBtn);
    });
  }

  // Move Folder Modal Handling
  function openMoveModal(gid) {
    lastActiveElement = document.activeElement;
    const item = state.downloads.find(d => d.gid === gid);
    pendingMoveGid = gid;
    if (elements.moveTargetName) {
      elements.moveTargetName.textContent = item ? (item.filename || gid) : gid;
    }
    if (elements.moveTargetDir) {
      elements.moveTargetDir.value = item ? (item.dir || '') : '';
    }
    openLayer(elements.moveModal, elements.moveTargetDir);
  }

  function closeMoveModal() {
    closeLayer(elements.moveModal);
  }

  if (elements.closeMoveModalBtn) {
    elements.closeMoveModalBtn.addEventListener('click', closeMoveModal);
  }
  if (elements.cancelMoveBtn) {
    elements.cancelMoveBtn.addEventListener('click', closeMoveModal);
  }
  if (elements.moveFolderForm) {
    elements.moveFolderForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (!pendingMoveGid) return;
      const gid = pendingMoveGid;
      const newDir = elements.moveTargetDir ? elements.moveTargetDir.value.trim() : '';
      if (!newDir) {
        if (elements.moveTargetDir) elements.moveTargetDir.focus();
        showToast('Enter a new folder path.', 'error');
        return;
      }
      closeMoveModal();
      try {
        const res = await fetch(`/api/downloads/${encodeURIComponent(gid)}/action`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ action: 'change_dir', newDir: newDir })
        });
        if (res.ok) {
          showToast('Folder changed. The download resumes there.', 'success');
          if (!state.isWebSocket) fetchDownloads();
        } else {
          const err = await res.json().catch(() => ({}));
          showToast(err.error || 'Could not change the folder.', 'error');
        }
      } catch (err) {
        console.error('Folder change failed:', err);
        showToast('Could not reach Conduit. The folder did not change.', 'error');
      }
    });
  }

  async function copyDownloadLink(gid) {
    const item = state.downloads.find(d => d.gid === gid);
    if (!item || !item.url) {
      showToast('This download has no link to copy.', 'error');
      return;
    }
    try {
      await navigator.clipboard.writeText(item.url);
      showToast('Link copied.', 'success');
    } catch (e) {
      showToast('Could not copy the link. Select it from the browser instead.', 'error');
    }
  }

  // Table Event Delegation (Pause, Resume, Retry, Delete, Open Folder, Move Folder, Reorder)
  if (elements.transferTableBody) {
    elements.transferTableBody.addEventListener('click', function (e) {
      const btn = e.target.closest('button');
      if (!btn) return;

      const gid = btn.dataset.gid;
      if (!gid) return;

      if (btn.classList.contains('action-pause')) {
        performAction(gid, 'pause', btn);
      } else if (btn.classList.contains('action-resume')) {
        performAction(gid, 'resume', btn);
      } else if (btn.classList.contains('action-retry')) {
        performAction(gid, 'retry', btn);
      } else if (btn.classList.contains('action-delete')) {
        openDeleteModal(gid);
      } else if (btn.classList.contains('action-open')) {
        performAction(gid, 'open_folder', btn);
      } else if (btn.classList.contains('action-open-file')) {
        performAction(gid, 'open_file', btn);
      } else if (btn.classList.contains('action-copy-link')) {
        copyDownloadLink(gid);
      } else if (btn.classList.contains('action-move-dir')) {
        openMoveModal(gid);
      } else if (btn.classList.contains('action-move-up')) {
        performAction(gid, 'move_up', btn);
      } else if (btn.classList.contains('action-move-down')) {
        performAction(gid, 'move_down', btn);
      }
    });
  }

  // Batch Action Controls
  async function performBatchAction(downloads, action) {
    const results = await Promise.all(downloads.map(d =>
      fetch(`/api/downloads/${encodeURIComponent(d.gid)}/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: action })
      }).then(res => res.ok).catch(() => false)
    ));
    return results.filter(Boolean).length;
  }

  function batchResultToast(action, total, succeeded) {
    const done = action === 'pause' ? 'Paused' : 'Resumed';
    const verb = action === 'pause' ? 'pause' : 'resume';
    const noun = total === 1 ? 'download' : 'downloads';
    if (succeeded === total) {
      showToast(`${done} ${total} ${noun}.`, 'success');
    } else {
      showToast(
        `${done} ${succeeded} of ${total} ${noun}. ${total - succeeded} could not ${verb}.`,
        'error'
      );
    }
  }

  if (elements.pauseAllBtn) {
    elements.pauseAllBtn.addEventListener('click', async () => {
      const activeDownloads = state.downloads.filter(d => d.status === 'active');
      if (activeDownloads.length === 0) {
        return;
      }
      elements.pauseAllBtn.disabled = true;
      try {
        const succeeded = await performBatchAction(activeDownloads, 'pause');
        batchResultToast('pause', activeDownloads.length, succeeded);
        if (!state.isWebSocket) fetchDownloads();
      } catch (e) {
        showToast('Could not pause every download. Some transfers may still run.', 'error');
      } finally {
        elements.pauseAllBtn.disabled = false;
      }
    });
  }

  if (elements.resumeAllBtn) {
    elements.resumeAllBtn.addEventListener('click', async () => {
      const pausedDownloads = state.downloads.filter(d => d.status === 'paused');
      if (pausedDownloads.length === 0) {
        return;
      }
      elements.resumeAllBtn.disabled = true;
      try {
        const succeeded = await performBatchAction(pausedDownloads, 'resume');
        batchResultToast('resume', pausedDownloads.length, succeeded);
        if (!state.isWebSocket) fetchDownloads();
      } catch (e) {
        showToast('Could not resume every download. Some transfers may stay paused.', 'error');
      } finally {
        elements.resumeAllBtn.disabled = false;
      }
    });
  }

  // Clear Finished: always confirm, and keep completed and failed separate.
  function clearTargets() {
    const keepFailed = elements.clearCompletedOnly ? elements.clearCompletedOnly.checked : true;
    return state.downloads.filter(d =>
      d.status === 'complete' || (!keepFailed && d.status === 'error')
    );
  }

  function countDownloads(status) {
    return state.downloads.filter(d => d.status === status).length;
  }

  function updateClearDialog() {
    const completed = countDownloads('complete');
    const failed = countDownloads('error');
    const keepFailed = elements.clearCompletedOnly ? elements.clearCompletedOnly.checked : true;
    const targets = clearTargets();
    const parts = [];
    if (completed > 0) {
      parts.push(`${completed} completed ${completed === 1 ? 'download' : 'downloads'}`);
    }
    if (!keepFailed && failed > 0) {
      parts.push(`${failed} failed ${failed === 1 ? 'download' : 'downloads'}`);
    }

    if (elements.clearSummary) {
      elements.clearSummary.textContent = parts.length > 0
        ? `This removes ${parts.join(' and ')} from the list.`
        : 'There is nothing to clear.';
    }

    if (elements.confirmClearBtn) {
      elements.confirmClearBtn.textContent = !keepFailed && failed > 0
        ? `Clear ${completed + failed} finished`
        : `Clear ${completed} completed`;
      elements.confirmClearBtn.disabled = targets.length === 0;
    }
  }

  function openClearModal() {
    const completed = countDownloads('complete');
    const failed = countDownloads('error');
    if (completed + failed === 0) {
      return;
    }
    lastActiveElement = document.activeElement;
    // "Keep failed" is the useful default when completed downloads exist.
    // With only failed downloads, checking it would open a dialog that says
    // there is nothing to clear, so leave it unchecked instead.
    if (elements.clearCompletedOnly) {
      elements.clearCompletedOnly.checked = completed > 0;
    }
    updateClearDialog();
    openLayer(elements.clearModal, elements.cancelClearBtn);
  }

  function closeClearModal() {
    closeLayer(elements.clearModal);
  }

  if (elements.clearCompletedBtn) {
    elements.clearCompletedBtn.addEventListener('click', openClearModal);
  }
  if (elements.clearCompletedOnly) {
    elements.clearCompletedOnly.addEventListener('change', updateClearDialog);
  }
  if (elements.closeClearModalBtn) {
    elements.closeClearModalBtn.addEventListener('click', closeClearModal);
  }
  if (elements.cancelClearBtn) {
    elements.cancelClearBtn.addEventListener('click', closeClearModal);
  }

  if (elements.confirmClearBtn) {
    elements.confirmClearBtn.addEventListener('click', async () => {
      const targets = clearTargets();
      if (targets.length === 0) {
        closeClearModal();
        return;
      }

      elements.confirmClearBtn.disabled = true;
      const gids = targets.map(d => d.gid);

      try {
        const results = await Promise.all(targets.map(d =>
          fetch(`/api/downloads/${encodeURIComponent(d.gid)}/action`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'remove', deleteFile: false })
          }).then(res => res.ok).catch(() => false)
        ));

        const clearedGids = new Set(gids.filter((_, i) => results[i]));
        const failedCount = gids.length - clearedGids.size;

        state.downloads = state.downloads.filter(d => !clearedGids.has(d.gid));
        updateMetrics();
        renderTable();

        if (failedCount > 0) {
          showToast(`Cleared ${clearedGids.size}. ${failedCount} could not be removed. Refresh to check the list.`, 'error');
        } else {
          showToast(`Cleared ${clearedGids.size} ${clearedGids.size === 1 ? 'download' : 'downloads'}. Files stay on disk.`, 'success');
        }
        closeClearModal();
        if (!state.isWebSocket) fetchDownloads();
      } catch (e) {
        showToast('Could not clear every download. Refresh to check the list.', 'error');
      } finally {
        if (elements.confirmClearBtn) elements.confirmClearBtn.disabled = false;
      }
    });
  }

  // Filter Tabs with WAI-ARIA tab pattern & keyboard navigation
  function setFilter(filterName) {
    state.filter = filterName;
    elements.tabBtns.forEach(btn => {
      const isMatch = btn.dataset.filter === filterName;
      btn.classList.toggle('active', isMatch);
      btn.setAttribute('aria-selected', isMatch ? 'true' : 'false');
      btn.tabIndex = isMatch ? 0 : -1;
    });
    renderTable();
  }

  elements.tabBtns.forEach((btn, idx) => {
    btn.addEventListener('click', () => {
      setFilter(btn.dataset.filter);
    });

    btn.addEventListener('keydown', (e) => {
      let targetIdx = null;
      if (e.key === 'ArrowRight') {
        targetIdx = (idx + 1) % elements.tabBtns.length;
      } else if (e.key === 'ArrowLeft') {
        targetIdx = (idx - 1 + elements.tabBtns.length) % elements.tabBtns.length;
      } else if (e.key === 'Home') {
        targetIdx = 0;
      } else if (e.key === 'End') {
        targetIdx = elements.tabBtns.length - 1;
      }
      if (targetIdx !== null) {
        e.preventDefault();
        const targetBtn = elements.tabBtns[targetIdx];
        targetBtn.focus();
        setFilter(targetBtn.dataset.filter);
      }
    });
  });

  // Empty State Actions
  if (elements.emptyAddBtn) {
    elements.emptyAddBtn.addEventListener('click', openAddModal);
  }

  if (elements.emptyActionBtn) {
    elements.emptyActionBtn.addEventListener('click', () => {
      state.search = '';
      if (elements.searchInput) elements.searchInput.value = '';
      setFilter('all');
    });
  }

  // Search Input
  if (elements.searchInput) {
    elements.searchInput.addEventListener('input', (e) => {
      state.search = e.target.value;
      renderTable();
    });
    elements.searchInput.addEventListener('keydown', (e) => {
      if (e.key === 'Escape' && e.target.value) {
        e.stopPropagation();
        e.target.value = '';
        state.search = '';
        renderTable();
      }
    });
  }

  // Add Download Modal
  function clearUrlError() {
    if (elements.downloadUrlError) {
      elements.downloadUrlError.textContent = '';
      elements.downloadUrlError.classList.add('hidden');
    }
    if (elements.downloadUrl) {
      elements.downloadUrl.removeAttribute('aria-invalid');
    }
  }

  function showUrlError(message) {
    if (elements.downloadUrlError) {
      elements.downloadUrlError.textContent = message;
      elements.downloadUrlError.classList.remove('hidden');
    }
    if (elements.downloadUrl) {
      elements.downloadUrl.setAttribute('aria-invalid', 'true');
      elements.downloadUrl.focus();
    }
  }

  function updateAddDestination() {
    if (!elements.addDestination) return;
    const dir = elements.cfgDownloadDir ? elements.cfgDownloadDir.value.trim() : '';
    elements.addDestination.textContent = dir || 'your download folder';
  }

  function openAddModal() {
    lastActiveElement = document.activeElement;
    clearUrlError();
    updateAddDestination();
    if (elements.submitAddBtn) {
      elements.submitAddBtn.disabled = false;
      elements.submitAddBtn.textContent = 'Add download';
    }
    openLayer(elements.addModal, elements.downloadUrl);
  }

  function closeAddModal() {
    closeLayer(elements.addModal);
  }

  if (elements.openAddModalBtn) {
    elements.openAddModalBtn.addEventListener('click', openAddModal);
  }

  if (elements.closeAddModalBtn) elements.closeAddModalBtn.addEventListener('click', closeAddModal);
  if (elements.cancelAddBtn) elements.cancelAddBtn.addEventListener('click', closeAddModal);

  if (elements.addDownloadForm) {
    elements.addDownloadForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      if (isAddingDownload) return;

      const urlVal = elements.downloadUrl ? elements.downloadUrl.value.trim() : '';
      const filenameVal = elements.downloadFilename ? elements.downloadFilename.value.trim() : '';
      clearUrlError();

      if (!urlVal) {
        showUrlError('Enter a download link.');
        return;
      }

      let parsedUrl = null;
      try {
        parsedUrl = new URL(urlVal);
      } catch (_) {
        showUrlError('Enter a full link, for example https://example.com/file.zip.');
        return;
      }

      if (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:' && parsedUrl.protocol !== 'ftp:') {
        showUrlError('Conduit accepts HTTP, HTTPS, and FTP links.');
        return;
      }

      isAddingDownload = true;
      if (elements.submitAddBtn) {
        elements.submitAddBtn.disabled = true;
        elements.submitAddBtn.textContent = 'Adding...';
      }

      try {
        const res = await fetch('/api/downloads', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            url: urlVal,
            filename: filenameVal || undefined,
          })
        });

        if (res.status === 201) {
          showToast('Added to the queue. It starts when a slot is free.', 'success');
          closeAddModal();
          if (!state.isWebSocket) fetchDownloads();
        } else if (res.status === 400 || res.status === 422) {
          const err = await res.json().catch(() => ({}));
          showUrlError(err.error || 'Conduit cannot download this link. Check the address.');
        } else if (res.status === 429) {
          showToast('Too many requests. Wait a moment and try again.', 'error');
        } else if (res.status >= 500) {
          showToast('The download engine could not add this link. Try again.', 'error');
        } else {
          const err = await res.json().catch(() => ({}));
          showToast(err.error || 'Could not add the download. Try again.', 'error');
        }
      } catch (err) {
        console.error('Add failed:', err);
        showToast('Could not reach Conduit. Check that the app is running.', 'error');
      } finally {
        isAddingDownload = false;
        if (elements.submitAddBtn) {
          elements.submitAddBtn.disabled = false;
          elements.submitAddBtn.textContent = 'Add download';
        }
      }
    });
  }

  if (elements.downloadUrl) {
    elements.downloadUrl.addEventListener('input', clearUrlError);
  }

  // Settings Slide-Over Drawer
  function openSettingsDrawer() {
    lastActiveElement = document.activeElement;
    openLayer(elements.settingsDrawer, elements.cfgDownloadDir || elements.closeSettingsBtn);
    fetchSettings();
  }

  function closeSettingsDrawer() {
    closeLayer(elements.settingsDrawer);
  }

  function syncQuietModeAccent(limit) {
    const wrap = document.querySelector('.speed-limiter-wrap');
    if (!wrap) return;
    const active = !!limit && limit !== '0';
    wrap.classList.toggle('is-limited', active);
    wrap.title = active
      ? 'Download limit on (Quiet Mode)'
      : 'Download limit';
  }

  // applySettings renders a config object. With keepOpenForm, an open
  // settings drawer is left alone so a pushed update never overwrites what the
  // user is typing; the drawer reloads settings each time it opens.
  function applySettings(cfg, { keepOpenForm = false } = {}) {
    const limit = cfg.maxDownloadLimit || '0';
    if (elements.speedLimiterSelect) elements.speedLimiterSelect.value = limit;
    syncQuietModeAccent(limit);

    const drawerOpen = openLayers.includes(elements.settingsDrawer);
    if (!keepOpenForm || !drawerOpen) {
      if (elements.cfgDownloadDir) elements.cfgDownloadDir.value = cfg.downloadDir || '';
      if (elements.cfgMaxConnections) elements.cfgMaxConnections.value = cfg.maxConnections || 16;
      if (elements.maxConnValue) elements.maxConnValue.textContent = cfg.maxConnections || 16;
      if (elements.cfgMaxConcurrent) elements.cfgMaxConcurrent.value = cfg.maxConcurrentDownloads || 5;
      if (elements.cfgAutoStart) elements.cfgAutoStart.checked = !!cfg.autoStart;
      if (elements.cfgShowOnStart) elements.cfgShowOnStart.checked = !!cfg.showOnStart;
      if (elements.cfgShowOnComplete) elements.cfgShowOnComplete.checked = !!cfg.showOnComplete;
      if (elements.cfgShowOnError) elements.cfgShowOnError.checked = !!cfg.showOnError;
      if (elements.cfgAutoCheckUpdates) elements.cfgAutoCheckUpdates.checked = !!cfg.autoCheckUpdates;
      if (elements.cfgAutoInstallUpdates) elements.cfgAutoInstallUpdates.checked = !!cfg.autoInstallUpdates;
      if (elements.cfgSpeedLimit) elements.cfgSpeedLimit.value = limit;
    }
    updateAddDestination();
  }

  async function showAppVersion() {
    if (!elements.appVersion) return;
    try {
      const res = await fetch('/api/health');
      const data = res.ok ? await res.json() : null;
      if (data && data.version) elements.appVersion.textContent = 'Conduit ' + data.version;
    } catch (e) {
      // The version line is informational; leave it empty when the daemon is unreachable.
    }
  }

  async function fetchSettings() {
    showAppVersion();
    refreshUpdate();
    try {
      const res = await fetch('/api/config');
      if (res.ok) {
        applySettings(await res.json());
      }
    } catch (e) {
      showToast('Could not load settings. Close this panel and open it again.', 'error');
    }
  }

  if (elements.speedLimiterSelect) {
    elements.speedLimiterSelect.addEventListener('change', async (e) => {
      const limit = e.target.value;
      const previous = elements.cfgSpeedLimit ? elements.cfgSpeedLimit.value : '0';
      const restore = () => {
        e.target.value = previous;
        syncQuietModeAccent(previous);
      };
      try {
        // Send only the changed field; the daemon merges it into the rest.
        const putRes = await fetch('/api/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ maxDownloadLimit: limit })
        });
        if (putRes.ok) {
          const readable = ({ '1M': '1 MB/s', '2M': '2 MB/s', '5M': '5 MB/s', '10M': '10 MB/s' })[limit] || limit;
          showToast(limit === '0' ? 'Download limit off.' : `Download limit set to ${readable}.`, 'success');
          if (elements.cfgSpeedLimit) elements.cfgSpeedLimit.value = limit;
          syncQuietModeAccent(limit);
        } else {
          restore();
          showToast('Could not change the download limit.', 'error');
        }
      } catch (err) {
        console.error('Limit change failed:', err);
        restore();
        showToast('Could not reach Conduit. The limit did not change.', 'error');
      }
    });
  }

  // ---- Updates ----
  // The daemon checks GitHub, downloads, and installs. This section shows where
  // that stands and runs the next step. The daemon pushes the status over the
  // WebSocket, so every open dashboard shows the same state.
  let updateStatus = null;
  let updateStepRunning = false;

  function formatCheckedAt(iso) {
    if (!iso) return '';
    const when = new Date(iso);
    if (Number.isNaN(when.getTime())) return '';
    return when.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
  }

  // describeUpdate turns a status into the button and the line under it.
  function describeUpdate(st) {
    const checked = formatCheckedAt(st.lastCheckedAt);
    const lastChecked = checked ? ` Last checked ${checked}.` : '';
    switch (st.state) {
      case 'checking':
        return { label: 'Checking…', action: 'check', disabled: true, text: 'Checking GitHub for a newer version.' };
      case 'available':
        return { label: 'Download update', action: 'download', text: `Version ${st.latestVersion} is available.` };
      case 'downloading': {
        const pct = st.progress && st.progress.total > 0 ? Math.floor((100 * st.progress.done) / st.progress.total) : 0;
        return { label: 'Downloading…', action: 'download', disabled: true, text: `Downloading version ${st.latestVersion}: ${pct}%.` };
      }
      case 'downloaded':
        if (!st.installable) {
          return { label: 'Install and restart', action: 'install', disabled: true, text: `Version ${st.latestVersion} is downloaded. Install it from the Windows setup or the Ubuntu package.` };
        }
        return { label: 'Install and restart', action: 'install', text: `Version ${st.latestVersion} is downloaded. Conduit restarts to install it.` };
      case 'installing':
        return { label: 'Installing…', action: 'install', disabled: true, text: 'Installing the update.' };
      case 'upToDate':
        return { label: 'Check for updates', action: 'check', text: `Conduit is up to date.${lastChecked}` };
      case 'error': {
        const reason = st.error ? ` ${st.error}` : '';
        return { label: 'Check for updates', action: 'check', error: true, text: `Could not finish the update.${reason} Check for updates to try again.` };
      }
      default:
        return { label: 'Check for updates', action: 'check', text: lastChecked.trim() || 'Not checked yet.' };
    }
  }

  function renderUpdate(st) {
    if (!elements.updateActionBtn) return;
    updateStatus = st;
    if (elements.updateSummary) {
      elements.updateSummary.textContent = st.installable
        ? `Version ${st.currentVersion}`
        : `Version ${st.currentVersion}. Installing updates needs the Windows setup or the Ubuntu package.`;
    }
    // A switch that is on stays editable, so a copy that cannot install never traps the user with it on.
    if (elements.cfgAutoInstallUpdates) {
      elements.cfgAutoInstallUpdates.disabled = !st.installable && !elements.cfgAutoInstallUpdates.checked;
    }

    const view = describeUpdate(st);
    elements.updateActionBtn.textContent = view.label;
    elements.updateActionBtn.dataset.action = view.action;
    elements.updateActionBtn.disabled = !!view.disabled || updateStepRunning;
    // Download and install are the next step once a release is found, so they carry the primary style.
    const primary = view.action !== 'check' && !view.disabled;
    elements.updateActionBtn.classList.toggle('btn-primary', primary);
    elements.updateActionBtn.classList.toggle('btn-secondary', !primary);
    elements.updateDetail.textContent = view.text;
    elements.updateDetail.classList.toggle('is-error', !!view.error);
  }

  function showUpdatePending(label, text) {
    elements.updateActionBtn.textContent = label;
    elements.updateActionBtn.disabled = true;
    elements.updateDetail.textContent = text;
    elements.updateDetail.classList.remove('is-error');
  }

  // postUpdateStep sends one update request. A refusal carries the daemon's reason.
  async function postUpdateStep(path) {
    const res = await fetch(path, { method: 'POST' });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(body.error || 'Conduit did not start that update step.');
    }
    return body;
  }

  async function refreshUpdate() {
    if (!elements.updateActionBtn) return;
    try {
      const res = await fetch('/api/update');
      if (res.ok) renderUpdate(await res.json());
    } catch (e) {
      // The connection status already reports an unreachable daemon.
    }
  }

  // runUpdateStep runs the button's step. A check that finds a release goes on
  // to download it, so one click takes the user from checking to a ready file.
  async function runUpdateStep(action) {
    updateStepRunning = true;
    elements.updateActionBtn.disabled = true;
    try {
      if (action === 'install') {
        // The daemon quits after it answers, so this page loses its connection.
        await postUpdateStep('/api/update/install');
        return;
      }
      if (action === 'download') {
        renderUpdate(await postUpdateStep('/api/update/download'));
        return;
      }
      showUpdatePending('Checking…', 'Checking GitHub for a newer version.');
      let st = await postUpdateStep('/api/update/check');
      if (st.state === 'available') {
        st = await postUpdateStep('/api/update/download');
      }
      renderUpdate(st);
    } catch (err) {
      showToast(err.message, 'error');
    } finally {
      updateStepRunning = false;
      await refreshUpdate();
    }
  }

  // onUpdateStatus renders a pushed status. It also tells the user once when a
  // version turns up, unless this page's own step just caused the change.
  function onUpdateStatus(st) {
    const before = updateStatus;
    renderUpdate(st);
    if (!before || updateStepRunning) return;
    if (before.state === st.state && before.latestVersion === st.latestVersion) return;
    if (st.state === 'available') {
      showToast(`Conduit ${st.latestVersion} is available.`, 'success', {
        persist: true,
        actions: [{ label: 'Download', onClick: () => runUpdateStep('download') }],
      });
    } else if (st.state === 'downloaded' && st.installable) {
      showToast(`Conduit ${st.latestVersion} is downloaded and ready to install.`, 'success', {
        persist: true,
        actions: [{ label: 'Install and restart', onClick: () => runUpdateStep('install') }],
      });
    }
  }

  if (elements.updateActionBtn) {
    elements.updateActionBtn.addEventListener('click', () => {
      runUpdateStep(elements.updateActionBtn.dataset.action || 'check');
    });
  }

  // Automatic install needs automatic checks, so turning install on turns checks
  // on, and turning checks off turns install off.
  if (elements.cfgAutoCheckUpdates && elements.cfgAutoInstallUpdates) {
    elements.cfgAutoInstallUpdates.addEventListener('change', () => {
      if (elements.cfgAutoInstallUpdates.checked) elements.cfgAutoCheckUpdates.checked = true;
    });
    elements.cfgAutoCheckUpdates.addEventListener('change', () => {
      if (!elements.cfgAutoCheckUpdates.checked) elements.cfgAutoInstallUpdates.checked = false;
    });
  }

  if (elements.openSettingsBtn) {
    elements.openSettingsBtn.addEventListener('click', openSettingsDrawer);
  }

  if (elements.closeSettingsBtn) {
    elements.closeSettingsBtn.addEventListener('click', closeSettingsDrawer);
  }

  if (elements.cfgMaxConnections && elements.maxConnValue) {
    elements.cfgMaxConnections.addEventListener('input', (e) => {
      elements.maxConnValue.textContent = e.target.value;
    });
  }

  if (elements.settingsForm) {
    elements.settingsForm.addEventListener('submit', async (e) => {
      e.preventDefault();
      const payload = {
        downloadDir: elements.cfgDownloadDir ? elements.cfgDownloadDir.value.trim() : '',
        maxConnections: elements.cfgMaxConnections ? parseInt(elements.cfgMaxConnections.value, 10) : 16,
        maxConcurrentDownloads: elements.cfgMaxConcurrent ? parseInt(elements.cfgMaxConcurrent.value, 10) : 5,
        autoStart: elements.cfgAutoStart ? elements.cfgAutoStart.checked : false,
        showOnStart: elements.cfgShowOnStart ? elements.cfgShowOnStart.checked : true,
        showOnComplete: elements.cfgShowOnComplete ? elements.cfgShowOnComplete.checked : true,
        showOnError: elements.cfgShowOnError ? elements.cfgShowOnError.checked : true,
        autoCheckUpdates: elements.cfgAutoCheckUpdates ? elements.cfgAutoCheckUpdates.checked : false,
        autoInstallUpdates: elements.cfgAutoInstallUpdates ? elements.cfgAutoInstallUpdates.checked : false,
        maxDownloadLimit: elements.cfgSpeedLimit ? elements.cfgSpeedLimit.value : (elements.speedLimiterSelect ? elements.speedLimiterSelect.value : '0'),
      };

      try {
        const res = await fetch('/api/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload)
        });
        if (res.ok) {
          showToast('Settings saved.', 'success');
          if (elements.speedLimiterSelect && payload.maxDownloadLimit !== undefined) {
            elements.speedLimiterSelect.value = payload.maxDownloadLimit;
          }
          syncQuietModeAccent(payload.maxDownloadLimit || '0');
          closeSettingsDrawer();
        } else {
          const err = await res.json().catch(() => ({}));
          showToast(err.error || 'Could not save settings.', 'error');
        }
      } catch (e) {
        console.error('Settings save failed:', e);
        showToast('Could not reach Conduit. Settings were not saved.', 'error');
      }
    });
  }

  // Keyboard Shortcuts. Escape closes one layer only, the top one.
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      if (closeTopLayer()) {
        e.preventDefault();
      }
      return;
    }
    const el = document.activeElement;
    const isTextField = !el || el.tagName === 'INPUT' || el.tagName === 'TEXTAREA'
      || el.tagName === 'SELECT' || el.isContentEditable;
    if (e.key === '/' && !isTextField && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      if (elements.searchInput) elements.searchInput.focus();
    }
  });

  // Hash Navigation Support (e.g. #settings or #config opens settings drawer)
  function checkUrlHash() {
    if (window.location.hash === '#settings' || window.location.hash === '#config') {
      openSettingsDrawer();
    }
  }
  window.addEventListener('hashchange', checkUrlHash);

  // 4. Toast Notification Utility
  // showToast shows a short message. With options.actions it also carries
  // buttons ({label, onClick}), and options.persist keeps it until dismissed.
  // The fade pauses while the pointer or keyboard focus is inside the toast.
  const MAX_TOASTS = 4;

  function showToast(message, type = 'success', options = {}) {
    if (!elements.toastContainer) return null;
    const actions = options.actions || [];
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    toast.setAttribute('role', type === 'error' ? 'alert' : 'status');

    const text = document.createElement('span');
    text.className = 'toast-message';
    text.textContent = message;
    toast.appendChild(text);

    let timer = null;
    const dismiss = () => {
      clearTimeout(timer);
      toast.style.opacity = '0';
      toast.style.transition = 'opacity 0.3s ease';
      setTimeout(() => toast.remove(), 300);
    };

    if (actions.length || options.persist) {
      const bar = document.createElement('span');
      bar.className = 'toast-actions';
      actions.forEach(action => {
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'btn btn-secondary btn-sm toast-action';
        btn.textContent = action.label;
        btn.addEventListener('click', () => {
          dismiss();
          action.onClick();
        });
        bar.appendChild(btn);
      });
      if (options.persist) {
        const close = document.createElement('button');
        close.type = 'button';
        close.className = 'btn btn-secondary btn-sm toast-action';
        close.textContent = 'Dismiss';
        close.addEventListener('click', dismiss);
        bar.appendChild(close);
      }
      toast.appendChild(bar);
    }

    // Older quiet toasts make room before a fifth one stacks up.
    const open = elements.toastContainer.querySelectorAll('.toast:not(.persist)');
    if (open.length >= MAX_TOASTS) open[0].remove();
    elements.toastContainer.appendChild(toast);

    if (options.persist) {
      toast.classList.add('persist');
      return toast;
    }

    const lifetime = options.lifetime || (type === 'error' ? 7000 : 4000);
    const arm = () => { timer = setTimeout(dismiss, lifetime); };
    arm();
    if (actions.length) {
      const hold = () => clearTimeout(timer);
      const resume = () => { clearTimeout(timer); arm(); };
      toast.addEventListener('mouseenter', hold);
      toast.addEventListener('focusin', hold);
      toast.addEventListener('mouseleave', resume);
      toast.addEventListener('focusout', resume);
    }
    return toast;
  }

  // 4b. Download events. The daemon sends these only when it has just brought
  // the window forward, so the toast always matches a pop-up the user is seeing.
  function announceDownloadEvents(events) {
    if (!events.length) return;
    const byKind = kind => events.filter(e => e.event === kind);
    const failed = byKind('error');
    const finished = byKind('complete');
    const started = byKind('start');

    if (events.length === 1) {
      const e = events[0];
      if (e.event === 'error') {
        const reason = e.errorMessage ? ` ${e.errorMessage}` : '';
        showToast(`Failed: ${e.filename}.${reason}`, 'error', {
          persist: true,
          actions: [{ label: 'Retry', onClick: () => performAction(e.gid, 'retry') }]
        });
      } else if (e.event === 'complete') {
        showToast(`Finished: ${e.filename}`, 'success', {
          lifetime: 8000,
          actions: [{ label: 'Open file', onClick: () => performAction(e.gid, 'open_file') }]
        });
      } else {
        showToast(`Added: ${e.filename}`, 'success');
      }
    } else {
      const parts = [];
      if (failed.length) parts.push(`${failed.length} failed`);
      if (finished.length) parts.push(`${finished.length} finished`);
      if (started.length) parts.push(`${started.length} added`);
      const summary = parts.join(', ') + '.';
      if (failed.length) {
        showToast(summary, 'error', {
          persist: true,
          actions: [{ label: 'Show failed', onClick: () => { clearSearch(); setFilter('error'); } }]
        });
      } else {
        showToast(summary, 'success', { lifetime: 6000 });
      }
    }

    // Point at the row that matters most: a failure, then a finish, then a start.
    const focus = failed[failed.length - 1] || finished[finished.length - 1] || started[started.length - 1];
    if (focus) revealDownloadRow(focus.gid);
  }

  function clearSearch() {
    state.search = '';
    if (elements.searchInput) elements.searchInput.value = '';
  }

  // revealDownloadRow makes the row visible and flashes it. It leaves the view
  // alone while the user is typing in the search box or the settings drawer.
  function revealDownloadRow(gid) {
    const typing = document.activeElement === elements.searchInput
      || openLayers.includes(elements.settingsDrawer);
    if (typing) return;
    if (!state.downloads.some(d => d.gid === gid)) return;

    let row = elements.transferTableBody.querySelector(`tr[data-gid="${CSS.escape(gid)}"]`);
    if (!row) {
      // The current filter or search hides it.
      clearSearch();
      setFilter('all');
      row = elements.transferTableBody.querySelector(`tr[data-gid="${CSS.escape(gid)}"]`);
    }
    if (!row) return;
    row.scrollIntoView({ block: 'nearest' });
    row.classList.remove('row-flash');
    void row.offsetWidth; // restart the animation if the row flashed before
    row.classList.add('row-flash');
    setTimeout(() => row.classList.remove('row-flash'), 2400);
  }

  // 5. Formatting Utilities
  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  function formatSpeed(bytesPerSec) {
    if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s';
    return formatBytes(bytesPerSec) + '/s';
  }

  function formatETA(total, completed, speed) {
    if (!speed || speed <= 0 || !total || completed >= total) return '--';
    const remainingSeconds = Math.max(0, Math.round((total - completed) / speed));
    if (remainingSeconds < 60) return `${remainingSeconds}s`;
    const minutes = Math.floor(remainingSeconds / 60);
    const seconds = remainingSeconds % 60;
    if (minutes < 60) return `${minutes}m ${seconds}s`;
    const hours = Math.floor(minutes / 60);
    const remMinutes = minutes % 60;
    return `${hours}h ${remMinutes}m`;
  }

  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  if (elements.retryConnectionBtn) {
    elements.retryConnectionBtn.addEventListener('click', () => {
      elements.retryConnectionBtn.disabled = true;
      reconnectDelay = 1000;
      fetchDownloads().finally(() => {
        elements.retryConnectionBtn.disabled = false;
      });
      if (!ws || ws.readyState === WebSocket.CLOSED || ws.readyState === WebSocket.CLOSING) {
        connectWebSocket();
      }
    });
  }

  if (elements.openExtensionGuideBtn) {
    elements.openExtensionGuideBtn.addEventListener('click', openExtensionGuide);
  }
  if (elements.emptyExtensionLink) {
    elements.emptyExtensionLink.addEventListener('click', openExtensionGuide);
  }
  if (elements.extensionGuideURL) {
    elements.extensionGuideURL.value = window.location.origin + '/extension';
  }

  // Initialize
  redirectIfSetupNeeded().then(function (left) {
    if (left) return;
    updateMetrics();
    initConnection();
    fetchSettings();
    checkUrlHash();
    if (shellMode) {
      refreshEngineStatus();
      window.setInterval(refreshEngineStatus, 2000);
    }
  });
})();
