// Conduit Download Interceptor - Content Script (In-Page Initiation Toast)

(function () {
  'use strict';

  if (window.__conduitContentScriptInjected) return;
  window.__conduitContentScriptInjected = true;

  // Listen for messages from background service worker
  if (typeof chrome !== 'undefined' && chrome.runtime && chrome.runtime.onMessage) {
    chrome.runtime.onMessage.addListener((message) => {
      if (message && message.type === 'CONDUIT_TOAST_INITIATED') {
        showInitiationToast(message.filename || 'Download', message.fileSize);
      }
    });
  }

  function showInitiationToast(filename, fileSize) {
    // The popup and offline pages keep the theme choice in extension storage. This script cannot read their localStorage.
    chrome.storage.local.get(['conduitTheme'], (data) => {
      renderInitiationToast(filename, fileSize, data && data.conduitTheme ? data.conduitTheme : 'system');
    });
  }

  function renderInitiationToast(filename, fileSize, theme) {
    // Check if host element already exists
    let host = document.getElementById('conduit-toast-host');
    if (!host) {
      host = document.createElement('div');
      host.id = 'conduit-toast-host';
      host.style.position = 'fixed';
      host.style.zIndex = '2147483647';
      host.style.bottom = '20px';
      host.style.right = '20px';
      host.style.pointerEvents = 'none';
      document.body.appendChild(host);
    }

    const shadow = host.shadowRoot || host.attachShadow({ mode: 'open' });

    // Toast element
    const toast = document.createElement('div');
    toast.className = 'conduit-toast';
    toast.dataset.theme = theme;

    const sizeStr = formatBytes(fileSize);
    const sizeMeta = sizeStr ? ` (${sizeStr})` : '';

    toast.innerHTML = `
      <style>
        .conduit-toast {
          background-color: #FFFFFF;
          color: #111111;
          border: 1px solid #CFCFC8;
          border-radius: 0;
          padding: 8px 14px;
          font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
          font-size: 13px;
          line-height: 1.4;
          display: flex;
          align-items: center;
          gap: 9px;
          opacity: 0;
          transform: translateY(10px);
          transition: opacity 0.25s ease, transform 0.25s ease;
          max-width: 380px;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .conduit-toast.visible {
          opacity: 1;
          transform: translateY(0);
        }
        @media (prefers-color-scheme: dark) {
          .conduit-toast:not([data-theme="light"]) {
            background-color: #1A1A1A;
            color: #F4F4F0;
            border-color: #3A3A38;
          }
        }
        .conduit-toast[data-theme="dark"] {
          background-color: #1A1A1A;
          color: #F4F4F0;
          border-color: #3A3A38;
        }
        .conduit-icon {
          flex-shrink: 0;
          width: 15px;
          height: 15px;
        }
        .conduit-text {
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .conduit-bold {
          font-weight: 600;
        }
      </style>
      <svg class="conduit-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M12 3v13m0 0l-4-4m4 4l4-4"/>
        <path d="M4 17v2a2 2 0 002 2h12a2 2 0 002-2v-2"/>
      </svg>
      <span class="conduit-text">Conduit: Downloading <span class="conduit-bold">${escapeHtml(filename)}</span>${sizeMeta}</span>
    `;

    shadow.appendChild(toast);

    // Trigger smooth fade-in
    requestAnimationFrame(() => {
      toast.classList.add('visible');
    });

    // Auto fade-out and cleanup after 2.5 seconds
    setTimeout(() => {
      toast.classList.remove('visible');
      setTimeout(() => {
        if (toast.parentNode === shadow) {
          shadow.removeChild(toast);
        }
        if (shadow.childNodes.length === 0 && host.parentNode) {
          host.parentNode.removeChild(host);
        }
      }, 300);
    }, 2500);
  }

  function formatBytes(bytes) {
    if (!bytes || bytes <= 0) return '';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
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
})();
