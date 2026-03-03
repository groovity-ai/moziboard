'use client';

import React, { useState } from 'react';
import useSWR, { mutate } from 'swr';
import Link from 'next/link';
import { Plus, LayoutDashboard, ChevronRight } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';

const fetcher = (url: string) => fetch(url).then((res) => res.json());

export default function Dashboard() {
    const { data: boards } = useSWR('/api/boards', fetcher);
    const [isCreating, setIsCreating] = useState(false);
    const [newTitle, setNewTitle] = useState('');
    const [newDesc, setNewDesc] = useState('');

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        await fetch('/api/boards', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ title: newTitle, description: newDesc }),
        });
        mutate('/api/boards');
        setIsCreating(false);
        setNewTitle('');
        setNewDesc('');
    };

    return (
        <div className="flex min-h-screen w-full flex-col bg-background">
            <header className="sticky top-0 z-50 flex h-16 shrink-0 items-center justify-between border-b bg-background/95 px-6 backdrop-blur supports-[backdrop-filter]:bg-background/60">
                <div className="flex items-center gap-3">
                    <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-brand text-brand-foreground shadow-sm">
                        <LayoutDashboard size={20} />
                    </div>
                    <h1 className="text-xl font-bold tracking-tight">Moziboard / Workspace</h1>
                </div>
                <Button onClick={() => setIsCreating(true)} size="sm">
                    <Plus className="mr-2 h-4 w-4" /> New Project
                </Button>
            </header>

            <main className="flex-1 p-8">
                <div className="mx-auto max-w-7xl">
                    <div className="mb-8 flex items-center justify-between">
                        <div>
                            <h2 className="text-2xl font-bold tracking-tight">Projects</h2>
                            <p className="text-muted-foreground">Manage your sprints and agent workspaces.</p>
                        </div>
                    </div>

                    <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                        {/* Empty State / Add New Card */}
                        <button
                            onClick={() => setIsCreating(true)}
                            className="group flex h-48 flex-col items-center justify-center rounded-2xl border-2 border-dashed border-muted-foreground/25 bg-transparent text-muted-foreground transition-all hover:border-brand hover:bg-brand/5 hover:text-brand"
                        >
                            <div className="rounded-full bg-muted/50 p-4 transition-colors group-hover:bg-brand/10">
                                <Plus size={24} />
                            </div>
                            <span className="mt-4 text-sm font-medium">Create New Board</span>
                        </button>

                        {boards?.map((board: any) => (
                            <Link key={board.id} href={`/board/${board.id}`}>
                                <div className="group relative flex h-48 flex-col justify-between rounded-2xl border bg-card p-6 shadow-sm transition-all hover:mb-1 hover:-mt-1 hover:shadow-md hover:border-brand/50">
                                    <div>
                                        <h3 className="text-lg font-bold group-hover:text-brand transition-colors">{board.title}</h3>
                                        <p className="mt-2 text-sm text-card-foreground/70 line-clamp-3 leading-relaxed">{board.description}</p>
                                    </div>
                                    <div className="flex items-center justify-between mt-4 border-t pt-4 border-border/50">
                                        <div className="text-xs font-medium text-muted-foreground">
                                            ID: {board.id.substring(0, 8)}...
                                        </div>
                                        <div className="rounded-full bg-secondary p-1.5 text-secondary-foreground opacity-0 transition-opacity group-hover:opacity-100">
                                            <ChevronRight size={14} />
                                        </div>
                                    </div>
                                </div>
                            </Link>
                        ))}
                    </div>
                </div>
            </main>

            {/* Create Modal */}
            <Dialog open={isCreating} onOpenChange={setIsCreating}>
                <DialogContent className="sm:max-w-[425px]">
                    <form onSubmit={handleCreate}>
                        <DialogHeader>
                            <DialogTitle>Create New Project</DialogTitle>
                            <DialogDescription>
                                Set up a new workspace for your squad to collaborate in.
                            </DialogDescription>
                        </DialogHeader>
                        <div className="grid gap-4 py-4">
                            <div className="grid gap-2">
                                <Label htmlFor="title">Project Title</Label>
                                <Input
                                    id="title"
                                    placeholder="e.g. Website Redesign"
                                    value={newTitle}
                                    onChange={(e) => setNewTitle(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="description">Description</Label>
                                <Input
                                    id="description"
                                    placeholder="Short description..."
                                    value={newDesc}
                                    onChange={(e) => setNewDesc(e.target.value)}
                                />
                            </div>
                        </div>
                        <DialogFooter>
                            <Button type="button" variant="outline" onClick={() => setIsCreating(false)}>
                                Cancel
                            </Button>
                            <Button type="submit">Create Project</Button>
                        </DialogFooter>
                    </form>
                </DialogContent>
            </Dialog>
        </div>
    );
}
