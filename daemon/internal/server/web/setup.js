(function () {
  'use strict';

  const params = new URLSearchParams(window.location.search);
  const shell = params.get('shell') === 'app';
  const folderStep = document.getElementById('stepFolder');
  const extensionStep = document.getElementById('stepExtension');
  const extensionTitle = document.getElementById('extensionTitle');
  const folderForm = document.getElementById('folderForm');
  const downloadDir = document.getElementById('downloadDir');
  const openGuideBtn = document.getElementById('openGuideBtn');
  const skipBtn = document.getElementById('skipBtn');
  const continueBtn = document.getElementById('continueBtn');
  const errorEl = document.getElementById('setupError');

  function queueURL() {
    return shell ? '/?shell=app' : '/';
  }

  function showError(message) {
    if (errorEl) errorEl.textContent = message || '';
  }

  async function readConfig() {
    const res = await fetch('/api/config');
    if (!res.ok) {
      throw new Error('Could not load settings');
    }
    return res.json();
  }

  async function writeConfig(cfg) {
    const res = await fetch('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    });
    if (!res.ok) {
      throw new Error('Could not save settings');
    }
    return res.json();
  }

  async function finishSetup() {
    const cfg = await readConfig();
    cfg.setupComplete = true;
    await writeConfig(cfg);
    window.location.replace(queueURL());
  }

  // The folder step leaves upward. The extension step rises into its place once the first has gone.
  function showExtensionStep() {
    folderStep.inert = true;
    ConduitMotion.leave(folderStep, () => {
      extensionStep.classList.remove('hidden');
      ConduitMotion.enter(extensionStep, 12);
      // The folder step held focus and has just hidden. The new step's heading takes it, so keyboard users keep their place.
      extensionTitle.focus({ preventScroll: true });
    });
  }

  folderForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    showError('');
    const dir = downloadDir.value.trim();
    if (!dir) {
      showError('Enter a download folder.');
      return;
    }
    try {
      const cfg = await readConfig();
      cfg.downloadDir = dir;
      await writeConfig(cfg);
      showExtensionStep();
    } catch (err) {
      showError(err.message);
    }
  });

  openGuideBtn.addEventListener('click', async () => {
    showError('');
    try {
      const res = await fetch('/api/extension/open-guide', { method: 'POST' });
      if (!res.ok) {
        throw new Error('Could not open the browser.');
      }
      continueBtn.classList.remove('hidden');
    } catch (err) {
      showError(err.message);
    }
  });

  skipBtn.addEventListener('click', async () => {
    showError('');
    try {
      await finishSetup();
    } catch (err) {
      showError(err.message);
    }
  });

  continueBtn.addEventListener('click', async () => {
    showError('');
    try {
      await finishSetup();
    } catch (err) {
      showError(err.message);
    }
  });

  readConfig().then((cfg) => {
    if (cfg.setupComplete !== false) {
      window.location.replace(queueURL());
      return;
    }
    downloadDir.value = cfg.downloadDir || '';
  }).catch((err) => {
    showError(err.message);
  });
})();
