// @ts-nocheck

/**
 * Copyright 2009 Google Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS-IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import padutils from './pad_utils'
import * as hooks from './pluginfw/hooks';
import html10n from './i18n';
let myUserInfo = {};

let colorPickerOpen = false;
let colorPickerSetup = false;
const q = (selector) => document.querySelector(selector);
const qa = (selector) => Array.from(document.querySelectorAll(selector));
const RECENT_PADS_STORAGE_KEY = 'recentPads';
const MAX_RECENT_PADS = 8;
const toHexColor = (color: string): string => {
  if (/^#[0-9a-f]{6}$/i.test(color)) return color;
  const match = color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/i);
  if (!match) return '#000000';
  const parts = match.slice(1).map((part) => Number.parseInt(part, 10).toString(16).padStart(2, '0'));
  return `#${parts.join('')}`;
};

const getCurrentPadName = (): string => {
  const pathSegments = window.location.pathname.split('/');
  return decodeURIComponent(pathSegments[pathSegments.length - 1] || '');
};

const normalizeRecentPad = (entry) => {
  if (typeof entry === 'string') {
    const name = entry.trim();
    if (!name) return null;
    return {
      name,
      url: `/p/${encodeURIComponent(name)}`,
      lastVisited: 0,
      members: 0,
    };
  }

  if (!entry || typeof entry !== 'object') return null;
  const name = typeof entry.name === 'string' ? entry.name.trim() : '';
  if (!name) return null;

  const lastVisited = Number.parseInt(`${entry.lastVisited ?? entry.updatedAt ?? 0}`, 10);
  const members = Number.parseInt(`${entry.members ?? 0}`, 10);

  return {
    name,
    url: typeof entry.url === 'string' && entry.url.length > 0 ? entry.url : `/p/${encodeURIComponent(name)}`,
    lastVisited: Number.isFinite(lastVisited) ? lastVisited : 0,
    members: Number.isFinite(members) && members > 0 ? members : 0,
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
      if (!existing || existing.lastVisited < normalized.lastVisited) {
        deduped.set(normalized.name, normalized);
      }
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
    // Ignore storage failures so editing still works in restricted browsers.
  }
};

const syncRecentPad = (members: number): void => {
  const padName = getCurrentPadName();
  if (!padName) return;

  const recentPads = loadRecentPads();
  const padRecord = {
    name: padName,
    url: `/p/${encodeURIComponent(padName)}`,
    lastVisited: Date.now(),
    members,
  };

  const existingIndex = recentPads.findIndex((pad) => pad.name === padName);
  if (existingIndex >= 0) {
    recentPads.splice(existingIndex, 1);
  }

  recentPads.unshift(padRecord);
  saveRecentPads(recentPads);
};

export const paduserlist = (() => {
  const rowManager = (() => {
    // The row manager handles rendering rows of the user list and animating
    // their insertion, removal, and reordering.  It manipulates TD height
    // and TD opacity.

    const nextRowId = () => `usertr${nextRowId.counter++}`;
    nextRowId.counter = 1;
    // objects are shared; fields are "domId","data","animationStep"
    const rowsFadingOut = []; // unordered set
    const rowsFadingIn = []; // unordered set
    const rowsPresent = []; // in order
    const ANIMATION_START = -12; // just starting to fade in
    const ANIMATION_END = 12; // just finishing fading out

    const animateStep = () => {
      // animation must be symmetrical
      for (let i = rowsFadingIn.length - 1; i >= 0; i--) { // backwards to allow removal
        const row = rowsFadingIn[i];
        const step = ++row.animationStep;
        const animHeight = getAnimationHeight(step, row.animationPower);
        const node = rowNode(row);
        if (!node) continue;
        const baseOpacity = (row.opacity === undefined ? 1 : row.opacity);
        if (step <= -OPACITY_STEPS) {
          setTdHeight(node, animHeight);
        } else if (step === -OPACITY_STEPS + 1) {
          node.innerHTML = '';
          node.append(...createUserRowTds(animHeight, row.data));
          setTdOpacity(node, baseOpacity * 1 / OPACITY_STEPS);
        } else if (step < 0) {
          setTdOpacity(node, baseOpacity * (OPACITY_STEPS - (-step)) / OPACITY_STEPS);
          setTdHeight(node, animHeight);
        } else if (step === 0) {
          // set HTML in case modified during animation
          node.innerHTML = '';
          node.append(...createUserRowTds(animHeight, row.data));
          setTdOpacity(node, baseOpacity * 1);
          setTdHeight(node, animHeight);
          rowsFadingIn.splice(i, 1); // remove from set
        }
      }
      for (let i = rowsFadingOut.length - 1; i >= 0; i--) { // backwards to allow removal
        const row = rowsFadingOut[i];
        const step = ++row.animationStep;
        const node = rowNode(row);
        if (!node) continue;
        const animHeight = getAnimationHeight(step, row.animationPower);
        const baseOpacity = (row.opacity === undefined ? 1 : row.opacity);
        if (step < OPACITY_STEPS) {
          setTdOpacity(node, baseOpacity * (OPACITY_STEPS - step) / OPACITY_STEPS);
          setTdHeight(node, animHeight);
        } else if (step === OPACITY_STEPS) {
          node.innerHTML = '';
          node.append(...createEmptyRowTds(animHeight));
        } else if (step <= ANIMATION_END) {
          setTdHeight(node, animHeight);
        } else {
          rowsFadingOut.splice(i, 1); // remove from set
          node.remove();
        }
      }

      handleOtherUserInputs();

      return (rowsFadingIn.length > 0) || (rowsFadingOut.length > 0); // is more to do
    };

    const getAnimationHeight = (step, power) => {
      let a = Math.abs(step / 12);
      if (power === 2) a **= 2;
      else if (power === 3) a **= 3;
      else if (power === 4) a **= 4;
      else if (power >= 5) a **= 5;
      return Math.round(26 * (1 - a));
    };
    const OPACITY_STEPS = 6;

    const ANIMATION_STEP_TIME = 20;
    const LOWER_FRAMERATE_FACTOR = 2;
    const {scheduleAnimation} =
        padutils.makeAnimationScheduler(animateStep, ANIMATION_STEP_TIME, LOWER_FRAMERATE_FACTOR);

    const NUMCOLS = 4;

    // we do lots of manipulation of table rows and stuff that JQuery makes ok, despite
    // IE's poor handling when manipulating the DOM directly.

    const setTdHeight = (tr, height) => {
      tr.querySelectorAll('td').forEach((td) => {
        td.style.height = `${height}px`;
      });
    };

    const setTdOpacity = (tr, opacity) => {
      tr.querySelectorAll('td').forEach((td) => {
        td.style.opacity = `${opacity}`;
      });
    };

    const createEmptyRowTds = (height) => {
      const td = document.createElement('td');
      td.colSpan = NUMCOLS;
      td.style.border = '0';
      td.style.height = `${height}px`;
      return [td];
    };

    const isNameEditable = (data) => (!data.name) && (data.status !== 'Disconnected');

    const replaceUserRowContents = (tr, height, data) => {
      const tds = createUserRowTds(height, data);
      if (isNameEditable(data) && tr.querySelector('td.usertdname input:enabled')) {
        // preserve input field node
        tds.forEach((td, i) => {
          const oldTd = tr.querySelectorAll('td')[i];
          if (!oldTd?.classList.contains('usertdname')) oldTd?.replaceWith(td);
        });
      } else {
        tr.innerHTML = '';
        tr.append(...tds);
      }
      return tr;
    };

    const createUserRowTds = (height, data) => {
      let name;
      if (data.name) {
        name = document.createTextNode(data.name);
      } else {
        const input = document.createElement('input');
        input.setAttribute('data-l10n-id', 'pad.userlist.unnamed');
        input.type = 'text';
        input.classList.add('editempty', 'newinput');
        input.value = html10n.get('pad.userlist.unnamed');
        if (isNameEditable(data)) input.disabled = true;
        name = input;
      }

      const tdSwatch = document.createElement('td');
      tdSwatch.style.height = `${height}px`;
      tdSwatch.className = 'usertdswatch';
      const swatch = document.createElement('div');
      swatch.className = 'swatch';
      swatch.style.background = padutils.escapeHtml(data.color);
      swatch.innerHTML = '&nbsp;';
      tdSwatch.appendChild(swatch);

      const tdName = document.createElement('td');
      tdName.style.height = `${height}px`;
      tdName.className = 'usertdname';
      tdName.append(name);

      const tdActivity = document.createElement('td');
      tdActivity.style.height = `${height}px`;
      tdActivity.className = 'activity';
      tdActivity.textContent = data.activity;

      return [tdSwatch, tdName, tdActivity];
    };

    const createRow = (id, contents, authorId) => {
      const tr = document.createElement('tr');
      tr.setAttribute('data-authorId', authorId);
      tr.id = id;
      tr.append(...contents);
      return tr;
    };

    const rowNode = (row) => document.getElementById(row.domId);

    const handleRowData = (row) => {
      if (row.data && row.data.status === 'Disconnected') {
        row.opacity = 0.5;
      } else {
        delete row.opacity;
      }
    };

    const handleOtherUserInputs = () => {
      // handle 'INPUT' elements for naming other unnamed users
      qa('#otheruserstable input.newinput').forEach((input) => {
        if (!(input instanceof HTMLInputElement)) return;
        const tr = input.closest('tr');
        if (tr) {
          const rows = Array.from(tr.parentElement?.children || []);
          const index = rows.indexOf(tr);
          if (index >= 0 && rowsPresent.length > index) {
            const userId = rowsPresent[index].data.id;
            rowManagerMakeNameEditor(input, userId);
          }
        }
        input.classList.remove('newinput');
      });
    };

    // animationPower is 0 to skip animation, 1 for linear, 2 for quadratic, etc.


    const insertRow = (position, data, animationPower) => {
      position = Math.max(0, Math.min(rowsPresent.length, position));
      animationPower = (animationPower === undefined ? 4 : animationPower);

      const domId = nextRowId();
      const row = {
        data,
        animationStep: ANIMATION_START,
        domId,
        animationPower,
      };
      const authorId = data.id;

      handleRowData(row);
      rowsPresent.splice(position, 0, row);
      let tr;
      if (animationPower === 0) {
        tr = createRow(domId, createUserRowTds(getAnimationHeight(0), data), authorId);
        row.animationStep = 0;
      } else {
        rowsFadingIn.push(row);
        tr = createRow(domId, createEmptyRowTds(getAnimationHeight(ANIMATION_START)), authorId);
      }
      const otherUserTable = q('table#otheruserstable');
      if (otherUserTable) otherUserTable.style.display = '';
      if (position === 0) {
        otherUserTable?.prepend(tr);
      } else {
        rowNode(rowsPresent[position - 1])?.after(tr);
      }

      if (animationPower !== 0) {
        scheduleAnimation();
      }

      handleOtherUserInputs();

      return row;
    };

    const updateRow = (position, data) => {
      const row = rowsPresent[position];
      if (row) {
        row.data = data;
        handleRowData(row);
        if (row.animationStep === 0) {
          // not currently animating
          const tr = rowNode(row);
          if (!tr) return;
          replaceUserRowContents(tr, getAnimationHeight(0), row.data);
          setTdOpacity(tr, (row.opacity === undefined ? 1 : row.opacity));
          handleOtherUserInputs();
        }
      }
    };

    const removeRow = (position, animationPower) => {
      animationPower = (animationPower === undefined ? 4 : animationPower);
      const row = rowsPresent[position];
      if (row) {
        rowsPresent.splice(position, 1); // remove
        if (animationPower === 0) {
          rowNode(row)?.remove();
        } else {
          row.animationStep = -row.animationStep; // use symmetry
          row.animationPower = animationPower;
          rowsFadingOut.push(row);
          scheduleAnimation();
        }
      }
      if (rowsPresent.length === 0) {
        const otherUserTable = q('table#otheruserstable');
        if (otherUserTable) otherUserTable.style.display = 'none';
      }
    };

    // newPosition is position after the row has been removed


    const moveRow = (oldPosition, newPosition, animationPower) => {
      animationPower = (animationPower === undefined ? 1 : animationPower); // linear is best
      const row = rowsPresent[oldPosition];
      if (row && oldPosition !== newPosition) {
        const rowData = row.data;
        removeRow(oldPosition, animationPower);
        insertRow(newPosition, rowData, animationPower);
      }
    };

    const self = {
      insertRow,
      removeRow,
      moveRow,
      updateRow,
    };
    return self;
  })(); // //////// rowManager
  const otherUsersInfo = [];
  const otherUsersData = [];

  const asInput = (node) => {
    if (node instanceof HTMLInputElement) return node;
    if (node && typeof node.get === 'function') {
      const el = node.get(0);
      if (el instanceof HTMLInputElement) return el;
    }
    return null;
  };

  const rowManagerMakeNameEditor = (jnode, userId) => {
    const inputNode = asInput(jnode);
    if (!(inputNode instanceof HTMLInputElement)) return;
    setUpEditable(inputNode, () => {
      const existingIndex = findExistingIndex(userId);
      if (existingIndex >= 0) {
        return otherUsersInfo[existingIndex].name || '';
      } else {
        return '';
      }
    }, (newName) => {
      if (!newName) {
        inputNode.classList.add('editempty');
        inputNode.value = html10n.get('pad.userlist.unnamed');
      } else {
        inputNode.disabled = true;
        pad.suggestUserName(userId, newName);
      }
    });
  };

  const findExistingIndex = (userId) => {
    let existingIndex = -1;
    for (let i = 0; i < otherUsersInfo.length; i++) {
      if (otherUsersInfo[i].userId === userId) {
        existingIndex = i;
        break;
      }
    }
    return existingIndex;
  };

  const setUpEditable = (jqueryNode, valueGetter, valueSetter) => {
    if (!(jqueryNode instanceof HTMLInputElement)) return;
    jqueryNode.addEventListener('focus', () => {
      const oldValue = valueGetter();
      if (jqueryNode.value !== oldValue) {
        jqueryNode.value = oldValue;
      }
      jqueryNode.classList.add('editactive');
      jqueryNode.classList.remove('editempty');
    });
    jqueryNode.addEventListener('blur', () => {
      jqueryNode.classList.remove('editactive');
      const newValue = jqueryNode.value;
      valueSetter(newValue);
    });
    padutils.bindEnterAndEscape(jqueryNode, () => {
      jqueryNode.blur();
    }, () => {
      jqueryNode.value = valueGetter();
      jqueryNode.blur();
    });
    jqueryNode.disabled = false;
    jqueryNode.classList.add('editable');
  };

  let pad = undefined;
  const self = {
    init: (myInitialUserInfo, _pad) => {
      pad = _pad;
      self.setMyUserInfo(myInitialUserInfo);

      if (q('#online_count') == null) {
        const target = q('#editbar [data-key=showusers] > a');
        if (target) {
          const onlineCount = document.createElement('span');
          onlineCount.id = 'online_count';
          onlineCount.textContent = '1';
          target.append(onlineCount);
        }
      }

      qa('#otheruserstable tr').forEach((tr) => tr.remove());

      const myUsernameEdit = q('#myusernameedit');
      myUsernameEdit?.classList.add('myusernameedithoverable');
      setUpEditable(myUsernameEdit, () => myUserInfo.name || '', (newValue) => {
        myUserInfo.name = newValue;
        pad.notifyChangeName(newValue);
        // wrap with setTimeout to do later because we get
        // a double "blur" fire in IE...
        window.setTimeout(() => {
          self.renderMyUserInfo();
        }, 0);
      });

      // color picker
      q('#myswatchbox')?.addEventListener('click', showColorPicker);
      q('#mycolorpicker')?.addEventListener('click', (event) => {
        const target = event.target;
        if (!(target instanceof HTMLElement)) return;
        if (!target.classList.contains('pickerswatchouter')) return;
        qa('#mycolorpicker .pickerswatchouter').forEach((el) => el.classList.remove('picked'));
        target.classList.add('picked');
      });
      q('#mycolorpickersave')?.addEventListener('click', () => {
        closeColorPicker(true);
      });
      q('#mycolorpickercancel')?.addEventListener('click', () => {
        closeColorPicker(false);
      });
      //
    },
    usersOnline: () => {
      // Returns an object of users who are currently online on this pad
      // Make a copy of the otherUsersInfo, otherwise every call to users
      // modifies the referenced array
      const userList = [].concat(otherUsersInfo);
      // Now we need to add ourselves..
      userList.push(myUserInfo);
      return userList;
    },
    users: () => {
      // Returns an object of users who have been on this pad
      const userList = self.usersOnline();

      // Now we add historical authors
      const historical = clientVars.collab_client_vars.historicalAuthorData;
      for (const [key, {userId}] of Object.entries(historical)) {
        // Check we don't already have this author in our array
        let exists = false;

        userList.forEach((user) => {
          if (user.userId === userId) exists = true;
        });

        if (exists === false) {
          userList.push(historical[key]);
        }
      }
      return userList;
    },
    setMyUserInfo: (info) => {
      // translate the colorId
      if (typeof info.colorId === 'number') {
        info.colorId = clientVars.colorPalette[info.colorId];
      }

      myUserInfo = Object.assign({}, info);

      self.renderMyUserInfo();
    },
    userJoinOrUpdate: (info) => {
      if ((!info.userId) || (info.userId === myUserInfo.userId)) {
        // not sure how this would happen
        return;
      }

      hooks.callAll('userJoinOrUpdate', {
        userInfo: info,
      });

      const userData = {};
      userData.color = typeof info.colorId === 'number'
        ? clientVars.colorPalette[info.colorId] : info.colorId;
      userData.name = info.name;
      userData.status = '';
      userData.activity = '';
      userData.id = info.userId;

      const existingIndex = findExistingIndex(info.userId);

      let numUsersBesides = otherUsersInfo.length;
      if (existingIndex >= 0) {
        numUsersBesides--;
      }
      const newIndex = padutils.binarySearch(numUsersBesides, (n) => {
        if (existingIndex >= 0 && n >= existingIndex) {
          // pretend existingIndex isn't there
          n++;
        }
        const infoN = otherUsersInfo[n];
        const nameN = (infoN.name || '').toLowerCase();
        const nameThis = (info.name || '').toLowerCase();
        const idN = infoN.userId;
        const idThis = info.userId;
        return (nameN > nameThis) || (nameN === nameThis && idN > idThis);
      });

      if (existingIndex >= 0) {
        // update
        if (existingIndex === newIndex) {
          otherUsersInfo[existingIndex] = info;
          otherUsersData[existingIndex] = userData;
          rowManager.updateRow(existingIndex, userData);
        } else {
          otherUsersInfo.splice(existingIndex, 1);
          otherUsersData.splice(existingIndex, 1);
          otherUsersInfo.splice(newIndex, 0, info);
          otherUsersData.splice(newIndex, 0, userData);
          rowManager.updateRow(existingIndex, userData);
          rowManager.moveRow(existingIndex, newIndex);
        }
      } else {
        otherUsersInfo.splice(newIndex, 0, info);
        otherUsersData.splice(newIndex, 0, userData);
        rowManager.insertRow(newIndex, userData);
      }

      self.updateNumberOfOnlineUsers();
    },
    updateNumberOfOnlineUsers: () => {
      let online = 1; // you are always online!
      for (let i = 0; i < otherUsersData.length; i++) {
        if (otherUsersData[i].status === '') {
          online++;
        }
      }

      syncRecentPad(online);

      const onlineCount = q('#online_count');
      if (onlineCount) onlineCount.textContent = `${online}`;

      return online;
    },
    userLeave: (info) => {
      const existingIndex = findExistingIndex(info.userId);
      if (existingIndex >= 0) {
        const userData = otherUsersData[existingIndex];
        userData.status = 'Disconnected';
        rowManager.updateRow(existingIndex, userData);
        if (userData.leaveTimer) {
          window.clearTimeout(userData.leaveTimer);
        }
        // set up a timer that will only fire if no leaves,
        // joins, or updates happen for this user in the
        // next N seconds, to remove the user from the list.
        const thisUserId = info.userId;
        const thisLeaveTimer = window.setTimeout(() => {
          const newExistingIndex = findExistingIndex(thisUserId);
          if (newExistingIndex >= 0) {
            const newUserData = otherUsersData[newExistingIndex];
            if (newUserData.status === 'Disconnected' &&
                newUserData.leaveTimer === thisLeaveTimer) {
              otherUsersInfo.splice(newExistingIndex, 1);
              otherUsersData.splice(newExistingIndex, 1);
              rowManager.removeRow(newExistingIndex);
              hooks.callAll('userLeave', {
                userInfo: info,
              });
            }
          }
        }, 8000); // how long to wait
        userData.leaveTimer = thisLeaveTimer;
      }

      self.updateNumberOfOnlineUsers();
    },
    renderMyUserInfo: () => {
      if (myUserInfo.name) {
        const myUsernameEdit = q('#myusernameedit');
        if (myUsernameEdit instanceof HTMLInputElement) {
          myUsernameEdit.classList.remove('editempty');
          myUsernameEdit.value = myUserInfo.name;
        }
      } else {
        const myUsernameEdit = q('#myusernameedit');
        if (myUsernameEdit instanceof HTMLInputElement) {
          myUsernameEdit.setAttribute('placeholder', html10n.get('pad.userlist.entername'));
        }
      }
      if (colorPickerOpen) {
        const mySwatchBox = q('#myswatchbox');
        mySwatchBox?.classList.add('myswatchboxunhoverable');
        mySwatchBox?.classList.remove('myswatchboxhoverable');
      } else {
        const mySwatchBox = q('#myswatchbox');
        mySwatchBox?.classList.add('myswatchboxhoverable');
        mySwatchBox?.classList.remove('myswatchboxunhoverable');
      }

      const mySwatch = q('#myswatch');
      if (mySwatch instanceof HTMLElement) mySwatch.style.backgroundColor = myUserInfo.colorId;
      const showUsersAnchor = q('li[data-key=showusers] > a');
      if (showUsersAnchor instanceof HTMLElement) {
        showUsersAnchor.style.boxShadow = `inset 0 0 30px ${myUserInfo.colorId}`;
      }
    },
  };
  return self;
})();

const getColorPickerSwatchIndex = (jnode) => {
  if (!(jnode instanceof HTMLElement)) return -1;
  const swatches = qa('#colorpickerswatches li');
  return swatches.indexOf(jnode);
};

const closeColorPicker = (accept) => {
  if (accept) {
    const preview = q('#mycolorpickerpreview');
    let newColor = preview instanceof HTMLElement ? getComputedStyle(preview).backgroundColor : '';
    const parts = newColor.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/);
    // parts now should be ["rgb(0, 70, 255", "0", "70", "255"]
    if (parts) {
      delete (parts[0]);
      for (let i = 1; i <= 3; ++i) {
        parts[i] = parseInt(parts[i]).toString(16);
        if (parts[i].length === 1) parts[i] = `0${parts[i]}`;
      }
      newColor = `#${parts.join('')}`; // "0070ff"
    }
    myUserInfo.colorId = newColor;
    pad.notifyChangeColor(newColor);
    paduserlist.renderMyUserInfo();
  } else {
    // pad.notifyChangeColor(previousColorId);
    // paduserlist.renderMyUserInfo();
  }

  colorPickerOpen = false;
  q('#mycolorpicker')?.classList.remove('popup-show');
};

const ensureNativeColorPicker = () => {
  const colorPickerHost = q('#colorpicker');
  if (!(colorPickerHost instanceof HTMLElement)) return null;

  let input = colorPickerHost.querySelector<HTMLInputElement>('input[type="color"][data-native-color-picker="true"]');
  if (!input) {
    input = document.createElement('input');
    input.type = 'color';
    input.setAttribute('data-native-color-picker', 'true');
    input.className = 'native-colorpicker-input';
    colorPickerHost.replaceChildren(input);
  }

  const preview = q('#mycolorpickerpreview');
  if (!input.dataset.listenerAttached) {
    input.addEventListener('input', () => {
      const color = input?.value ?? '#000000';
      if (preview instanceof HTMLElement) preview.style.backgroundColor = color;
      pad.notifyChangeColor(color);
    });
    input.dataset.listenerAttached = 'true';
  }
  return input;
};

const showColorPicker = () => {
  const colorInput = ensureNativeColorPicker();
  if (colorInput instanceof HTMLInputElement) {
    colorInput.value = toHexColor(String(myUserInfo.colorId ?? '#000000'));
    const preview = q('#mycolorpickerpreview');
    if (preview instanceof HTMLElement) preview.style.backgroundColor = colorInput.value;
  }

  if (!colorPickerOpen) {
    const palette = pad.getColorPalette();

    if (!colorPickerSetup) {
      const colorsList = q('#colorpickerswatches');
      for (let i = 0; i < palette.length; i++) {
        const li = document.createElement('li');
        li.style.background = palette[i];
        colorsList?.appendChild(li);

        li.addEventListener('click', (event) => {
          qa('#colorpickerswatches li').forEach((el) => el.classList.remove('picked'));
          if (event.target instanceof HTMLElement) event.target.classList.add('picked');
          const newColorId = getColorPickerSwatchIndex(q('#colorpickerswatches .picked'));
          pad.notifyChangeColor(newColorId);
        });
      }

      colorPickerSetup = true;
    }

    q('#mycolorpicker')?.classList.add('popup-show');
    colorPickerOpen = true;

    qa('#colorpickerswatches li').forEach((el) => el.classList.remove('picked'));
    const current = qa('#colorpickerswatches li')[myUserInfo.colorId];
    current?.classList.add('picked');
  }
};
