import {showCountDownTimerToReconnectOnModal} from './pad_automatic_reconnect';
import {padeditbar} from './pad_editbar';

export type PadLike = {
  socket?: {
    connect: () => void;
    once: (event: string, cb: () => void) => void;
  };
};

let currentPad: PadLike | undefined;

const getConnectivityMessage = (messageId: string): HTMLElement | null =>
  document.querySelector<HTMLElement>(`#connectivity .${messageId}`);

export const padmodals = {
  init: (pad: PadLike): void => {
    currentPad = pad;
  },
  showModal: (messageId: string): void => {
    padeditbar.toggleDropDown('none');
    for (const element of document.querySelectorAll('#connectivity .visible')) {
      element.classList.remove('visible');
    }

    const modal = getConnectivityMessage(messageId);
    if (modal == null) return;
    modal.classList.add('visible');
    showCountDownTimerToReconnectOnModal(modal, currentPad);
    padeditbar.toggleDropDown('connectivity');
  },
  showOverlay: (): void => {
    const overlay = document.getElementById('toolbar-overlay');
    if (overlay != null) overlay.style.display = '';
  },
  hideOverlay: (): void => {
    const overlay = document.getElementById('toolbar-overlay');
    if (overlay != null) overlay.style.display = 'none';
  },
};
