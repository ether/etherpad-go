// @ts-nocheck
import * as skinVariants from './skin_variants';

/**
 * pad.ts — Main pad controller (EventBus migration).
 *
 * This is a clean rewrite that routes all cross-module communication through
 * the EventBus instead of importing chat, padconnectionstatus, padmodals,
 * and notifications directly.
 *
 * The collab_client setup, ace editor initialisation, and MessageQueue are
 * preserved as-is — they are complex and tightly coupled to the editor core.
 */

/**
 * Copyright 2009 Google Inc., 2011 Peter 'Pita' Martischka (Primary Technology Ltd)
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

let socket;

import html10n from './i18n';
import notifications from './notifications';

import padutils, {Cookies, randomString} from './pad_utils';
import {chat} from './chat';
import {getCollabClient} from './collab_client';
import {padconnectionstatus} from './pad_connectionstatus';
import {padcookie} from './pad_cookie';
import {padeditbar} from './pad_editbar';
import {padeditor} from './pad_editor';
import {padimpexp} from './pad_impexp';
import {padmodals} from './pad_modals';
import * as padsavedrevs from './pad_savedrevs';
import {paduserlist} from './pad_userlist';
import * as socketio from './socketio';
import {colorutils} from './colorutils';
import {editorBus} from './core/EventBus';

// ---------------------------------------------------------------------------
// Native DOM helpers (no jQuery)
// ---------------------------------------------------------------------------

const byId = (id: string) => document.getElementById(id);
const setDisplay = (id: string, value: string) => {
    const el = byId(id);
    if (el) el.style.display = value;
};
const hideById = (id: string) => setDisplay(id, 'none');
const showById = (id: string, display = 'block') => setDisplay(id, display);
const setCheckedById = (id: string, value: boolean) => {
    const el = byId(id);
    if (!el) return;
    if (el.tagName === 'EP-CHECKBOX') { (el as any).checked = value; return; }
    if (el instanceof HTMLInputElement) el.checked = value;
};

// ---------------------------------------------------------------------------
// GET-parameter driven settings
// ---------------------------------------------------------------------------

const getParameters = [
    {
        name: 'noColors',
        checkVal: 'true',
        callback: (val) => {
            settings.noColors = true;
            hideById('clearAuthorship');
        },
    },
    {
        name: 'showControls',
        checkVal: 'true',
        callback: (val) => {
            showById('editbar', 'flex');
        },
    },
    {
        name: 'showChat',
        checkVal: null,
        callback: (val) => {
            if (val === 'false') {
                settings.hideChat = true;
                chat.hide();
                hideById('chaticon');
            }
        },
    },
    {
        name: 'showLineNumbers',
        checkVal: 'false',
        callback: (val) => {
            settings.LineNumbersDisabled = true;
        },
    },
    {
        name: 'useMonospaceFont',
        checkVal: 'true',
        callback: (val) => {
            settings.useMonospaceFontGlobal = true;
        },
    },
    {
        name: 'userName',
        checkVal: null,
        callback: (val) => {
            settings.globalUserName = val;
            clientVars.userName = val;
        },
    },
    {
        name: 'userColor',
        checkVal: null,
        callback: (val) => {
            settings.globalUserColor = val;
            clientVars.userColor = val;
        },
    },
    {
        name: 'rtl',
        checkVal: 'true',
        callback: (val) => {
            settings.rtlIsTrue = true;
        },
    },
    {
        name: 'alwaysShowChat',
        checkVal: 'true',
        callback: (val) => {
            if (!settings.hideChat) chat.stickToScreen();
        },
    },
    {
        name: 'chatAndUsers',
        checkVal: 'true',
        callback: (val) => {
            chat.chatAndUsers();
        },
    },
    {
        name: 'lang',
        checkVal: null,
        callback: (val) => {
            console.log('Val is', val);
            html10n.localize([val, 'en']);
            Cookies.set('language', val, {expires: 36500});
        },
    },
];

const getParams = () => {
    // Tries server enforced options first..
    for (const setting of getParameters) {
        let value = clientVars.padOptions[setting.name];
        if (value == null) continue;
        value = value.toString();
        if (value === setting.checkVal || setting.checkVal == null) {
            setting.callback(value);
        }
    }

    // Then URL applied stuff
    const params = getUrlVars();
    for (const setting of getParameters) {
        const value = params.get(setting.name);
        if (value && (value === setting.checkVal || setting.checkVal == null)) {
            setting.callback(value);
        }
    }
};

const getUrlVars = () => new URL(window.location.href).searchParams;

// ---------------------------------------------------------------------------
// Client ready / handshake
// ---------------------------------------------------------------------------

const sendClientReady = (isReconnect) => {
    let padId = document.location.pathname.substring(document.location.pathname.lastIndexOf('/') + 1);
    padId = decodeURIComponent(padId);

    if (!isReconnect) {
        const titleArray = document.title.split('|');
        const title = titleArray[titleArray.length - 1];
        document.title = `${padId.replace(/_+/g, ' ')} | ${title}`;
    }

    let token = Cookies.get('token');
    if (token == null || !padutils.isValidAuthorToken(token)) {
        token = padutils.generateAuthorToken();
        Cookies.set('token', token, {expires: 60});
    }

    const params = getUrlVars();
    const userInfo = {
        colorId: params.get('userColor'),
        name: params.get('userName'),
    };

    const msg = {
        component: 'pad',
        type: 'CLIENT_READY',
        padId,
        sessionID: Cookies.get('sessionID'),
        token,
        userInfo,
    };

    if (isReconnect) {
        msg.client_rev = pad.collabClient.getCurrentRevisionNumber();
        msg.reconnect = true;
    }

    socket.emit('message', msg);
};

const handshake = async () => {
    let receivedClientVars = false;
    let padId = document.location.pathname.substring(document.location.pathname.lastIndexOf('/') + 1);
    padId = decodeURIComponent(padId);

    socket = pad.socket = socketio.connect(baseURL, '/', {
        query: {padId},
        reconnectionAttempts: 5,
        reconnection: true,
        reconnectionDelay: 1000,
        reconnectionDelayMax: 5000,
    });

    // ----- socket connect -----
    socket.once('connect', () => {
        editorBus.emit('connection:connected');
        sendClientReady(false);
    });

    // ----- socket reconnect -----
    socket.on('reconnect', () => {
        console.log('Socket reconnected');
        if (pad.collabClient != null) {
            pad.collabClient.setChannelState('CONNECTED');
        }
        editorBus.emit('connection:connected');
        sendClientReady(receivedClientVars);
    });

    const socketReconnecting = () => {
        if (pad.collabClient != null) {
            pad.collabClient.setStateIdle();
            pad.collabClient.setIsPendingRevision(true);
            pad.collabClient.setChannelState('RECONNECTING');
        }
        editorBus.emit('connection:reconnecting');
    };

    // ----- socket disconnect -----
    socket.on('disconnect', (reason: CloseEvent) => {
        console.log(`Socket disconnected: ${reason.reason}`);
        // Don't attempt reconnect if the user was deliberately disconnected
        if (padconnectionstatus.getStatus().what === 'disconnected') return;
        socketReconnecting();
    });

    // ----- admin shout messages -----
    socket.on('shout', (obj) => {
        if (obj.type === 'COLLABROOM') {
            const date = new Date(obj.data.payload.timestamp);
            notifications.add({
                title: 'Admin message',
                text: '[' + date.toLocaleTimeString() + ']: ' + obj.data.payload.message.message,
                sticky: obj.data.payload.message.sticky,
            });
        }
    });

    socket.on('reconnect_attempt', socketReconnecting);

    // ----- reconnect failed -----
    socket.on('reconnect_failed', (error) => {
        if (pad.collabClient != null) {
            pad.collabClient.setChannelState('DISCONNECTED', 'reconnect_timeout');
        } else {
            throw new Error('Reconnect timed out');
        }
    });

    // ----- socket error -----
    socket.on('error', (error) => {
        if (pad.collabClient != null) {
            pad.collabClient.setStateIdle();
            pad.collabClient.setIsPendingRevision(true);
        }
    });

    // ----- message handler -----
    socket.on('message', (obj) => {
        // access denied
        if (obj.accessStatus) {
            if (obj.accessStatus === 'deny') {
                hideById('loading');
                showById('permissionDenied');

                if (receivedClientVars) {
                    hideById('editorcontainer');
                    showById('editorloadingbox');
                }
            }
        } else if (!receivedClientVars && obj.type === 'CLIENT_VARS') {
            receivedClientVars = true;
            window.clientVars = obj.data;
            if (window.clientVars.sessionRefreshInterval) {
                const ping = () => fetch('../_extendExpressSessionLifetime', {method: 'PUT'}).catch(() => {});
                setInterval(ping, window.clientVars.sessionRefreshInterval);
            }

            if (!window.clientVars.collab_client_vars.isInitialAuthor) {
                const padDeleteButton = document.getElementById('delete-pad');
                if (padDeleteButton) {
                    padDeleteButton.remove();
                }
            }

            if (window.clientVars.mode === 'development') {
                console.warn('Enabling development mode with live update');
                socket.on('liveupdate', () => {
                    console.log('Live reload update received');
                    location.reload();
                });
            }
        } else if (obj.disconnect) {
            // Server-initiated disconnect
            editorBus.emit('connection:disconnected', {reason: obj.disconnect});
            padconnectionstatus.disconnected(obj.disconnect);
            socket.disconnect();

            padeditor.disable();
            padeditbar.disable();
            padimpexp.disable();

            return;
        } else {
            pad._messageQ.enqueue(obj);
        }
    });

    await new Promise((resolve) => {
        const h = (obj) => {
            if (obj.accessStatus || obj.type !== 'CLIENT_VARS') return;
            socket.off('message', h);
            resolve();
        };
        socket.on('message', h);
    });
};

// ---------------------------------------------------------------------------
// MessageQueue — defers messages until collabClient is ready
// ---------------------------------------------------------------------------

class MessageQueue {
    constructor() {
        this._q = [];
        this._cc = null;
    }

    setCollabClient(cc) {
        this._cc = cc;
        this.enqueue(); // Flush.
    }

    enqueue(...msgs) {
        if (this._cc == null) {
            this._q.push(...msgs);
        } else {
            while (this._q.length > 0) this._cc.handleMessageFromServer(this._q.shift());
            for (const msg of msgs) this._cc.handleMessageFromServer(msg);
        }
    }
}

// ---------------------------------------------------------------------------
// EditorBridge — inlined (formerly core/EditorBridge.ts)
// ---------------------------------------------------------------------------

/** Keeps track of unsubscribe functions so the bridge can be torn down. */
const bridgeTeardownFns: Array<() => void> = [];

function setupEditorBridge(): void {
    // EventBus -> chat send path via collabClient
    const unsubChatSend = editorBus.on('chat:message:send', ({message}) => {
        if (pad.collabClient && message) {
            pad.collabClient.sendMessage({
                type: 'CHAT_MESSAGE',
                message,
            });
        }
    });
    bridgeTeardownFns.push(unsubChatSend);

    // EventBus -> chat visibility
    const unsubChatVis = editorBus.on('chat:visibility:changed', ({visible}) => {
        if (visible) {
            chat.show?.();
        } else {
            chat.hide?.();
        }
    });
    bridgeTeardownFns.push(unsubChatVis);

    // EventBus -> pad view settings
    const unsubSettings = editorBus.on('settings:changed', ({key, value}) => {
        pad.changeViewOption(key, value);
    });
    bridgeTeardownFns.push(unsubSettings);
}

// ---------------------------------------------------------------------------
// The pad object
// ---------------------------------------------------------------------------

const pad = {
    collabClient: null,
    myUserInfo: null,
    diagnosticInfo: {},
    initTime: 0,
    clientTimeOffset: null,
    padOptions: {},
    _messageQ: new MessageQueue(),

    // Accessors
    getPadId: () => clientVars.padId,
    getClientIp: () => clientVars.clientIp,
    getColorPalette: () => clientVars.colorPalette,
    getPrivilege: (name) => clientVars.accountPrivs[name],
    getUserId: () => pad.myUserInfo.userId,
    getUserName: () => pad.myUserInfo.name,
    userList: () => paduserlist.users(),
    sendClientMessage: (msg) => {
        pad.collabClient.sendClientMessage(msg);
    },

    init() {
        padutils.setupGlobalExceptionHandler();
        const onReady = async () => {
            if (window.customStart != null) window.customStart();
            byId('readonlyinput')?.addEventListener('click', () => padeditbar.setEmbedLinks());
            byId('qrreadonlyinput')?.addEventListener('click', () => {
                void padeditbar.setQrCode();
            });
            byId('qrcodeclose')?.addEventListener('click', () => padeditbar.toggleDropDown('share_qr'));
            const qrPopup = byId('share_qr');
            qrPopup?.addEventListener('click', (event) => {
                if (event.target === qrPopup) padeditbar.toggleDropDown('share_qr');
            });
            padcookie.init();
            await handshake();
            this._afterHandshake();
        };
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => void onReady(), {once: true});
        } else {
            void onReady();
        }
    },

    _afterHandshake() {
        pad.clientTimeOffset = Date.now() - clientVars.serverTimestamp;
        // initialize the chat
        chat.init(this);
        getParams();

        padcookie.init();
        pad.initTime = +(new Date());
        pad.padOptions = clientVars.initialOptions;

        pad.myUserInfo = {
            userId: clientVars.userId,
            name: clientVars.userName,
            ip: pad.getClientIp(),
            colorId: clientVars.userColor,
        };

        const postAceInit = () => {
            padeditbar.init();
            setTimeout(() => {
                padeditor.ace.focus();
            }, 0);
            byId('options-stickychat')?.addEventListener('ep-change', () => chat.stickToScreen());
            byId('options-chatandusers')?.addEventListener('ep-change', () => chat.chatAndUsers());
            if (padcookie.getPref('chatAlwaysVisible')) {
                chat.stickToScreen(true);
                setCheckedById('options-stickychat', true);
            }
            if (padcookie.getPref('chatAndUsers')) {
                chat.chatAndUsers(true);
                setCheckedById('options-chatandusers', true);
            }
            if (padcookie.getPref('showAuthorshipColors') === false) {
                pad.changeViewOption('showAuthorColors', false);
            }
            if (padcookie.getPref('showLineNumbers') === false) {
                pad.changeViewOption('showLineNumbers', false);
            }
            if (padcookie.getPref('rtlIsTrue') === true) {
                pad.changeViewOption('rtlIsTrue', true);
            }
            pad.changeViewOption('padFontFamily', padcookie.getPref('padFontFamily'));
            const viewFontMenu = byId('viewfontmenu');
            if (viewFontMenu instanceof HTMLSelectElement) {
                viewFontMenu.value = String(padcookie.getPref('padFontFamily') ?? '');
            }

            const checkChatAndUsersVisibility = (x) => {
                if (x.matches) {
                    const chatAndUsers = byId('options-chatandusers') as any;
                    if (chatAndUsers?.checked) {
                        chatAndUsers.checked = false;
                        chatAndUsers.dispatchEvent(new CustomEvent('ep-change', {detail: {checked: false}}));
                    }
                    const stickyChat = byId('options-stickychat') as any;
                    if (stickyChat?.checked) {
                        stickyChat.checked = false;
                        stickyChat.dispatchEvent(new CustomEvent('ep-change', {detail: {checked: false}}));
                    }
                }
            };
            const mobileMatch = window.matchMedia('(max-width: 800px)');
            mobileMatch.addEventListener('change', checkChatAndUsersVisibility);
            setTimeout(() => {
                checkChatAndUsersVisibility(mobileMatch);
            }, 0);

            byId('editorcontainer')?.classList.add('initialized');

            if (window.clientVars.enableDarkMode) {
                showById('theme-switcher', 'flex');
            }

            if (window.location.hash.toLowerCase() !== '#skinvariantsbuilder' && window.clientVars.enableDarkMode && (window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches) && !skinVariants.isWhiteModeEnabledInLocalStorage()) {
                skinVariants.updateSkinVariantsClasses(['super-dark-editor', 'dark-background', 'super-dark-toolbar']);
            }

            // EventBus: emit editor:ready after ace is fully initialized
            editorBus.emit('editor:ready', {ace: padeditor.ace});
        };

        // order of inits is important here:
        padimpexp.init(this);
        padsavedrevs.init(this);
        padeditor.init(pad.padOptions.view || {}, this).then(postAceInit);
        paduserlist.init(pad.myUserInfo, this);
        padconnectionstatus.init(socket);
        padmodals.init(this);

        pad.collabClient = getCollabClient(
            padeditor.ace, clientVars.collab_client_vars, pad.myUserInfo,
            {colorPalette: pad.getColorPalette()}, pad);
        this._messageQ.setCollabClient(this.collabClient);

        // Wire collab_client callbacks — emit EventBus events directly
        pad.collabClient.setOnUserJoin((userInfo) => {
            paduserlist.userJoinOrUpdate(userInfo);
            editorBus.emit('user:join', {
                userId: userInfo.userId,
                name: userInfo.name,
                colorId: userInfo.colorId,
            });
        });

        pad.collabClient.setOnUpdateUserInfo((userInfo) => {
            paduserlist.userJoinOrUpdate(userInfo);
            editorBus.emit('user:info:updated', {
                userId: userInfo.userId,
                name: userInfo.name,
                colorId: userInfo.colorId,
            });
        });

        pad.collabClient.setOnUserLeave((userInfo) => {
            paduserlist.userLeave(userInfo);
            editorBus.emit('user:leave', {userId: userInfo.userId});
        });

        pad.collabClient.setOnClientMessage(pad.handleClientMessage);

        pad.collabClient.setOnChannelStateChange(pad.handleChannelStateChange);

        pad.collabClient.setOnInternalAction(pad.handleCollabAction);

        // Set up the EventBus bridge (toolbar commands, chat send, settings)
        setupEditorBridge();

        // load initial chat-messages
        if (clientVars.chatHead !== -1) {
            const chatHead = clientVars.chatHead;
            const start = Math.max(chatHead - 100, 0);
            pad.collabClient.sendMessage({type: 'GET_CHAT_MESSAGES', start, end: chatHead});
        } else {
            hideById('chatloadmessagesbutton');
        }

        if (window.clientVars.readonly) {
            chat.hide();
            const myusernameedit = byId('myusernameedit');
            if (myusernameedit instanceof HTMLInputElement) myusernameedit.disabled = true;
            const chatinput = byId('chatinput');
            if (chatinput instanceof HTMLInputElement) chatinput.disabled = true;
            hideById('chaticon');
            byId('options-chatandusers')?.parentElement?.setAttribute('style', 'display: none;');
            byId('options-stickychat')?.parentElement?.setAttribute('style', 'display: none;');
        } else if (!settings.hideChat) {
            showById('chaticon');
        }

        document.body.classList.add(window.clientVars.readonly ? 'readonly' : 'readwrite');

        padeditor.ace.callWithAce((ace) => {
            ace.ace_setEditable(!window.clientVars.readonly);
        });

        const mayUseLineNumberDisabled = Boolean(settings.LineNumbersDisabled);
        if (mayUseLineNumberDisabled) {
            this.changeViewOption('showLineNumbers', false);
        }

        const mayUseNoColors = Boolean(settings.noColors);
        if (mayUseNoColors) {
            this.changeViewOption('noColors', true);
        }

        const mayUseRTL = Boolean(settings.rtlIsTrue);
        if (mayUseRTL) {
            this.changeViewOption('rtlIsTrue', true);
        }

        const mayUseMonospace = Boolean(settings.useMonospaceFontGlobal);
        if (mayUseMonospace) {
            this.changeViewOption('padFontFamily', 'RobotoMono');
        }

        const mayBeGlobalUsername = Boolean(settings.globalUserName);
        if (mayBeGlobalUsername) {
            console.error('NOTIFY change!! ' + settings.globalUserName);
            this.notifyChangeName(settings.globalUserName);
            this.myUserInfo.name = settings.globalUserName;
            const myusernameedit = byId('myusernameedit');
            if (myusernameedit instanceof HTMLInputElement) myusernameedit.value = String(settings.globalUserName);
        }

        const mayBeGlobalUserColor = Boolean(settings.globalUserColor);
        if (mayBeGlobalUserColor && colorutils.isCssHex(settings.globalUserColor)) {
            console.error('NOTIFYING things ' + settings.globalUserName);
            this.myUserInfo.globalUserColor = settings.globalUserColor;
            this.notifyChangeColor(settings.globalUserColor);
            paduserlist.setMyUserInfo(this.myUserInfo);
        }
    },

    dispose: () => {
        // Tear down EventBus bridge subscriptions
        for (const fn of bridgeTeardownFns) {
            try { fn(); } catch { /* ignore */ }
        }
        bridgeTeardownFns.length = 0;
        padeditor.dispose();
    },

    notifyChangeName: (newName) => {
        pad.myUserInfo.name = newName;
        pad.collabClient.updateUserInfo(pad.myUserInfo);
    },

    notifyChangeColor: (newColorId) => {
        pad.myUserInfo.colorId = newColorId;
        pad.collabClient.updateUserInfo(pad.myUserInfo);
    },

    changePadOption: (key, value) => {
        const options = {};
        options[key] = value;
        pad.handleOptionsChange(options);
        pad.collabClient.sendClientMessage(
            {
                type: 'padoptions',
                options,
                changedBy: pad.myUserInfo.name || 'unnamed',
            });
    },

    changeViewOption: (key, value) => {
        const options = {
            view: {},
        };
        options.view[key] = value;
        pad.handleOptionsChange(options);
    },

    handleOptionsChange: (opts) => {
        if (opts.view) {
            if (!pad.padOptions.view) {
                pad.padOptions.view = {};
            }
            for (const [k, v] of Object.entries(opts.view)) {
                pad.padOptions.view[k] = v;
                padcookie.setPref(k, v);
            }
            padeditor.setViewOptions(pad.padOptions.view);
        }
    },

    getPadOptions: () => pad.padOptions,

    suggestUserName: (userId, name) => {
        pad.collabClient.sendClientMessage(
            {
                type: 'suggestUserName',
                unnamedId: userId,
                newName: name,
            });
    },

    handleClientMessage: (msg) => {
        if (msg.type === 'suggestUserName') {
            if (msg.unnamedId === pad.myUserInfo.userId && msg.newName && !pad.myUserInfo.name) {
                pad.notifyChangeName(msg.newName);
                paduserlist.setMyUserInfo(pad.myUserInfo);
            }
        } else if (msg.type === 'newRevisionList') {
            padsavedrevs.newRevisionList(msg.revisionList);
        } else if (msg.type === 'revisionLabel') {
            padsavedrevs.newRevisionList(msg.revisionList);
        } else if (msg.type === 'padoptions') {
            const opts = msg.options;
            pad.handleOptionsChange(opts);
        }
    },

    /**
     * Channel state change handler — routes through EventBus.
     *
     * Instead of calling padconnectionstatus.connected() / .disconnected() /
     * .reconnecting() directly here, we emit the appropriate EventBus events.
     * The padconnectionstatus module still needs to be called to keep its
     * internal state correct (isFullyConnected, getStatus), so we call it too.
     */
    handleChannelStateChange: (newState, message) => {
        const oldFullyConnected = !!padconnectionstatus.isFullyConnected();
        const wasConnecting = (padconnectionstatus.getStatus().what === 'connecting');

        if (newState === 'CONNECTED') {
            padeditor.enable();
            padeditbar.enable();
            padimpexp.enable();
            padconnectionstatus.connected();
            editorBus.emit('connection:connected');
        } else if (newState === 'RECONNECTING') {
            padeditor.disable();
            padeditbar.disable();
            padimpexp.disable();
            padconnectionstatus.reconnecting();
            editorBus.emit('connection:reconnecting');
        } else if (newState === 'DISCONNECTED') {
            pad.diagnosticInfo.disconnectedMessage = message;
            pad.diagnosticInfo.padId = pad.getPadId();
            pad.diagnosticInfo.socket = {};

            for (const [i, value] of Object.entries(socket.socket || {})) {
                const type = typeof value;
                if (type === 'string' || type === 'number') {
                    pad.diagnosticInfo.socket[i] = value;
                }
            }

            pad.asyncSendDiagnosticInfo();
            if (typeof window.ajlog === 'string') {
                window.ajlog += (`Disconnected: ${message}\n`);
            }
            padeditor.disable();
            padeditbar.disable();
            padimpexp.disable();

            padconnectionstatus.disconnected(message);
            editorBus.emit('connection:disconnected', {reason: message ?? 'unknown'});
        }

        const newFullyConnected = !!padconnectionstatus.isFullyConnected();
        if (newFullyConnected !== oldFullyConnected) {
            pad.handleIsFullyConnected(newFullyConnected, wasConnecting);
        }
    },

    handleIsFullyConnected: (isConnected, isInitialConnect) => {
        pad.determineChatVisibility(isConnected && !isInitialConnect);
        pad.determineChatAndUsersVisibility(isConnected && !isInitialConnect);
        pad.determineAuthorshipColorsVisibility();
        setTimeout(() => {
            padeditbar.toggleDropDown('none');
        }, 1000);
    },

    determineChatVisibility: (asNowConnectedFeedback) => {
        const chatVisCookie = padcookie.getPref('chatAlwaysVisible');
        if (chatVisCookie) {
            chat.stickToScreen(true);
            setCheckedById('options-stickychat', true);
        } else {
            setCheckedById('options-stickychat', false);
        }
    },

    determineChatAndUsersVisibility: (asNowConnectedFeedback) => {
        const chatAUVisCookie = padcookie.getPref('chatAndUsersVisible');
        if (chatAUVisCookie) {
            chat.chatAndUsers(true);
            setCheckedById('options-chatandusers', true);
        } else {
            setCheckedById('options-chatandusers', false);
        }
    },

    determineAuthorshipColorsVisibility: () => {
        const authColCookie = padcookie.getPref('showAuthorshipColors');
        if (authColCookie) {
            pad.changeViewOption('showAuthorColors', true);
            setCheckedById('options-colorscheck', true);
        } else {
            setCheckedById('options-colorscheck', false);
        }
    },

    handleCollabAction: (action) => {
        if (action === 'commitPerformed') {
            padeditbar.setSyncStatus('syncing');
        } else if (action === 'newlyIdle') {
            padeditbar.setSyncStatus('done');
        }
    },

    asyncSendDiagnosticInfo: () => {
        fetch('../ep/pad/connection-diagnostic-info', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                diagnosticInfo: pad.diagnosticInfo,
            }),
        }).catch((error) => {
            console.error('Error sending diagnostic info:', error);
        });
    },

    forceReconnect: () => {
        const reconnectForm = document.querySelector('form#reconnectform');
        const padIdInput = reconnectForm?.querySelector('input.padId');
        if (padIdInput instanceof HTMLInputElement) {
            padIdInput.value = pad.getPadId();
        }
        pad.diagnosticInfo.collabDiagnosticInfo = pad.collabClient.getDiagnosticInfo();
        const diagnosticInfoInput = reconnectForm?.querySelector('input.diagnosticInfo');
        if (diagnosticInfoInput instanceof HTMLInputElement) {
            diagnosticInfoInput.value = JSON.stringify(pad.diagnosticInfo);
        }
        const missedChangesInput = reconnectForm?.querySelector('input.missedChanges');
        if (missedChangesInput instanceof HTMLInputElement) {
            missedChangesInput.value = JSON.stringify(pad.collabClient.getMissedChanges());
        }
        if (reconnectForm instanceof HTMLFormElement) reconnectForm.requestSubmit();
    },

    callWhenNotCommitting: (f) => {
        pad.collabClient.callWhenNotCommitting(f);
    },

    getCollabRevisionNumber: () => pad.collabClient.getCurrentRevisionNumber(),

    isFullyConnected: () => padconnectionstatus.isFullyConnected(),

    addHistoricalAuthors: (data) => {
        if (!pad.collabClient) {
            window.setTimeout(() => {
                pad.addHistoricalAuthors(data);
            }, 1000);
        } else {
            pad.collabClient.addHistoricalAuthors(data);
        }
    },
};

// ---------------------------------------------------------------------------
// Module-level init & settings
// ---------------------------------------------------------------------------

const init = () => pad.init();

const settings = {
    LineNumbersDisabled: false,
    noColors: false,
    useMonospaceFontGlobal: false,
    globalUserName: false,
    globalUserColor: false,
    rtlIsTrue: false,
};

pad.settings = settings;

export let baseURL = '';
export const setBaseURL = (url: string) => {
    baseURL = url;
};
export {settings, randomString, getParams, pad, init};
