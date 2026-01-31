import {useStore} from "../store/store.ts";
import {IconButton} from "../components/IconButton.tsx";
import {Save} from "lucide-react";
import {JsonEditor} from "../components/JsonEditor.tsx";

export const AdvancedJsonSettings = () => {
    const settings = useStore(s => s.settings);
    const settingSocket = useStore(state=>state.settingSocket)

    return (
        <div className="settings-section advanced">
            <h2>Advanced JSON</h2>

            <p className="warning-text">
                ⚠ Änderungen hier können Etherpad unbenutzbar machen.
            </p>

            {settings && <JsonEditor
                value={settings}
                onChange={(v: any) =>
                    useStore.setState({
                        settings: JSON.stringify(v, null, 2),
                    })
                }
            />}

            <IconButton
                icon={<Save />}
                title="Speichern & Neustarten"
                onClick={() => {
                    settingSocket?.emit("saveSettings", settings);
                    settingSocket?.emit("restartServer");
                }}
            />
        </div>
    );
};