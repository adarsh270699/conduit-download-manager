// Conduit colour scheme for the extension pages. The daemon's setting is the source of truth, the same one the
// dashboard reads. This copy lets a page paint the right scheme before its first frame, and it keeps a change made
// while the daemon is offline until the daemon is back to receive it.
(function (global) {
  'use strict';

  const KEY = 'conduit-theme';
  const PENDING_KEY = 'conduit-theme-pending';
  const CONFIG_URL = 'http://127.0.0.1:18800/api/config';
  const ORDER = ['system', 'light', 'dark'];
  const LABELS = { system: 'follows the system', light: 'light', dark: 'dark' };

  function normalize(value) {
    return ORDER.includes(value) ? value : 'system';
  }

  function read() {
    try {
      return normalize(localStorage.getItem(KEY));
    } catch (_) {
      return 'system';
    }
  }

  // Content scripts run on every page and cannot read this origin's localStorage. The choice is also kept in
  // extension storage, where the in-page toast reads it.
  function write(value) {
    try {
      localStorage.setItem(KEY, value);
    } catch (_) {
      // Storage may be refused; the choice still applies to this page.
    }
    if (typeof chrome !== 'undefined' && chrome.storage && chrome.storage.local) {
      chrome.storage.local.set({ conduitTheme: value }, () => void chrome.runtime.lastError);
    }
  }

  function isPending() {
    try {
      return localStorage.getItem(PENDING_KEY) === '1';
    } catch (_) {
      return false;
    }
  }

  function setPending(on) {
    try {
      if (on) localStorage.setItem(PENDING_KEY, '1');
      else localStorage.removeItem(PENDING_KEY);
    } catch (_) {
      // Storage may be refused; the change then reaches the daemon the next time it is made.
    }
  }

  function apply(value) {
    const choice = normalize(value);
    if (choice === 'system') {
      delete document.documentElement.dataset.theme;
    } else {
      document.documentElement.dataset.theme = choice;
    }
  }

  // The order a click moves through: system, light, dark, and back to system.
  function next(value) {
    const index = ORDER.indexOf(normalize(value));
    return ORDER[(index + 1) % ORDER.length];
  }

  // Save a choice on this page and mark it to be sent to the daemon.
  function save(value) {
    const choice = normalize(value);
    write(choice);
    setPending(true);
    apply(choice);
  }

  // Send a pending choice to the daemon. Resolves true when nothing is left pending.
  async function flush() {
    if (!isPending()) return true;
    try {
      const res = await fetch(CONFIG_URL, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ theme: read() }),
        cache: 'no-store'
      });
      if (!res.ok) return false;
      setPending(false);
      return true;
    } catch (_) {
      return false;
    }
  }

  // Take the daemon's setting, unless this page has a choice the daemon has not received yet. Resolves true
  // when the daemon answered.
  async function sync() {
    if (isPending()) return flush();
    try {
      const res = await fetch(CONFIG_URL, { cache: 'no-store' });
      if (!res.ok) return false;
      const cfg = await res.json();
      write(normalize(cfg.theme));
      apply(cfg.theme);
      return true;
    } catch (_) {
      return false;
    }
  }

  // Show the current choice on a theme button. The icon is chosen in CSS from data-theme-choice.
  function paintButton(button) {
    if (!button) return;
    const choice = read();
    button.dataset.themeChoice = choice;
    const label = `Theme: ${LABELS[choice]}. Select to change`;
    button.title = label;
    button.setAttribute('aria-label', label);
  }

  // Paint the saved scheme as soon as this script loads, before the stylesheet does.
  apply(read());

  global.ConduitTheme = { read, apply, next, save, flush, sync, paintButton };
})(window);
