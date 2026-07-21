import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { CrudToolbarActionButtons } from './CrudToolbarActionButtons';

type Item = { id: string; name: string };

describe('CrudToolbarActionButtons', () => {
  it('renders only visible actions and wires reload/setError helpers', async () => {
    const onClick = vi.fn();
    const reload = vi.fn(async () => undefined);
    const setError = vi.fn();

    render(
      <CrudToolbarActionButtons<Item>
        actions={[
          {
            id: 'visible',
            label: 'Visible',
            kind: 'primary',
            isVisible: ({ archived }) => !archived,
            onClick,
          },
          {
            id: 'hidden',
            label: 'Hidden',
            isVisible: () => false,
            onClick: async () => undefined,
          },
        ]}
        items={[{ id: '1', name: 'Item 1' }]}
        archived={false}
        reload={reload}
        setError={setError}
      />,
    );

    expect(screen.getByRole('button', { name: 'Visible' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Hidden' })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Visible' }));

    expect(onClick).toHaveBeenCalledTimes(1);
    const helpers = onClick.mock.calls[0][0];
    expect(helpers.items).toEqual([{ id: '1', name: 'Item 1' }]);
    await helpers.reload();
    expect(reload).toHaveBeenCalledTimes(1);
    helpers.setError('boom');
    expect(setError).toHaveBeenCalledWith('boom');
  });
});
