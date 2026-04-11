import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useCrudRemotePaginatedList } from './useCrudRemotePaginatedList';

type Row = { id: string; name: string };

describe('useCrudRemotePaginatedList', () => {
  it('carga la primera página y permite loadMore', async () => {
    const fetchPage = vi.fn(
      async ({
        after,
      }: {
        limit: number;
        search: string;
        archived: boolean;
        after: string | null;
        signal: AbortSignal;
      }) => {
        if (after == null) {
          return {
            items: [{ id: '1', name: 'A' }] as Row[],
            total: 2,
            has_more: true,
            next_cursor: 'c1',
          };
        }
        return {
          items: [{ id: '2', name: 'B' }] as Row[],
          total: 2,
          has_more: false,
          next_cursor: null,
        };
      },
    );

    const { result } = renderHook(() =>
      useCrudRemotePaginatedList<Row>({
        pageSize: 10,
        deferredSearch: '',
        archived: false,
        reloadKey: 0,
        fetchPage,
      }),
    );

    await waitFor(() => {
      expect(result.current.loading).toBe(false);
    });

    expect(result.current.items).toEqual([{ id: '1', name: 'A' }]);
    expect(result.current.hasMore).toBe(true);

    await act(async () => {
      await result.current.loadMore();
    });

    expect(fetchPage).toHaveBeenCalledTimes(2);
    expect(result.current.items).toHaveLength(2);
    expect(result.current.hasMore).toBe(false);
  });
});
