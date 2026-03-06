window.addEventListener('load', () => {
  if (document.body.dataset.resetPkce === 'true') {
    sessionStorage.removeItem('pkce_code_verifier');
    sessionStorage.removeItem('pkce_state');
  }
});
