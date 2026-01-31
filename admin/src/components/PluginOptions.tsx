import {Switch} from "./Switch.tsx";
import {useStore} from "../store/store.ts";

export const PluginOptions = ({
                           pluginName,
                           config,
                       }: {
    pluginName: string;
    config: Record<string, any>;
}) => {
    const update = useStore(s => s.updateSetting);

    return (
        <div className="plugin-options">
            {Object.entries(config)
                .filter(([key]) => key !== "enabled")
                .map(([key, value]) => {
                    const path = `plugins.${pluginName}.${key}`;

                    if (typeof value === "boolean") {
                        return (
                            <label
                                key={key}
                                className="switch-row"
                            >
                                {key}
                                <Switch
                                    checked={value}
                                    onCheckedChange={v =>
                                        update(path, v)
                                    }
                                />
                            </label>
                        );
                    }

                    if (typeof value === "number") {
                        return (
                            <label key={key}>
                                {key}
                                <input
                                    type="number"
                                    value={value}
                                    onChange={e =>
                                        update(
                                            path,
                                            Number(e.target.value)
                                        )
                                    }
                                />
                            </label>
                        );
                    }

                    return (
                        <label key={key}>
                            {key}
                            <input
                                value={String(value)}
                                onChange={e =>
                                    update(path, e.target.value)
                                }
                            />
                        </label>
                    );
                })}
        </div>
    );
};