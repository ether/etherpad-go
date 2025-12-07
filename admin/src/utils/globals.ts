import {connect} from "./socketio.ts";
import {useStore} from "../store/store.ts";


const initSettingsSocket = (token: string)=>{
    const settingSocket = connect(token);
    useStore.setState({
        settingSocket: settingSocket
    })


    settingSocket.on('connect', () => {
        useStore.getState().setShowLoading(false)
        settingSocket?.emit('load', {});
        console.log('connected');
    });

    settingSocket.on('disconnect', (reason: string) => {
        // The settingSocket.io client will automatically try to reconnect for all reasons other than "io
        // server disconnect".
        useStore.getState().setShowLoading(true)
        if (reason === 'io server disconnect') {
            settingSocket?.connect();
        }
    });

    settingSocket.on('settings', (settings: {
        results: string
    }) => {
        /* Check whether the settings.json is authorized to be viewed */
        if (settings.results === 'NOT_ALLOWED') {
            console.log('Not allowed to view settings.json')
            return;
        }

        /* Check to make sure the JSON is clean before proceeding */
        useStore.getState().setSettings(JSON.stringify(settings.results));
        useStore.getState().setShowLoading(false);
    });

    settingSocket.on('saveprogress', (status: string) => {
        console.log(status)
    })


}

export {initSettingsSocket};

