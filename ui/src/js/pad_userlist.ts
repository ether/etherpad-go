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

import padutils from './pad_utils';
import {editorBus} from './core';
import html10n from './i18n';
import {pad} from "./pad.ts";

let myUserInfo: Record<string, any> = {};

let colorPickerOpen = false;
let colorPickerSetup = false;

const q = (selector: string): HTMLElement | null => document.querySelector(selector);
const qa = (selector: string): HTMLElement[] => Array.from(document.querySelectorAll(selector));

const toHexColor = (color: string): string => {
  if (/^#[0-9a-f]{6}$/i.test(color)) return color;
  const match = color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/i);
  if (!match) return '#000000';
  const parts = match.slice(1).map((part) => Number.parseInt(part, 10).toString(16).padStart(2, '0'));
  return `#${parts.join('')}`;
};

export const paduserlist = (() => {
  const rowManager = (() => {
    // The row manager handles rendering rows of the user list and animating
    // their insertion, removal, and reordering.  It manipulates TD height
    // and TD opacity.

    const nextRowId = () => `usertr${nextRowId.counter++}`;
    nextRowId.counter = 1;
    // objects are shared; fields are "domId","data","animationStep"
    const rowsFadingOut: any[] = []; // unordered set
    const rowsFadingIn: any[] = []; // unordered set
    const rowsPresent: any[] = []; // in order
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

    const getAnimationHeight = (step: number, power: number) => {
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

    const setTdHeight = (tr: HTMLElement, height: number) => {
      tr.querySelectorAll('td').forEach((td) => {
        td.style.height = `${height}px`;
      });
    };

    const setTdOpacity = (tr: HTMLElement, opacity: number) => {
      tr.querySelectorAll('td').forEach((td) => {
        td.style.opacity = `${opacity}`;
      });
    };

    const createEmptyRowTds = (height: number): HTMLElement[] => {
      const td = document.createElement('td');
      td.colSpan = NUMCOLS;
      td.style.border = '0';
      td.style.height = `${height}px`;
      return [td];
    };

    const isNameEditable = (data: any) => (!data.name) && (data.status !== 'Disconnected');

    const replaceUserRowContents = (tr: HTMLElement, height: number, data: any) => {
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

    const createUserRowTds = (height: number, data: any): HTMLElement[] => {
      let name: Node;
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

    const createRow = (id: string, contents: HTMLElement[], authorId: string): HTMLElement => {
      const tr = document.createElement('tr');
      tr.setAttribute('data-authorId', authorId);
      tr.id = id;
      tr.append(...contents);
      return tr;
    };

    const rowNode = (row: any): HTMLElement | null => document.getElementById(row.domId);

    const handleRowData = (row: any) => {
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

    const insertRow = (position: number, data: any, animationPower?: number) => {
      position = Math.max(0, Math.min(rowsPresent.length, position));
      animationPower = (animationPower === undefined ? 4 : animationPower);

      const domId = nextRowId();
      const row: any = {
        data,
        animationStep: ANIMATION_START,
        domId,
        animationPower,
      };
      const authorId = data.id;

      handleRowData(row);
      rowsPresent.splice(position, 0, row);
      let tr: HTMLElement;
      if (animationPower === 0) {
        tr = createRow(domId, createUserRowTds(getAnimationHeight(0, 0), data), authorId);
        row.animationStep = 0;
      } else {
        rowsFadingIn.push(row);
        tr = createRow(domId, createEmptyRowTds(getAnimationHeight(ANIMATION_START, animationPower)), authorId);
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

    const updateRow = (position: number, data: any) => {
      const row = rowsPresent[position];
      if (row) {
        row.data = data;
        handleRowData(row);
        if (row.animationStep === 0) {
          // not currently animating
          const tr = rowNode(row);
          if (!tr) return;
          replaceUserRowContents(tr, getAnimationHeight(0, 0), row.data);
          setTdOpacity(tr, (row.opacity === undefined ? 1 : row.opacity));
          handleOtherUserInputs();
        }
      }
    };

    const removeRow = (position: number, animationPower?: number) => {
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

    const moveRow = (oldPosition: number, newPosition: number, animationPower?: number) => {
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

  const otherUsersInfo: any[] = [];
  const otherUsersData: any[] = [];

  const asInput = (node: any): HTMLInputElement | null => {
    if (node instanceof HTMLInputElement) return node;
    if (node && typeof node.get === 'function') {
      const el = node.get(0);
      if (el instanceof HTMLInputElement) return el;
    }
    return null;
  };

  const rowManagerMakeNameEditor = (jnode: any, userId: string) => {
    const inputNode = asInput(jnode);
    if (!(inputNode instanceof HTMLInputElement)) return;
    setUpEditable(inputNode, () => {
      const existingIndex = findExistingIndex(userId);
      if (existingIndex >= 0) {
        return otherUsersInfo[existingIndex].name || '';
      } else {
        return '';
      }
    }, (newName: string) => {
      if (!newName) {
        inputNode.classList.add('editempty');
        inputNode.value = html10n.get('pad.userlist.unnamed');
      } else {
        inputNode.disabled = true;
        pad.suggestUserName(userId, newName);
      }
    });
  };

  const findExistingIndex = (userId: string): number => {
    let existingIndex = -1;
    for (let i = 0; i < otherUsersInfo.length; i++) {
      if (otherUsersInfo[i].userId === userId) {
        existingIndex = i;
        break;
      }
    }
    return existingIndex;
  };

  const setUpEditable = (node: HTMLInputElement, valueGetter: () => string, valueSetter: (val: string) => void) => {
    if (!(node instanceof HTMLInputElement)) return;
    node.addEventListener('focus', () => {
      const oldValue = valueGetter();
      if (node.value !== oldValue) {
        node.value = oldValue;
      }
      node.classList.add('editactive');
      node.classList.remove('editempty');
    });
    node.addEventListener('blur', () => {
      node.classList.remove('editactive');
      const newValue = node.value;
      valueSetter(newValue);
    });
    padutils.bindEnterAndEscape(node, () => {
      node.blur();
    }, () => {
      node.value = valueGetter();
      node.blur();
    });
    node.disabled = false;
    node.classList.add('editable');
  };

  const emitUserlistUpdated = () => {
    editorBus.emit('custom:userlist:updated', {
      users: self.usersOnline(),
      count: self.updateNumberOfOnlineUsers(),
    });
  };

  let pad: any = undefined;

  // Listen for EventBus user events
  editorBus.on('user:join', (data) => {
    if (data.userId && data.userId !== myUserInfo.userId) {
      self.userJoinOrUpdate({
        userId: data.userId,
        name: data.name ?? null,
        colorId: data.colorId,
      });
    }
  });

  editorBus.on('user:leave', () => {
    // The user:leave event is also emitted *by* this module when the leave timer
    // fires, so we only react if the user is still in our list and not already
    // marked as disconnected-and-leaving.
  });

  editorBus.on('user:info:updated', () => {
    // Externally triggered info updates (e.g. from the server via collab)
    // are handled through userJoinOrUpdate, which already emits this event.
    // Avoid infinite loops by not re-processing our own emits.
  });

  const self = {
    init: (myInitialUserInfo: any, _pad: any) => {
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
      if (myUsernameEdit instanceof HTMLInputElement) {
        myUsernameEdit.classList.add('myusernameedithoverable');
        setUpEditable(myUsernameEdit, () => myUserInfo.name || '', (newValue: string) => {
          myUserInfo.name = newValue;
          pad.notifyChangeName(newValue);
          // wrap with setTimeout to do later because we get
          // a double "blur" fire in IE...
          window.setTimeout(() => {
            self.renderMyUserInfo();
          }, 0);
        });
      }

      // color picker
      q('#myswatchbox')?.addEventListener('click', showColorPicker);
      q('#mycolorpicker')?.addEventListener('click', (event: Event) => {
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
    },
    usersOnline: () => {
      // Returns an object of users who are currently online on this pad
      // Make a copy of the otherUsersInfo, otherwise every call to users
      // modifies the referenced array
      const userList = ([] as any[]).concat(otherUsersInfo);
      // Now we need to add ourselves..
      userList.push(myUserInfo);
      return userList;
    },
    users: () => {
      // Returns an object of users who have been on this pad
      const userList = self.usersOnline();

      // Now we add historical authors
      const historical = (window as any).clientVars.collab_client_vars.historicalAuthorData;
      for (const [key, {userId}] of Object.entries<any>(historical)) {
        // Check we don't already have this author in our array
        let exists = false;

        userList.forEach((user: any) => {
          if (user.userId === userId) exists = true;
        });

        if (exists === false) {
          userList.push(historical[key]);
        }
      }
      return userList;
    },
    setMyUserInfo: (info: any) => {
      // translate the colorId
      if (typeof info.colorId === 'number') {
        info.colorId = (window as any).clientVars.colorPalette[info.colorId];
      }

      myUserInfo = Object.assign({}, info);

      self.renderMyUserInfo();
      emitUserlistUpdated();
    },
    userJoinOrUpdate: (info: any) => {
      if ((!info.userId) || (info.userId === myUserInfo.userId)) {
        // not sure how this would happen
        return;
      }

      editorBus.emit('user:info:updated', {
        userId: info.userId,
        name: info.name,
        colorId: typeof info.colorId === 'number' ? undefined : info.colorId,
      });

      const userData: any = {};
      userData.color = typeof info.colorId === 'number'
        ? (window as any).clientVars.colorPalette[info.colorId] : info.colorId;
      userData.name = info.name;
      userData.status = '';
      userData.activity = '';
      userData.id = info.userId;

      const existingIndex = findExistingIndex(info.userId);

      let numUsersBesides = otherUsersInfo.length;
      if (existingIndex >= 0) {
        numUsersBesides--;
      }
      const newIndex = padutils.binarySearch(numUsersBesides, (n: number) => {
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
      emitUserlistUpdated();
    },
    updateNumberOfOnlineUsers: () => {
      let online = 1; // you are always online!
      for (let i = 0; i < otherUsersData.length; i++) {
        if (otherUsersData[i].status === '') {
          online++;
        }
      }

      if (localStorage.getItem('recentPads') != null) {
        const recentPadsList = JSON.parse(localStorage.getItem('recentPads')!);
        const pathSegments = window.location.pathname.split('/');
        const padName = pathSegments[pathSegments.length - 1];
        const existingPad = recentPadsList.find((pad: any) => pad.name === padName);
        if (existingPad) {
          existingPad.members = online;
        }
        localStorage.setItem('recentPads', JSON.stringify(recentPadsList));
      }

      const onlineCount = q('#online_count');
      if (onlineCount) onlineCount.textContent = `${online}`;

      return online;
    },
    userLeave: (info: any) => {
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
              editorBus.emit('user:leave', {userId: info.userId});
            }
          }
        }, 8000); // how long to wait
        userData.leaveTimer = thisLeaveTimer;
      }

      self.updateNumberOfOnlineUsers();
      emitUserlistUpdated();
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

const closeColorPicker = (accept: boolean) => {
  if (accept) {
    const preview = q('#mycolorpickerpreview');
    const previewColor = preview instanceof HTMLElement ? getComputedStyle(preview).backgroundColor : '';
    const newColor = toHexColor(previewColor);

    myUserInfo.colorId = newColor;
    pad?.notifyChangeColor?.(newColor);
    paduserlist.renderMyUserInfo();
  }

  colorPickerOpen = false;
  q('#mycolorpicker')?.classList.remove('popup-show');
};

const ensureNativeColorPicker = (): HTMLInputElement | null => {
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
    });
    input.dataset.listenerAttached = 'true';
  }
  return input;
};

const showColorPicker = () => {
  const colorInput = ensureNativeColorPicker();
  if (colorInput instanceof HTMLInputElement) {
    const currentColor = toHexColor(String(myUserInfo.colorId ?? '#000000'));
    colorInput.value = currentColor;
    const preview = q('#mycolorpickerpreview');
    if (preview instanceof HTMLElement) preview.style.backgroundColor = currentColor;
  }

  if (!colorPickerOpen) {
    if (!colorPickerSetup) {
      colorPickerSetup = true;
    }

    q('#mycolorpicker')?.classList.add('popup-show');
    colorPickerOpen = true;
  }
};
