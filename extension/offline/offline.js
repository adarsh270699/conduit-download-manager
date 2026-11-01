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
  const reconnectText = document.getElementById('reconnectText');
  const attemptCount = document.getElementById('attemptCount');
  const retryBtn = document.getElementById('retryBtn');
  const retryBtnText = document.getElementById('retryBtnText');
  const heldQueueBox = document.getElementById('heldQueueBox');
  const heldCountBadge = document.getElementById('heldCountBadge');
  const heldItemsList = document.getElementById('heldItemsList');

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
    reconnectBanner.classList.add('is-connected');
    retryBtn.disabled = true;
    retryBtnText.textContent = 'Redirecting...';

    // Send a theme chosen while offline first, so the dashboard opens in it.
    ConduitTheme.flush().finally(() => {
      setTimeout(() => {
        window.location.href = DASHBOARD_URL;
      }, 400);
    });
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
              <li class="held-item-row held-more">
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

  // Colour scheme: the same setting as the dashboard and the popup. A change made here is kept until the
  // daemon is reachable, and it is sent before the dashboard opens.
  const themeBtn = document.getElementById('themeBtn');
  if (themeBtn) {
    themeBtn.addEventListener('click', async () => {
      ConduitTheme.change(ConduitTheme.next(ConduitTheme.read()), () => ConduitTheme.paintButton(themeBtn));
      await ConduitTheme.flush();
      ConduitTheme.paintButton(themeBtn);
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
    loadHeldQueue();
    startHealthLoop();
    ConduitTheme.paintButton(themeBtn);
    ConduitTheme.sync().then(() => ConduitTheme.paintButton(themeBtn));
  });
})();
