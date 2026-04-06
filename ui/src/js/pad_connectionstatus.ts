import {padmodals} from './pad_modals';

type ConnectionState =
  | {what: 'connecting'}
  | {what: 'connected'}
  | {what: 'reconnecting'}
  | {what: 'disconnected'; why: unknown};

type ReconnectableSocket = {
  forceReconnect?: () => void;
};

let status: ConnectionState = {what: 'connecting'};
let socketReference: ReconnectableSocket | null = null;

export const padconnectionstatus = {
  init: (socket: ReconnectableSocket): void => {
    socketReference = socket;
    const reconnectButton = document.querySelector<HTMLButtonElement>('button#forcereconnect');
    reconnectButton?.addEventListener('click', () => {
      if (socketReference?.forceReconnect != null) {
        socketReference.forceReconnect();
        padconnectionstatus.reconnecting();
      } else {
        window.location.reload();
      }
    });
  },
  connected: (): void => {
    status = {what: 'connected'};
    padmodals.showModal('connected');
    padmodals.hideOverlay();
  },
  reconnecting: (): void => {
    status = {what: 'reconnecting'};
    padmodals.showModal('reconnecting');
    padmodals.showOverlay();
  },
  disconnected: (msg: unknown): void => {
    if (status.what === 'disconnected') return;
    status = {what: 'disconnected', why: msg};

    const knownReasons = new Set([
      'badChangeset',
      'corruptPad',
      'deleted',
      'disconnected',
      'initsocketfail',
      'kicked',
      'looping',
      'rateLimited',
      'rejected',
      'reconnect_timeout',
      'slowcommit',
      'unauth',
      'userdup',
    ]);
    const reason = String(msg);
    padmodals.showModal(knownReasons.has(reason) ? reason : 'disconnected');
    padmodals.showOverlay();
  },
  isFullyConnected: (): boolean => status.what === 'connected',
  getStatus: (): ConnectionState => status,
};
