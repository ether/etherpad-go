import {useStore} from "../store/store";
import {useMemo} from "react";
import {parseSettings} from "./GeneralSettings.tsx";
import {Switch} from "../components/Switch.tsx";

export const ServerSettings = () => {
    const settings = useStore(s => s.settings);
    const update = useStore(s => s.updateSetting);
    const settingsJSON = useMemo(()=>{return parseSettings(settings)}, [settings])

    return (
        <div className="settings-section">
            <h2>Server</h2>

            {/* Network */}
            <fieldset>
                <legend>Network</legend>

                <label>
                    IP Address
                    <input
                        value={settingsJSON.ip}
                        onChange={e => update("ip", e.target.value)}
                        placeholder="0.0.0.0"
                    />
                </label>

                <label>
                    Port
                    <input
                        type="number"
                        min={1}
                        max={65535}
                        value={settingsJSON.port}
                        onChange={e =>
                            update("port", e.target.value)
                        }
                    />
                </label>

                <label className="switch-row">
                    Trust Proxy
                    <Switch
                        checked={settingsJSON.trustProxy}
                        onCheckedChange={v =>
                            update("trustProxy", v.toString())
                        }
                    />
                </label>
            </fieldset>

            {/* SSL */}
            <fieldset>
                <legend>SSL / HTTPS</legend>

                <label className="switch-row">
                    Enable SSL
                    <Switch
                        checked={!!settingsJSON.ssl?.key}
                        onCheckedChange={enabled => {
                            update("ssl", enabled.toString());
                        }}
                    />
                </label>

                {settingsJSON.ssl && (
                    <>
                        <label>
                            Certificate Path
                            <input
                                value={settingsJSON.ssl.cert || ""}
                                onChange={e =>
                                    update("ssl.cert", e.target.value)
                                }
                            />
                        </label>

                        <label>
                            Key Path
                            <input
                                value={settingsJSON.ssl.key || ""}
                                onChange={e =>
                                    update("ssl.key", e.target.value)
                                }
                            />
                        </label>
                    </>
                )}
            </fieldset>

            {/* Runtime */}
            <fieldset>
                <legend>Runtime</legend>

                <label>
                    Log Level
                    <select
                        value={settingsJSON.loglevel}
                        onChange={e =>
                            update("loglevel", e.target.value)
                        }
                    >
                        <option value="DEBUG">DEBUG</option>
                        <option value="INFO">INFO</option>
                        <option value="WARN">WARN</option>
                        <option value="ERROR">ERROR</option>
                    </select>
                </label>

                <label className="switch-row">
                    Minify Assets
                    <Switch
                        checked={settingsJSON.minify}
                        onCheckedChange={v => update("minify", v.toString())}
                    />
                </label>

                <label>
                    Cache maxAge (seconds)
                    <input
                        type="number"
                        min={0}
                        value={settingsJSON.maxAge}
                        onChange={e =>
                            update("maxAge", e.target.value)
                        }
                    />
                </label>

                <label className="switch-row">
                    Dev Mode
                    <Switch
                        checked={settingsJSON.devMode}
                        onCheckedChange={v => update("devMode", v.toString())}
                    />
                </label>
            </fieldset>
        </div>
    );
};