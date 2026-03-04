import html10n from './i18n';
import type {PadLike} from './pad_modals';

const localize = (element: Element): void => {
  if (element instanceof HTMLElement) {
    html10n.translateElement(html10n.translations, element);
  }
};

const createCountDownElementsIfNecessary = (modal: HTMLElement): void => {
  if (modal.querySelector('#cancelreconnect') != null) return;

  const defaultMessage = modal.querySelector('#defaulttext');
  const reconnectButton = modal.querySelector('#forcereconnect');
  if (defaultMessage == null || reconnectButton == null) return;

  const reconnectTimerMessage = document.createElement('p');
  reconnectTimerMessage.classList.add('reconnecttimer');

  const reconnectText = document.createElement('span');
  reconnectText.setAttribute('data-l10n-id', 'pad.modals.reconnecttimer');
  reconnectText.textContent = 'Trying to reconnect in';

  const timeToExpire = document.createElement('span');
  timeToExpire.classList.add('timetoexpire');

  reconnectTimerMessage.append(reconnectText, ' ', timeToExpire);

  const cancelReconnect = document.createElement('button');
  cancelReconnect.id = 'cancelreconnect';
  cancelReconnect.setAttribute('data-l10n-id', 'pad.modals.cancel');
  cancelReconnect.textContent = 'Cancel';

  localize(reconnectTimerMessage);
  localize(cancelReconnect);

  defaultMessage.insertAdjacentElement('afterend', reconnectTimerMessage);
  reconnectButton.insertAdjacentElement('afterend', cancelReconnect);
};

const toggleAutomaticReconnectionOption = (modal: HTMLElement, disableAutomaticReconnect: boolean): void => {
  for (const element of modal.querySelectorAll('#cancelreconnect, .reconnecttimer')) {
    element.classList.toggle('hidden', disableAutomaticReconnect);
  }
  const defaultText = modal.querySelector('#defaulttext');
  if (defaultText != null) defaultText.classList.toggle('hidden', !disableAutomaticReconnect);
};

const disableAutomaticReconnection = (modal: HTMLElement): void => {
  toggleAutomaticReconnectionOption(modal, true);
};

const enableAutomaticReconnection = (modal: HTMLElement): void => {
  toggleAutomaticReconnectionOption(modal, false);
};

const waitUntilClientCanConnectToServerAndThen = (callback: () => void, pad: PadLike): void => {
  if (pad.socket != null && reconnectionTries.counter === 1) {
    pad.socket.once('connect', callback);
  }
  pad.socket?.connect();
};

const forceReconnection = (modal: HTMLElement): void => {
  const reconnectButton = modal.querySelector<HTMLButtonElement>('#forcereconnect');
  reconnectButton?.click();
};

const updateCountDownTimerMessage = (modal: HTMLElement, minutes: number, seconds: number): void => {
  const safeMinutes = minutes < 10 ? `0${minutes}` : `${minutes}`;
  const safeSeconds = seconds < 10 ? `0${seconds}` : `${seconds}`;
  const target = modal.querySelector<HTMLElement>('.timetoexpire');
  if (target != null) target.textContent = `${safeMinutes}:${safeSeconds}`;
};

const reconnectionTries = {
  counter: 0,
  nextTry(): number {
    const nextCounterFactor = 2 ** this.counter;
    this.counter++;
    return nextCounterFactor;
  },
};

class CountDownTimer {
  private readonly duration: number;
  private readonly granularity: number;
  private running = false;
  private timeoutId?: number;
  private readonly onTickCallbacks: Array<(minutes: number, seconds: number) => void> = [];
  private readonly onExpireCallbacks: Array<() => void> = [];

  constructor(duration: number, granularity = 1000) {
    this.duration = duration;
    this.granularity = granularity;
  }

  start(): void {
    if (this.running) return;
    this.running = true;
    const start = Date.now();
    const timer = () => {
      const diff = this.duration - Math.floor((Date.now() - start) / 1000);
      if (diff > 0) {
        this.timeoutId = window.setTimeout(timer, this.granularity);
        this.tick(diff);
      } else {
        this.running = false;
        this.tick(0);
        this.expire();
      }
    };
    timer();
  }

  private tick(diff: number): void {
    const {minutes, seconds} = CountDownTimer.parse(diff);
    for (const callback of this.onTickCallbacks) callback(minutes, seconds);
  }

  private expire(): void {
    for (const callback of this.onExpireCallbacks) callback();
  }

  onTick(callback: (minutes: number, seconds: number) => void): CountDownTimer {
    this.onTickCallbacks.push(callback);
    return this;
  }

  onExpire(callback: () => void): CountDownTimer {
    this.onExpireCallbacks.push(callback);
    return this;
  }

  cancel(): CountDownTimer {
    this.running = false;
    if (this.timeoutId != null) clearTimeout(this.timeoutId);
    return this;
  }

  static parse(seconds: number): {minutes: number; seconds: number} {
    return {
      minutes: (seconds / 60) | 0,
      seconds: (seconds % 60) | 0,
    };
  }
}

const createTimerForModal = (modal: HTMLElement, pad: PadLike): CountDownTimer => {
  const timeout = Number(window.clientVars.automaticReconnectionTimeout ?? 0);
  const timeUntilReconnection = timeout * reconnectionTries.nextTry();
  const timer = new CountDownTimer(timeUntilReconnection);

  timer.onTick((minutes, seconds) => {
    updateCountDownTimerMessage(modal, minutes, seconds);
  }).onExpire(() => {
    const wasNetworkError = modal.classList.contains('disconnected');
    if (wasNetworkError) {
      waitUntilClientCanConnectToServerAndThen(() => {
        forceReconnection(modal);
      }, pad);
    } else {
      forceReconnection(modal);
    }
  }).start();

  return timer;
};

export const showCountDownTimerToReconnectOnModal = (modal: HTMLElement, pad: PadLike | undefined): void => {
  const timeout = Number(window.clientVars.automaticReconnectionTimeout ?? 0);
  if (!timeout || !modal.classList.contains('with_reconnect_timer') || pad == null) return;

  createCountDownElementsIfNecessary(modal);
  const timer = createTimerForModal(modal, pad);

  const cancelReconnect = modal.querySelector<HTMLButtonElement>('#cancelreconnect');
  cancelReconnect?.addEventListener('click', () => {
    timer.cancel();
    disableAutomaticReconnection(modal);
  }, {once: true});

  enableAutomaticReconnection(modal);
};
