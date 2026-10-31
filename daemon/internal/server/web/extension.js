(function () {
  'use strict';

  const chooseChromium = document.getElementById('chooseChromium');
  const chooseFirefox = document.getElementById('chooseFirefox');
  const stepsChromium = document.getElementById('stepsChromium');
  const stepsFirefox = document.getElementById('stepsFirefox');
  const pathMissing = document.getElementById('pathMissing');
  const pathBlock = document.getElementById('pathBlock');
  const pathValue = document.getElementById('pathValue');
  const copyPathBtn = document.getElementById('copyPathBtn');
  const copyStatus = document.getElementById('copyStatus');
  let copyTimer = null;

  function resetCopyLabel() {
    copyPathBtn.textContent = 'Copy path';
    if (copyStatus) copyStatus.textContent = '';
  }

  function selectPathText() {
    const range = document.createRange();
    range.selectNodeContents(pathValue);
    const selection = window.getSelection();
    selection.removeAllRanges();
    selection.addRange(range);
  }

  function selectBrowser(name) {
    const chromium = name === 'chromium';
    stepsChromium.classList.toggle('hidden', !chromium);
    stepsFirefox.classList.toggle('hidden', chromium);
    chooseChromium.className = chromium ? 'btn btn-primary' : 'btn btn-secondary';
    chooseFirefox.className = chromium ? 'btn btn-secondary' : 'btn btn-primary';
    chooseChromium.setAttribute('aria-pressed', chromium ? 'true' : 'false');
    chooseFirefox.setAttribute('aria-pressed', chromium ? 'false' : 'true');
  }

  chooseChromium.addEventListener('click', function () {
    selectBrowser('chromium');
  });
  chooseFirefox.addEventListener('click', function () {
    selectBrowser('firefox');
  });

  copyPathBtn.addEventListener('click', async function () {
    const path = pathValue.textContent || '';
    if (!path) return;
    try {
      await navigator.clipboard.writeText(path);
      copyPathBtn.textContent = 'Copied';
      if (copyStatus) copyStatus.textContent = 'Path copied to the clipboard.';
    } catch (err) {
      copyPathBtn.textContent = 'Copy path';
      if (copyStatus) copyStatus.textContent = 'Copy failed. The path is selected. Press Ctrl+C to copy it.';
      selectPathText();
    }
    clearTimeout(copyTimer);
    copyTimer = setTimeout(resetCopyLabel, 4000);
  });

  fetch('/api/extension').then(function (res) {
    if (!res.ok) throw new Error('missing');
    return res.json();
  }).then(function (data) {
    if (!data || !data.exists || !data.path) {
      pathMissing.classList.remove('hidden');
      pathBlock.classList.add('hidden');
      return;
    }
    pathValue.textContent = data.path;
  }).catch(function () {
    pathMissing.classList.remove('hidden');
    pathBlock.classList.add('hidden');
  });
})();
