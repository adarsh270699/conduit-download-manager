// Conduit Extension Popup Logic
(function () {
  'use strict';

  // Core Header & Status Elements
  const statusBadge = document.getElementById('status-badge');
  const statusText = document.getElementById('status-text');
  const interceptionToggle = document.getElementById('interception-toggle');

  // Site Bypass Elements
  const siteBypassRow = document.getElementById('site-bypass-row');
  const siteHostname = document.getElementById('site-hostname');
  const siteBypassBtn = document.getElementById('site-bypass-btn');
  const siteBypassStatus = document.getElementById('site-bypass-status');

  // Active Downloads Elements
  const activeSection = document.getElementById('active-section');
  const activeCountBadge = document.getElementById('active-count-badge');
  const aggregateSpeed = document.getElementById('aggregate-speed');
  const activePrimaryCard = document.getElementById('active-primary-card');
  const activeFilename = document.getElementById('active-filename');
  const activePauseBtn = document.getElementById('active-pause-btn');
  const iconPause = document.getElementById('icon-pause');
  const iconResume = document.getElementById('icon-resume');
  const activeCancelBtn = document.getElementById('active-cancel-btn');
  const activeProgressTrack = document.getElementById('active-progress-track');
  const activeProgressBar = document.getElementById('active-progress-bar');
  const activeLength = document.getElementById('active-length');
  const activeEta = document.getElementById('active-eta');
  const activeMoreSummary = document.getElementById('active-more-summary');
  const activeMoreText = document.getElementById('active-more-text');
  const activeIdleBox = document.getElementById('active-idle-box');

  // Held Queue Elements
  const queueSection = document.getElementById('queue-section');
  const queueBadge = document.getElementById('queue-badge');
  const flushBtn = document.getElementById('flush-btn');
  const queueEmpty = document.getElementById('queue-empty');
  const queueList = document.getElementById('queue-list');
  const openDashboardBtn = document.getElementById('open-dashboard-btn');

  let currentOnlineState = false;
  let currentDomain = '';
  let excludedDomainsList = [];
  let primaryActiveJob = null;
  let pollInterval = null;

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let val = Number(bytes);
    let i = 0;
    while (val >= 1024 && i < units.length - 1) {
      val /= 1024;
      i++;
    }
    return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
  }

  function formatSpeed(bytesPerSec) {
    if (!bytesPerSec || bytesPerSec <= 0) return '0.0 KB/s';
    const bytes = Number(bytesPerSec);
    if (bytes < 1024) return `${bytes.toFixed(0)} B/s`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB/s`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB/s`;
  }

  function formatETA(remainingBytes, speed) {
    if (!speed || speed <= 0 || !remainingBytes || remainingBytes <= 0) return '--:--';
    const seconds = Math.round(remainingBytes / speed);
    if (seconds < 60) return `${seconds}s ETA`;
    const mins = Math.floor(seconds / 60);
    const remSecs = seconds % 60;
    if (mins < 60) return `${mins}m ${remSecs}s ETA`;
    const hrs = Math.floor(mins / 60);
    return `${hrs}h ${mins % 60}m ETA`;
  }

  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function setStatus(isOnline) {
    currentOnlineState = isOnline;
    statusBadge.classList.remove('checking', 'online', 'offline');
    if (isOnline) {
      statusBadge.classList.add('online');
      statusText.textContent = 'Connected';
    } else {
      statusBadge.classList.add('offline');
      statusText.textContent = 'Offline';
    }
  }

  // --- Per-Site Exclusion Logic ---
  function updateSiteBypassButton() {
    if (!currentDomain) return;
    const isExcluded = excludedDomainsList.some((d) => {
      const clean = (d || '').trim().toLowerCase();
      if (!clean) return false;
      return currentDomain === clean || currentDomain.endsWith('.' + clean);
    });

    if (isExcluded) {
      siteBypassBtn.classList.add('is-excluded');
      siteBypassStatus.textContent = 'Excluded';
      siteBypassBtn.title = `Interception bypassed for ${currentDomain}. Click to re-enable.`;
    } else {
      siteBypassBtn.classList.remove('is-excluded');
      siteBypassStatus.textContent = 'Active';
      siteBypassBtn.title = `Click to exclude ${currentDomain} from interception.`;
    }
  }

  function detectCurrentTab() {
    if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.query) {
      return;
    }
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (chrome.runtime.lastError || !tabs || !tabs[0] || !tabs[0].url) {
        siteBypassRow.style.display = 'none';
        return;
      }
      try {
        const parsed = new URL(tabs[0].url);
        if (parsed.protocol === 'http:' || parsed.protocol === 'https:') {
          currentDomain = parsed.hostname.toLowerCase();
          siteHostname.textContent = currentDomain;
          siteHostname.title = currentDomain;
          siteBypassRow.style.display = 'flex';
          updateSiteBypassButton();
        } else {
          siteBypassRow.style.display = 'none';
        }
      } catch (_) {
        siteBypassRow.style.display = 'none';
      }
    });
  }

  siteBypassBtn.addEventListener('click', () => {
    if (!currentDomain) return;
    chrome.runtime.sendMessage(
      { type: 'TOGGLE_DOMAIN_EXCLUSION', domain: currentDomain },
      (resp) => {
        if (resp && Array.isArray(resp.excludedDomains)) {
          excludedDomainsList = resp.excludedDomains;
          updateSiteBypassButton();
        }
      }
    );
  });

  // --- Active Transfers Rendering ---
  function renderActiveTransfers(resp) {
    if (!resp || !resp.isOnline) {
      activeSection.style.display = 'none';
      queueSection.style.display = 'flex';
      return;
    }

    activeSection.style.display = 'flex';
    const activeList = resp.activeList || [];
    activeCountBadge.textContent = String(activeList.length);
    aggregateSpeed.textContent = formatSpeed(resp.activeSpeed || 0);

    if (activeList.length === 0) {
      primaryActiveJob = null;
      activePrimaryCard.style.display = 'none';
      activeMoreSummary.style.display = 'none';
      activeIdleBox.style.display = 'flex';
      return;
    }

    activeIdleBox.style.display = 'none';
    activePrimaryCard.style.display = 'flex';

    primaryActiveJob = activeList[0];
    activeFilename.textContent = primaryActiveJob.filename || 'download';
    activeFilename.title = primaryActiveJob.filename || 'download';

    const total = Number(primaryActiveJob.totalLength) || 0;
    const completed = Number(primaryActiveJob.completedLength) || 0;
    const speed = Number(primaryActiveJob.downloadSpeed) || 0;
    const isPaused = primaryActiveJob.status === 'paused';

    if (isPaused) {
      iconPause.style.display = 'none';
      iconResume.style.display = 'block';
      activeProgressBar.classList.add('paused');
    } else {
      iconPause.style.display = 'block';
      iconResume.style.display = 'none';
      activeProgressBar.classList.remove('paused');
    }

    const pct = total > 0 ? Math.min(100, Math.max(0, (completed / total) * 100)) : 0;
    activeProgressBar.style.width = `${pct.toFixed(1)}%`;
    activeProgressTrack.setAttribute('aria-valuenow', Math.round(pct));

    activeLength.textContent = `${formatBytes(completed)} / ${total > 0 ? formatBytes(total) : 'Unknown'}`;
    activeEta.textContent = isPaused ? 'Paused' : formatETA(total - completed, speed);

    if (activeList.length > 1) {
      activeMoreSummary.style.display = 'block';
      activeMoreText.textContent = `+${activeList.length - 1} other active transfer${activeList.length > 2 ? 's' : ''}`;
    } else {
      activeMoreSummary.style.display = 'none';
    }
  }

  function fetchActiveTransfers() {
    if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.sendMessage) {
      return;
    }
    chrome.runtime.sendMessage({ type: 'GET_ACTIVE_DOWNLOADS' }, (resp) => {
      if (chrome.runtime.lastError || !resp) {
        return;
      }
      setStatus(Boolean(resp.isOnline));
      renderActiveTransfers(resp);
    });
  }

  // Active Card Controls (Pause / Resume & Cancel)
  activePauseBtn.addEventListener('click', () => {
    if (!primaryActiveJob || !primaryActiveJob.gid) return;
    const action = primaryActiveJob.status === 'paused' ? 'resume' : 'pause';
    activePauseBtn.disabled = true;
    chrome.runtime.sendMessage(
      { type: 'DOWNLOAD_ACTION', gid: primaryActiveJob.gid, action },
      () => {
        activePauseBtn.disabled = false;
        fetchActiveTransfers();
      }
    );
  });

  activeCancelBtn.addEventListener('click', () => {
    if (!primaryActiveJob || !primaryActiveJob.gid) return;
    activeCancelBtn.disabled = true;
    chrome.runtime.sendMessage(
      { type: 'DOWNLOAD_ACTION', gid: primaryActiveJob.gid, action: 'remove' },
      () => {
        activeCancelBtn.disabled = false;
        fetchActiveTransfers();
      }
    );
  });

  // --- Held Queue Rendering ---
  function renderQueue(items) {
    const list = Array.isArray(items) ? items : [];
    queueBadge.textContent = String(list.length);

    if (list.length === 0) {
      queueEmpty.style.display = 'flex';
      queueList.style.display = 'none';
      queueList.innerHTML = '';
      flushBtn.disabled = true;

      // When online and no held items, gracefully collapse queue section to prevent bloat
      if (currentOnlineState) {
        queueSection.style.display = 'none';
      } else {
        queueSection.style.display = 'flex';
      }
      return;
    }

    queueSection.style.display = 'flex';
    queueEmpty.style.display = 'none';
    queueList.style.display = 'flex';
    queueList.innerHTML = '';
    flushBtn.disabled = !currentOnlineState;

    list.forEach((job) => {
      const card = document.createElement('div');
      card.className = 'queue-card';

      let domain = '';
      try {
        domain = new URL(job.url).hostname;
      } catch (_) {
        domain = 'Direct URL';
      }

      const sizeStr = formatBytes(job.fileSize);
      const canResume = !job.manual && typeof job.browserDownloadId === 'number';
      const resumeButton = canResume ? `
          <button class="btn-resume" data-id="${job.browserDownloadId}" title="Resume download directly in browser">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"></circle>
              <polygon points="10 8 16 12 10 16 10 8"></polygon>
            </svg>
            Resume in Browser
          </button>` : '';

      card.innerHTML = `
        <div class="queue-card-top">
          <div class="queue-item-name" title="${escapeHTML(job.filename || 'download')}">${escapeHTML(job.filename || 'download')}</div>
        </div>
        <div class="queue-item-meta">
          <span>${escapeHTML(domain)}</span>
          <span>${sizeStr}</span>
        </div>
        <div class="queue-card-actions">
          <button class="btn-cancel-held" data-id="${job.browserDownloadId}" title="Discard this download">
            <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
            Cancel
          </button>${resumeButton}
        </div>
      `;

      // Event listener for Resume in Browser
      const resumeBtn = card.querySelector('.btn-resume');
      if (resumeBtn) {
        resumeBtn.addEventListener('click', () => {
          resumeBtn.disabled = true;
          resumeBtn.textContent = 'Resuming...';
          chrome.runtime.sendMessage(
            { type: 'RESUME_BROWSER', downloadId: job.browserDownloadId },
            (resp) => {
              if (resp && resp.heldQueue) {
                renderQueue(resp.heldQueue);
              } else {
                refreshStatus();
              }
            }
          );
        });
      }

      // Event listener for Cancel
      const cancelBtn = card.querySelector('.btn-cancel-held');
      cancelBtn.addEventListener('click', () => {
        cancelBtn.disabled = true;
        chrome.runtime.sendMessage(
          { type: 'CANCEL_HELD', downloadId: job.browserDownloadId },
          (resp) => {
            if (resp && resp.heldQueue) {
              renderQueue(resp.heldQueue);
            } else {
              refreshStatus();
            }
          }
        );
      });

      queueList.appendChild(card);
    });
  }

  function refreshStatus() {
    if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.sendMessage) {
      return;
    }

    chrome.runtime.sendMessage({ type: 'GET_STATUS' }, (resp) => {
      if (chrome.runtime.lastError || !resp) {
        setStatus(false);
        renderQueue([]);
        return;
      }

      setStatus(Boolean(resp.isOnline));
      if (typeof resp.interceptionEnabled === 'boolean') {
        interceptionToggle.checked = resp.interceptionEnabled;
      }
      if (Array.isArray(resp.excludedDomains)) {
        excludedDomainsList = resp.excludedDomains;
        updateSiteBypassButton();
      }
      renderQueue(resp.heldQueue || []);
      if (resp.isOnline) {
        fetchActiveTransfers();
      }
    });
  }

  // Interception toggle switch listener
  interceptionToggle.addEventListener('change', () => {
    chrome.runtime.sendMessage({
      type: 'TOGGLE_INTERCEPTION',
      enabled: interceptionToggle.checked
    });
  });

  // Flush Queue button listener
  flushBtn.addEventListener('click', () => {
    flushBtn.disabled = true;
    flushBtn.textContent = 'Flushing...';
    chrome.runtime.sendMessage({ type: 'FLUSH_QUEUE' }, (resp) => {
      flushBtn.innerHTML = `
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="17 8 12 3 7 8"></polyline>
          <line x1="12" y1="3" x2="12" y2="15"></line>
          <path d="M20 21H4"></path>
        </svg>
        Flush All
      `;
      if (resp && resp.heldQueue) {
        renderQueue(resp.heldQueue);
      } else {
        refreshStatus();
      }
    });
  });

  // Open Web Dashboard button listener
  openDashboardBtn.addEventListener('click', () => {
    const offlineUrl = (typeof chrome !== 'undefined' && chrome.runtime && chrome.runtime.getURL)
      ? chrome.runtime.getURL('offline/offline.html')
      : 'http://127.0.0.1:18800/';
    const targetUrl = currentOnlineState ? 'http://127.0.0.1:18800/' : offlineUrl;

    if (typeof chrome !== 'undefined' && chrome.tabs && chrome.tabs.create) {
      chrome.tabs.create({ url: targetUrl });
    } else {
      window.open(targetUrl, '_blank');
    }
  });

  // Listen to storage changes for reactive UI updates
  if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.onChanged) {
    chrome.storage.onChanged.addListener((changes, area) => {
      if (area === 'local') {
        if (changes.heldQueue) {
          renderQueue(changes.heldQueue.newValue || []);
        }
        if (changes.interceptionEnabled) {
          interceptionToggle.checked = Boolean(changes.interceptionEnabled.newValue);
        }
        if (changes.excludedDomains) {
          excludedDomainsList = changes.excludedDomains.newValue || [];
          updateSiteBypassButton();
        }
      }
    });
  }

  // Ephemeral polling management (active only while popup is rendered)
  function startPolling() {
    fetchActiveTransfers();
    if (!pollInterval) {
      pollInterval = setInterval(fetchActiveTransfers, 1000);
    }
  }

  function stopPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  window.addEventListener('unload', stopPolling);

  // Initial load
  document.addEventListener('DOMContentLoaded', () => {
    detectCurrentTab();
    refreshStatus();
    startPolling();
  });
})();
