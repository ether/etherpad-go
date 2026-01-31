import {useStore} from "../store/store.ts";
import {useMemo} from "react";
import {Switch} from "@radix-ui/react-switch";


export const parseSettings = (settingsStr: string|undefined) => {
    return settingsStr?JSON.parse(settingsStr): {}
}

export const GeneralSettings = () => {
    const settings = useStore(state=>state.settings);

    const settingsJSON = useMemo(()=>{return parseSettings(settings)}, [settings])

    return (
        <div className="settings-section">
            <h2>Allgemein</h2>

            <label>
                Titel
                <input
                    value={settingsJSON.title}
                    onChange={e =>
                        useStore.getState().updateSetting("title", e.target.value)
                    }
                />
            </label>

            <label className="switch-row">
                Dark Mode
                <Switch
                    checked={settingsJSON.enableDarkMode}
                    onCheckedChange={v =>
                        useStore
                            .getState()
                            .updateSetting("enableDarkMode", v.toString())
                    }
                />
            </label>

            <label>
                Sprache
                <select
                    value={settingsJSON.padOptions?.Lang || "en-gb"}
                    onChange={e =>
                        useStore
                            .getState()
                            .updateSetting("padOptions.Lang", e.target.value)
                    }
                >
                    <option value="en-gb">English</option>
                    <option value="de-de">Deutsch</option>
                </select>
            </label>
        </div>
    );
};