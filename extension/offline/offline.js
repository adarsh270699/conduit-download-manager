// Conduit Offline Placeholder Screen Logic
(function () {
  'use strict';

  const DAEMON_HEALTH_URL = 'http://127.0.0.1:18800/api/health';
  const DASHBOARD_URL = 'http://127.0.0.1:18800/';
  const CHECK_INTERVAL_MS = 1500;

  // DOM Elements
  const pillStatus = document.getElementById('pillStatus');
  const pillText = document.getElementById('pillText');
  const mainHeadline = document.getElementById('mainHeadline');
  const reconnectBanner = document.getElementById('reconnectBanner');
  const reconnectSpinner = document.getElementById('reconnectSpinner');
  const reconnectText = document.getElementById('reconnectText');
  const attemptCount = document.getElementById('attemptCount');
  const retryBtn = document.getElementById('retryBtn');
  const retryBtnText = document.getElementById('retryBtnText');
  const heldQueueBox = document.getElementById('heldQueueBox');
  const heldCountBadge = document.getElementById('heldCountBadge');
  const heldItemsList = document.getElementById('heldItemsList');

  // OS Tabs
  const tabLinux = document.getElementById('tabLinux');
  const tabWindows = document.getElementById('tabWindows');
  const panelLinux = document.getElementById('panelLinux');
  const panelWindows = document.getElementById('panelWindows');
  const copyBtns = document.querySelectorAll('.btn-copy');

  let attempts = 0;
  let checkTimer = null;
  let isChecking = false;
  let isConnected = false;

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    let val = bytes;
    let i = 0;
    while (val >= 1024 && i < units.length - 1) {
      val /= 1024;
      i++;
    }
    return `${val.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
  }

  // 1. Check Daemon Health
  async function checkDaemonHealth() {
    if (isChecking || isConnected) return;
    isChecking = true;
    attempts++;
    attemptCount.textContent = `attempt ${attempts}`;

    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 1200);

      const res = await fetch(DAEMON_HEALTH_URL, {
        method: 'GET',
        cache: 'no-store',
        signal: controller.signal
      });

      clearTimeout(timeoutId);

      if (res.ok) {
        onDaemonConnected();
        return;
      }
    } catch (_) {
      // Daemon remains offline
    } finally {
      isChecking = false;
    }
  }

  function onDaemonConnected() {
    isConnected = true;
    if (checkTimer) clearInterval(checkTimer);

    // Transition UI to connected
    pillStatus.className = 'status-indicator-pill connected';
    pillText.textContent = 'Daemon Online';
    mainHeadline.textContent = 'Daemon Connected!';
    reconnectText.textContent = 'Launching Web Dashboard...';
    reconnectBanner.style.borderColor = 'rgba(16, 185, 129, 0.4)';
    reconnectBanner.style.backgroundColor = 'rgba(16, 185, 129, 0.1)';
    retryBtn.disabled = true;
    retryBtnText.textContent = 'Redirecting...';

    // Redirect to the dashboard
    setTimeout(() => {
      window.location.href = DASHBOARD_URL;
    }, 400);
  }

  function startHealthLoop() {
    checkDaemonHealth();
    checkTimer = setInterval(checkDaemonHealth, CHECK_INTERVAL_MS);
  }

  // 2. Load Held Queue from chrome.storage
  function loadHeldQueue() {
    if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
      chrome.storage.local.get(['heldQueue'], (data) => {
        const queue = Array.isArray(data.heldQueue) ? data.heldQueue : [];
        if (queue.length > 0) {
          heldCountBadge.textContent = String(queue.length);
          heldItemsList.innerHTML = queue.slice(0, 5).map((job) => {
            const name = job.filename || 'download';
            const size = formatBytes(job.fileSize);
            return `
              <li class="held-item-row">
                <span class="held-item-name" title="${escapeHtml(name)}">${escapeHtml(name)}</span>
                <span class="held-item-size">${size}</span>
              </li>
            `;
          }).join('');

          if (queue.length > 5) {
            heldItemsList.innerHTML += `
              <li class="held-item-row" style="justify-content: center; color: var(--text-muted);">
                + ${queue.length - 5} more downloads held
              </li>
            `;
          }
          heldQueueBox.classList.remove('hidden');
        } else {
          heldQueueBox.classList.add('hidden');
        }
      });
    }
  }

  function escapeHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  // 3. Setup OS Tabs & Auto-Detect
  function setupOsTabs() {
    const isWindows = navigator.platform.indexOf('Win') !== -1 ||
                      navigator.userAgent.indexOf('Windows') !== -1;

    function selectOs(os) {
      if (os === 'windows') {
        tabWindows.classList.add('active');
        tabLinux.classList.remove('active');
        panelWindows.classList.remove('hidden');
        panelLinux.classList.add('hidden');
      } else {
        tabLinux.classList.add('active');
        tabWindows.classList.remove('active');
        panelLinux.classList.remove('hidden');
        panelWindows.classList.add('hidden');
      }
    }

    tabLinux.addEventListener('click', () => selectOs('linux'));
    tabWindows.addEventListener('click', () => selectOs('windows'));

    // Default based on detected platform
    selectOs(isWindows ? 'windows' : 'linux');
  }

  // 4. Setup Copy Buttons
  function setupCopyButtons() {
    copyBtns.forEach((btn) => {
      btn.addEventListener('click', async () => {
        const targetId = btn.getAttribute('data-target');
        const codeEl = document.getElementById(targetId);
        if (codeEl) {
          const textToCopy = codeEl.textContent.trim();
          try {
            await navigator.clipboard.writeText(textToCopy);
            const span = btn.querySelector('span');
            const originalText = span.textContent;
            btn.classList.add('copied');
            span.textContent = 'Copied!';
            setTimeout(() => {
              btn.classList.remove('copied');
              span.textContent = originalText;
            }, 1800);
          } catch (_) {
            // Fallback selection
          }
        }
      });
    });
  }

  // Event Listeners
  retryBtn.addEventListener('click', () => {
    reconnectText.textContent = 'Pinging daemon...';
    checkDaemonHealth();
  });

  // Listen to chrome.storage changes in case heldQueue updates
  if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.onChanged) {
    chrome.storage.onChanged.addListener((changes, area) => {
      if (area === 'local' && changes.heldQueue) {
        loadHeldQueue();
      }
    });
  }

  // Initialize
  document.addEventListener('DOMContentLoaded', () => {
    setupOsTabs();
    setupCopyButtons();
    loadHeldQueue();
    startHealthLoop();
  });
})();
