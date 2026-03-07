import { describe, it, expect } from 'vitest';
import { parseDiff, buildSplitLines, buildHunkPatch, getLanguageFromPath } from './diffParser';

const SAMPLE_PATCH = `diff --git a/src/main.ts b/src/main.ts
index abc1234..def5678 100644
--- a/src/main.ts
+++ b/src/main.ts
@@ -10,7 +10,8 @@ function hello() {
   const a = 1;
   const b = 2;
-  const c = a + b;
+  const c = a + b + 1;
+  const d = c * 2;
   return c;
 }
@@ -20,3 +21,3 @@ function goodbye() {
-  console.log("bye");
+  console.log("goodbye");
   return;`;

describe('parseDiff', () => {
  it('parses headers correctly', () => {
    const result = parseDiff(SAMPLE_PATCH);
    expect(result.headers).toHaveLength(4);
    expect(result.headers[0]).toContain('diff --git');
    expect(result.headers[2]).toContain('---');
    expect(result.headers[3]).toContain('+++');
  });

  it('parses hunks correctly', () => {
    const result = parseDiff(SAMPLE_PATCH);
    expect(result.hunks).toHaveLength(2);

    const hunk1 = result.hunks[0];
    expect(hunk1.oldStart).toBe(10);
    expect(hunk1.oldCount).toBe(7);
    expect(hunk1.newStart).toBe(10);
    expect(hunk1.newCount).toBe(8);
  });

  it('assigns line numbers correctly', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const lines = result.hunks[0].lines.filter(l => l.type !== 'hunk-header');

    // First context line
    expect(lines[0].type).toBe('context');
    expect(lines[0].oldLineNo).toBe(10);
    expect(lines[0].newLineNo).toBe(10);

    // Deletion line
    const delLine = lines.find(l => l.type === 'deletion');
    expect(delLine?.oldLineNo).toBe(12);
    expect(delLine?.newLineNo).toBeUndefined();

    // Addition lines
    const addLines = lines.filter(l => l.type === 'addition');
    expect(addLines[0].newLineNo).toBe(12);
    expect(addLines[1].newLineNo).toBe(13);
  });

  it('computes word-level diffs for paired changes', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const hunk1Lines = result.hunks[0].lines.filter(l => l.type !== 'hunk-header');

    const delLine = hunk1Lines.find(l => l.type === 'deletion')!;
    const addLine = hunk1Lines.find(l => l.type === 'addition')!;

    expect(delLine.wordDiff).toBeDefined();
    expect(addLine.wordDiff).toBeDefined();

    // The word diff should highlight the changed parts
    const delChanged = delLine.wordDiff!.filter(s => s.type === 'deleted');
    const addChanged = addLine.wordDiff!.filter(s => s.type === 'added');
    expect(delChanged.length).toBeGreaterThan(0);
    expect(addChanged.length).toBeGreaterThan(0);
  });

  it('handles second hunk correctly', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const hunk2 = result.hunks[1];
    expect(hunk2.oldStart).toBe(20);
    expect(hunk2.newStart).toBe(21);

    const hunk2Lines = hunk2.lines.filter(l => l.type !== 'hunk-header');
    const del = hunk2Lines.find(l => l.type === 'deletion')!;
    const add = hunk2Lines.find(l => l.type === 'addition')!;
    expect(del.content).toContain('bye');
    expect(add.content).toContain('goodbye');
  });
});

describe('buildSplitLines', () => {
  it('pairs deletions with additions', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const splitLines = buildSplitLines(result.hunks);

    expect(splitLines.length).toBeGreaterThan(0);

    // Find a pair where left is deletion and right is addition
    const modifiedPair = splitLines.find(
      p => p.left?.type === 'deletion' && p.right?.type === 'addition'
    );
    expect(modifiedPair).toBeDefined();
  });

  it('shows context lines on both sides', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const splitLines = buildSplitLines(result.hunks);

    const contextPair = splitLines.find(
      p => p.left?.type === 'context' && p.right?.type === 'context'
    );
    expect(contextPair).toBeDefined();
    expect(contextPair!.left!.oldLineNo).toBeDefined();
    expect(contextPair!.right!.newLineNo).toBeDefined();
  });
});

describe('buildHunkPatch', () => {
  it('reconstructs a valid patch for a hunk', () => {
    const result = parseDiff(SAMPLE_PATCH);
    const patch = buildHunkPatch(result.headers, result.hunks[0]);

    expect(patch).toContain('diff --git');
    expect(patch).toContain('@@');
    expect(patch).toContain('-  const c = a + b;');
    expect(patch).toContain('+  const c = a + b + 1;');
  });
});

describe('getLanguageFromPath', () => {
  it('maps common extensions', () => {
    expect(getLanguageFromPath('foo.ts')).toBe('typescript');
    expect(getLanguageFromPath('bar.tsx')).toBe('typescript');
    expect(getLanguageFromPath('baz.go')).toBe('go');
    expect(getLanguageFromPath('qux.py')).toBe('python');
    expect(getLanguageFromPath('unknown.xyz')).toBe('text');
  });
});
