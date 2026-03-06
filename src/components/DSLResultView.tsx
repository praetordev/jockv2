import type { DSLResult, DSLCommitResult } from '../types';
import DSLHighlighter from './DSLHighlighter';

interface DSLResultViewProps {
  query: string;
  result: DSLResult;
  onCommitClick?: (hash: string) => void;
}

export default function DSLResultView({ query, result, onCommitClick }: DSLResultViewProps) {
  return (
    <div>
      <div className="text-zinc-400">
        <span className="text-[#F14E32]">jock&gt;</span>{' '}
        <DSLHighlighter value={query} />
      </div>
      <div className="mt-1">{renderResult(result, onCommitClick)}</div>
    </div>
  );
}

function renderResult(result: DSLResult, onCommitClick?: (hash: string) => void) {
  switch (result.resultKind) {
    case 'commits':
      return renderCommits(result, onCommitClick);
    case 'branches':
      return renderBranches(result);
    case 'blame':
      return renderBlame(result);
    case 'stashes':
      return renderStashes(result);
    case 'count':
      return <div className="text-amber-400">{result.count}</div>;
    case 'aggregate':
      return renderAggregates(result);
    case 'formatted':
      return (
        <pre className={`whitespace-pre font-mono text-sm ${result.formatType === 'json' ? 'text-amber-400' : 'text-zinc-300'}`}>
          {result.formattedOutput}
        </pre>
      );
    case 'action_report':
      return renderActionReport(result);
    case 'error':
      return <div className="text-rose-400">{result.error}</div>;
    default:
      return <div className="text-zinc-500">unknown result type</div>;
  }
}

const ROW_HEIGHT = 32;

function renderCommits(result: DSLResult, onCommitClick?: (hash: string) => void) {
  const commits = result.commits;
  if (!commits || commits.length === 0) {
    return <div className="text-zinc-500 italic">no commits found</div>;
  }

  // Only render the graph when commits have meaningful connections.
  // Filtered/sparse results (e.g. where author == "x") produce disconnected
  // nodes with no lines — fall back to the cleaner table view.
  const totalConns = commits.reduce(
    (sum, c) => sum + (c.graph?.connections?.length ?? 0),
    0,
  );
  const hasGraph = commits.some((c) => c.graph?.color) && totalConns > 0;
  if (!hasGraph) {
    return renderCommitTable(commits, onCommitClick);
  }

  const maxCol = commits.reduce((max, c) => Math.max(max, c.graph?.column ?? 0), 0);
  const graphWidth = 24 + (maxCol + 1) * 20;

  return (
    <div className="relative">
      <svg
        className="absolute top-0 left-0 h-full pointer-events-none"
        style={{ width: graphWidth, minHeight: commits.length * ROW_HEIGHT }}
      >
        {commits.map((c, i) => {
          if (!c.graph?.color) return null;
          const startX = 12 + c.graph.column * 20;
          const startY = i * ROW_HEIGHT + ROW_HEIGHT / 2;
          return (c.graph.connections || []).map((conn, connIdx) => {
            const endX = 12 + conn.toColumn * 20;
            const endY = conn.toRow * ROW_HEIGHT + ROW_HEIGHT / 2;
            let path = '';
            if (startX === endX) {
              path = `M ${startX} ${startY} L ${endX} ${endY}`;
            } else {
              const midY = (startY + endY) / 2;
              path = `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`;
            }
            return (
              <path
                key={`p-${c.hash}-${connIdx}`}
                d={path}
                fill="none"
                stroke={conn.color}
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            );
          });
        })}
        {commits.map((c, i) => {
          if (!c.graph?.color) return null;
          return (
            <circle
              key={`n-${c.hash}`}
              cx={12 + c.graph.column * 20}
              cy={i * ROW_HEIGHT + ROW_HEIGHT / 2}
              r="3.5"
              fill={c.graph.color}
              stroke="#09090b"
              strokeWidth="2"
            />
          );
        })}
      </svg>
      <div>
        {commits.map((c) => (
          <div
            key={c.hash}
            className={`flex items-center gap-2 text-xs ${onCommitClick ? 'hover:bg-zinc-800/50 cursor-pointer' : ''}`}
            style={{ height: ROW_HEIGHT, paddingLeft: graphWidth + 8 }}
            onClick={() => onCommitClick?.(c.hash)}
          >
            <span className="text-amber-400 w-14 flex-shrink-0">{c.hash.slice(0, 7)}</span>
            <span className="text-zinc-300 truncate flex-1">{c.message}</span>
            {c.branches?.map((b) => (
              <span key={b} className="px-1 py-0.5 rounded text-[10px] font-mono border truncate" style={{ color: c.graph?.color, borderColor: c.graph?.color, backgroundColor: `${c.graph?.color}15` }}>
                {b}
              </span>
            ))}
            <span className="text-cyan-400 flex-shrink-0">{c.author}</span>
            <span className="text-zinc-500 flex-shrink-0">{c.date}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function renderCommitTable(commits: DSLCommitResult[], onCommitClick?: (hash: string) => void) {
  return (
    <table className="w-full text-left">
      <thead>
        <tr className="text-zinc-500">
          <th className="pr-3 font-medium">HASH</th>
          <th className="pr-3 font-medium">AUTHOR</th>
          <th className="pr-3 font-medium">DATE</th>
          <th className="font-medium">MESSAGE</th>
        </tr>
      </thead>
      <tbody>
        {commits.map((c) => (
          <tr
            key={c.hash}
            className={onCommitClick ? 'hover:bg-zinc-800/50 cursor-pointer' : ''}
            onClick={() => onCommitClick?.(c.hash)}
          >
            <td className="pr-3 text-amber-400">{c.hash.slice(0, 7)}</td>
            <td className="pr-3 text-cyan-400">{c.author}</td>
            <td className="pr-3 text-zinc-500">{c.date}</td>
            <td className="text-zinc-300 truncate max-w-[400px]">
              {c.message}
              {c.branches && c.branches.length > 0 && (
                <>
                  {' '}
                  {c.branches.map((b) => (
                    <span key={b} className="text-emerald-400 ml-1">
                      [{b}]
                    </span>
                  ))}
                </>
              )}
              {c.tags && c.tags.length > 0 && (
                <>
                  {' '}
                  {c.tags.map((t) => (
                    <span key={t} className="text-yellow-400 ml-1">
                      ({t})
                    </span>
                  ))}
                </>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function renderBranches(result: DSLResult) {
  const branches = result.branches;
  if (!branches || branches.length === 0) {
    return <div className="text-zinc-500 italic">no branches found</div>;
  }

  return (
    <div>
      {branches.map((b) => (
        <div key={b.name} className="flex items-center gap-2">
          <span className={b.isCurrent ? 'text-emerald-400' : 'text-zinc-400'}>
            {b.isCurrent ? '* ' : '  '}
            {b.name}
          </span>
          {b.remote && <span className="text-zinc-600">→ {b.remote}</span>}
        </div>
      ))}
    </div>
  );
}

function renderAggregates(result: DSLResult) {
  const rows = result.aggregates;
  if (!rows || rows.length === 0) {
    return <div className="text-zinc-500 italic">no results</div>;
  }

  const formatValue = (v: number) =>
    result.aggFunc === 'avg' ? v.toFixed(1) : String(Math.round(v));

  // Ungrouped scalar (single row, empty group)
  if (rows.length === 1 && !rows[0].group) {
    const label = [result.aggFunc, result.aggField].filter(Boolean).join(' ');
    return (
      <div className="text-amber-400">
        {label}: {formatValue(rows[0].value)}
      </div>
    );
  }

  // Grouped table
  const headerLabel = [result.aggFunc, result.aggField].filter(Boolean).join(' ').toUpperCase();
  return (
    <table className="w-full text-left">
      <thead>
        <tr className="text-zinc-500">
          <th className="pr-3 font-medium">{(result.groupField || 'GROUP').toUpperCase()}</th>
          <th className="font-medium">{headerLabel || 'VALUE'}</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i}>
            <td className="pr-3 text-cyan-400">{row.group}</td>
            <td className="text-amber-400">{formatValue(row.value)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function renderBlame(result: DSLResult) {
  const lines = result.blameLines;
  if (!lines || lines.length === 0) {
    return <div className="text-zinc-500 italic">no blame data</div>;
  }

  return (
    <table className="w-full text-left">
      <thead>
        <tr className="text-zinc-500">
          <th className="pr-2 font-medium">LINE</th>
          <th className="pr-2 font-medium">HASH</th>
          <th className="pr-2 font-medium">AUTHOR</th>
          <th className="pr-2 font-medium">DATE</th>
          <th className="font-medium">CONTENT</th>
        </tr>
      </thead>
      <tbody>
        {lines.map((line, i) => (
          <tr key={i}>
            <td className="pr-2 text-zinc-600 text-right">{line.lineNo}</td>
            <td className="pr-2 text-amber-400">{line.hash.slice(0, 7)}</td>
            <td className="pr-2 text-cyan-400">{line.author}</td>
            <td className="pr-2 text-zinc-500">{line.date}</td>
            <td className="text-zinc-300 whitespace-pre">{line.content}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function renderStashes(result: DSLResult) {
  const stashes = result.stashes;
  if (!stashes || stashes.length === 0) {
    return <div className="text-zinc-500 italic">no stashes found</div>;
  }

  return (
    <table className="w-full text-left">
      <thead>
        <tr className="text-zinc-500">
          <th className="pr-3 font-medium">#</th>
          <th className="pr-3 font-medium">BRANCH</th>
          <th className="pr-3 font-medium">DATE</th>
          <th className="font-medium">MESSAGE</th>
        </tr>
      </thead>
      <tbody>
        {stashes.map((s) => (
          <tr key={s.index}>
            <td className="pr-3 text-amber-400">{s.index}</td>
            <td className="pr-3 text-emerald-400">{s.branch}</td>
            <td className="pr-3 text-zinc-500">{s.date}</td>
            <td className="text-zinc-300">{s.message}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function renderActionReport(result: DSLResult) {
  const report = result.actionReport;
  if (!report) {
    return <div className="text-zinc-500">action completed</div>;
  }

  return (
    <div>
      {report.dryRun && <span className="text-yellow-400 mr-2">[DRY RUN]</span>}
      <span className={`whitespace-pre-wrap ${report.success ? 'text-emerald-400' : 'text-rose-400'}`}>
        {report.description}
      </span>
      {report.affectedHashes && report.affectedHashes.length > 0 && (
        <div className="mt-1 text-zinc-500">
          affected: {report.affectedHashes.map((h) => h.slice(0, 7)).join(', ')}
        </div>
      )}
      {report.errors && report.errors.length > 0 && (
        <div className="mt-1">
          {report.errors.map((e, i) => (
            <div key={i} className="text-rose-400">
              {e}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
