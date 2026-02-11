import React, { useState, useEffect } from 'react';
import { Search } from 'lucide-react';
import { Task } from './Board';

interface SearchBarProps {
  onTaskSelect: (task: Task) => void;
}

export function SearchBar({ onTaskSelect }: SearchBarProps) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<Task[]>([]);
  const [loading, setLoading] = useState(false);
  const [showResults, setShowResults] = useState(false);

  useEffect(() => {
    const timer = setTimeout(async () => {
      if (query.trim().length < 3) {
        setResults([]);
        setShowResults(false);
        return;
      }

      setLoading(true);
      try {
        const res = await fetch(`/api/search?q=${encodeURIComponent(query)}`);
        if (res.ok) {
          const data = await res.json();
          setResults(data || []);
          setShowResults(true);
        }
      } catch (error) {
        console.error("Search failed:", error);
      } finally {
        setLoading(false);
      }
    }, 500); // Debounce 500ms

    return () => clearTimeout(timer);
  }, [query]);

  return (
    <div className="relative w-full max-w-md mx-auto mb-6">
      <div className="relative">
        <input
          type="text"
          placeholder="Search tasks semantically..."
          className="w-full rounded-full border border-gray-300 bg-white py-2 pl-10 pr-4 shadow-sm focus:border-rose-500 focus:outline-none focus:ring-1 focus:ring-rose-500 dark:border-zinc-700 dark:bg-zinc-900"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => query.length >= 3 && setShowResults(true)}
          onBlur={() => setTimeout(() => setShowResults(false), 200)}
        />
        <Search className="absolute left-3 top-2.5 h-5 w-5 text-gray-400" />
        {loading && (
          <div className="absolute right-3 top-2.5 h-5 w-5 animate-spin rounded-full border-2 border-gray-300 border-t-rose-500" />
        )}
      </div>

      {showResults && results.length > 0 && (
        <div className="absolute z-50 mt-2 w-full overflow-hidden rounded-xl bg-white shadow-xl ring-1 ring-black ring-opacity-5 dark:bg-zinc-800">
          <ul className="max-h-60 overflow-y-auto py-1">
            {results.map((task) => (
              <li
                key={task.id}
                onClick={() => {
                  onTaskSelect(task);
                  setShowResults(false);
                  setQuery('');
                }}
                className="cursor-pointer px-4 py-2 hover:bg-rose-50 dark:hover:bg-zinc-700"
              >
                <div className="font-medium text-gray-900 dark:text-white">{task.title}</div>
                <div className="truncate text-xs text-gray-500">{task.description}</div>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
