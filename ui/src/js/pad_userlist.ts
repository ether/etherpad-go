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
import '@samtv12345/etherpad-webcomponents/EpUserList.js';
import type {EpUser, EpUserList} from '@samtv12345/etherpad-webcomponents';

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

/** The <ep-user-list> in the users popup. */
const userListEl = (): EpUserList | null => document.getElementById('userlist') as EpUserList | null;

export const paduserlist = (() => {
  // Rendering: the other users plus the own entry go to <ep-user-list>,
  // which sorts them (own entry first) and shows status dots. Disconnected
  // users stay visible as "offline" until their leave timer removes them.
  const rowManager = (() => {
    const rows: any[] = [];
    let scheduled = false;

    const render = () => {
      scheduled = false;
      const el = userListEl();
      if (!el) return;
      const users: EpUser[] = rows.map((data) => ({
        id: String(data.id),
        name: data.name || '',
        color: String(data.color ?? ''),
        status: data.status === 'Disconnected' ? 'offline' : 'online',
      }));
      if (myUserInfo.userId) {
        users.unshift({
          id: String(myUserInfo.userId),
          name: myUserInfo.name || '',
          color: String(myUserInfo.colorId ?? ''),
          self: true,
        });
      }
      el.users = users;
    };

    const scheduleRender = () => {
      if (scheduled) return;
      scheduled = true;
      queueMicrotask(render);
    };

    return {
      render,
      scheduleRender,
      insertRow: (position: number, data: any) => {
        rows.splice(Math.max(0, Math.min(rows.length, position)), 0, data);
        scheduleRender();
      },
      removeRow: (position: number) => {
        rows.splice(position, 1);
        scheduleRender();
      },
      moveRow: (oldPosition: number, newPosition: number) => {
        const [row] = rows.splice(oldPosition, 1);
        if (row) rows.splice(newPosition, 0, row);
        scheduleRender();
      },
      updateRow: (position: number, data: any) => {
        if (rows[position]) rows[position] = data;
        scheduleRender();
      },
    };
  })(); // //////// rowManager

  const otherUsersInfo: any[] = [];
  const otherUsersData: any[] = [];

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
          // role="status" + aria-live="polite" announces the count when it
          // changes; the localized aria-label (set by updateNumberOfOnlineUsers
          // below) turns the bare badge digit into "N connected users" so AT
          // users get context, not a stray "5". Upstream #7255 / #7777.
          const onlineCount = document.createElement('span');
          onlineCount.id = 'online_count';
          onlineCount.setAttribute('role', 'status');
          onlineCount.setAttribute('aria-live', 'polite');
          onlineCount.textContent = '1';
          target.append(onlineCount);
        }
      }
      // Set the initial aria-label. updateNumberOfOnlineUsers otherwise only
      // fires on userJoin / userLeave / status change — on a solo-author pad
      // those events never come, and the badge would ship to AT with no
      // accessible name. Upstream #7255 / #7777.
      self.updateNumberOfOnlineUsers();

      const list = userListEl();
      if (list) {
        if (!(window as any).clientVars?.readonly) list.readonly = false;
        list.addEventListener('ep-user-rename', (e: Event) => {
          const newValue = String((e as CustomEvent).detail?.name ?? '');
          myUserInfo.name = newValue;
          pad.notifyChangeName(newValue);
          self.renderMyUserInfo();
        });
        list.addEventListener('ep-user-color-request', showColorPicker);
        // Texts set as attributes follow a language switch.
        html10n.bind('localized', () => {
          const t = (key: string) => html10n.get(key) as string | undefined;
          const set = (attr: string, key: string) => { const v = t(key); if (v) list.setAttribute(attr, v); };
          set('name-placeholder', 'pad.userlist.entername');
          set('unnamed-label', 'pad.userlist.unnamed');
          set('color-label', 'pad.userlist.changeColor');
        });
      }
      rowManager.render();

      // color picker
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

      // Set both visible text (the badge digit) and the accessible name in
      // one place so they can't drift. html10n.get returns undefined if the
      // locale bundle hasn't loaded yet — fall back to an English template
      // so AT never reads back "undefined". Upstream #7255 / #7777.
      const onlineCount = q('#online_count');
      if (onlineCount) {
        onlineCount.textContent = `${online}`;
        const label = (html10n.get('pad.userlist.onlineCount', {count: online} as any) as string | undefined)
            || `${online} connected user${online === 1 ? '' : 's'}`;
        onlineCount.setAttribute('aria-label', label);
      }

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
    /** Sets the own name shown in the list (e.g. a global user name from the URL). */
    setMyName: (name: string) => {
      myUserInfo.name = name;
      rowManager.scheduleRender();
    },
    renderMyUserInfo: () => {
      rowManager.scheduleRender();
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
