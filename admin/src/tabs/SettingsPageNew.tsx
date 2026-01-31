import {useState} from "react";
import {SettingsTabs, TabKey} from "./SettingsTabs.tsx";
import {GeneralSettings} from "./GeneralSettings.tsx";
import {AdvancedJsonSettings} from "./AdvancedJsonSettings.tsx";
import {ServerSettings} from "./ServerSettings.tsx";
import {AuthSettings} from "./AuthSettings.tsx";
import {PluginSettings} from "./PluginSettings.tsx";

export const SettingsPageNew = () => {
    const [tab, setTab] = useState<TabKey>("general");

    return (
        <div className="settings-page">
            <h1>Settings</h1>

            <SettingsTabs active={tab} onChange={setTab} />

            <div className="settings-panel">
                {tab === "general" && <GeneralSettings />}
                {tab === "server" && <ServerSettings />}
                {tab === "auth" && <AuthSettings />}
                {tab === "plugins" && <PluginSettings />}
                {tab === "advanced" && <AdvancedJsonSettings />}
            </div>
        </div>
    );
};