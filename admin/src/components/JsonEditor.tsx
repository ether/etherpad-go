import {useEffect, useRef} from "react";
import JSONEditor from "jsoneditor";
import "jsoneditor/dist/jsoneditor.css";

type JsonEditorProps = {
    value: string;
    onChange: (value: any) => void;
};

export const JsonEditor = ({
                               value,
                               onChange,
                           }: JsonEditorProps) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const editorRef = useRef<JSONEditor | null>(null);

    useEffect(() => {
        if (!containerRef.current) return;

        editorRef.current = new JSONEditor(containerRef.current, {
            mode: "tree",
            modes: ["tree", "code"],
            onChangeJSON: json => {
                onChange(json);
            },
        });

        editorRef.current.set(JSON.parse(value));

        return () => {
            editorRef.current?.destroy();
            editorRef.current = null;
        };
    }, []);

    // Update editor if external value changes
    useEffect(() => {
        if (!editorRef.current) return;

        try {
            const current = editorRef.current.get();
            const next = JSON.parse(value);

            if (JSON.stringify(current) !== JSON.stringify(next)) {
                editorRef.current.update(next);
            }
        } catch {
        }
    }, [value]);

    return <div ref={containerRef} style={{height: "100%"}} />;
};