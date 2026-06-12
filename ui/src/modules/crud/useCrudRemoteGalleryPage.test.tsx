import { act, renderHook, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { useCrudRemoteGalleryPage } from './useCrudRemoteGalleryPage';

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>;
}

describe('useCrudRemoteGalleryPage', () => {
  it('loads items, manages selection and triggers reloads', async () => {
    const fetchPage = vi
      .fn()
      .mockResolvedValueOnce({
        items: [{ id: 'one', name: 'Uno' }],
        total: 1,
        has_more: false,
        next_cursor: null,
      })
      .mockResolvedValueOnce({
        items: [{ id: 'two', name: 'Dos' }],
        total: 1,
        has_more: false,
        next_cursor: null,
      });

    const { result } = renderHook(
      () =>
        useCrudRemoteGalleryPage<{ id: string; name: string }>({
          pageSize: 20,
          fetchPage,
        }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.items).toHaveLength(1);
    });
    expect(result.current.items[0]?.id).toBe('one');
    expect(result.current.archived).toBe(false);

    act(() => {
      result.current.selectItem('one');
    });
    expect(result.current.selectedId).toBe('one');

    act(() => {
      result.current.closeDetail();
    });
    expect(result.current.selectedId).toBeNull();

    await result.current.reload();

    await waitFor(() => {
      expect(result.current.items[0]?.id).toBe('two');
    });
    expect(fetchPage).toHaveBeenCalledTimes(2);
  });

  it('toggles archived and closes detail on archive toggle', async () => {
    const fetchPage = vi.fn().mockResolvedValue({
      items: [{ id: 'one', name: 'Uno' }],
      total: 1,
      has_more: false,
      next_cursor: null,
    });

    const { result } = renderHook(
      () =>
        useCrudRemoteGalleryPage<{ id: string; name: string }>({
          pageSize: 20,
          fetchPage,
        }),
      { wrapper },
    );

    await waitFor(() => {
      expect(result.current.items).toHaveLength(1);
    });

    act(() => {
      result.current.selectItem('one');
    });
    expect(result.current.selectedId).toBe('one');

    act(() => {
      result.current.handleArchiveToggle();
    });

    await waitFor(() => {
      expect(result.current.archived).toBe(true);
    });
    expect(result.current.selectedId).toBeNull();
  });
});
