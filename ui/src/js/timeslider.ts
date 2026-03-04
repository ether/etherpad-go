import {Cookies, randomString} from './pad_utils';
import padutils from './pad_utils';
import {padimpexp} from './pad_impexp';
import html10n from './i18n';
import * as socketio from './socketio';
import * as hooks from './pluginfw/hooks';

type ServerMessage = {
  type?: string;
  accessStatus?: string;
  data?: Record<string, unknown>;
};

type SocketLike = {
  on: (event: string, cb: (arg: any) => void) => void;
  emit: (event: string, payload: unknown) => void;
  connect: () => void;
};

type BroadcastSliderLike = {
  showReconnectUI: () => void;
  onSlider: (cb: (rev: number) => void) => void;
};

let token = '';
let padId = '';
let exportLinks: HTMLAnchorElement[] = [];
export let socket: SocketLike | undefined;
export let BroadcastSlider: BroadcastSliderLike | undefined;
let changesetLoader: {handleMessageFromServer: (msg: ServerMessage) => void} | undefined;

export let baseURL = '';
export const setBaseURL = (url: string): void => {
  baseURL = url;
};

const waitForDocumentReady = async (): Promise<void> => {
  if (document.readyState !== 'loading') return;
  await new Promise<void>((resolve) => {
    document.addEventListener('DOMContentLoaded', () => resolve(), {once: true});
  });
};

const refreshSessionLifetime = async (): Promise<void> => {
  try {
    await fetch('../../_extendExpressSessionLifetime', {method: 'PUT'});
  } catch {
    // ignore keepalive failures
  }
};

const sendSocketMsg = (type: string, data: Record<string, unknown>): void => {
  socket?.emit('message', {
    component: 'pad',
    type,
    data,
    padId,
    token,
    sessionID: Cookies.get('sessionID'),
  });
};

const fireWhenAllScriptsAreLoaded: Array<() => void> = [];

const handleClientVars = async (message: ServerMessage): Promise<void> => {
  window.clientVars = (message.data ?? {}) as typeof window.clientVars;

  if (window.clientVars.sessionRefreshInterval) {
    window.setInterval(refreshSessionLifetime, Number(window.clientVars.sessionRefreshInterval));
  }

  if (window.clientVars.mode === 'development') {
    console.warn('Enabling development mode with live update');
    socket?.on('liveupdate', () => {
      console.log('Doing live reload');
      location.reload();
    });
  }

  const [{loadBroadcastSliderJS}, {loadBroadcastRevisionsJS}, {loadBroadcastJS}] = await Promise.all([
    import('./broadcast_slider'),
    import('./broadcast_revisions'),
    import('./broadcast'),
  ]);
  BroadcastSlider = loadBroadcastSliderJS(fireWhenAllScriptsAreLoaded) as BroadcastSliderLike;
  loadBroadcastRevisionsJS();
  changesetLoader = loadBroadcastJS(socket, sendSocketMsg, fireWhenAllScriptsAreLoaded, BroadcastSlider);

  padimpexp.init(undefined);

  const pathName = document.location.pathname;
  BroadcastSlider.onSlider((revno) => {
    for (const link of exportLinks) {
      if (!link.href) continue;
      const type = link.href.split('export/')[1];
      let href = pathName.split('timeslider')[0];
      href += `${revno}/export/${type}`;
      link.setAttribute('href', href);
    }
  });

  for (const startFn of fireWhenAllScriptsAreLoaded) startFn();

  const sliderHandle = document.getElementById('ui-slider-handle');
  const sliderBar = document.getElementById('ui-slider-bar');
  if (sliderHandle != null && sliderBar != null) {
    const width = sliderBar.getBoundingClientRect().width;
    sliderHandle.setAttribute('style', `left:${width - 2}px`);
  }

  const playPause = document.getElementById('playpause_button_icon');
  const leftStep = document.getElementById('leftstep');
  const rightStep = document.getElementById('rightstep');
  playPause?.setAttribute('title', html10n.get('timeslider.playPause'));
  leftStep?.setAttribute('title', html10n.get('timeslider.backRevision'));
  rightStep?.setAttribute('title', html10n.get('timeslider.forwardRevision'));

  const viewFontMenu = document.getElementById('viewfontmenu');
  viewFontMenu?.addEventListener('change', () => {
    const menu = viewFontMenu as HTMLSelectElement;
    const innerDocBody = document.getElementById('innerdocbody');
    if (innerDocBody instanceof HTMLElement) {
      innerDocBody.style.fontFamily = menu.value || '';
    }
  });
};

export const init = async (): Promise<void> => {
  padutils.setupGlobalExceptionHandler();
  await waitForDocumentReady();

  if (typeof window.customStart === 'function') window.customStart();

  const urlParts = document.location.pathname.split('/');
  padId = decodeURIComponent(urlParts[urlParts.length - 2]);
  document.title = `${padId.replace(/_+/g, ' ')} | ${document.title.replace('{{appTitle}}', '')}`;

  token = Cookies.get('token') ?? '';
  if (!token) {
    token = `t.${randomString()}`;
    Cookies.set('token', token, {expires: 60});
  }

  const currentSocket = socketio.connect(baseURL, '/', {query: {padId}}) as SocketLike;
  socket = currentSocket;
  currentSocket.on('connect', () => sendSocketMsg('CLIENT_READY', {}));
  currentSocket.on('disconnect', (reason: string) => {
    BroadcastSlider?.showReconnectUI();
    if (reason === 'io server disconnect') currentSocket.connect();
  });

  currentSocket.on('message', (message: ServerMessage) => {
    if (message.type === 'CLIENT_VARS') {
      void handleClientVars(message);
    } else if (message.accessStatus) {
      document.body.innerHTML = '<h2>You have no permission to access this pad</h2>';
    } else if (message.type === 'CHANGESET_REQ' || message.type === 'COLLABROOM') {
      changesetLoader?.handleMessageFromServer(message);
    }
  });

  exportLinks = Array.from(document.querySelectorAll<HTMLAnchorElement>('#export > .exportlink'));
  const forceReconnectButton = document.querySelector<HTMLButtonElement>('button#forcereconnect');
  forceReconnectButton?.addEventListener('click', () => {
    window.location.reload();
  });

  await hooks.aCallAll('postTimesliderInit', {});
};
