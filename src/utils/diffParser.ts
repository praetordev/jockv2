export interface WordDiffSegment {
  text: string;
  type: 'equal' | 'added' | 'deleted';
}

export interface DiffLine {
  type: 'context' | 'addition' | 'deletion' | 'header' | 'hunk-header';
  content: string;
  rawLine: string;
  oldLineNo?: number;
  newLineNo?: number;
  wordDiff?: WordDiffSegment[];
}

export interface DiffHunk {
  header: string;
  oldStart: number;
  oldCount: number;
  newStart: number;
  newCount: number;
  lines: DiffLine[];
}

export interface ParsedDiff {
  headers: string[];
  hunks: DiffHunk[];
}

const HUNK_HEADER_RE = /^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$/;

export function parseDiff(patch: string): ParsedDiff {
  const rawLines = patch.split('\n');
  const headers: string[] = [];
  const hunks: DiffHunk[] = [];
  let currentHunk: DiffHunk | null = null;
  let oldLine = 0;
  let newLine = 0;

  for (const raw of rawLines) {
    const hunkMatch = raw.match(HUNK_HEADER_RE);

    if (hunkMatch) {
      currentHunk = {
        header: raw,
        oldStart: parseInt(hunkMatch[1], 10),
        oldCount: hunkMatch[2] !== undefined ? parseInt(hunkMatch[2], 10) : 1,
        newStart: parseInt(hunkMatch[3], 10),
        newCount: hunkMatch[4] !== undefined ? parseInt(hunkMatch[4], 10) : 1,
        lines: [],
      };
      oldLine = currentHunk.oldStart;
      newLine = currentHunk.newStart;
      currentHunk.lines.push({
        type: 'hunk-header',
        content: hunkMatch[5] || '',
        rawLine: raw,
      });
      hunks.push(currentHunk);
    } else if (!currentHunk) {
      headers.push(raw);
    } else if (raw.startsWith('+')) {
      currentHunk.lines.push({
        type: 'addition',
        content: raw.slice(1),
        rawLine: raw,
        newLineNo: newLine++,
      });
    } else if (raw.startsWith('-')) {
      currentHunk.lines.push({
        type: 'deletion',
        content: raw.slice(1),
        rawLine: raw,
        oldLineNo: oldLine++,
      });
    } else if (raw.startsWith('\\')) {
      // "\ No newline at end of file" — skip
    } else {
      // Context line (starts with space or is empty in the diff)
      currentHunk.lines.push({
        type: 'context',
        content: raw.startsWith(' ') ? raw.slice(1) : raw,
        rawLine: raw,
        oldLineNo: oldLine++,
        newLineNo: newLine++,
      });
    }
  }

  // Compute word-level diffs for paired add/delete lines
  for (const hunk of hunks) {
    computeWordDiffs(hunk.lines);
  }

  return { headers, hunks };
}

function computeWordDiffs(lines: DiffLine[]): void {
  let i = 0;
  while (i < lines.length) {
    // Find consecutive deletion lines followed by consecutive addition lines
    if (lines[i].type === 'deletion') {
      const delStart = i;
      while (i < lines.length && lines[i].type === 'deletion') i++;
      const addStart = i;
      while (i < lines.length && lines[i].type === 'addition') i++;
      const addEnd = i;

      const delCount = addStart - delStart;
      const addCount = addEnd - addStart;
      const pairs = Math.min(delCount, addCount);

      for (let p = 0; p < pairs; p++) {
        const del = lines[delStart + p];
        const add = lines[addStart + p];
        const [delDiff, addDiff] = diffWords(del.content, add.content);
        del.wordDiff = delDiff;
        add.wordDiff = addDiff;
      }
    } else {
      i++;
    }
  }
}

function diffWords(oldStr: string, newStr: string): [WordDiffSegment[], WordDiffSegment[]] {
  const oldTokens = tokenize(oldStr);
  const newTokens = tokenize(newStr);
  const lcs = longestCommonSubsequence(oldTokens, newTokens);

  const oldResult: WordDiffSegment[] = [];
  const newResult: WordDiffSegment[] = [];

  let oi = 0, ni = 0, li = 0;

  while (oi < oldTokens.length || ni < newTokens.length) {
    if (li < lcs.length) {
      // Emit deleted tokens before match
      while (oi < oldTokens.length && oldTokens[oi] !== lcs[li]) {
        oldResult.push({ text: oldTokens[oi++], type: 'deleted' });
      }
      // Emit added tokens before match
      while (ni < newTokens.length && newTokens[ni] !== lcs[li]) {
        newResult.push({ text: newTokens[ni++], type: 'added' });
      }
      // Emit equal token
      if (oi < oldTokens.length && ni < newTokens.length) {
        oldResult.push({ text: oldTokens[oi++], type: 'equal' });
        newResult.push({ text: newTokens[ni++], type: 'equal' });
        li++;
      }
    } else {
      while (oi < oldTokens.length) {
        oldResult.push({ text: oldTokens[oi++], type: 'deleted' });
      }
      while (ni < newTokens.length) {
        newResult.push({ text: newTokens[ni++], type: 'added' });
      }
    }
  }

  return [oldResult, newResult];
}

function tokenize(str: string): string[] {
  // Split into words and whitespace, preserving whitespace
  return str.match(/\S+|\s+/g) || [];
}

function longestCommonSubsequence(a: string[], b: string[]): string[] {
  const m = a.length;
  const n = b.length;

  // For very long sequences, use a simpler approach to avoid O(n*m) memory
  if (m * n > 100000) {
    return simpleLCS(a, b);
  }

  const dp: number[][] = Array.from({ length: m + 1 }, () => new Array(n + 1).fill(0));

  for (let i = 1; i <= m; i++) {
    for (let j = 1; j <= n; j++) {
      if (a[i - 1] === b[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }

  // Backtrack
  const result: string[] = [];
  let i = m, j = n;
  while (i > 0 && j > 0) {
    if (a[i - 1] === b[j - 1]) {
      result.unshift(a[i - 1]);
      i--;
      j--;
    } else if (dp[i - 1][j] > dp[i][j - 1]) {
      i--;
    } else {
      j--;
    }
  }

  return result;
}

function simpleLCS(a: string[], b: string[]): string[] {
  // Greedy matching for very long lines
  const result: string[] = [];
  let j = 0;
  for (let i = 0; i < a.length && j < b.length; i++) {
    const idx = b.indexOf(a[i], j);
    if (idx !== -1) {
      result.push(a[i]);
      j = idx + 1;
    }
  }
  return result;
}

// Build split-view pairs from parsed hunks
export interface SplitLine {
  left?: DiffLine;
  right?: DiffLine;
}

export function buildSplitLines(hunks: DiffHunk[]): SplitLine[] {
  const result: SplitLine[] = [];

  for (const hunk of hunks) {
    // Add hunk header as a full-width row
    result.push({
      left: hunk.lines[0], // hunk-header line
      right: hunk.lines[0],
    });

    let i = 1; // skip hunk-header
    while (i < hunk.lines.length) {
      const line = hunk.lines[i];

      if (line.type === 'context') {
        result.push({ left: line, right: line });
        i++;
      } else if (line.type === 'deletion') {
        // Collect consecutive deletions and additions
        const dels: DiffLine[] = [];
        while (i < hunk.lines.length && hunk.lines[i].type === 'deletion') {
          dels.push(hunk.lines[i++]);
        }
        const adds: DiffLine[] = [];
        while (i < hunk.lines.length && hunk.lines[i].type === 'addition') {
          adds.push(hunk.lines[i++]);
        }

        const maxLen = Math.max(dels.length, adds.length);
        for (let j = 0; j < maxLen; j++) {
          result.push({
            left: j < dels.length ? dels[j] : undefined,
            right: j < adds.length ? adds[j] : undefined,
          });
        }
      } else if (line.type === 'addition') {
        result.push({ left: undefined, right: line });
        i++;
      } else {
        i++;
      }
    }
  }

  return result;
}

// Reconstruct a valid patch for a single hunk (for git apply --cached)
export function buildHunkPatch(headers: string[], hunk: DiffHunk): string {
  const lines: string[] = [];
  // Include the diff headers (diff --git, index, ---, +++)
  for (const h of headers) {
    lines.push(h);
  }
  // Include the hunk header and its content lines
  lines.push(hunk.header);
  for (const line of hunk.lines) {
    if (line.type === 'hunk-header') continue;
    lines.push(line.rawLine);
  }
  lines.push(''); // trailing newline
  return lines.join('\n');
}

// Get file extension from path for syntax highlighting
export function getLanguageFromPath(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() || '';
  const langMap: Record<string, string> = {
    ts: 'typescript', tsx: 'typescript',
    js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
    go: 'go',
    py: 'python',
    rs: 'rust',
    rb: 'ruby',
    java: 'java',
    c: 'c', h: 'c',
    cpp: 'cpp', cc: 'cpp', cxx: 'cpp', hpp: 'cpp',
    cs: 'csharp',
    swift: 'swift',
    kt: 'kotlin',
    sh: 'shell', bash: 'shell', zsh: 'shell',
    json: 'json',
    yaml: 'yaml', yml: 'yaml',
    toml: 'toml',
    md: 'markdown',
    css: 'css', scss: 'css', less: 'css',
    html: 'html', htm: 'html',
    xml: 'xml',
    sql: 'sql',
    proto: 'protobuf',
    dockerfile: 'dockerfile',
    makefile: 'makefile',
  };
  return langMap[ext] || 'text';
}
