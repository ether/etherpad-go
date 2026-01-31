export type TabKey =
    | "general"
    | "server"
    | "auth"
    | "plugins"
    | "advanced";

const TABS: { key: TabKey; label: string }[] = [
    { key: "general", label: "Allgemein" },
    { key: "server", label: "Server" },
    { key: "auth", label: "Auth / SSO" },
    { key: "plugins", label: "Plugins" },
    { key: "advanced", label: "⚙ Advanced" },
];

export const SettingsTabs = ({
                                 active,
                                 onChange,
                             }: {
    active: TabKey;
    onChange: (key: TabKey) => void;
}) => (
    <div className="settings-tabs">
        {TABS.map(tab => (
            <button
                key={tab.key}
                className={
                    "settings-tab" + (active === tab.key ? " active" : "")
                }
                onClick={() => onChange(tab.key)}
            >
                {tab.label}
            </button>
        ))}
    </div>
);