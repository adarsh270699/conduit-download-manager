// Automated Test Suite for Conduit Extension Engine
// Run with: node extension/test/test_extension.js

const assert = require('assert');

// 1. Setup Mock Browser Extension Environment
const mockStorageData = {};
let mockDownloadItems = [];
const mockCalls = {
  downloads: { paused: [], resumed: [], cancelled: [], erased: [], searched: [] },
  badges: [],
  notifications: [],
  alarms: [],
  fetches: [],
  tabMessages: []
};

global.navigator = {
  userAgent: 'Mozilla/5.0 (MockBrowser/1.0) ConduitTest'
};

global.chrome = {
  downloads: {
    pause: async (id) => {
      mockCalls.downloads.paused.push(id);
    },
    resume: async (id) => {
      mockCalls.downloads.resumed.push(id);
    },
    cancel: async (id) => {
      mockCalls.downloads.cancelled.push(id);
    },
    erase: async (query) => {
      mockCalls.downloads.erased.push(query.id);
    },
    search: async (query) => {
      mockCalls.downloads.searched.push(query);
      if (query && typeof query.id !== 'undefined') {
        return mockDownloadItems.filter((d) => d.id === query.id);
      }
      return mockDownloadItems;
    },
    onCreated: {
      addListener: () => {}
    }
  },
  cookies: {
    getAll: async ({ url }) => {
      if (url.includes('example.com')) {
        return [
          { name: 'session_id', value: 'sess_abc123' },
          { name: 'auth_token', value: 'tok_xyz987' }
        ];
      }
      if (url.includes('ankergames.net')) {
        return [{ name: 'csrf_token', value: 'csrf_secret_111' }];
      }
      if (url.includes('dlproxy.uk')) {
        return [{ name: 'storage_auth', value: 'tunnel_token_222' }];
      }
      return [];
    }
  },
  storage: {
    local: {
      get: async (keys) => {
        const result = {};
        const keyList = Array.isArray(keys) ? keys : [keys];
        for (const k of keyList) {
          result[k] = mockStorageData[k];
        }
        return result;
      },
      set: async (obj) => {
        Object.assign(mockStorageData, obj);
      }
    },
    onChanged: {
      addListener: () => {}
    }
  },
  action: {
    setBadgeText: async ({ text }) => {
      mockCalls.badges.push(text);
    },
    setBadgeBackgroundColor: async () => {}
  },
  notifications: {
    create: (opts) => {
      mockCalls.notifications.push(opts);
    }
  },
  alarms: {
    create: (name, opts) => {
      mockCalls.alarms.push({ name, opts });
    },
    onAlarm: {
      addListener: () => {}
    }
  },
  tabs: {
    query: async () => [{ id: 1 }],
    sendMessage: async (tabId, msg) => {
      mockCalls.tabMessages.push({ tabId, msg });
    }
  },
  runtime: {
    onInstalled: { addListener: () => {} },
    onStartup: { addListener: () => {} },
    onMessage: { addListener: () => {} }
  }
};

// Mock fetch handler
let mockFetchHandler = null;
global.fetch = async (url, opts) => {
  mockCalls.fetches.push({ url, opts });
  if (mockFetchHandler) {
    return mockFetchHandler(url, opts);
  }
  throw new Error('Network error: Daemon offline');
};

// Import background script module
const bg = require('../background.js');

async function runTests() {
  console.log('--- Conduit Extension Test Suite ---');

  // Test 1: URL Filtering Rules
  console.log('Test 1: isInterceptable filter rules...');
  assert.strictEqual(
    bg.isInterceptable({ id: 1, url: 'https://example.com/file.zip' }),
    true,
    'HTTPS download should be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 2, url: 'http://example.com/file.iso' }),
    true,
    'HTTP download should be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 3, url: 'ftp://example.com/file.tar.gz' }),
    true,
    'FTP download should be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 4, url: 'blob:https://example.com/uuid' }),
    false,
    'blob: URLs must be ignored'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 5, url: 'data:text/plain;base64,SGVsbG8=' }),
    false,
    'data: URLs must be ignored'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 6, url: 'chrome-extension://xyz/app.js' }),
    false,
    'Extension URLs must be ignored'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 7, url: 'filesystem:https://example.com/temporary/file' }),
    false,
    'filesystem: URLs must be ignored'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 8, url: 'https://example.com/small.png', totalBytes: 500 * 1024 }),
    false,
    'Files smaller than 1MB when size is known should be ignored'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 9, url: 'https://example.com/large.iso', totalBytes: 50 * 1024 * 1024 }),
    true,
    'Files larger than 1MB should be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 10, url: 'https://example.com/stream.bin', totalBytes: -1 }),
    true,
    'Files with unknown size (-1) should be intercepted'
  );

  const testBypassSet = new Set([99]);
  assert.strictEqual(
    bg.isInterceptable({ id: 99, url: 'https://example.com/resume.zip' }, true, testBypassSet),
    false,
    'Bypassed download ID must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 100, url: 'https://example.com/toggle.zip' }, false),
    false,
    'Downloads when interception is disabled must be ignored'
  );

  // Historical & state-based filtering
  assert.strictEqual(
    bg.isInterceptable({ id: 101, url: 'https://example.com/done.iso', state: 'complete' }),
    false,
    'Completed downloads must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 102, url: 'https://example.com/stopped.iso', state: 'interrupted' }),
    false,
    'Interrupted downloads must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 103, url: 'https://example.com/ended.iso', endTime: '2026-10-19T10:00:00Z' }),
    false,
    'Ended downloads must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({ id: 104, url: 'https://example.com/old.iso', startTime: '2026-01-01T00:00:00Z' }),
    false,
    'Historical downloads (>2 mins old) must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable({
      id: 105,
      url: 'https://example.com/fresh.iso',
      state: 'in_progress',
      startTime: new Date().toISOString()
    }),
    true,
    'Fresh active in_progress download must be intercepted'
  );
  console.log('✓ Test 1 Passed: Filter rules verified.');

  // Test 2: Cookie Header Formatting
  console.log('Test 2: formatCookiesHeader serialization...');
  assert.strictEqual(bg.formatCookiesHeader([]), '');
  assert.strictEqual(bg.formatCookiesHeader(null), '');
  assert.strictEqual(
    bg.formatCookiesHeader([
      { name: 'session', value: '123' },
      { name: 'theme', value: 'dark' }
    ]),
    'session=123; theme=dark'
  );
  console.log('✓ Test 2 Passed: Cookie serialization verified.');

  // Test 3: Filename Extraction
  console.log('Test 3: extractFilename logic...');
  assert.strictEqual(
    bg.extractFilename({ url: 'https://example.com/path/conduit_test.iso', filename: 'C:\\Downloads\\conduit_test.iso' }),
    'conduit_test.iso'
  );
  assert.strictEqual(
    bg.extractFilename({ url: 'https://example.com/path/archive.tar.gz?auth=1', filename: '' }),
    'archive.tar.gz'
  );
  assert.strictEqual(
    bg.extractFilename({ url: 'https://example.com/downloads%20file.pdf', filename: '' }),
    'downloads file.pdf'
  );
  console.log('✓ Test 3 Passed: Filename extraction verified.');

  // Test 4: Online Branch Interception
  console.log('Test 4: Online branch interception (daemon online)...');
  mockCalls.downloads.paused = [];
  mockCalls.downloads.cancelled = [];
  mockCalls.downloads.erased = [];
  mockStorageData.heldQueue = [];

  mockFetchHandler = async (url, opts) => {
    if (url.includes('/api/health')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({ app: 'conduit', status: 'ok', version: '1.0.0' })
      };
    }
    if (url.includes('/api/downloads')) {
      const body = JSON.parse(opts.body);
      assert.strictEqual(body.url, 'https://example.com/archive.zip');
      assert.strictEqual(body.cookies, 'session_id=sess_abc123; auth_token=tok_xyz987');
      return {
        ok: true,
        status: 201,
        json: async () => ({ gid: 'gid_online_001' })
      };
    }
    throw new Error('Unknown endpoint ' + url);
  };

  await bg.handleDownloadCreated({
    id: 101,
    url: 'https://example.com/archive.zip',
    filename: 'archive.zip',
    referrer: 'https://example.com/list',
    totalBytes: 50 * 1024 * 1024
  });

  assert.ok(mockCalls.downloads.paused.includes(101), 'Browser download should have been paused');
  assert.ok(mockCalls.downloads.cancelled.includes(101), 'Browser download should be cancelled after daemon accepts');
  assert.ok(mockCalls.downloads.erased.includes(101), 'Browser download shelf entry should be erased');
  assert.strictEqual((mockStorageData.heldQueue || []).length, 0, 'No item should be left in heldQueue');
  mockFetchHandler = async () => ({
    ok: true,
    json: async () => ({ app: 'conduit', status: 'ok', engine: 'offline' })
  });
  assert.strictEqual(await bg.checkDaemonHealth(), false, 'A daemon with an offline engine must be treated as unavailable');
  console.log('✓ Test 4 Passed: Online dispatch cleanly handed off to daemon.');

  // Test 5: Offline Branch Interception (Daemon offline)
  console.log('Test 5: Offline branch holding queue (daemon offline)...');
  mockCalls.downloads.paused = [];
  mockCalls.downloads.cancelled = [];
  mockCalls.badges = [];
  mockCalls.notifications = [];
  mockCalls.alarms = [];
  mockStorageData.heldQueue = [];

  // Simulate offline daemon (fetch throws)
  mockFetchHandler = null;

  await bg.handleDownloadCreated({
    id: 202,
    url: 'https://example.com/heavy_linux.iso',
    filename: 'heavy_linux.iso',
    referrer: 'https://example.com/distro',
    totalBytes: 800 * 1024 * 1024
  });

  assert.ok(mockCalls.downloads.paused.includes(202), 'Browser download must be paused while held');
  assert.strictEqual(mockCalls.downloads.cancelled.includes(202), false, 'Download must NOT be cancelled while offline');
  assert.strictEqual(mockStorageData.heldQueue.length, 1, 'Job must be held in heldQueue storage');
  assert.strictEqual(mockStorageData.heldQueue[0].browserDownloadId, 202);
  assert.strictEqual(mockStorageData.heldQueue[0].filename, 'heavy_linux.iso');
  assert.strictEqual(mockCalls.badges[mockCalls.badges.length - 1], '1', 'Badge must show count of held jobs');
  assert.ok(mockCalls.notifications.length > 0, 'Notification must alert user that daemon is offline');
  assert.ok(mockCalls.alarms.some((a) => a.name === bg.ALARM_NAME), 'Polling alarm must be registered');

  // Two offline downloads arriving together must both survive the queue
  // read-modify-write, not overwrite each other.
  mockStorageData.heldQueue = [];
  await Promise.all([
    bg.handleDownloadCreated({
      id: 203,
      url: 'https://example.com/parallel-a.iso',
      filename: 'parallel-a.iso',
      totalBytes: 10 * 1024 * 1024
    }),
    bg.handleDownloadCreated({
      id: 204,
      url: 'https://example.com/parallel-b.iso',
      filename: 'parallel-b.iso',
      totalBytes: 10 * 1024 * 1024
    })
  ]);
  assert.strictEqual(mockStorageData.heldQueue.length, 2, 'Concurrent offline downloads must both be queued');
  mockStorageData.heldQueue = [];
  console.log('✓ Test 5 Passed: Offline holding state machine captured and held job.');

  // Test 6: Automatic Queue Recovery (Daemon comes online)
  console.log('Test 6: Queue recovery and automatic flush...');
  mockCalls.downloads.cancelled = [];
  mockCalls.downloads.erased = [];
  mockCalls.notifications = [];

  // Daemon is now online
  mockFetchHandler = async (url, opts) => {
    if (url.includes('/api/health')) {
      return { ok: true, status: 200, json: async () => ({ app: 'conduit', status: 'ok' }) };
    }
    if (url.includes('/api/downloads')) {
      return { ok: true, status: 201, json: async () => ({ gid: 'gid_flushed_202' }) };
    }
    throw new Error('Unknown URL');
  };

  mockStorageData.heldQueue = [{
    browserDownloadId: 202,
    url: 'https://example.com/heavy_linux.iso',
    filename: 'heavy_linux.iso',
    referrer: 'https://example.com/distro',
    cookies: '',
    userAgent: 'Mozilla/5.0',
    fileSize: 800 * 1024 * 1024,
    timestamp: Date.now()
  }];
  mockDownloadItems = [{ id: 202, state: 'in_progress', paused: true }];
  const flushedCount = await bg.flushHeldQueue();
  assert.strictEqual(flushedCount, 1, 'One job should be flushed');
  mockDownloadItems = [];
  assert.ok(mockCalls.downloads.cancelled.includes(202), 'Held browser download must be cancelled upon flush');
  assert.strictEqual(mockStorageData.heldQueue.length, 0, 'Held queue must be cleared');
  assert.strictEqual(mockCalls.badges[mockCalls.badges.length - 1], '', 'Badge must be cleared');
  console.log('✓ Test 6 Passed: Automatic queue flush forwarded held downloads.');

  // Test 7: Browser Fallback Resume ("Download via Browser Instead")
  console.log('Test 7: Resume in browser fallback and anti-looping bypass...');
  mockCalls.downloads.paused = [];
  mockCalls.downloads.resumed = [];
  mockStorageData.heldQueue = [
    {
      browserDownloadId: 303,
      url: 'https://example.com/fallback.zip',
      filename: 'fallback.zip',
      fileSize: 12345
    }
  ];

  const resumed = await bg.resumeInBrowser(303);
  assert.strictEqual(resumed, true, 'Resume in browser should succeed');
  assert.ok(mockCalls.downloads.resumed.includes(303), 'chrome.downloads.resume must be called');
  assert.strictEqual(mockStorageData.heldQueue.length, 0, 'Item must be removed from held queue');
  assert.ok(bg.bypassedDownloadIds.has(303), 'Download ID 303 must be in bypassed set');

  // Verify that an item in the bypass set is NEVER re-intercepted
  assert.strictEqual(
    bg.isInterceptable({ id: 303, url: 'https://example.com/fallback.zip' }),
    false,
    'Resumed download must never be re-intercepted'
  );
  console.log('✓ Test 7 Passed: Browser fallback resume and bypass prevent infinite loops.');

  // Test 8: Prune Completed and Missing Downloads from heldQueue
  console.log('Test 8: pruneHeldQueue cleans up historical and finished downloads...');
  mockStorageData.heldQueue = [
    { browserDownloadId: 401, url: 'https://example.com/done.zip', filename: 'done.zip' },
    { browserDownloadId: 402, url: 'https://example.com/active.zip', filename: 'active.zip' },
    { browserDownloadId: 403, url: 'https://example.com/missing.zip', filename: 'missing.zip' }
  ];
  mockDownloadItems = [
    { id: 401, state: 'complete', url: 'https://example.com/done.zip' },
    { id: 402, state: 'in_progress', paused: true, url: 'https://example.com/active.zip' }
    // 403 is missing from browser
  ];

  const pruned = await bg.pruneHeldQueue();
  assert.strictEqual(pruned.length, 1, 'Only active downloads should remain in heldQueue');
  assert.strictEqual(pruned[0].browserDownloadId, 402, 'Active download 402 should be preserved');
  assert.strictEqual(mockStorageData.heldQueue.length, 1, 'mockStorageData.heldQueue must be synced to length 1');
  assert.strictEqual(mockStorageData.heldQueue[0].browserDownloadId, 402);
  console.log('✓ Test 8 Passed: Held queue correctly prunes completed and ghost downloads.');

  // Test 9: handleNavigationError intercepts daemon navigation failures
  console.log('Test 9: handleNavigationError redirects to offline placeholder...');
  let redirectedTabId = null;
  let redirectedUrl = null;
  const mockTabUpdater = (tabId, updateProps) => {
    redirectedTabId = tabId;
    redirectedUrl = updateProps.url;
  };
  const mockGetUrl = (path) => `chrome-extension://conduit-test/${path}`;

  // Case 9a: Main frame navigation to 127.0.0.1:18800 fails
  const handled127 = bg.handleNavigationError(
    { frameId: 0, tabId: 99, url: 'http://127.0.0.1:18800/api/downloads' },
    mockTabUpdater,
    mockGetUrl
  );
  assert.strictEqual(handled127, true, 'Navigation error on 127.0.0.1:18800 must be intercepted');
  assert.strictEqual(redirectedTabId, 99);
  assert.strictEqual(redirectedUrl, 'chrome-extension://conduit-test/offline/offline.html');

  // Case 9b: Main frame navigation to localhost:18800 fails
  const handledLocalhost = bg.handleNavigationError(
    { frameId: 0, tabId: 100, url: 'http://localhost:18800/' },
    mockTabUpdater,
    mockGetUrl
  );
  assert.strictEqual(handledLocalhost, true, 'Navigation error on localhost:18800 must be intercepted');
  assert.strictEqual(redirectedTabId, 100);

  // Case 9c: Subframe navigation should not redirect the entire tab
  const handledSubframe = bg.handleNavigationError(
    { frameId: 1, tabId: 101, url: 'http://127.0.0.1:18800/' },
    mockTabUpdater,
    mockGetUrl
  );
  assert.strictEqual(handledSubframe, false, 'Subframe navigation errors must not trigger full tab redirect');

  // Case 9d: Unrelated URL errors should not be intercepted
  const handledUnrelated = bg.handleNavigationError(
    { frameId: 0, tabId: 102, url: 'https://example.com/other' },
    mockTabUpdater,
    mockGetUrl
  );
  assert.strictEqual(handledUnrelated, false, 'Unrelated URLs must not be intercepted');
  console.log('✓ Test 9 Passed: Connection errors to daemon port cleanly redirect to offline placeholder.');

  // Test 10: Per-site domain exclusion rules
  console.log('Test 10: Per-site domain exclusion logic...');
  assert.strictEqual(
    bg.isDomainExcluded('https://github.com/archive.zip', ['github.com']),
    true,
    'Exact domain match should be excluded'
  );
  assert.strictEqual(
    bg.isDomainExcluded('https://raw.githubusercontent.com/file.tar.gz', ['githubusercontent.com']),
    true,
    'Subdomain should match parent excluded domain'
  );
  assert.strictEqual(
    bg.isDomainExcluded('https://gitlab.com/file.zip', ['github.com']),
    false,
    'Non-excluded domain must not be excluded'
  );
  assert.strictEqual(
    bg.isInterceptable(
      { id: 501, url: 'https://speedtest.net/speedtest.bin', totalBytes: 10 * 1024 * 1024 },
      true,
      new Set(),
      ['speedtest.net']
    ),
    false,
    'Excluded domain download must not be intercepted'
  );
  assert.strictEqual(
    bg.isInterceptable(
      { id: 502, url: 'https://other-site.org/archive.tar.gz', totalBytes: 10 * 1024 * 1024 },
      true,
      new Set(),
      ['speedtest.net']
    ),
    true,
    'Non-excluded domain download should be intercepted'
  );
  console.log('✓ Test 10 Passed: Per-site domain exclusion rules verified.');

  // Test 11: Dynamic toolbar badge states
  console.log('Test 11: Dynamic toolbar badge states...');
  mockCalls.badges.length = 0;
  await bg.updateBadge({ isOnline: true, activeCount: 3, heldCount: 0 });
  assert.strictEqual(mockCalls.badges[mockCalls.badges.length - 1], '3', 'Active count badge should show 3');

  await bg.updateBadge({ isOnline: false, activeCount: 0, heldCount: 5 });
  assert.strictEqual(mockCalls.badges[mockCalls.badges.length - 1], '5', 'Offline held badge should show 5');

  await bg.updateBadge({ isOnline: true, activeCount: 0, heldCount: 0 });
  assert.strictEqual(mockCalls.badges[mockCalls.badges.length - 1], '', 'Idle state badge should be cleared');
  console.log('✓ Test 11 Passed: Dynamic toolbar badge states verified.');

  // Test 12: Manual context menu download
  console.log('Test 12: Manual context menu download dispatch...');
  // Case 12a: Daemon online
  mockFetchHandler = async (url) => {
    if (url.includes('/api/health')) {
      return { ok: true, json: async () => ({ app: 'conduit', status: 'ok' }) };
    }
    if (url.includes('/api/downloads')) {
      return { status: 201, ok: true, json: async () => ({ gid: 'gid-manual-123' }) };
    }
    throw new Error('404');
  };

  const manualResult = await bg.handleManualDownload('https://example.com/manual-file.zip', 'https://example.com/page');
  assert.strictEqual(manualResult.success, true);
  assert.strictEqual(manualResult.gid, 'gid-manual-123');
  assert.strictEqual(
    mockCalls.tabMessages.some((m) => m.msg && m.msg.type === 'CONDUIT_TOAST_INITIATED'),
    true,
    'In-page toast should be emitted to active tab for started download'
  );

  // Case 12b: Daemon offline
  mockFetchHandler = async () => {
    throw new Error('Connection refused');
  };
  mockStorageData.heldQueue = [];
  const manualOfflineResult = await bg.handleManualDownload('https://example.com/offline-doc.pdf');
  assert.strictEqual(manualOfflineResult.success, true);
  assert.strictEqual(manualOfflineResult.held, true);
  assert.strictEqual(mockStorageData.heldQueue.length, 1);
  assert.strictEqual(mockStorageData.heldQueue[0].filename, 'offline-doc.pdf');
  assert.strictEqual(mockStorageData.heldQueue[0].manual, true, 'Manual jobs must be marked as browserless');
  assert.strictEqual(
    typeof mockStorageData.heldQueue[0].browserDownloadId,
    'string',
    'Manual jobs must not impersonate a numeric browser download ID'
  );
  const prunedManual = await bg.pruneHeldQueue();
  assert.strictEqual(prunedManual.length, 1, 'Manual queue jobs must survive pruning');
  mockStorageData.heldQueue = [];
  console.log('✓ Test 12 Passed: Manual context menu download dispatch verified.');

  // Test 13: Native Save As dialog suppression via handleDeterminingFilename
  console.log('Test 13: Native Save As dialog suppression via handleDeterminingFilename...');
  mockCalls.downloads.cancelled.length = 0;
  mockCalls.downloads.erased.length = 0;
  let suggestCalled = false;
  const mockSuggest = () => { suggestCalled = true; };

  // Set daemon online for Save As suppression test
  mockFetchHandler = async (url) => {
    if (url.includes('/api/health')) return { ok: true, json: async () => ({ app: 'conduit', status: 'ok' }) };
    if (url.includes('/api/downloads')) return { status: 201, ok: true, json: async () => ({ gid: 'gid-901' }) };
    throw new Error('404');
  };

  // Case 13a: Interceptable download should be fast-cancelled to kill OS file dialog
  await bg.handleDeterminingFilename({ id: 901, url: 'https://example.com/bigfile.iso' }, mockSuggest);
  assert.strictEqual(mockCalls.downloads.cancelled.includes(901), true, 'Interceptable download 901 must be fast-cancelled to kill OS file picker');
  assert.strictEqual(mockCalls.downloads.erased.includes(901), true, 'Download 901 must be erased from browser list');
  assert.strictEqual(suggestCalled, false, 'Suggest should not be called when fast-cancelling');

  // Case 13b: Non-interceptable download (e.g. blob) should pass through to suggest()
  suggestCalled = false;
  await bg.handleDeterminingFilename({ id: 902, url: 'blob:https://example.com/blob-data' }, mockSuggest);
  assert.strictEqual(suggestCalled, true, 'Non-interceptable download should proceed to browser suggest()');

  // Case 13c: Save dialog stays with the browser when the daemon does not take the file.
  // The download must start. It must not remain paused.
  mockCalls.downloads.paused = [];
  mockCalls.downloads.resumed = [];
  mockCalls.downloads.cancelled = [];
  mockStorageData.heldQueue = [];
  suggestCalled = false;
  mockFetchHandler = async (url) => {
    if (url.includes('/api/health')) return { ok: true, status: 200, json: async () => ({ app: 'conduit', status: 'ok' }) };
    return { ok: false, status: 500, json: async () => ({ error: 'rejected' }) };
  };
  await bg.handleDeterminingFilename({
    id: 903,
    url: 'https://s4.snapyyt.app/video.mp4',
    filename: 'Leciercene-SnapYT-App.mp4',
    totalBytes: 284 * 1024 * 1024
  }, mockSuggest);
  assert.strictEqual(suggestCalled, true, 'Save dialog must be allowed to finish');
  assert.strictEqual(mockCalls.downloads.paused.includes(903), false, 'Browser download must not be paused when the user saves it');
  assert.strictEqual(mockCalls.downloads.cancelled.includes(903), false, 'Browser download must not be cancelled');
  assert.strictEqual((mockStorageData.heldQueue || []).length, 0, 'A saved browser download must not sit in the held queue');

  // Case 13d: Chromium always provides suggest(). When the daemon is offline
  // the download must still be paused and held instead of silently released.
  mockCalls.downloads.paused = [];
  mockCalls.downloads.resumed = [];
  mockCalls.downloads.cancelled = [];
  mockStorageData.heldQueue = [];
  suggestCalled = false;
  mockDownloadItems = [{ id: 904, state: 'in_progress', paused: true }];
  mockFetchHandler = null;
  await bg.handleDeterminingFilename({
    id: 904,
    url: 'https://example.com/held-by-chrome.iso',
    filename: 'held-by-chrome.iso',
    totalBytes: 120 * 1024 * 1024
  }, mockSuggest);

  assert.strictEqual(suggestCalled, true, 'Chrome suggest() must be called before the download can be paused');
  assert.ok(mockCalls.downloads.paused.includes(904), 'Chrome download must be paused when the daemon is offline');
  assert.strictEqual(mockCalls.downloads.cancelled.includes(904), false, 'Held Chrome download must not be cancelled');
  assert.strictEqual((mockStorageData.heldQueue || []).length, 1, 'Chrome download must enter the held queue');
  assert.strictEqual(mockStorageData.heldQueue[0].browserDownloadId, 904);
  mockStorageData.heldQueue = [];
  mockDownloadItems = [];
  console.log('✓ Test 13 Passed: Native Save As dialog suppression verified.');

  // Test 14: Redirect Resolution & IDM-Style Handoff via handleDeterminingFilename
  console.log('Test 14: Redirect resolution, cookie merging, and finalUrl capture...');
  let dispatchedPayload = null;
  mockFetchHandler = async (url, opts) => {
    if (url.includes('/api/health')) return { ok: true, json: async () => ({ app: 'conduit', status: 'ok' }) };
    if (url.includes('/api/downloads')) {
      dispatchedPayload = JSON.parse(opts.body);
      return { status: 201, ok: true, json: async () => ({ gid: 'gid-redirect-14' }) };
    }
    throw new Error('404');
  };

  const redirectItem = {
    id: 1401,
    url: 'https://ankergames.net/download-file/b9b6daba29ced239bd9ad0bcf7c83ac46a5321b7468da74c',
    finalUrl: 'https://tunnel5.dlproxy.uk/download/nGw5A-kxTG7-bukYw?sig=xyz',
    filename: 'Call-of-Duty-Modern-Warfare-4-AnkerGames.zip',
    totalBytes: 5 * 1024 * 1024 * 1024,
    referrer: ''
  };

  await bg.handleDeterminingFilename(redirectItem, () => {});
  assert.ok(dispatchedPayload, 'Download should be dispatched to daemon');
  assert.strictEqual(
    dispatchedPayload.url,
    'https://tunnel5.dlproxy.uk/download/nGw5A-kxTG7-bukYw?sig=xyz',
    'Daemon must receive the resolved finalUrl, not the initial token URL'
  );
  assert.strictEqual(
    dispatchedPayload.filename,
    'Call-of-Duty-Modern-Warfare-4-AnkerGames.zip',
    'Daemon must receive the Content-Disposition filename'
  );
  assert.strictEqual(
    dispatchedPayload.referrer,
    'https://ankergames.net/download-file/b9b6daba29ced239bd9ad0bcf7c83ac46a5321b7468da74c',
    'Initial URL should be used as referrer for storage proxy tunnel'
  );
  assert.ok(
    dispatchedPayload.cookies.includes('storage_auth=tunnel_token_222') &&
    dispatchedPayload.cookies.includes('csrf_token=csrf_secret_111'),
    'Cookies from both target domain and initiating domain must be merged'
  );
  console.log('✓ Test 14 Passed: Redirect resolution, cookie merging, and finalUrl capture verified.');

  // Test 15: Firefox compatibility fallback (onDeterminingFilename undefined)
  console.log('Test 15: Firefox fallback path when onDeterminingFilename is undefined...');
  let ffDispatched = false;
  mockFetchHandler = async (url) => {
    if (url.includes('/api/health')) return { ok: true, json: async () => ({ app: 'conduit', status: 'ok' }) };
    if (url.includes('/api/downloads')) {
      ffDispatched = true;
      return { status: 201, ok: true, json: async () => ({ gid: 'gid-ff-15' }) };
    }
    throw new Error('404');
  };

  delete global.chrome.downloads.onDeterminingFilename;

  await bg.handleDownloadCreated({
    id: 1501,
    url: 'https://example.com/firefox-download.tar.gz',
    filename: 'firefox-download.tar.gz',
    totalBytes: 10 * 1024 * 1024
  });

  assert.strictEqual(ffDispatched, true, 'Download must be dispatched immediately in Firefox');
  console.log('✓ Test 15 Passed: Firefox fallback path verified.');

  // Test 16: Overlapping flushes (alarm + popup Retry) send each job once
  console.log('Test 16: Concurrent flushes do not dispatch a held job twice...');
  let posts16 = 0;
  mockFetchHandler = async (url, opts) => {
    if (url.includes('/api/health')) return { ok: true, status: 200, json: async () => ({ app: 'conduit', status: 'ok' }) };
    if (url.includes('/api/downloads') && opts && opts.method === 'POST') {
      posts16++;
      await new Promise((r) => setTimeout(r, 20));
      return { ok: true, status: 201, json: async () => ({ gid: 'gid-16' }) };
    }
    throw new Error('404');
  };
  mockStorageData.heldQueue = [{ browserDownloadId: 'manual-16', manual: true, url: 'https://example.com/a16.zip', filename: 'a16.zip' }];
  const [f1, f2] = await Promise.all([bg.flushHeldQueue(), bg.flushHeldQueue()]);
  assert.strictEqual(posts16, 1, 'A held job must be sent to Conduit exactly once');
  assert.strictEqual(f1 + f2, 2, 'Both callers share the same flush result');
  assert.strictEqual(mockStorageData.heldQueue.length, 0, 'Queue must be empty after the shared flush');
  console.log('✓ Test 16 Passed: Overlapping flushes share one run.');

  // Test 17: Health responses without the Conduit identity are not trusted
  console.log('Test 17: Unidentified local service is treated as offline...');
  mockFetchHandler = async () => ({ ok: true, status: 200, json: async () => ({ status: 'ok', version: '1.0.0', engine: 'ready' }) });
  assert.strictEqual(await bg.checkDaemonHealth(), false, 'A service without app=conduit must not receive downloads');
  console.log('✓ Test 17 Passed: Health identity is required.');

  console.log('\n======================================');
  console.log('ALL EXTENSION TESTS PASSED SUCCESSFULLY');
  console.log('======================================\n');
}

runTests().catch((err) => {
  console.error('Test Suite Failed:', err);
  process.exit(1);
});
