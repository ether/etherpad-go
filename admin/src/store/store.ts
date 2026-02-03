import {create} from "zustand";
import {PadSearchResult} from "../utils/PadSearch.ts";
import {InstalledPlugin} from "../pages/Plugin.ts";
import {SocketIoWrapper} from "../utils/socketIoWrapper.ts";

type ToastState = {
    description?:string,
    title: string,
    open: boolean,
    success: boolean
}

type SettingValue =
    | string
    | number
    | boolean
    | null
    | Record<string, any>
    | any[];


function setByPath(
    obj: any,
    path: string,
    value: SettingValue
) {
    const keys = path.split(".");

    let current = obj;

    for (let i = 0; i < keys.length - 1; i++) {
        const key = keys[i];
        const nextKey = keys[i + 1];
        const isIndex = !isNaN(Number(key));

        if (isIndex) {
            const index = Number(key);
            if (!Array.isArray(current)) {
                throw new Error("Expected array at " + keys.slice(0, i).join("."));
            }
            if (!current[index]) {
                current[index] =
                    !isNaN(Number(nextKey)) ? [] : {};
            }
            current = current[index];
        } else {
            if (!(key in current)) {
                current[key] =
                    !isNaN(Number(nextKey)) ? [] : {};
            }
            current = current[key];
        }
    }

    const lastKey = keys[keys.length - 1];
    const isLastIndex = !isNaN(Number(lastKey));

    if (isLastIndex) {
        current[Number(lastKey)] = value;
    } else {
        current[lastKey] = value;
    }
}

type StoreState = {
    settings: string|undefined,
    setSettings: (settings: string) => void,
    showLoading: boolean,
    setShowLoading: (show: boolean) => void,
    toastState: ToastState,
    setToastState: (val: ToastState)=>void,
    pads: PadSearchResult|undefined,
    setPads: (pads: PadSearchResult)=>void,
    installedPlugins: InstalledPlugin[],
    setInstalledPlugins: (plugins: InstalledPlugin[])=>void
    settingSocket: SocketIoWrapper|undefined,
    updateSetting: (path: string, value: SettingValue) => void,
    updateCheckResult: {
        currentVersion: string,
        latestVersion: string,
        updateAvailable: boolean
    } | undefined,
    setUpdateCheckResult: (result: {
        currentVersion: string,
        latestVersion: string,
        updateAvailable: boolean
    }) => void
}


export const useStore = create<StoreState>()((set) => ({
    settings: undefined,
    setSettings: (settings: string) => set({settings}),
    showLoading: true,
    setShowLoading: (show: boolean) => set({showLoading: show}),
    setToastState: (val )=>set({toastState: val}),
    toastState: {
        open: false,
        title: '',
        description:'',
        success: false
    },
    settingSocket: undefined,
    pads: undefined,
    setPads: (pads)=>set({pads}),
    installedPlugins: [],
    setInstalledPlugins: (plugins)=>set({installedPlugins: plugins}),
    updateCheckResult: undefined,
    setUpdateCheckResult: (result) => set({updateCheckResult: result}),
    updateSetting: (path: string, value: SettingValue) =>
        set(state => {
            if (!state.settings) return state;

            let settingsObj: any;

            try {
                settingsObj = JSON.parse(state.settings);
            } catch {
                return state;
            }

            setByPath(settingsObj, path, value);

            return {
                settings: JSON.stringify(settingsObj, null, 2),
            };
        })
}));
