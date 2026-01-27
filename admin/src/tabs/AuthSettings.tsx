import {Switch} from "../components/Switch";
import {useStore} from "../store/store";
import * as Select from "@radix-ui/react-select";
import {ChevronDown} from "lucide-react";
import {useMemo} from "react";
import {parseSettings} from "./GeneralSettings.tsx";

export const AuthSettings = () => {
    const settings = useStore(s => s.settings);

    const update = useStore(s => s.updateSetting);
    const settingsJSON = useMemo(()=>{return parseSettings(settings)}, [settings])

    const isSSO = settingsJSON?.authenticationMethod === "sso";

    return (
        <div className="settings-section">
            <h2>Authentication</h2>

            {/* Mode */}
            <fieldset>
                <legend>Mode</legend>

                <label>
                    Authentication Method
                    <Select.Root
                        value={settingsJSON.authenticationMethod}
                        onValueChange={v =>
                            update("authenticationMethod", v)
                        }
                    >
                        <Select.Trigger className="SelectTrigger">
                            <Select.Value />
                            <Select.Icon>
                                <ChevronDown size={16} />
                            </Select.Icon>
                        </Select.Trigger>

                        <Select.Content className="SelectContent">
                            <Select.Item value="none">None</Select.Item>
                            <Select.Item value="password">Password</Select.Item>
                            <Select.Item value="sso">SSO (OAuth2)</Select.Item>
                        </Select.Content>
                    </Select.Root>
                </label>

                <label className="switch-row">
                    Require Authentication
                    <Switch
                        checked={settingsJSON.requireAuthentication}
                        onCheckedChange={v =>
                            update("requireAuthentication", v)
                        }
                    />
                </label>

                <label className="switch-row">
                    Require Authorization
                    <Switch
                        checked={settingsJSON.requireAuthorization}
                        onCheckedChange={v =>
                            update("requireAuthorization", v)
                        }
                    />
                </label>
            </fieldset>

            {/* SSO */}
            {isSSO && (
                <fieldset>
                    <legend>SSO / OAuth2</legend>

                    <label>
                        Issuer URL
                        <input
                            value={settingsJSON.sso.issuer}
                            onChange={e =>
                                update("sso.issuer", e.target.value)
                            }
                        />
                    </label>

                    <div className="sso-clients">
                        {settingsJSON.sso.clients.map(
                            (client: any, index: number) => (
                                <div className="sso-client-card" key={index}>
                                    <h3>
                                        {client.display_name || "Client"}
                                    </h3>

                                    <label>
                                        Client ID
                                        <input
                                            value={client.client_id}
                                            onChange={e =>
                                                update(
                                                    `sso.clients.${index}.client_id`,
                                                    e.target.value
                                                )
                                            }
                                        />
                                    </label>

                                    <label>
                                        Client Secret
                                        <input
                                            type="password"
                                            value={client.client_secret}
                                            onChange={e =>
                                                update(
                                                    `sso.clients.${index}.client_secret`,
                                                    e.target.value
                                                )
                                            }
                                        />
                                    </label>

                                    <label>
                                        Redirect URIs
                                        <textarea
                                            value={client.redirect_uris.join("\n")}
                                            onChange={e =>
                                                update(
                                                    `sso.clients.${index}.redirect_uris`,
                                                    e.target.value
                                                        .split("\n")
                                                        .filter(Boolean)
                                                )
                                            }
                                        />
                                    </label>

                                    <label>
                                        Grant Types
                                        <input
                                            value={client.grant_types.join(", ")}
                                            onChange={e =>
                                                update(
                                                    `sso.clients.${index}.grant_types`,
                                                    e.target.value
                                                        .split(",")
                                                        .map(s => s.trim())
                                                )
                                            }
                                        />
                                    </label>

                                    <button
                                        className="icon-button"
                                        onClick={() =>
                                            update(
                                                "sso.clients",
                                                settingsJSON.sso.clients.filter(
                                                    (_: any, i: number) =>
                                                        i !== index
                                                )
                                            )
                                        }
                                    >
                                        Remove Client
                                    </button>
                                </div>
                            )
                        )}
                    </div>

                    <button
                        className="icon-button"
                        onClick={() =>
                            update("sso.clients", [
                                ...settingsJSON.sso.clients,
                                {
                                    client_id: "",
                                    client_secret: "",
                                    redirect_uris: [],
                                    grant_types: [],
                                    response_types: ["code"],
                                    display_name: "",
                                    type: "",
                                },
                            ])
                        }
                    >
                        + Add Client
                    </button>
                </fieldset>
            )}

            {isSSO && (
                <p className="warning-text">
                    Changes to Auth settings require a server restart to take
                </p>
            )}
        </div>
    );
};