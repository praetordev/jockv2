import { render } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import DSLHighlighter from './DSLHighlighter';

describe('DSLHighlighter', () => {
  function getTokenTexts(container: HTMLElement) {
    return Array.from(container.querySelectorAll('span > span')).map(el => ({
      text: el.textContent,
      className: el.className,
    }));
  }

  it('highlights source keywords in orange', () => {
    const { container } = render(<DSLHighlighter value="commits" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: 'commits', className: expect.stringContaining('text-[#F14E32]') }));
  });

  it('highlights pipe operator', () => {
    const { container } = render(<DSLHighlighter value="commits | count" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: '|', className: expect.stringContaining('text-zinc-500') }));
  });

  it('highlights keywords in purple', () => {
    const { container } = render(<DSLHighlighter value={'commits | where author == "alice"'} />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: 'where', className: expect.stringContaining('text-purple-400') }));
  });

  it('highlights fields in sky blue', () => {
    const { container } = render(<DSLHighlighter value="commits | sort author" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: 'author', className: expect.stringContaining('text-sky-400') }));
  });

  it('highlights string literals in green', () => {
    const { container } = render(<DSLHighlighter value='where author == "alice"' />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: '"alice"', className: expect.stringContaining('text-emerald-400') }));
  });

  it('highlights numbers in amber', () => {
    const { container } = render(<DSLHighlighter value="first 10" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: '10', className: expect.stringContaining('text-amber-400') }));
  });

  it('highlights operators in cyan', () => {
    const { container } = render(<DSLHighlighter value="additions >= 5" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: '>=', className: expect.stringContaining('text-cyan-400') }));
    expect(tokens).not.toContainEqual(expect.objectContaining({ text: 'contains' }));
  });

  it('highlights word operators like contains', () => {
    const { container } = render(<DSLHighlighter value='message contains "fix"' />);
    const tokens = getTokenTexts(container);
    expect(tokens).toContainEqual(expect.objectContaining({ text: 'contains', className: expect.stringContaining('text-cyan-400') }));
  });

  it('handles empty input', () => {
    const { container } = render(<DSLHighlighter value="" />);
    const tokens = getTokenTexts(container);
    expect(tokens).toHaveLength(0);
  });

  it('handles escaped quotes in strings', () => {
    const { container } = render(<DSLHighlighter value={'where message == "say \\"hello\\""'} />);
    const tokens = getTokenTexts(container);
    const stringToken = tokens.find(t => t.className?.includes('text-emerald-400'));
    expect(stringToken).toBeDefined();
  });
});
