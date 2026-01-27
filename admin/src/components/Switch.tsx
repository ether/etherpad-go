import * as RadixSwitch from "@radix-ui/react-switch";
import {forwardRef} from "react";
import clsx from "clsx";

type SwitchProps = RadixSwitch.SwitchProps & {
    className?: string;
};

export const Switch = forwardRef<
    HTMLButtonElement,
    SwitchProps
>(({className, ...props}, ref) => {
    return (
        <RadixSwitch.Root
            ref={ref}
            className={clsx("SwitchRoot", className)}
            {...props}
        >
            <RadixSwitch.Thumb className="SwitchThumb" />
        </RadixSwitch.Root>
    );
});

Switch.displayName = "Switch";