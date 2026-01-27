import {Switch} from "../components/Switch";
import {useStore} from "../store/store";
import {useMemo} from "react";
import {parseSettings} from "./GeneralSettings.tsx";
import {PluginOptions} from "../components/PluginOptions.tsx";

export const PluginSettings = () => {
    const settings = useStore(s => s.settings);
    const settingsJSON = useMemo(()=>{return parseSettings(settings)}, [settings])

    const update = useStore(s => s.updateSetting);

    return (
        <div className="settings-section">
            <h2>Plugins</h2>

            <div className="plugin-grid">
                {Object.entries(settingsJSON.plugins).map(
                    ([pluginName, pluginConfig]: [
                        string,
                        any
                    ]) => (
                        <div
                            key={pluginName}
                            className="plugin-card"
                        >
                            <div className="plugin-header">
                                <h3>{pluginName}</h3>

                                <Switch
                                    checked={!!pluginConfig.enabled}
                                    onCheckedChange={enabled =>
                                        update(
                                            `plugins.${pluginName}.enabled`,
                                            enabled
                                        )
                                    }
                                />
                            </div>

                            {pluginConfig.enabled && (
                                <PluginOptions
                                    pluginName={pluginName}
                                    config={pluginConfig}
                                />
                            )}
                        </div>
                    )
                )}
            </div>

            <p className="warning-text">
                Änderungen an Plugin‑Einstellungen
                erfordern einen Server‑Neustart.
            </p>
        </div>
    );
};