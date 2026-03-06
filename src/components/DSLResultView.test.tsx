import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import DSLResultView from './DSLResultView';
import type { DSLResult } from '../types';

describe('DSLResultView', () => {
  it('renders commit results as a table', () => {
    const result: DSLResult = {
      resultKind: 'commits',
      commits: [
        { hash: 'abc1234', message: 'initial commit', author: 'Alice', date: '2 days ago', additions: 3, deletions: 0 },
        { hash: 'def5678', message: 'add tests', author: 'Bob', date: '1 day ago', additions: 5, deletions: 1 },
      ],
    };

    render(<DSLResultView query="commits" result={result} />);

    expect(screen.getByText('HASH')).toBeInTheDocument();
    expect(screen.getByText('abc1234')).toBeInTheDocument();
    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('initial commit')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('renders count result', () => {
    const result: DSLResult = {
      resultKind: 'count',
      count: 42,
    };

    render(<DSLResultView query="commits | count" result={result} />);

    expect(screen.getByText('42')).toBeInTheDocument();
  });

  it('renders error result', () => {
    const result: DSLResult = {
      resultKind: 'error',
      error: 'parse error: unexpected token',
    };

    render(<DSLResultView query="invalid" result={result} />);

    expect(screen.getByText('parse error: unexpected token')).toBeInTheDocument();
  });

  it('renders aggregate result', () => {
    const result: DSLResult = {
      resultKind: 'aggregate',
      aggregates: [
        { group: 'Alice', value: 3 },
        { group: 'Bob', value: 2 },
      ],
      aggFunc: 'count',
      aggField: '',
      groupField: 'author',
    };

    render(<DSLResultView query="commits | group by author | count" result={result} />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
    expect(screen.getByText('Bob')).toBeInTheDocument();
    expect(screen.getByText('AUTHOR')).toBeInTheDocument();
  });

  it('renders formatted output', () => {
    const result: DSLResult = {
      resultKind: 'formatted',
      formattedOutput: '{"count": 5}',
      formatType: 'json',
    };

    render(<DSLResultView query="commits | count | format json" result={result} />);

    expect(screen.getByText('{"count": 5}')).toBeInTheDocument();
  });

  it('renders empty commits message', () => {
    const result: DSLResult = {
      resultKind: 'commits',
      commits: [],
    };

    render(<DSLResultView query="commits" result={result} />);

    expect(screen.getByText('no commits found')).toBeInTheDocument();
  });

  it('renders branch results', () => {
    const result: DSLResult = {
      resultKind: 'branches',
      branches: [
        { name: 'main', isCurrent: true },
        { name: 'feature', isCurrent: false, remote: 'origin/feature' },
      ],
    };

    render(<DSLResultView query="branches" result={result} />);

    expect(screen.getByText(/main/)).toBeInTheDocument();
    expect(screen.getAllByText(/feature/).length).toBeGreaterThanOrEqual(1);
  });

  it('renders action report', () => {
    const result: DSLResult = {
      resultKind: 'action_report',
      actionReport: {
        action: 'tag',
        affectedHashes: ['abc1234'],
        success: true,
        dryRun: false,
        description: 'tagged commit abc1234 as v1.0',
        errors: [],
      },
    };

    render(<DSLResultView query='commits | first 1 | tag "v1.0"' result={result} />);

    expect(screen.getByText('tagged commit abc1234 as v1.0')).toBeInTheDocument();
  });
});
