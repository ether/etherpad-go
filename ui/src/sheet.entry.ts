const root = document.getElementById('sheet-root');
if (root) {
  root.textContent = `Spreadsheet editor for "${root.dataset.padName ?? ''}" — coming soon.`;
}
