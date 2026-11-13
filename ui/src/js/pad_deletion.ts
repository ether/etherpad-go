// Pad deletion in the pad page (etherpad-lite #7926, #7959, #7996).
//
// - The creator gets the pad's deletion token once in CLIENT_VARS and is shown
//   a dialog to save it, or, with suppressPadDeletionTokenModal, can reveal it
//   in the settings (it is kept in this browser for that).
// - "Delete pad" is offered when this session may delete without a token
//   (clientVars.canDeletePad); "Delete pad with token" when a token is needed.
// - The page leaves only after the server confirmed the deletion, so a
//   refused deletion does not navigate away.

type DeletionVars = {
  padId: string;
  padDeletionToken?: string | null;
  canDeleteWithoutToken?: boolean;
  canDeletePad?: boolean;
  suppressPadDeletionTokenModal?: boolean;
};

type Deps = {
  vars: () => DeletionVars;
  t: (key: string) => string;
  confirm: (message: string) => boolean;
  alert: (message: string) => void;
  send: (msg: {type: 'PAD_DELETE', data: {padId: string, deletionToken?: string}}) => void;
  navigateHome: () => void;
  storage: () => Storage | null;
};

const byId = (id: string) => document.getElementById(id) as any;

const storageKey = (padId: string) => `padDeletionToken:${padId}`;

/** What the deletion UI shows for this session. */
export const deletionUiState = (vars: DeletionVars) => ({
  showDeleteButton: !!vars.canDeletePad,
  showTokenField: !vars.canDeleteWithoutToken,
  // The one-time dialog, unless the operator suppressed it.
  showTokenModal: !!vars.padDeletionToken && !vars.suppressPadDeletionTokenModal,
});

export const createPadDeletion = (deps: Deps) => {
  let pending = false;

  const saveToken = (token: string) => {
    try { deps.storage()?.setItem(storageKey(deps.vars().padId), token); } catch (_e) { /* no storage */ }
  };
  const savedToken = (): string | null => {
    try { return deps.storage()?.getItem(storageKey(deps.vars().padId)) ?? null; } catch (_e) { return null; }
  };

  const copy = async (text: string, input: HTMLInputElement | null) => {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch (_e) {
      input?.select();
      try { return document.execCommand('copy'); } catch (_e2) { return false; }
    }
  };

  const showTokenModal = (token: string) => {
    const modal = byId('deletiontoken-modal');
    const input = byId('deletiontoken-value') as HTMLInputElement | null;
    if (!modal || !input) return;
    input.value = token;
    const previouslyFocused = document.activeElement as HTMLElement | null;
    modal.hidden = false;
    modal.classList.add('popup-show');
    setTimeout(() => input.focus(), 0);
    byId('deletiontoken-copy')?.addEventListener('click', async () => {
      if (await copy(token, input)) byId('deletiontoken-copy').textContent = deps.t('pad.deletionToken.copied');
    });
    byId('deletiontoken-ack')?.addEventListener('click', () => {
      input.value = '';
      modal.hidden = true;
      modal.classList.remove('popup-show');
      deps.vars().padDeletionToken = null;
      if (previouslyFocused && document.body.contains(previouslyFocused)) previouslyFocused.focus();
    }, {once: true});
  };

  const revealInSettings = (token: string) => {
    const row = byId('delete-pad-token-reveal');
    const toggle = byId('delete-pad-token-toggle');
    const panel = byId('delete-pad-token-reveal-panel');
    const input = byId('delete-pad-token-reveal-value') as HTMLInputElement | null;
    const status = byId('delete-pad-token-reveal-status');
    if (!row || !toggle || !panel || !input) return;
    row.hidden = false;
    toggle.addEventListener('ep-change', async (e: CustomEvent) => {
      const on = e.detail?.checked ?? toggle.checked;
      if (status) status.textContent = '';
      if (!on) {
        input.value = '';
        panel.hidden = true;
        return;
      }
      input.value = token;
      panel.hidden = false;
      input.select();
      if (await copy(token, input) && status) status.textContent = deps.t('pad.deletionToken.copied');
    });
  };

  const requestDeletion = (deletionToken?: string) => {
    if (!deps.confirm(deps.t('pad.delete.confirm'))) return;
    pending = true;
    const data: {padId: string, deletionToken?: string} = {padId: deps.vars().padId};
    if (deletionToken) data.deletionToken = deletionToken;
    deps.send({type: 'PAD_DELETE', data});
  };

  return {
    /** Sets up the controls and the token dialog; call once CLIENT_VARS is in. */
    init: () => {
      const vars = deps.vars();
      const state = deletionUiState(vars);
      const button = byId('delete-pad');
      if (button) {
        button.hidden = !state.showDeleteButton;
        button.addEventListener('click', () => requestDeletion());
      }
      const withToken = byId('delete-pad-with-token');
      if (withToken) withToken.hidden = !state.showTokenField;
      byId('delete-pad-token-submit')?.addEventListener('click', () => {
        const token = String(byId('delete-pad-token-input')?.value ?? '').trim();
        if (token) requestDeletion(token);
      });

      if (vars.suppressPadDeletionTokenModal) {
        if (vars.padDeletionToken) saveToken(vars.padDeletionToken);
        const token = vars.padDeletionToken || savedToken();
        // Only where this browser may delete: a token saved for a pad that
        // was deleted and recreated by someone else is not offered.
        if (token && vars.canDeletePad) revealInSettings(token);
      } else if (state.showTokenModal) {
        showTokenModal(vars.padDeletionToken as string);
      }
    },

    /** The server deleted the pad. Returns true when this page asked for it. */
    handleDeleted: () => {
      try { deps.storage()?.removeItem(storageKey(deps.vars().padId)); } catch (_e) { /* no storage */ }
      if (!pending) return false;
      pending = false;
      deps.navigateHome();
      return true;
    },

    /** A refusal (shout with a pad.deletionToken.* key). Returns true if handled. */
    handleShout: (messageKey: unknown) => {
      if (typeof messageKey !== 'string' || !messageKey.startsWith('pad.deletionToken.')) return false;
      pending = false;
      deps.alert(deps.t(messageKey));
      return true;
    },

    isPending: () => pending,
  };
};
