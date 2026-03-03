"use client";

import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import React, { useEffect } from "react";

interface RichTextEditorProps {
    content: string;
    onChange: (value: string) => void;
    placeholder?: string;
    className?: string;
}

const MenuBar = ({ editor }: { editor: any }) => {
    if (!editor) {
        return null;
    }

    return (
        <div className="flex flex-wrap items-center gap-1 border-b p-2 bg-gray-50 dark:bg-zinc-800/50 dark:border-zinc-700/50">
            <button
                type="button"
                onClick={() => editor.chain().focus().toggleBold().run()}
                disabled={!editor.can().chain().focus().toggleBold().run()}
                className={`px-2 py-1 rounded text-sm ${editor.isActive("bold") ? "bg-zinc-200 dark:bg-zinc-700 font-bold" : "hover:bg-zinc-100 dark:hover:bg-zinc-700"
                    }`}
            >
                B
            </button>
            <button
                type="button"
                onClick={() => editor.chain().focus().toggleItalic().run()}
                disabled={!editor.can().chain().focus().toggleItalic().run()}
                className={`px-2 py-1 rounded text-sm italic ${editor.isActive("italic") ? "bg-zinc-200 dark:bg-zinc-700" : "hover:bg-zinc-100 dark:hover:bg-zinc-700"
                    }`}
            >
                I
            </button>
            <button
                type="button"
                onClick={() => editor.chain().focus().toggleCodeBlock().run()}
                className={`px-2 py-1 rounded text-sm font-mono ${editor.isActive("codeBlock") ? "bg-zinc-200 dark:bg-zinc-700" : "hover:bg-zinc-100 dark:hover:bg-zinc-700"
                    }`}
            >
                &lt;/&gt;
            </button>
            <button
                type="button"
                onClick={() => editor.chain().focus().toggleBulletList().run()}
                className={`px-2 py-1 rounded text-sm ${editor.isActive("bulletList") ? "bg-zinc-200 dark:bg-zinc-700" : "hover:bg-zinc-100 dark:hover:bg-zinc-700"
                    }`}
            >
                • List
            </button>
        </div>
    );
};

export function RichTextEditor({
    content,
    onChange,
    placeholder = "Write something...",
    className = "",
}: RichTextEditorProps) {
    const editor = useEditor({
        extensions: [
            StarterKit,
            Placeholder.configure({
                placeholder,
            }),
        ],
        content,
        onUpdate: ({ editor }) => {
            onChange(editor.getHTML());
        },
        editorProps: {
            attributes: {
                class:
                    "prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[150px] p-4",
            },
        },
    });

    // Sync content if it changes externally
    useEffect(() => {
        if (editor && content !== editor.getHTML()) {
            editor.commands.setContent(content);
        }
    }, [content, editor]);

    return (
        <div
            className={`flex flex-col rounded-xl overflow-hidden border bg-white dark:bg-zinc-900 border-gray-200 dark:border-zinc-700 shadow-sm focus-within:ring-2 focus-within:ring-rose-500 focus-within:border-transparent ${className}`}
        >
            <MenuBar editor={editor} />
            <div className="flex-1 overflow-y-auto cursor-text" onClick={() => editor?.commands.focus()}>
                <EditorContent editor={editor} />
            </div>
        </div>
    );
}
