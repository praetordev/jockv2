import { useState, useMemo, useCallback, useRef } from 'react';
import { Columns2, Rows3, MessageSquarePlus, Plus, Minus, X, ChevronDown, ChevronRight } from 'lucide-react';
import { parseDiff, buildSplitLines, getLanguageFromPath } from '../utils/diffParser';
import type { DiffLine, DiffHunk, WordDiffSegment, SplitLine } from '../utils/diffParser';
import { highlightLine } from '../utils/syntaxHighlight';

export type DiffViewMode = 'unified' | 'split';

export interface DiffAnnotation {
  id: string;
  lineIndex: number;
  hunkIndex: number;
  text: string;
  author?: string;
  timestamp?: string;
}

interface DiffViewerProps {
  patch: string;
  filePath?: string;
  mode?: DiffViewMode;
  onModeChange?: (mode: DiffViewMode) => void;
  showToolbar?: boolean;
  annotations?: DiffAnnotation[];
  onAddAnnotation?: (hunkIndex: number, lineIndex: number, text: string) => void;
  onDeleteAnnotation?: (id: string) => void;
  onStageHunk?: (hunkIndex: number) => void;
  onUnstageHunk?: (hunkIndex: number) => void;
  onStageLine?: (hunkIndex: number, lineIndex: number) => void;
  showStaging?: boolean;
  className?: string;
}

export default function DiffViewer({
  patch,
  filePath,
  mode: controlledMode,
  onModeChange,
  showToolbar = true,
  annotations = [],
  onAddAnnotation,
  onDeleteAnnotation,
  onStageHunk,
  onUnstageHunk,
  onStageLine,
  showStaging = false,
  className = '',
}: DiffViewerProps) {
  const [internalMode, setInternalMode] = useState<DiffViewMode>('unified');
  const mode = controlledMode ?? internalMode;
  const setMode = onModeChange ?? setInternalMode;

  const [collapsedHunks, setCollapsedHunks] = useState<Set<number>>(new Set());
  const [annotatingLine, setAnnotatingLine] = useState<{ hunk: number; line: number } | null>(null);
  const [annotationText, setAnnotationText] = useState('');
  const annotationInputRef = useRef<HTMLTextAreaElement>(null);

  const language = filePath ? getLanguageFromPath(filePath) : 'text';
  const parsed = useMemo(() => parseDiff(patch), [patch]);

  const toggleHunk = useCallback((idx: number) => {
    setCollapsedHunks(prev => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }, []);

  const handleAddAnnotation = useCallback(() => {
    if (annotatingLine && annotationText.trim() && onAddAnnotation) {
      onAddAnnotation(annotatingLine.hunk, annotatingLine.line, annotationText.trim());
      setAnnotatingLine(null);
      setAnnotationText('');
    }
  }, [annotatingLine, annotationText, onAddAnnotation]);

  const getAnnotationsForLine = useCallback(
    (hunkIdx: number, lineIdx: number) =>
      annotations.filter(a => a.hunkIndex === hunkIdx && a.lineIndex === lineIdx),
    [annotations]
  );

  return (
    <div className={`flex flex-col min-h-0 ${className}`}>
      {showToolbar && (
        <DiffToolbar
          mode={mode}
          onModeChange={setMode}
          hunkCount={parsed.hunks.length}
          additions={countLines(parsed.hunks, 'addition')}
          deletions={countLines(parsed.hunks, 'deletion')}
        />
      )}
      <div className="flex-1 overflow-auto font-mono text-[11px] leading-relaxed">
        {/* Diff headers */}
        {parsed.headers.length > 0 && (
          <div className="whitespace-pre">
            {parsed.headers.map((h, i) => (
              <div key={i} className="px-2 py-0.5 text-zinc-500 bg-zinc-800/20">
                {h}
              </div>
            ))}
          </div>
        )}
        {mode === 'unified' ? (
          <UnifiedView
            hunks={parsed.hunks}
            language={language}
            collapsedHunks={collapsedHunks}
            onToggleHunk={toggleHunk}
            showStaging={showStaging}
            onStageHunk={onStageHunk}
            onUnstageHunk={onUnstageHunk}
            onStageLine={onStageLine}
            annotations={annotations}
            getAnnotationsForLine={getAnnotationsForLine}
            annotatingLine={annotatingLine}
            annotationText={annotationText}
            annotationInputRef={annotationInputRef}
            onSetAnnotatingLine={setAnnotatingLine}
            onSetAnnotationText={setAnnotationText}
            onAddAnnotation={onAddAnnotation ? handleAddAnnotation : undefined}
            onDeleteAnnotation={onDeleteAnnotation}
          />
        ) : (
          <SplitView
            hunks={parsed.hunks}
            language={language}
            collapsedHunks={collapsedHunks}
            onToggleHunk={toggleHunk}
            showStaging={showStaging}
            onStageHunk={onStageHunk}
            onUnstageHunk={onUnstageHunk}
          />
        )}
      </div>
    </div>
  );
}

function DiffToolbar({
  mode,
  onModeChange,
  hunkCount,
  additions,
  deletions,
}: {
  mode: DiffViewMode;
  onModeChange: (m: DiffViewMode) => void;
  hunkCount: number;
  additions: number;
  deletions: number;
}) {
  return (
    <div className="flex items-center gap-2 px-3 py-1.5 border-b border-zinc-800/60 bg-zinc-900/40 text-xs shrink-0">
      <div className="flex items-center gap-1 bg-zinc-800/50 rounded p-0.5">
        <button
          onClick={() => onModeChange('unified')}
          className={`flex items-center gap-1 px-2 py-1 rounded transition-colors ${
            mode === 'unified' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
          }`}
          title="Unified view"
        >
          <Rows3 size={12} />
          <span>Unified</span>
        </button>
        <button
          onClick={() => onModeChange('split')}
          className={`flex items-center gap-1 px-2 py-1 rounded transition-colors ${
            mode === 'split' ? 'bg-zinc-700 text-zinc-100' : 'text-zinc-500 hover:text-zinc-300'
          }`}
          title="Side-by-side view"
        >
          <Columns2 size={12} />
          <span>Split</span>
        </button>
      </div>
      <div className="flex-1" />
      <span className="text-zinc-500">{hunkCount} {hunkCount === 1 ? 'hunk' : 'hunks'}</span>
      <span className="text-emerald-500">+{additions}</span>
      <span className="text-rose-500">-{deletions}</span>
    </div>
  );
}

function countLines(hunks: DiffHunk[], type: DiffLine['type']): number {
  let count = 0;
  for (const h of hunks) {
    for (const l of h.lines) {
      if (l.type === type) count++;
    }
  }
  return count;
}

// --- Unified View ---

interface UnifiedViewProps {
  hunks: DiffHunk[];
  language: string;
  collapsedHunks: Set<number>;
  onToggleHunk: (idx: number) => void;
  showStaging: boolean;
  onStageHunk?: (idx: number) => void;
  onUnstageHunk?: (idx: number) => void;
  onStageLine?: (hunkIdx: number, lineIdx: number) => void;
  annotations: DiffAnnotation[];
  getAnnotationsForLine: (h: number, l: number) => DiffAnnotation[];
  annotatingLine: { hunk: number; line: number } | null;
  annotationText: string;
  annotationInputRef: React.RefObject<HTMLTextAreaElement | null>;
  onSetAnnotatingLine: (v: { hunk: number; line: number } | null) => void;
  onSetAnnotationText: (v: string) => void;
  onAddAnnotation?: () => void;
  onDeleteAnnotation?: (id: string) => void;
}

function UnifiedView({
  hunks,
  language,
  collapsedHunks,
  onToggleHunk,
  showStaging,
  onStageHunk,
  onUnstageHunk,
  onStageLine,
  getAnnotationsForLine,
  annotatingLine,
  annotationText,
  annotationInputRef,
  onSetAnnotatingLine,
  onSetAnnotationText,
  onAddAnnotation,
  onDeleteAnnotation,
}: UnifiedViewProps) {
  return (
    <div className="whitespace-pre" role="table" aria-label="Unified diff view">
      {hunks.map((hunk, hunkIdx) => (
        <div key={hunkIdx}>
          {/* Hunk header */}
          <div className="flex items-center bg-zinc-800/30 sticky top-0 z-10 group">
            <button
              className="w-6 flex items-center justify-center text-zinc-500 hover:text-zinc-300 shrink-0"
              onClick={() => onToggleHunk(hunkIdx)}
              aria-label={collapsedHunks.has(hunkIdx) ? 'Expand hunk' : 'Collapse hunk'}
            >
              {collapsedHunks.has(hunkIdx) ? <ChevronRight size={12} /> : <ChevronDown size={12} />}
            </button>
            <span className="text-zinc-500 flex-1 px-2 py-1 select-none">{hunk.header}</span>
            {showStaging && (
              <div className="flex items-center gap-1 mr-2 opacity-0 group-hover:opacity-100 transition-opacity">
                {onStageHunk && (
                  <button
                    onClick={() => onStageHunk(hunkIdx)}
                    className="flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30"
                    title="Stage this hunk"
                  >
                    <Plus size={10} /> Stage
                  </button>
                )}
                {onUnstageHunk && (
                  <button
                    onClick={() => onUnstageHunk(hunkIdx)}
                    className="flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] rounded bg-rose-500/20 text-rose-400 hover:bg-rose-500/30"
                    title="Unstage this hunk"
                  >
                    <Minus size={10} /> Unstage
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Hunk lines */}
          {!collapsedHunks.has(hunkIdx) &&
            hunk.lines.slice(1).map((line, lineIdx) => {
              const lineAnnotations = getAnnotationsForLine(hunkIdx, lineIdx);
              const isAnnotating =
                annotatingLine?.hunk === hunkIdx && annotatingLine?.line === lineIdx;

              return (
                <div key={lineIdx}>
                  <div
                    className={`flex group/line ${lineBackground(line)}`}
                    role="row"
                  >
                    {/* Old line number */}
                    <span className="w-12 text-right pr-1 text-zinc-600 select-none shrink-0 border-r border-zinc-800/30">
                      {line.oldLineNo ?? ''}
                    </span>
                    {/* New line number */}
                    <span className="w-12 text-right pr-1 text-zinc-600 select-none shrink-0 border-r border-zinc-800/30">
                      {line.newLineNo ?? ''}
                    </span>
                    {/* Prefix */}
                    <span className={`w-5 text-center select-none shrink-0 ${lineTextColor(line)}`}>
                      {linePrefix(line)}
                    </span>
                    {/* Content */}
                    <span className={`flex-1 px-1 ${lineTextColor(line)}`}>
                      <LineContent line={line} language={language} />
                    </span>
                    {/* Action buttons */}
                    <span className="w-16 flex items-center gap-0.5 opacity-0 group-hover/line:opacity-100 transition-opacity shrink-0 pr-1">
                      {onAddAnnotation && line.type !== 'hunk-header' && (
                        <button
                          onClick={() => {
                            onSetAnnotatingLine({ hunk: hunkIdx, line: lineIdx });
                            setTimeout(() => annotationInputRef.current?.focus(), 50);
                          }}
                          className="p-0.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-blue-400"
                          title="Add comment"
                        >
                          <MessageSquarePlus size={12} />
                        </button>
                      )}
                      {showStaging && onStageLine && (line.type === 'addition' || line.type === 'deletion') && (
                        <button
                          onClick={() => onStageLine(hunkIdx, lineIdx)}
                          className="p-0.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-emerald-400"
                          title="Stage this line"
                        >
                          <Plus size={12} />
                        </button>
                      )}
                    </span>
                  </div>

                  {/* Annotations */}
                  {lineAnnotations.map(ann => (
                    <div key={ann.id} className="flex ml-[7.25rem] mr-4 my-1 bg-blue-500/10 border border-blue-500/20 rounded px-2 py-1.5 text-xs">
                      <div className="flex-1">
                        {ann.author && (
                          <span className="text-blue-400 font-medium mr-2">{ann.author}</span>
                        )}
                        <span className="text-zinc-300 whitespace-pre-wrap">{ann.text}</span>
                      </div>
                      {onDeleteAnnotation && (
                        <button
                          onClick={() => onDeleteAnnotation(ann.id)}
                          className="ml-2 p-0.5 rounded hover:bg-zinc-700 text-zinc-500 hover:text-rose-400 shrink-0"
                        >
                          <X size={12} />
                        </button>
                      )}
                    </div>
                  ))}

                  {/* Annotation input */}
                  {isAnnotating && (
                    <div className="flex flex-col ml-[7.25rem] mr-4 my-1 bg-zinc-800/50 border border-zinc-700 rounded p-2">
                      <textarea
                        ref={annotationInputRef}
                        value={annotationText}
                        onChange={e => onSetAnnotationText(e.target.value)}
                        className="bg-zinc-900 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-200 resize-none h-14 focus:outline-none focus:border-blue-500"
                        placeholder="Add a comment..."
                        onKeyDown={e => {
                          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                            onAddAnnotation();
                          } else if (e.key === 'Escape') {
                            onSetAnnotatingLine(null);
                            onSetAnnotationText('');
                          }
                        }}
                      />
                      <div className="flex items-center gap-1 mt-1">
                        <button
                          onClick={onAddAnnotation}
                          className="px-2 py-0.5 text-[10px] rounded bg-blue-500/20 text-blue-400 hover:bg-blue-500/30"
                        >
                          Comment (⌘↵)
                        </button>
                        <button
                          onClick={() => {
                            onSetAnnotatingLine(null);
                            onSetAnnotationText('');
                          }}
                          className="px-2 py-0.5 text-[10px] rounded text-zinc-500 hover:text-zinc-300"
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
        </div>
      ))}
    </div>
  );
}

// --- Split View ---

function SplitView({
  hunks,
  language,
  collapsedHunks,
  onToggleHunk,
  showStaging,
  onStageHunk,
  onUnstageHunk,
}: {
  hunks: DiffHunk[];
  language: string;
  collapsedHunks: Set<number>;
  onToggleHunk: (idx: number) => void;
  showStaging: boolean;
  onStageHunk?: (idx: number) => void;
  onUnstageHunk?: (idx: number) => void;
}) {
  const splitData = useMemo(() => {
    const result: { hunkIdx: number; lines: SplitLine[] }[] = [];
    for (let i = 0; i < hunks.length; i++) {
      result.push({ hunkIdx: i, lines: buildSplitLines([hunks[i]]) });
    }
    return result;
  }, [hunks]);

  return (
    <div className="whitespace-pre" role="table" aria-label="Split diff view">
      {splitData.map(({ hunkIdx, lines }) => (
        <div key={hunkIdx}>
          {/* Hunk header */}
          <div className="flex items-center bg-zinc-800/30 sticky top-0 z-10 group">
            <button
              className="w-6 flex items-center justify-center text-zinc-500 hover:text-zinc-300 shrink-0"
              onClick={() => onToggleHunk(hunkIdx)}
            >
              {collapsedHunks.has(hunkIdx) ? <ChevronRight size={12} /> : <ChevronDown size={12} />}
            </button>
            <span className="text-zinc-500 flex-1 px-2 py-1 select-none">{hunks[hunkIdx].header}</span>
            {showStaging && (
              <div className="flex items-center gap-1 mr-2 opacity-0 group-hover:opacity-100 transition-opacity">
                {onStageHunk && (
                  <button
                    onClick={() => onStageHunk(hunkIdx)}
                    className="flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30"
                  >
                    <Plus size={10} /> Stage
                  </button>
                )}
                {onUnstageHunk && (
                  <button
                    onClick={() => onUnstageHunk(hunkIdx)}
                    className="flex items-center gap-0.5 px-1.5 py-0.5 text-[10px] rounded bg-rose-500/20 text-rose-400 hover:bg-rose-500/30"
                  >
                    <Minus size={10} /> Unstage
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Split lines (skip first which is the hunk header) */}
          {!collapsedHunks.has(hunkIdx) &&
            lines.slice(1).map((pair, idx) => (
              <div key={idx} className="flex">
                {/* Left (old) side */}
                <div className={`w-1/2 flex border-r border-zinc-800/40 ${
                  !pair.left ? 'bg-zinc-900/30' : lineBackground(pair.left)
                }`}>
                  {pair.left ? (
                    <>
                      <span className="w-10 text-right pr-1 text-zinc-600 select-none shrink-0">
                        {pair.left.oldLineNo ?? ''}
                      </span>
                      <span className="w-5 text-center select-none shrink-0 text-zinc-600">
                        {pair.left.type === 'deletion' ? '-' : ' '}
                      </span>
                      <span className={`flex-1 px-1 ${lineTextColor(pair.left)}`}>
                        <LineContent line={pair.left} language={language} />
                      </span>
                    </>
                  ) : (
                    <span className="flex-1" />
                  )}
                </div>
                {/* Right (new) side */}
                <div className={`w-1/2 flex ${
                  !pair.right ? 'bg-zinc-900/30' : lineBackground(pair.right)
                }`}>
                  {pair.right ? (
                    <>
                      <span className="w-10 text-right pr-1 text-zinc-600 select-none shrink-0">
                        {pair.right.newLineNo ?? ''}
                      </span>
                      <span className="w-5 text-center select-none shrink-0 text-zinc-600">
                        {pair.right.type === 'addition' ? '+' : ' '}
                      </span>
                      <span className={`flex-1 px-1 ${lineTextColor(pair.right)}`}>
                        <LineContent line={pair.right} language={language} />
                      </span>
                    </>
                  ) : (
                    <span className="flex-1" />
                  )}
                </div>
              </div>
            ))}
        </div>
      ))}
    </div>
  );
}

// --- Shared helpers ---

function lineBackground(line: DiffLine): string {
  switch (line.type) {
    case 'addition':
      return 'bg-emerald-500/10 hover:bg-emerald-500/15';
    case 'deletion':
      return 'bg-rose-500/10 hover:bg-rose-500/15';
    case 'hunk-header':
      return 'bg-zinc-800/30';
    default:
      return 'hover:bg-zinc-900';
  }
}

function lineTextColor(line: DiffLine): string {
  switch (line.type) {
    case 'addition':
      return 'text-emerald-400';
    case 'deletion':
      return 'text-rose-400';
    case 'hunk-header':
    case 'header':
      return 'text-zinc-500';
    default:
      return 'text-zinc-400';
  }
}

function linePrefix(line: DiffLine): string {
  switch (line.type) {
    case 'addition': return '+';
    case 'deletion': return '-';
    default: return ' ';
  }
}

function LineContent({ line, language }: { line: DiffLine; language: string }) {
  // Word-level diff takes priority
  if (line.wordDiff && line.wordDiff.length > 0) {
    return <WordDiffContent segments={line.wordDiff} lineType={line.type} />;
  }

  // Syntax highlighting for context and changed lines
  if (language !== 'text' && line.type !== 'hunk-header' && line.type !== 'header') {
    const highlighted = highlightLine(line.content, language);
    if (highlighted) {
      return <span dangerouslySetInnerHTML={{ __html: highlighted }} />;
    }
  }

  return <>{line.content}</>;
}

function WordDiffContent({
  segments,
  lineType,
}: {
  segments: WordDiffSegment[];
  lineType: DiffLine['type'];
}) {
  return (
    <>
      {segments.map((seg, i) => {
        if (seg.type === 'equal') {
          return <span key={i}>{seg.text}</span>;
        }
        const isHighlight =
          (lineType === 'deletion' && seg.type === 'deleted') ||
          (lineType === 'addition' && seg.type === 'added');

        if (!isHighlight) return <span key={i}>{seg.text}</span>;

        const cls =
          lineType === 'deletion'
            ? 'bg-rose-500/30 rounded-sm px-[1px]'
            : 'bg-emerald-500/30 rounded-sm px-[1px]';

        return (
          <span key={i} className={cls}>
            {seg.text}
          </span>
        );
      })}
    </>
  );
}
