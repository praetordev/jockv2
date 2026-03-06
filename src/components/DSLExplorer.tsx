import React, { useState, useEffect } from 'react';
import { BookOpen, Play, History, ChevronRight } from 'lucide-react';
import { isElectron } from '../lib/electron';

interface DSLExplorerProps {
  onRunQuery?: (query: string) => void;
}

const EXAMPLE_QUERIES = [
  { query: 'commits | where author == "name" | first 10', desc: 'Recent commits by author' },
  { query: 'commits | where date within last 7 days | count', desc: 'Commit count this week' },
  { query: 'commits | where message contains "fix" | sort date desc', desc: 'Find fix commits' },
  { query: 'commits | group by author | count', desc: 'Commits per author' },
  { query: 'branches | where name contains "feature"', desc: 'Feature branches' },
  { query: 'blame "src/main.ts" | where author == "name"', desc: 'Blame by author' },
  { query: 'commits | where additions > 100 | sort additions desc | first 5', desc: 'Largest commits' },
  { query: 'commits | where date within last 30 days | group by author | count', desc: 'Monthly contributor stats' },
  { query: 'tags | sort date desc | first 10', desc: 'Recent tags' },
  { query: 'stash | first 5', desc: 'Recent stashes' },
];

const SYNTAX_SECTIONS = [
  {
    title: 'Sources',
    items: [
      { syntax: 'commits', desc: 'All commits in history' },
      { syntax: 'branches', desc: 'Local branches' },
      { syntax: 'tags', desc: 'All tags' },
      { syntax: 'blame "file"', desc: 'Line-by-line blame for a file' },
      { syntax: 'stash', desc: 'Stash entries' },
    ],
  },
  {
    title: 'Filters & Transforms',
    items: [
      { syntax: 'where <field> <op> <value>', desc: 'Filter rows by condition' },
      { syntax: 'select <fields...>', desc: 'Choose specific fields' },
      { syntax: 'sort <field> [asc|desc]', desc: 'Sort results' },
      { syntax: 'first N / last N / skip N', desc: 'Limit results' },
      { syntax: 'unique <field>', desc: 'Deduplicate by field' },
      { syntax: 'reverse', desc: 'Reverse result order' },
      { syntax: 'count', desc: 'Count matching rows' },
    ],
  },
  {
    title: 'Aggregation',
    items: [
      { syntax: 'group by <field>', desc: 'Group rows by field' },
      { syntax: 'sum / avg / min / max', desc: 'Aggregate functions' },
      { syntax: 'having <condition>', desc: 'Filter after grouping' },
    ],
  },
  {
    title: 'Actions',
    items: [
      { syntax: 'cherry-pick', desc: 'Cherry-pick matched commits' },
      { syntax: 'revert', desc: 'Revert matched commits' },
      { syntax: 'rebase', desc: 'Rebase onto matched target' },
      { syntax: 'tag "name"', desc: 'Tag matched commits' },
    ],
  },
  {
    title: 'Operators',
    items: [
      { syntax: '==, !=, >, <, >=, <=', desc: 'Comparison operators' },
      { syntax: 'contains', desc: 'Substring match' },
      { syntax: 'matches', desc: 'Regex match' },
      { syntax: 'within last N days/weeks', desc: 'Date range filter' },
    ],
  },
];

type Tab = 'reference' | 'examples' | 'history';

export default function DSLExplorer({ onRunQuery }: DSLExplorerProps) {
  const [activeTab, setActiveTab] = useState<Tab>('examples');
  const [history, setHistory] = useState<string[]>([]);
  const [expandedSection, setExpandedSection] = useState<string | null>('Sources');

  useEffect(() => {
    if (!isElectron) return;
    window.electronAPI.invoke('dsl:get-history').then(h => setHistory(h || []));
  }, []);

  const tabs: { key: Tab; label: string; icon: React.ReactNode }[] = [
    { key: 'examples', label: 'Examples', icon: <Play className="w-3 h-3" /> },
    { key: 'reference', label: 'Syntax', icon: <BookOpen className="w-3 h-3" /> },
    { key: 'history', label: 'History', icon: <History className="w-3 h-3" /> },
  ];

  return (
    <div className="flex flex-col h-full bg-zinc-950 text-zinc-300">
      {/* Tab bar */}
      <div className="flex border-b border-zinc-800/60 px-2">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs border-b-2 transition-colors ${
              activeTab === tab.key
                ? 'border-[#F14E32] text-zinc-200'
                : 'border-transparent text-zinc-500 hover:text-zinc-300'
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-3">
        {activeTab === 'examples' && (
          <div className="space-y-1">
            {EXAMPLE_QUERIES.map((ex, i) => (
              <button
                key={i}
                onClick={() => onRunQuery?.(ex.query)}
                className="w-full text-left px-3 py-2 rounded hover:bg-zinc-900 transition-colors group"
              >
                <div className="font-mono text-xs text-[#F14E32] group-hover:text-[#ff6b4f]">{ex.query}</div>
                <div className="text-[10px] text-zinc-500 mt-0.5">{ex.desc}</div>
              </button>
            ))}
          </div>
        )}

        {activeTab === 'reference' && (
          <div className="space-y-1">
            {SYNTAX_SECTIONS.map(section => (
              <div key={section.title}>
                <button
                  onClick={() => setExpandedSection(expandedSection === section.title ? null : section.title)}
                  className="w-full flex items-center gap-1 px-2 py-1.5 text-xs font-semibold text-zinc-400 hover:text-zinc-200 transition-colors"
                >
                  <ChevronRight className={`w-3 h-3 transition-transform ${expandedSection === section.title ? 'rotate-90' : ''}`} />
                  {section.title}
                </button>
                {expandedSection === section.title && (
                  <div className="ml-4 space-y-1 mb-2">
                    {section.items.map((item, j) => (
                      <div key={j} className="px-2 py-1 rounded bg-zinc-900/30">
                        <code className="text-xs text-emerald-400 font-mono">{item.syntax}</code>
                        <div className="text-[10px] text-zinc-500">{item.desc}</div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}

        {activeTab === 'history' && (
          <div className="space-y-1">
            {history.length === 0 ? (
              <div className="text-xs text-zinc-500 text-center py-4">No query history yet</div>
            ) : (
              history.map((q, i) => (
                <button
                  key={i}
                  onClick={() => onRunQuery?.(q)}
                  className="w-full text-left px-3 py-1.5 rounded hover:bg-zinc-900 transition-colors font-mono text-xs text-zinc-400 hover:text-zinc-200"
                >
                  {q}
                </button>
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}
