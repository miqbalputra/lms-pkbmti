// Apply the saved theme before paint without requiring an inline CSP exception.
(function () {
  try {
    if (localStorage.getItem('theme') === 'dark') {
      document.documentElement.classList.add('dark');
    }
  } catch {}
})();
