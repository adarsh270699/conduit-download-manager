// Conduit Download Interceptor - Background Service Worker (Manifest V3)
// Compatible with Google Chrome and Mozilla Firefox

const DAEMON_HEALTH_URL = 'http://127.0.0.1:18800/api/health';
const DAEMON_DOWNLOADS_URL = 'http://127.0.0.1:18800/api/downloads';
const ALARM_NAME = 'conduit-check-daemon';
const MIN_FILE_SIZE_BYTES = 1024 * 1024; // 1MB threshold when size is known

// In-memory sets to guard against re-entrancy and race conditions
const handlingDownloadIds = new Set();
const interceptedDownloadIds = new Set();
const bypassedDownloadIds = new Set();

/**
 * Checks whether a URL's hostname matches or is a subdomain of an excluded domain.
 * @param {string} urlStr
 * @param {Array<string>} excludedDomains
 * @returns {boolean}
 */
function isDomainExcluded(urlStr, excludedDomains) {
  if (!urlStr || !Array.isArray(excludedDomains) || excludedDomains.length === 0) return false;
  try {
    const parsed = new URL(urlStr);
    const hostname = parsed.hostname.toLowerCase();
    return excludedDomains.some((d) => {
      const clean = (d || '').trim().toLowerCase();
      if (!clean) return false;
      return hostname === clean || hostname.endsWith('.' + clean);
    });
  } catch (_) {
    return false;
  }
}

/**
 * Checks whether a download URL and item should be intercepted.
 * @param {object} item - chrome.downloads.DownloadItem
 * @param {boolean} interceptionEnabled - User toggle state
 * @param {Set<number>} bypassSet - Set of bypassed IDs
 * @param {Array<string>} excludedDomains - List of per-site excluded domains
 * @returns {boolean}
 */
function isInterceptable(item, interceptionEnabled = true, bypassSet = bypassedDownloadIds, excludedDomains = []) {
  if (!interceptionEnabled) return false;
  if (!item || !item.url) return false;
  if (bypassSet.has(item.id)) return false;

  // Filter out downloads from excluded domains
  if (isDomainExcluded(item.url, excludedDomains)) {
    return false;
  }

  // Filter out downloads that are already completed or interrupted
  if (item.state && item.state !== 'in_progress') {
    return false;
  }

  // Filter out downloads that have already ended
  if (item.endTime) {
    return false;
  }

  // Filter out historical downloads re-hydrated on browser startup (older than 2 minutes)
  if (item.startTime) {
    const startMs = new Date(item.startTime).getTime();
    if (!isNaN(startMs) && Date.now() - startMs > 120 * 1000) {
      return false;
    }
  }

  const url = item.url.trim().toLowerCase();
  // Filter non-downloadable or internal schemes
  if (
    url.startsWith('blob:') ||
    url.startsWith('data:') ||
    url.startsWith('filesystem:') ||
    url.startsWith('chrome:') ||
    url.startsWith('chrome-extension:') ||
    url.startsWith('moz-extension:') ||
    url.startsWith('about:') ||
    url.startsWith('javascript:')
  ) {
    return false;
  }

  // Must be HTTP, HTTPS, or FTP
  if (!url.startsWith('http://') && !url.startsWith('https://') && !url.startsWith('ftp://')) {
    return false;
  }

  // Filter out tiny files only if totalBytes is known and positive
  if (item.totalBytes && item.totalBytes > 0 && item.totalBytes < MIN_FILE_SIZE_BYTES) {
    return false;
  }

  return true;
}

/**
 * Serializes cookies array into standard Cookie header format.
 * @param {Array<{name: string, value: string}>} cookies
 * @returns {string}
 */
function formatCookiesHeader(cookies) {
  if (!cookies || !Array.isArray(cookies) || cookies.length === 0) {
    return '';
  }
  return cookies.map((c) => `${c.name}=${c.value}`).join('; ');
}

/**
 * Extracts and merges cookies for target and original URLs.
 * Preserves cross-domain session state across 302/307 redirects.
 * @param {string} targetUrl
 * @param {string} originalUrl
 * @returns {Promise<string>}
 */
async function extractMergedCookies(targetUrl, originalUrl = '') {
  if (typeof chrome === 'undefined' || !chrome.cookies || !chrome.cookies.getAll) {
    return '';
  }
  try {
    const primary = targetUrl ? await chrome.cookies.getAll({ url: targetUrl }) : [];
    const cookieMap = new Map();
    (primary || []).forEach((c) => {
      if (c && c.name) cookieMap.set(c.name, c.value);
    });

    if (originalUrl && originalUrl !== targetUrl) {
      try {
        const secondary = await chrome.cookies.getAll({ url: originalUrl });
        (secondary || []).forEach((c) => {
          if (c && c.name && !cookieMap.has(c.name)) {
            cookieMap.set(c.name, c.value);
          }
        });
      } catch (_) {}
    }

    const merged = Array.from(cookieMap.entries()).map(([name, value]) => ({ name, value }));
    return formatCookiesHeader(merged);
  } catch (_) {
    return '';
  }
}

/**
 * Extracts a clean filename from a download item or URL.
 * Prioritizes browser-resolved filename (from Content-Disposition) and finalUrl.
 * @param {object} item
 * @returns {string}
 */
function extractFilename(item) {
  if (item && item.filename && typeof item.filename === 'string') {
    const clean = item.filename.replace(/\\/g, '/');
    const base = clean.split('/').pop();
    if (base && base.trim().length > 0) return base.trim();
  }

  const candidateUrl = (item && (item.finalUrl || item.url)) || '';
  try {
    const parsed = new URL(candidateUrl);
    const pathname = parsed.pathname;
    const base = pathname.split('/').pop();
    if (base && base.trim().length > 0) {
      return decodeURIComponent(base.trim());
    }
  } catch (_) {}

  return 'download';
}

/**
 * Checks if the local Conduit daemon is online.
 * @param {number} timeoutMs
 * @returns {Promise<boolean>}
 */
async function checkDaemonHealth(timeoutMs = 800) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(DAEMON_HEALTH_URL, {
      method: 'GET',
      cache: 'no-store',
      signal: controller.signal
    });
    if (!res.ok) return false;
    const data = await res.json();
    if (!data || data.status !== 'ok') return false;
    // The daemon answers while its engine is starting or offline. Treat those
    // as unavailable so the extension holds the download instead of accepting
    // it and then releasing it back to the browser.
    if (data.engine && data.engine !== 'ready') return false;
    // Require the Conduit identity so an unrelated local service that happens
    // to occupy port 18800 is not handed the user's download cookies.
    return data.app === 'conduit';
  } catch (_) {
    return false;
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Dispatches a download payload to the Conduit daemon.
 * @param {object} payload
 * @returns {Promise<string|null>} GID on success, null on failure
 */
async function dispatchToDaemon(payload) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 10000);
  try {
    const res = await fetch(DAEMON_DOWNLOADS_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(payload),
      signal: controller.signal
    });
    if (res.status === 201 || res.status === 200) {
      const data = await res.json();
      return data.gid || 'ok';
    }
    return null;
  } catch (_) {
    return null;
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * Updates extension action badge based on state (held queue, active count, or completion).
 * @param {object|number} opts
 */
async function updateBadge(opts) {
  if (typeof chrome === 'undefined' || !chrome.action) return;

  if (typeof opts === 'number') {
    const count = opts;
    if (count > 0) {
      await chrome.action.setBadgeText({ text: String(count) });
      if (chrome.action.setBadgeBackgroundColor) {
        await chrome.action.setBadgeBackgroundColor({ color: '#B83A2E' });
      }
    } else {
      await chrome.action.setBadgeText({ text: '' });
    }
    return;
  }

  const { isOnline, heldCount = 0, activeCount = 0 } = opts || {};
  if (!isOnline && heldCount > 0) {
    await chrome.action.setBadgeText({ text: String(heldCount) });
    if (chrome.action.setBadgeBackgroundColor) {
      await chrome.action.setBadgeBackgroundColor({ color: '#B83A2E' });
    }
  } else if (isOnline && activeCount > 0) {
    await chrome.action.setBadgeText({ text: String(activeCount) });
    if (chrome.action.setBadgeBackgroundColor) {
      await chrome.action.setBadgeBackgroundColor({ color: '#315B8C' });
    }
  } else {
    await chrome.action.setBadgeText({ text: '' });
  }
}

let completionBadgeTimeout = null;
async function flashCompletionBadge() {
  if (typeof chrome === 'undefined' || !chrome.action) return;
  try {
    if (completionBadgeTimeout) clearTimeout(completionBadgeTimeout);
    await chrome.action.setBadgeText({ text: '✓' });
    if (chrome.action.setBadgeBackgroundColor) {
      await chrome.action.setBadgeBackgroundColor({ color: '#2D7A54' });
    }
    completionBadgeTimeout = setTimeout(async () => {
      const queue = await getHeldQueue();
      await updateBadge(queue.length);
    }, 3000);
  } catch (_) {}
}

/**
 * Emits an in-page toast notification to the active browser tab.
 * @param {object} item
 * @param {string} filename
 */
async function triggerInPageToast(item, filename) {
  if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.query) return;
  try {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tabs && tabs.length > 0 && tabs[0].id) {
      chrome.tabs.sendMessage(tabs[0].id, {
        type: 'CONDUIT_TOAST_INITIATED',
        filename: filename || 'file',
        fileSize: item ? item.totalBytes : 0
      }).catch(() => {});
    }
  } catch (_) {}
}

/**
 * Primary interception point in Chromium browsers: resolves redirects,
 * extracts finalUrl and headers, and suppresses the browser's native Save As dialog.
 * @param {object} item
 * @param {function} suggest
 */
async function handleDeterminingFilename(item, suggest) {
  return await processDownloadInterception(item, suggest);
}

/**
 * Handles explicit user downloads initiated via right-click context menu.
 * @param {string} targetUrl
 * @param {string} referrer
 * @returns {Promise<object>}
 */
async function handleManualDownload(targetUrl, referrer = '') {
  if (!targetUrl) return { success: false, reason: 'No URL provided' };

  const cookieString = await extractMergedCookies(targetUrl, referrer);
  const filename = extractFilename({ url: targetUrl });
  const payload = {
    url: targetUrl,
    filename: filename,
    referrer: referrer || '',
    userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : 'Conduit/1.0',
    cookies: cookieString
  };

  const isOnline = await checkDaemonHealth();
  if (isOnline) {
    const gid = await dispatchToDaemon(payload);
    if (gid) {
      await triggerInPageToast({ totalBytes: 0 }, filename);
      await flashCompletionBadge();
      return { success: true, gid };
    }
  }

  const timestamp = Date.now();
  const idSuffix = Math.random().toString(36).slice(2, 8);
  await withQueueLock(async () => {
    const queue = await getHeldQueue();
    queue.push({
      browserDownloadId: `manual-${timestamp}-${idSuffix}`,
      manual: true,
      url: targetUrl,
      filename: filename,
      referrer: referrer || '',
      cookies: cookieString,
      userAgent: payload.userAgent,
      fileSize: 0,
      timestamp
    });
    await saveHeldQueue(queue);
  });
  syncRetryAlarm(1);

  if (typeof chrome !== 'undefined' && chrome.notifications && chrome.notifications.create) {
    chrome.notifications.create({
      type: 'basic',
      iconUrl: 'icons/icon-128.png',
      title: 'Conduit Daemon Offline',
      message: `"${filename}" held in offline queue. Start daemon to begin download.`
    });
  }

  return { success: true, held: true };
}

/**
 * Retrieves held queue from local storage.
 * @returns {Promise<Array>}
 */
async function getHeldQueue() {
  if (typeof chrome === 'undefined' || !chrome.storage || !chrome.storage.local) {
    return [];
  }
  const data = await chrome.storage.local.get(['heldQueue']);
  return Array.isArray(data.heldQueue) ? data.heldQueue : [];
}

/**
 * Saves held queue to local storage and updates badge.
 * @param {Array} queue
 */
async function saveHeldQueue(queue) {
  if (typeof chrome === 'undefined' || !chrome.storage || !chrome.storage.local) {
    return;
  }
  await chrome.storage.local.set({ heldQueue: queue });
  await updateBadge(queue.length);
}

// Service-worker events and alarms can overlap. Every read-modify-write of the
// held queue goes through this chain so two downloads cannot overwrite each
// other's queue entry.
let queueLock = Promise.resolve();

function withQueueLock(operation) {
  const run = queueLock.then(operation, operation);
  queueLock = run.then(
    () => undefined,
    () => undefined
  );
  return run;
}

/**
 * Prunes completed, cancelled, or missing downloads from the held queue.
 * @returns {Promise<Array>} Remaining valid jobs
 */
async function pruneHeldQueue() {
  return withQueueLock(async () => {
    const queue = await getHeldQueue();
    if (queue.length === 0) return [];
    if (typeof chrome === 'undefined' || !chrome.downloads || !chrome.downloads.search) {
      return queue;
    }

    const validJobs = [];
    for (const job of queue) {
      // Context-menu jobs have no browser download to search for.
      if (job.manual || typeof job.browserDownloadId !== 'number') {
        validJobs.push(job);
        continue;
      }
      try {
        const items = await chrome.downloads.search({ id: job.browserDownloadId });
        if (items && items.length > 0) {
          const item = items[0];
          // Only keep if download is still active/in-progress (paused items remain in_progress)
          if (item.state === 'in_progress') {
            validJobs.push(job);
          }
        }
      } catch (_) {
        // If query fails, do not discard blindly
        validJobs.push(job);
      }
    }

    if (validJobs.length !== queue.length) {
      await saveHeldQueue(validJobs);
    }
    return validJobs;
  });
}

/**
 * Completes Chrome's Save dialog and clears a pause so the download can start.
 * @param {object} item
 * @param {function} suggest
 */
async function releaseBrowserDownload(item, suggest) {
  suggestBrowserFilename(item, suggest);
  if (item && item.id && typeof chrome !== 'undefined' && chrome.downloads && chrome.downloads.resume) {
    try {
      await chrome.downloads.resume(item.id);
    } catch (_) {}
  }
}

function suggestBrowserFilename(item, suggest) {
  if (typeof suggest !== 'function') return false;
  const filename = extractFilename(item);
  try {
    suggest({ filename: filename, conflictAction: 'uniquify' });
    return true;
  } catch (_) {
    try {
      suggest();
      return true;
    } catch (_) {
      return false;
    }
  }
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Chrome requires onDeterminingFilename listeners to call suggest() before
 * the browser download can start. Once it is running, pause it so an offline
 * daemon can hold the transfer without losing the browser's session state.
 * @param {object} item
 * @returns {Promise<boolean>} true when the browser download is paused
 */
async function pauseBrowserDownloadForHold(item) {
  if (!item || !item.id || typeof chrome === 'undefined' || !chrome.downloads || !chrome.downloads.pause) {
    return false;
  }

  for (let attempt = 0; attempt < 25; attempt++) {
    try {
      await chrome.downloads.pause(item.id);
    } catch (_) {
      // The download may not have entered the downloads list yet.
    }

    if (!chrome.downloads.search) {
      return true;
    }
    try {
      const matches = await chrome.downloads.search({ id: item.id });
      const download = matches && matches[0];
      if (download && download.paused) return true;
      if (download && download.state && download.state !== 'in_progress') {
        // The browser already finished or the user cancelled the save dialog.
        return false;
      }
    } catch (_) {
      // Pause itself succeeded; a failed status probe should not lose the job.
      return true;
    }
    await sleep(100);
  }
  return false;
}

/**
 * Core download interception and dispatch logic.
 * Handles redirect resolution, cookie merging, daemon dispatch, and browser download lifecycle.
 * @param {object} item - chrome.downloads.DownloadItem
 * @param {function|null} suggest - Suggest callback from onDeterminingFilename, if applicable
 * @returns {Promise<object>}
 */
async function processDownloadInterception(item, suggest = null) {
  if (!item || !item.id) {
    if (typeof suggest === 'function') suggest();
    return { success: false, reason: 'Invalid item' };
  }

  if (interceptedDownloadIds.has(item.id) || handlingDownloadIds.has(item.id)) {
    // The Save dialog is still waiting. Let Chrome finish it.
    if (typeof suggest === 'function') {
      const filename = extractFilename(item);
      try {
        suggest({ filename: filename, conflictAction: 'uniquify' });
      } catch (_) {
        try { suggest(); } catch (_) {}
      }
    }
    return { success: false, reason: 'Already handled' };
  }

  // Retrieve settings
  let interceptionEnabled = true;
  let excludedDomains = [];
  if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
    try {
      const stored = await chrome.storage.local.get(['interceptionEnabled', 'bypassedIds', 'excludedDomains']);
      if (typeof stored.interceptionEnabled === 'boolean') {
        interceptionEnabled = stored.interceptionEnabled;
      }
      if (Array.isArray(stored.bypassedIds)) {
        stored.bypassedIds.forEach((id) => bypassedDownloadIds.add(id));
      }
      if (Array.isArray(stored.excludedDomains)) {
        excludedDomains = stored.excludedDomains;
      }
    } catch (_) {}
  }

  if (!isInterceptable(item, interceptionEnabled, bypassedDownloadIds, excludedDomains)) {
    if (typeof suggest === 'function') suggest();
    return { success: false, reason: 'Not interceptable' };
  }

  handlingDownloadIds.add(item.id);
  const saveDialogOpen = typeof suggest === 'function';

  try {
    // Pause only when no Save dialog is open. A pause during that dialog
    // makes the download start paused after the user clicks Save.
    if (!saveDialogOpen && typeof chrome !== 'undefined' && chrome.downloads && chrome.downloads.pause) {
      try {
        await chrome.downloads.pause(item.id);
      } catch (pauseErr) {
        // May already be paused or completed
      }
    }

    // 2. Extract resolved target URL, original URL, filename, cookies, and referrer
    const targetUrl = item.finalUrl || item.url;
    const originalUrl = item.url || '';
    const filename = extractFilename(item);
    const cookieString = await extractMergedCookies(targetUrl, originalUrl);
    const referrer = item.referrer || (originalUrl && originalUrl !== targetUrl ? originalUrl : '');

    const payload = {
      url: targetUrl,
      filename: filename,
      referrer: referrer,
      userAgent: typeof navigator !== 'undefined' ? navigator.userAgent : 'Conduit/1.0',
      cookies: cookieString
    };

    // 3. Check if daemon is online
    const isOnline = await checkDaemonHealth();

    if (isOnline) {
      // Branch A: Daemon is online
      const gid = await dispatchToDaemon(payload);
      if (gid) {
        interceptedDownloadIds.add(item.id);
        // Download accepted by Conduit -> cancel and erase browser download
        if (typeof chrome !== 'undefined' && chrome.downloads) {
          try {
            await chrome.downloads.cancel(item.id);
            await chrome.downloads.erase({ id: item.id });
          } catch (_) {}
        }
        await triggerInPageToast(item, filename);
        await flashCompletionBadge();
        return { success: true, gid };
      }
    }

    // Chromium routes every download through onDeterminingFilename, so a
    // suggest callback is present even when no Save dialog is open. If the
    // daemon is offline, let Chrome create the browser download and then pause
    // it, so the offline holding queue works on Chrome as well as Firefox.
    if (saveDialogOpen) {
      if (isOnline) {
        // The daemon answered but rejected this file. Let the browser keep it.
        interceptedDownloadIds.add(item.id);
        await releaseBrowserDownload(item, suggest);
        return { success: true, released: true };
      }

      suggestBrowserFilename(item, suggest);
      const held = await pauseBrowserDownloadForHold(item);
      if (!held) {
        // The browser finished, the user cancelled, or pause is unavailable.
        interceptedDownloadIds.add(item.id);
        return { success: true, released: true };
      }
    }

    // Branch B: Daemon is offline (or dispatch failed) -> Hold in queue
    interceptedDownloadIds.add(item.id);
    const job = {
      browserDownloadId: item.id,
      url: targetUrl,
      filename: filename,
      referrer: referrer,
      cookies: cookieString,
      userAgent: payload.userAgent,
      fileSize: item.totalBytes || 0,
      timestamp: Date.now()
    };

    await withQueueLock(async () => {
      const queue = await getHeldQueue();
      const existingIndex = queue.findIndex((j) => j.browserDownloadId === item.id);
      if (existingIndex >= 0) {
        queue[existingIndex] = job;
      } else {
        queue.push(job);
      }
      await saveHeldQueue(queue);
    });

    // Show desktop notification
    if (typeof chrome !== 'undefined' && chrome.notifications && chrome.notifications.create) {
      try {
        chrome.notifications.create({
          type: 'basic',
          iconUrl: 'icons/icon-128.png',
          title: 'Conduit Daemon Offline',
          message: 'Download paused and held. Start Conduit to accelerate, or resume in browser.'
        });
      } catch (_) {}
    }

    syncRetryAlarm(1);

    return { success: true, held: true };
  } finally {
    handlingDownloadIds.delete(item.id);
  }
}

/**
 * Handles download creation event.
 * In Chromium, defers to onDeterminingFilename with a safety fallback timer.
 * In Firefox, executes interception immediately.
 * @param {object} item
 */
async function handleDownloadCreated(item) {
  if (!item || !item.id) return;

  const hasDetermining = typeof chrome !== 'undefined' &&
                         chrome.downloads &&
                         typeof chrome.downloads.onDeterminingFilename !== 'undefined';

  if (hasDetermining) {
    // In Chromium, onDeterminingFilename fires shortly after onCreated with finalUrl and resolved filename.
    // Register a safety timeout fallback in case onDeterminingFilename never fires for this item.
    setTimeout(async () => {
      if (!interceptedDownloadIds.has(item.id) && !handlingDownloadIds.has(item.id)) {
        await processDownloadInterception(item);
      }
    }, 1500);
    return;
  }

  // Firefox / environments without onDeterminingFilename:
  return await processDownloadInterception(item);
}

/**
 * Starts the retry alarm while jobs are held and stops it once none are.
 * @param {number} heldCount
 */
function syncRetryAlarm(heldCount) {
  if (typeof chrome === 'undefined' || !chrome.alarms) return;
  if (heldCount > 0) {
    if (chrome.alarms.create) chrome.alarms.create(ALARM_NAME, { periodInMinutes: 0.5 });
  } else if (chrome.alarms.clear) {
    chrome.alarms.clear(ALARM_NAME);
  }
}

// The alarm, browser startup, and the popup's "Retry" can all flush at once.
// Sharing one run stops the same held job from being sent to Conduit twice.
let flushInFlight = null;

/**
 * Flushes held downloads to the Conduit daemon when online.
 * @returns {Promise<number>} Number of successfully dispatched jobs
 */
function flushHeldQueue() {
  if (!flushInFlight) {
    flushInFlight = flushHeldQueueOnce().finally(() => {
      flushInFlight = null;
    });
  }
  return flushInFlight;
}

async function flushHeldQueueOnce() {
  const isOnline = await checkDaemonHealth();
  if (!isOnline) return 0;

  const queue = await pruneHeldQueue();
  if (queue.length === 0) {
    await updateBadge(0);
    syncRetryAlarm(0);
    return 0;
  }

  let dispatchedCount = 0;

  for (const job of queue) {
    const payload = {
      url: job.url,
      filename: job.filename,
      referrer: job.referrer || '',
      userAgent: job.userAgent || '',
      cookies: job.cookies || ''
    };

    const gid = await dispatchToDaemon(payload);
    if (gid) {
      dispatchedCount++;
      if (typeof job.browserDownloadId === 'number' && typeof chrome !== 'undefined' && chrome.downloads) {
        try {
          await chrome.downloads.cancel(job.browserDownloadId);
          await chrome.downloads.erase({ id: job.browserDownloadId });
        } catch (_) {}
      }
      // Remove only the job that was forwarded. A concurrent interception must
      // not lose its entry to a stale queue snapshot.
      await withQueueLock(async () => {
        const current = await getHeldQueue();
        const next = current.filter((queued) => queued.browserDownloadId !== job.browserDownloadId);
        if (next.length !== current.length) {
          await saveHeldQueue(next);
        }
      });
    }
  }

  syncRetryAlarm((await getHeldQueue()).length);

  if (dispatchedCount > 0 && typeof chrome !== 'undefined' && chrome.notifications) {
    try {
      chrome.notifications.create({
        type: 'basic',
        iconUrl: 'icons/icon-128.png',
        title: 'Downloads Forwarded',
        message: `${dispatchedCount} held download(s) forwarded to Conduit daemon.`
      });
    } catch (_) {}
  }

  return dispatchedCount;
}

/**
 * Resumes a held download in the browser instead of Conduit.
 * @param {number} downloadId
 * @returns {Promise<boolean>}
 */
async function resumeInBrowser(downloadId) {
  if (typeof downloadId !== 'number') {
    // Context-menu jobs have no browser download to resume.
    return false;
  }

  bypassedDownloadIds.add(downloadId);

  // Persist bypass set
  if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
    try {
      const stored = await chrome.storage.local.get(['bypassedIds']);
      const list = Array.isArray(stored.bypassedIds) ? stored.bypassedIds : [];
      if (!list.includes(downloadId)) {
        list.push(downloadId);
        await chrome.storage.local.set({ bypassedIds: list });
      }
    } catch (_) {}
  }

  // Remove from held queue
  await withQueueLock(async () => {
    const queue = await getHeldQueue();
    const filtered = queue.filter((j) => j.browserDownloadId !== downloadId);
    await saveHeldQueue(filtered);
  });

  // Resume browser download
  if (typeof chrome !== 'undefined' && chrome.downloads && chrome.downloads.resume) {
    try {
      await chrome.downloads.resume(downloadId);
      return true;
    } catch (err) {
      return false;
    }
  }
  return true;
}

/**
 * Cancels a held download and removes it from the queue.
 * @param {number} downloadId
 * @returns {Promise<boolean>}
 */
async function cancelHeldDownload(downloadId) {
  await withQueueLock(async () => {
    const queue = await getHeldQueue();
    const filtered = queue.filter((j) => j.browserDownloadId !== downloadId);
    await saveHeldQueue(filtered);
  });

  if (typeof downloadId === 'number' && typeof chrome !== 'undefined' && chrome.downloads) {
    try {
      await chrome.downloads.cancel(downloadId);
      await chrome.downloads.erase({ id: downloadId });
    } catch (_) {}
  }
  return true;
}

/**
 * Intercepts connection errors to Conduit daemon port and redirects to offline placeholder.
 * @param {object} details - webNavigation details object
 * @param {function} tabUpdater - chrome.tabs.update or mock
 * @param {function} getUrlFn - chrome.runtime.getURL or mock
 * @returns {boolean} whether redirect was triggered
 */
function handleNavigationError(details, tabUpdater, getUrlFn) {
  if (!details || details.frameId !== 0) return false;
  const url = (details.url || '').toLowerCase();
  if (
    url.startsWith('http://127.0.0.1:18800') ||
    url.startsWith('http://localhost:18800')
  ) {
    const updater = tabUpdater || (typeof chrome !== 'undefined' && chrome.tabs ? chrome.tabs.update : null);
    const urlGetter = getUrlFn || (typeof chrome !== 'undefined' && chrome.runtime ? chrome.runtime.getURL : null);
    const offlinePage = urlGetter ? urlGetter('offline/offline.html') : 'offline/offline.html';
    if (updater && typeof details.tabId !== 'undefined') {
      updater(details.tabId, { url: offlinePage });
      return true;
    }
  }
  return false;
}

// Register Chrome Extension Lifecycle Listeners
if (typeof chrome !== 'undefined') {
  // Intercept failed navigations to the local dashboard (127.0.0.1:18800)
  // and replace the browser's "This site can’t be reached" error page with our offline placeholder screen
  if (chrome.webNavigation && chrome.webNavigation.onErrorOccurred) {
    chrome.webNavigation.onErrorOccurred.addListener((details) => {
      handleNavigationError(details);
    });
  }
  // Download creation interception
  if (chrome.downloads && chrome.downloads.onCreated) {
    chrome.downloads.onCreated.addListener((item) => {
      handleDownloadCreated(item).catch(() => {});
    });
  }

  // Intercept filename determination to suppress native OS File Explorer dialogs
  if (chrome.downloads && chrome.downloads.onDeterminingFilename) {
    chrome.downloads.onDeterminingFilename.addListener((item, suggest) => {
      handleDeterminingFilename(item, suggest).catch(() => {
        // Never leave Chrome waiting for a filename callback.
        try {
          suggest();
        } catch (_) {}
      });
      return true;
    });
  }

  // Context menus setup
  if (chrome.contextMenus && chrome.contextMenus.create) {
    try {
      chrome.contextMenus.removeAll(() => {
        chrome.contextMenus.create({
          id: 'conduit-download-link',
          title: 'Download with Conduit',
          contexts: ['link']
        });
        chrome.contextMenus.create({
          id: 'conduit-download-media',
          title: 'Download Media with Conduit',
          contexts: ['image', 'video', 'audio']
        });
      });
    } catch (_) {}
  }

  if (chrome.contextMenus && chrome.contextMenus.onClicked) {
    chrome.contextMenus.onClicked.addListener((info, tab) => {
      const targetUrl = info.linkUrl || info.srcUrl;
      if (targetUrl) {
        handleManualDownload(targetUrl, tab ? tab.url : '').catch(() => {});
      }
    });
  }

  // Periodic health check alarm
  if (chrome.alarms && chrome.alarms.onAlarm) {
    chrome.alarms.onAlarm.addListener((alarm) => {
      if (alarm.name === ALARM_NAME) {
        flushHeldQueue().catch(() => {});
      }
    });
  }

  // Initialization on install / startup
  if (chrome.runtime && chrome.runtime.onInstalled) {
    chrome.runtime.onInstalled.addListener(async () => {
      await pruneHeldQueue();
      const data = await getHeldQueue();
      await updateBadge(data.length);
      if (chrome.alarms) {
        chrome.alarms.create(ALARM_NAME, { periodInMinutes: 0.5 });
      }
    });
  }

  if (chrome.runtime && chrome.runtime.onStartup) {
    chrome.runtime.onStartup.addListener(async () => {
      // Firefox numbers downloads from 1 again each session, so IDs saved
      // last session would wrongly bypass new downloads. They only matter
      // while that download runs, and browser restarts end it.
      bypassedDownloadIds.clear();
      await chrome.storage.local.remove('bypassedIds');
      await pruneHeldQueue();
      const data = await getHeldQueue();
      await updateBadge(data.length);
      // Alarms are not guaranteed to survive a browser restart. Conduit often
      // starts at login alongside the browser, so keep retrying.
      syncRetryAlarm(data.length);
      await flushHeldQueue();
    });
  }

  // Communication message listener (from popup)
  if (chrome.runtime && chrome.runtime.onMessage) {
    chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
      if (!message || !message.type) return false;

      if (message.type === 'GET_STATUS') {
        (async () => {
          await pruneHeldQueue();
          const isOnline = await checkDaemonHealth();
          const queue = await getHeldQueue();
          let interceptionEnabled = true;
          let excludedDomains = [];
          const stored = await chrome.storage.local.get(['interceptionEnabled', 'excludedDomains']);
          if (typeof stored.interceptionEnabled === 'boolean') {
            interceptionEnabled = stored.interceptionEnabled;
          }
          if (Array.isArray(stored.excludedDomains)) {
            excludedDomains = stored.excludedDomains;
          }
          sendResponse({ isOnline, heldQueue: queue, interceptionEnabled, excludedDomains });
        })();
        return true; // Keep channel open for async response
      }

      if (message.type === 'GET_ACTIVE_DOWNLOADS') {
        (async () => {
          const isOnline = await checkDaemonHealth();
          if (!isOnline) {
            const queue = await getHeldQueue();
            await updateBadge({ isOnline: false, heldCount: queue.length });
            sendResponse({ success: false, isOnline: false, downloads: [], activeList: [], activeSpeed: 0, activeCount: 0 });
            return;
          }

          try {
            const res = await fetch(DAEMON_DOWNLOADS_URL);
            if (!res.ok) {
              sendResponse({ success: false, isOnline: true, downloads: [], activeList: [], activeSpeed: 0, activeCount: 0 });
              return;
            }
            const items = await res.json();
            const list = Array.isArray(items) ? items : [];
            const activeList = list.filter((d) => d.status === 'active');
            let totalSpeed = 0;
            activeList.forEach((d) => {
              totalSpeed += Number(d.downloadSpeed) || 0;
            });

            const queue = await getHeldQueue();
            await updateBadge({ isOnline: true, activeCount: activeList.length, heldCount: queue.length });

            sendResponse({
              success: true,
              isOnline: true,
              downloads: list,
              activeList: activeList,
              activeSpeed: totalSpeed,
              activeCount: activeList.length
            });
          } catch (_) {
            sendResponse({ success: false, isOnline: false, downloads: [], activeList: [], activeSpeed: 0, activeCount: 0 });
          }
        })();
        return true;
      }

      if (message.type === 'DOWNLOAD_ACTION') {
        (async () => {
          if (!message.gid || !message.action) {
            sendResponse({ success: false, error: 'Missing gid or action' });
            return;
          }
          try {
            const res = await fetch(`${DAEMON_DOWNLOADS_URL}/${encodeURIComponent(message.gid)}/action`, {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ action: message.action })
            });
            sendResponse({ success: res.ok });
          } catch (err) {
            sendResponse({ success: false, error: err.message });
          }
        })();
        return true;
      }

      if (message.type === 'TOGGLE_DOMAIN_EXCLUSION') {
        (async () => {
          const domain = (message.domain || '').trim().toLowerCase();
          if (!domain) {
            sendResponse({ success: false });
            return;
          }
          const stored = await chrome.storage.local.get(['excludedDomains']);
          let list = Array.isArray(stored.excludedDomains) ? stored.excludedDomains : [];
          if (list.includes(domain)) {
            list = list.filter((d) => d !== domain);
          } else {
            list.push(domain);
          }
          await chrome.storage.local.set({ excludedDomains: list });
          sendResponse({ success: true, excludedDomains: list, isExcluded: list.includes(domain) });
        })();
        return true;
      }

      if (message.type === 'GET_EXCLUDED_DOMAINS') {
        (async () => {
          const stored = await chrome.storage.local.get(['excludedDomains']);
          const list = Array.isArray(stored.excludedDomains) ? stored.excludedDomains : [];
          sendResponse({ excludedDomains: list });
        })();
        return true;
      }

      if (message.type === 'FLUSH_QUEUE') {
        (async () => {
          const count = await flushHeldQueue();
          const queue = await getHeldQueue();
          sendResponse({ flushed: count, heldQueue: queue });
        })();
        return true;
      }

      if (message.type === 'RESUME_BROWSER') {
        (async () => {
          const ok = await resumeInBrowser(message.downloadId);
          const queue = await getHeldQueue();
          sendResponse({ success: ok, heldQueue: queue });
        })();
        return true;
      }

      if (message.type === 'CANCEL_HELD') {
        (async () => {
          const ok = await cancelHeldDownload(message.downloadId);
          const queue = await getHeldQueue();
          sendResponse({ success: ok, heldQueue: queue });
        })();
        return true;
      }

      if (message.type === 'TOGGLE_INTERCEPTION') {
        (async () => {
          const enabled = Boolean(message.enabled);
          await chrome.storage.local.set({ interceptionEnabled: enabled });
          sendResponse({ success: true, interceptionEnabled: enabled });
        })();
        return true;
      }

      return false;
    });
  }
}

// Export for Node.js unit testing
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    isInterceptable,
    isDomainExcluded,
    formatCookiesHeader,
    extractMergedCookies,
    extractFilename,
    checkDaemonHealth,
    dispatchToDaemon,
    updateBadge,
    flashCompletionBadge,
    handleManualDownload,
    getHeldQueue,
    saveHeldQueue,
    pruneHeldQueue,
    processDownloadInterception,
    handleDownloadCreated,
    flushHeldQueue,
    resumeInBrowser,
    cancelHeldDownload,
    bypassedDownloadIds,
    handlingDownloadIds,
    interceptedDownloadIds,
    handleNavigationError,
    handleDeterminingFilename,
    triggerInPageToast,
    DAEMON_HEALTH_URL,
    DAEMON_DOWNLOADS_URL,
    ALARM_NAME
  };
}
