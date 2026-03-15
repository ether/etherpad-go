const RECENT_PADS_STORAGE_KEY = 'recentPads';
const MAX_RECENT_PADS = 8;

const randomPadName = () => {
  // the number of distinct chars (64) is chosen to ensure that the selection will be uniform when
  // using the PRNG below
  const chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-_';
  // the length of the pad name is chosen to get 120-bit security: log2(64^20) = 120
  const stringLength = 20;
  // make room for 8-bit integer values that span from 0 to 255.
  const randomarray = new Uint8Array(stringLength);
  crypto.getRandomValues(randomarray);
  let randomstring = '';
  for (let i = 0; i < stringLength; i++) {
    const rnum = Math.floor(randomarray[i] / 4);
    randomstring += chars.substring(rnum, rnum + 1);
  }
  return randomstring;
};

const byId = (id) => {
  const element = document.getElementById(id);
  if (element == null) throw new Error(`Element #${id} not found`);
  return element;
};

const getUiString = (key, vars = {}) => {
  const source = document.body.dataset[key] ?? '';
  return source
      .replace(/\{\{\s*([^{}]+)\s*\}\}/g, (full, name) => `${vars[name.trim()] ?? full}`)
      .replace(/\{([^{}]+)\}/g, (full, name) => `${vars[name.trim()] ?? full}`);
};

const createPadUrl = (padName) => `/p/${encodeURIComponent(padName)}`;

const parsePadMembers = (value) => {
  const members = Number.parseInt(`${value ?? 0}`, 10);
  return Number.isFinite(members) && members > 0 ? members : 0;
};

const normalizeRecentPad = (entry) => {
  if (typeof entry === 'string') {
    const name = entry.trim();
    if (!name) return null;
    return {name, url: createPadUrl(name), lastVisited: 0, members: 0};
  }

  if (entry == null || typeof entry !== 'object') return null;

  const name = typeof entry.name === 'string' ? entry.name.trim() : '';
  if (!name) return null;

  const lastVisited = Number.parseInt(`${entry.lastVisited ?? entry.updatedAt ?? 0}`, 10);
  const url = typeof entry.url === 'string' && entry.url.length > 0 ? entry.url : createPadUrl(name);

  return {
    name,
    url,
    lastVisited: Number.isFinite(lastVisited) ? lastVisited : 0,
    members: parsePadMembers(entry.members),
  };
};

const loadRecentPads = () => {
  try {
    const raw = localStorage.getItem(RECENT_PADS_STORAGE_KEY);
    if (raw == null) return [];

    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];

    const deduped = new Map();
    parsed.forEach((entry) => {
      const normalized = normalizeRecentPad(entry);
      if (!normalized) return;

      const existing = deduped.get(normalized.name);
      if (!existing || existing.lastVisited < normalized.lastVisited) deduped.set(normalized.name, normalized);
    });

    return Array.from(deduped.values())
        .sort((left, right) => right.lastVisited - left.lastVisited)
        .slice(0, MAX_RECENT_PADS);
  } catch {
    return [];
  }
};

const saveRecentPads = (recentPads) => {
  try {
    localStorage.setItem(RECENT_PADS_STORAGE_KEY, JSON.stringify(recentPads.slice(0, MAX_RECENT_PADS)));
  } catch {
    // Ignore storage failures so the welcome screen still works in restricted browsers.
  }
};

const formatRelativeTime = (timestamp) => {
  if (!timestamp) return getUiString('openedBefore');

  const elapsed = timestamp - Date.now();
  const absElapsed = Math.abs(elapsed);
  const units = [
    ['day', 1000 * 60 * 60 * 24],
    ['hour', 1000 * 60 * 60],
    ['minute', 1000 * 60],
  ];
  const formatter = new Intl.RelativeTimeFormat(navigator.language, {numeric: 'auto'});

  for (const [unit, unitSize] of units) {
    if (absElapsed >= unitSize || unit === 'minute') {
      const value = Math.round(elapsed / unitSize);
      return formatter.format(value, unit);
    }
  }

  return new Intl.DateTimeFormat(navigator.language, {
    month: 'short',
    day: 'numeric',
  }).format(new Date(timestamp));
};

const renderRecentPads = () => {
  const list = document.getElementById('recentPadsList');
  const emptyState = document.getElementById('recentPadsEmpty');
  const historyCount = document.getElementById('historyCount');
  const historyMembers = document.getElementById('historyMembers');

  const recentPads = loadRecentPads();
  const totalMembers = recentPads.reduce((sum, pad) => sum + pad.members, 0);

  if (historyCount) historyCount.textContent = `${recentPads.length}`;
  if (historyMembers) historyMembers.textContent = `${totalMembers}`;
  if (!list || !emptyState) return;

  list.replaceChildren();

  if (recentPads.length === 0) {
    emptyState.hidden = false;
    return;
  }

  emptyState.hidden = true;

  recentPads.forEach((pad) => {
    const item = document.createElement('a');
    item.className = 'recent-pad';
    item.href = pad.url;

    const top = document.createElement('div');
    top.className = 'recent-pad__top';

    const name = document.createElement('span');
    name.className = 'recent-pad__name';
    name.textContent = pad.name;

    const resume = document.createElement('span');
    resume.className = 'recent-pad__cta';
    resume.textContent = getUiString('resumePad');

    top.append(name, resume);

    const meta = document.createElement('div');
    meta.className = 'recent-pad__meta';

    const lastVisited = document.createElement('span');
    lastVisited.className = 'meta-pill';
    lastVisited.textContent = formatRelativeTime(pad.lastVisited);

    const members = document.createElement('span');
    members.className = 'meta-pill';
    members.textContent = pad.members > 0
      ? getUiString('membersOnlineTemplate', {count: pad.members})
      : getUiString('lastSeenHere');

    meta.append(lastVisited, members);
    item.append(top, meta);
    list.append(item);
  });
};

const updateFormHint = (message, isError = false) => {
  const formHint = document.getElementById('formHint');
  if (!formHint) return;
  formHint.textContent = message;
  formHint.classList.toggle('is-error', isError);
};

const openPad = (padName) => {
  window.location.href = createPadUrl(padName);
};

const initWelcomeScreen = () => {
  const goToNameForm = byId('go2Name');
  const padNameInput = byId('padname');
  const randomButton = byId('button');
  const clearHistoryButton = document.getElementById('clearHistory');
  const suggestionButtons = Array.from(document.querySelectorAll('[data-pad-suggestion]'));

  const submitPadName = () => {
    const padname = padNameInput.value.trim();
    if (padname.length === 0) {
      padNameInput.classList.add('is-invalid');
      padNameInput.focus();
      updateFormHint(getUiString('formHintError'), true);
      return;
    }

    updateFormHint(getUiString('formHint'), false);
    openPad(padname);
  };

  goToNameForm.addEventListener('submit', (event) => {
    event.preventDefault();
    submitPadName();
  });

  padNameInput.addEventListener('input', () => {
    if (padNameInput.value.trim().length > 0) {
      padNameInput.classList.remove('is-invalid');
      updateFormHint(getUiString('formHint'), false);
    }
  });

  randomButton.addEventListener('click', () => {
    openPad(randomPadName());
  });

  suggestionButtons.forEach((button) => {
    button.addEventListener('click', () => {
      const suggestion = button.getAttribute('data-pad-suggestion');
      if (!suggestion) return;
      padNameInput.value = suggestion;
      padNameInput.classList.remove('is-invalid');
      updateFormHint(getUiString('formHint'), false);
      padNameInput.focus();
      padNameInput.select();
    });
  });

  clearHistoryButton?.addEventListener('click', () => {
    saveRecentPads([]);
    renderRecentPads();
  });

  renderRecentPads();
  if (typeof window.customStart === 'function') window.customStart();
};

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', initWelcomeScreen, {once: true});
} else {
  initWelcomeScreen();
}
