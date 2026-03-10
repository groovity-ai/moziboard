import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Search, FileText, CheckCircle, Loader2, Command } from 'lucide-react';
import { Task } from './Board';

interface SearchResult {
  id: string | number;
  title: string;
  type: 'task' | 'doc';
  subtitle?: string;
  preview?: string;
  data: any;
}

interface SearchBarProps {
  boardId: string;
  onTaskSelect?: (task: Task) => void;
  onDocSelect?: (docId: string) => void;
  className?: string;
}

export function SearchBar({ boardId, onTaskSelect, onDocSelect, className }: SearchBarProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [showResults, setShowResults] = useState(false);
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  const handleSearch = useCallback(async (q: string) => {
    if (q.trim().length < 2) {
      setResults([]);
      setShowResults(false);
      return;
    }

    setLoading(true);
    try {
      const res = await fetch(`/api/search?board_id=${encodeURIComponent(boardId)}&q=${encodeURIComponent(q)}`);
      if (res.ok) {
        const data = await res.json();
        const mapped: SearchResult[] = (data || []).map((item: any) => ({
          id: item.id,
          title: item.title,
          type: item.type,
          subtitle: item.subtitle,
          preview: item.preview,
          data: item.data,
        }));
        setResults(mapped);
        setShowResults(true);
        setActiveIndex(0);
      }
    } catch (error) {
      console.error("Search failed:", error);
    } finally {
      setLoading(false);
    }
  }, [boardId]);

  useEffect(() => {
    const timer = setTimeout(() => handleSearch(query), 300);
    return () => clearTimeout(timer);
  }, [query, handleSearch]);

  // Command+K or / shortcut
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        inputRef.current?.focus();
      }
      if (e.key === '/' && document.activeElement?.tagName !== 'INPUT' && document.activeElement?.tagName !== 'TEXTAREA') {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const selectResult = (result: SearchResult) => {
    if (result.type === 'task' && onTaskSelect) {
      onTaskSelect(result.data);
    } else if (result.type === 'doc' && onDocSelect) {
      onDocSelect(String(result.id));
    }
    setShowResults(false);
    setQuery('');
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setActiveIndex(prev => (prev + 1) % results.length);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setActiveIndex(prev => (prev - 1 + results.length) % results.length);
    } else if (e.key === 'Enter' && results[activeIndex]) {
      e.preventDefault();
      selectResult(results[activeIndex]);
    } else if (e.key === 'Escape') {
      setShowResults(false);
      inputRef.current?.blur();
    }
  };

  return (
    <div className={`relative z-[80] w-full ${className}`}>
      <div className="relative group">
        <input
          ref={inputRef}
          type="text"
          placeholder="Search task or doc... (Cmd+K)"
          className="w-full h-9 rounded-lg border border-input bg-background pl-9 pr-12 text-sm ring-offset-background transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500/20 focus-visible:ring-offset-0 dark:border-zinc-800 dark:bg-zinc-950"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => query.length >= 2 && setShowResults(true)}
          onKeyDown={onKeyDown}
        />
        <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground group-focus-within:text-rose-500 transition-colors" />
        
        <div className="absolute right-3 top-2 flex items-center gap-1 pointer-events-none">
          {loading ? (
            <Loader2 className="h-4 w-4 animate-spin text-rose-500" />
          ) : (
            <div className="hidden sm:flex items-center gap-0.5 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground opacity-100">
              <span className="text-xs">⌘</span>K
            </div>
          )}
        </div>
      </div>

      {showResults && results.length > 0 && (
        <>
          <div 
            className="fixed inset-0 z-[85] bg-transparent" 
            onClick={() => setShowResults(false)}
          />
          <div className="absolute z-[90] mt-2 w-full min-w-[320px] overflow-hidden rounded-xl border bg-popover text-popover-foreground shadow-2xl ring-1 ring-black/5 animate-in fade-in zoom-in-95 dark:border-zinc-800">
            <div className="p-2 text-[10px] font-medium uppercase tracking-wider text-muted-foreground bg-muted/30 border-b dark:border-zinc-800">
              Results
            </div>
            <ul className="max-h-[380px] overflow-y-auto p-1">
              {results.map((result, index) => (
                <li
                  key={`${result.type}-${result.id}`}
                  onClick={() => selectResult(result)}
                  onMouseEnter={() => setActiveIndex(index)}
                  className={`group flex items-center gap-3 cursor-pointer rounded-lg px-3 py-2 transition-colors ${
                    index === activeIndex ? 'bg-rose-500/10 text-rose-600 dark:bg-rose-500/20 dark:text-rose-400' : 'hover:bg-muted'
                  }`}
                >
                  <div className={`flex h-8 w-8 shrink-0 items-center justify-center rounded-md border ${
                    index === activeIndex ? 'border-rose-500/30 bg-rose-500/10' : 'bg-background'
                  }`}>
                    {result.type === 'task' ? (
                      <CheckCircle className="h-4 w-4" />
                    ) : (
                      <FileText className="h-4 w-4" />
                    )}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-sm truncate">{result.title}</div>
                    {result.subtitle && (
                      <div className="text-[10px] opacity-70 truncate uppercase font-semibold">
                        {result.subtitle}
                      </div>
                    )}
                    {result.preview && (
                      <div className="text-[11px] opacity-60 truncate mt-0.5">
                        {result.preview.replace(/<[^>]*>?/gm, '')}
                      </div>
                    )}
                  </div>
                  <Command className={`h-3 w-3 opacity-0 transition-opacity ${index === activeIndex ? 'opacity-40' : ''}`} />
                </li>
              ))}
            </ul>
          </div>
        </>
      )}
    </div>
  );
}
