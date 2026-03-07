// Lightweight syntax highlighting for diff viewer
// Returns HTML string with <span> tags for coloring, or null if no highlighting

interface Rule {
  pattern: RegExp;
  className: string;
}

const SHARED_RULES: Rule[] = [
  // Strings (double-quoted)
  { pattern: /"(?:[^"\\]|\\.)*"/g, className: 'text-amber-300' },
  // Strings (single-quoted)
  { pattern: /'(?:[^'\\]|\\.)*'/g, className: 'text-amber-300' },
  // Template literals
  { pattern: /`(?:[^`\\]|\\.)*`/g, className: 'text-amber-300' },
  // Numbers
  { pattern: /\b(?:0x[\da-fA-F]+|0b[01]+|0o[0-7]+|\d+\.?\d*(?:[eE][+-]?\d+)?)\b/g, className: 'text-purple-400' },
  // Single-line comments (//)
  { pattern: /\/\/.*/g, className: 'text-zinc-600 italic' },
  // Hash comments (#)
  { pattern: /#.*/g, className: 'text-zinc-600 italic' },
];

const LANG_KEYWORDS: Record<string, string[]> = {
  typescript: [
    'import', 'export', 'from', 'const', 'let', 'var', 'function', 'return',
    'if', 'else', 'for', 'while', 'switch', 'case', 'break', 'continue',
    'class', 'extends', 'implements', 'interface', 'type', 'enum',
    'new', 'this', 'super', 'async', 'await', 'yield',
    'try', 'catch', 'finally', 'throw',
    'true', 'false', 'null', 'undefined', 'void',
    'typeof', 'instanceof', 'in', 'of', 'as', 'is',
    'default', 'readonly', 'private', 'public', 'protected', 'static', 'abstract',
  ],
  javascript: [
    'import', 'export', 'from', 'const', 'let', 'var', 'function', 'return',
    'if', 'else', 'for', 'while', 'switch', 'case', 'break', 'continue',
    'class', 'extends', 'new', 'this', 'super', 'async', 'await', 'yield',
    'try', 'catch', 'finally', 'throw',
    'true', 'false', 'null', 'undefined', 'void',
    'typeof', 'instanceof', 'in', 'of', 'default',
  ],
  go: [
    'package', 'import', 'func', 'return', 'var', 'const', 'type', 'struct',
    'interface', 'map', 'chan', 'go', 'defer', 'select',
    'if', 'else', 'for', 'range', 'switch', 'case', 'default', 'break', 'continue',
    'true', 'false', 'nil', 'error', 'string', 'int', 'bool', 'byte',
    'make', 'append', 'len', 'cap', 'new', 'delete', 'close',
  ],
  python: [
    'import', 'from', 'as', 'def', 'return', 'class', 'self',
    'if', 'elif', 'else', 'for', 'while', 'in', 'not', 'and', 'or', 'is',
    'try', 'except', 'finally', 'raise', 'with', 'yield', 'async', 'await',
    'True', 'False', 'None', 'pass', 'break', 'continue', 'lambda',
    'global', 'nonlocal', 'del', 'assert',
  ],
  rust: [
    'use', 'mod', 'pub', 'fn', 'let', 'mut', 'const', 'static',
    'struct', 'enum', 'impl', 'trait', 'type', 'where',
    'if', 'else', 'for', 'while', 'loop', 'match', 'break', 'continue', 'return',
    'self', 'Self', 'super', 'crate', 'as', 'in', 'ref', 'move',
    'true', 'false', 'async', 'await', 'unsafe', 'extern',
  ],
  css: [
    'important', 'inherit', 'initial', 'unset', 'none', 'auto',
    'flex', 'grid', 'block', 'inline', 'absolute', 'relative', 'fixed', 'sticky',
  ],
  json: [], // JSON just uses string/number rules
  yaml: [],
  shell: [
    'if', 'then', 'else', 'elif', 'fi', 'for', 'do', 'done', 'while', 'until',
    'case', 'esac', 'function', 'return', 'exit', 'echo', 'export', 'source',
    'local', 'readonly', 'set', 'unset', 'shift', 'eval', 'exec',
  ],
  sql: [
    'SELECT', 'FROM', 'WHERE', 'JOIN', 'LEFT', 'RIGHT', 'INNER', 'OUTER',
    'ON', 'AND', 'OR', 'NOT', 'IN', 'EXISTS', 'BETWEEN', 'LIKE',
    'INSERT', 'INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE',
    'CREATE', 'ALTER', 'DROP', 'TABLE', 'INDEX', 'VIEW',
    'GROUP', 'BY', 'ORDER', 'HAVING', 'LIMIT', 'OFFSET',
    'AS', 'DISTINCT', 'UNION', 'ALL', 'NULL', 'IS', 'CASE', 'WHEN', 'THEN', 'ELSE', 'END',
  ],
};

// Languages that use # for comments instead of //
const HASH_COMMENT_LANGS = new Set(['python', 'ruby', 'shell', 'yaml', 'toml', 'makefile', 'dockerfile']);
const SLASH_COMMENT_LANGS = new Set(['typescript', 'javascript', 'go', 'rust', 'java', 'c', 'cpp', 'csharp', 'swift', 'kotlin', 'protobuf', 'css']);

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

export function highlightLine(content: string, language: string): string | null {
  if (!content.trim()) return null;

  const keywords = LANG_KEYWORDS[language];
  if (!keywords && !HASH_COMMENT_LANGS.has(language) && !SLASH_COMMENT_LANGS.has(language)) {
    return null;
  }

  // We use a simple approach: find all matches, sort by position, and build the result
  // ensuring no overlapping highlights

  interface Match {
    start: number;
    end: number;
    className: string;
    text: string;
  }

  const matches: Match[] = [];

  // Apply relevant rules
  for (const rule of SHARED_RULES) {
    // Skip hash comments for non-hash languages
    if (rule.pattern.source === '#.*' && !HASH_COMMENT_LANGS.has(language)) continue;
    // Skip // comments for non-slash languages
    if (rule.pattern.source === '\\/\\/.*' && !SLASH_COMMENT_LANGS.has(language)) continue;

    const re = new RegExp(rule.pattern.source, rule.pattern.flags);
    let m;
    while ((m = re.exec(content)) !== null) {
      matches.push({
        start: m.index,
        end: m.index + m[0].length,
        className: rule.className,
        text: m[0],
      });
    }
  }

  // Keyword highlighting
  if (keywords && keywords.length > 0) {
    const kwPattern = new RegExp(`\\b(${keywords.join('|')})\\b`, 'g');
    let m;
    while ((m = kwPattern.exec(content)) !== null) {
      matches.push({
        start: m.index,
        end: m.index + m[0].length,
        className: 'text-sky-400',
        text: m[0],
      });
    }
  }

  if (matches.length === 0) return null;

  // Sort by start position, longer matches first for ties
  matches.sort((a, b) => a.start - b.start || b.end - a.end);

  // Remove overlapping matches (keep earlier/longer ones)
  const filtered: Match[] = [];
  let lastEnd = 0;
  for (const m of matches) {
    if (m.start >= lastEnd) {
      filtered.push(m);
      lastEnd = m.end;
    }
  }

  // Build HTML
  let result = '';
  let pos = 0;
  for (const m of filtered) {
    if (m.start > pos) {
      result += escapeHtml(content.slice(pos, m.start));
    }
    result += `<span class="${m.className}">${escapeHtml(m.text)}</span>`;
    pos = m.end;
  }
  if (pos < content.length) {
    result += escapeHtml(content.slice(pos));
  }

  return result;
}
