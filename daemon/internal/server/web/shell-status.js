// The desktop window says Starting, not Connecting, until the dashboard has loaded.
if (new URLSearchParams(location.search).get('shell') === 'app') {
  document.getElementById('connectionStatusText').textContent = 'Starting';
}
