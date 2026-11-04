// Paint the saved colour scheme and tab icon before the first frame. The daemon's setting replaces them once loaded.
try {
  var savedTheme = localStorage.getItem('conduit-theme');
  if (savedTheme === 'light' || savedTheme === 'dark') {
    document.documentElement.dataset.theme = savedTheme;
    document.getElementById('appIcon').href = '/favicon-' + savedTheme + '.svg';
  }
} catch (e) {
  // Storage may be refused; the page then follows the system.
}
