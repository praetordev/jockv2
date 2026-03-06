import { useMemo } from 'react';

interface Token {
  text: string;
  kind: 'keyword' | 'source' | 'operator' | 'string' | 'number' | 'field' | 'pipe' | 'bracket' | 'plain';
}

const SOURCES = new Set(['commits', 'branches', 'tags', 'files']);
const KEYWORDS = new Set([
  'where', 'select', 'sort', 'first', 'last', 'skip', 'count', 'unique', 'reverse',
  'group', 'by', 'sum', 'avg', 'min', 'max', 'format', 'except', 'intersect', 'union',
  'help', 'alias', 'aliases', 'unalias', 'asc', 'desc',
  'cherry-pick', 'revert', 'rebase', 'tag', 'log', 'diff', 'show', 'onto',
  'within', 'last', 'between', 'and', 'or', 'not',
]);
const FIELDS = new Set([
  'author', 'message', 'date', 'hash', 'branch', 'files', 'additions', 'deletions', 'path',
]);
const OPERATORS = new Set(['==', '!=', '>=', '<=', '>', '<', 'contains', 'matches']);

function tokenize(input: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;

  while (i < input.length) {
    // Whitespace
    if (/\s/.test(input[i])) {
      let start = i;
      while (i < input.length && /\s/.test(input[i])) i++;
      tokens.push({ text: input.slice(start, i), kind: 'plain' });
      continue;
    }

    // Pipe
    if (input[i] === '|') {
      tokens.push({ text: '|', kind: 'pipe' });
      i++;
      continue;
    }

    // Brackets
    if (input[i] === '[' || input[i] === ']' || input[i] === '(' || input[i] === ')') {
      tokens.push({ text: input[i], kind: 'bracket' });
      i++;
      continue;
    }

    // Comma
    if (input[i] === ',') {
      tokens.push({ text: ',', kind: 'plain' });
      i++;
      continue;
    }

    // Multi-char operators
    if (i + 1 < input.length && ['==', '!=', '>=', '<=', '..'].includes(input.slice(i, i + 2))) {
      tokens.push({ text: input.slice(i, i + 2), kind: 'operator' });
      i += 2;
      continue;
    }

    // Single-char operators
    if (input[i] === '>' || input[i] === '<' || input[i] === '=') {
      tokens.push({ text: input[i], kind: 'operator' });
      i++;
      continue;
    }

    // String literals
    if (input[i] === '"') {
      let start = i;
      i++; // skip opening quote
      while (i < input.length && input[i] !== '"') {
        if (input[i] === '\\' && i + 1 < input.length) i++; // skip escaped char
        i++;
      }
      if (i < input.length) i++; // skip closing quote
      tokens.push({ text: input.slice(start, i), kind: 'string' });
      continue;
    }

    // Numbers
    if (/\d/.test(input[i])) {
      let start = i;
      while (i < input.length && /\d/.test(input[i])) i++;
      tokens.push({ text: input.slice(start, i), kind: 'number' });
      continue;
    }

    // Identifiers (words)
    if (/[a-zA-Z_\-~]/.test(input[i])) {
      let start = i;
      while (i < input.length && /[a-zA-Z0-9_\-~/]/.test(input[i])) i++;
      const word = input.slice(start, i);

      if (SOURCES.has(word)) {
        tokens.push({ text: word, kind: 'source' });
      } else if (OPERATORS.has(word)) {
        tokens.push({ text: word, kind: 'operator' });
      } else if (FIELDS.has(word)) {
        tokens.push({ text: word, kind: 'field' });
      } else if (KEYWORDS.has(word)) {
        tokens.push({ text: word, kind: 'keyword' });
      } else {
        tokens.push({ text: word, kind: 'plain' });
      }
      continue;
    }

    // Fallback
    tokens.push({ text: input[i], kind: 'plain' });
    i++;
  }

  return tokens;
}

const COLOR_MAP: Record<Token['kind'], string> = {
  source: 'text-[#F14E32]',       // jock orange for sources
  keyword: 'text-purple-400',      // purple for keywords
  operator: 'text-cyan-400',       // cyan for operators
  string: 'text-emerald-400',      // green for strings
  number: 'text-amber-400',        // amber for numbers
  field: 'text-sky-400',           // sky blue for fields
  pipe: 'text-zinc-500',           // muted for pipes
  bracket: 'text-zinc-400',        // light for brackets
  plain: 'text-zinc-200',          // default text
};

interface DSLHighlighterProps {
  value: string;
}

export default function DSLHighlighter({ value }: DSLHighlighterProps) {
  const tokens = useMemo(() => tokenize(value), [value]);

  return (
    <span className="whitespace-pre font-mono text-xs" aria-hidden>
      {tokens.map((tok, i) => (
        <span key={i} className={COLOR_MAP[tok.kind]}>
          {tok.text}
        </span>
      ))}
    </span>
  );
}
