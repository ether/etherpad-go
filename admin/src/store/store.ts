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


type StoreState = {
    settings: string|undefined,
    setSettings: (settings: string) => void,
    settingsSocket: SocketIoWrapper|undefined,
    setSettingsSocket: (socket: SocketIoWrapper) => void,
    showLoading: boolean,
    setShowLoading: (show: boolean) => void,
    setPluginsSocket: (socket: SocketIoWrapper) => void
    pluginsSocket: SocketIoWrapper|undefined,
    toastState: ToastState,
    setToastState: (val: ToastState)=>void,
    pads: PadSearchResult|undefined,
    setPads: (pads: PadSearchResult)=>void,
    installedPlugins: InstalledPlugin[],
    setInstalledPlugins: (plugins: InstalledPlugin[])=>void
}


export const useStore = create<StoreState>()((set) => ({
    settings: undefined,
    setSettings: (settings: string) => set({settings}),
    settingsSocket: undefined,
    setSettingsSocket: (socket: SocketIoWrapper) => set({settingsSocket: socket}),
    showLoading: false,
    setShowLoading: (show: boolean) => set({showLoading: show}),
    pluginsSocket: undefined,
    setPluginsSocket: (socket: SocketIoWrapper) => set({pluginsSocket: socket}),
    setToastState: (val )=>set({toastState: val}),
    toastState: {
        open: false,
        title: '',
        description:'',
        success: false
    },
    pads: undefined,
    setPads: (pads)=>set({pads}),
    installedPlugins: [],
    setInstalledPlugins: (plugins)=>set({installedPlugins: plugins})
}));
