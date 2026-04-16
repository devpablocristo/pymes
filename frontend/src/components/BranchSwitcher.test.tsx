import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../lib/i18n';

const branchMocks = vi.hoisted(() => ({
  useBranchSelection: vi.fn(),
}));

vi.mock('../lib/branchContext', () => ({
  useBranchSelection: () => branchMocks.useBranchSelection(),
}));

import { BranchSwitcher } from './BranchSwitcher';

describe('BranchSwitcher', () => {
  beforeEach(() => {
    branchMocks.useBranchSelection.mockReset();
  });

  it('stays hidden when there is zero or one available branch', () => {
    branchMocks.useBranchSelection.mockReturnValue({
      availableBranches: [{ id: 'branch-a', name: 'Casa Central' }],
      selectedBranchId: 'branch-a',
      setSelectedBranchId: vi.fn(),
      isLoading: false,
      isError: false,
    });

    render(
      <LanguageProvider initialLanguage="es">
        <BranchSwitcher />
      </LanguageProvider>,
    );

    expect(screen.queryByLabelText('Sucursal activa')).toBeNull();
  });

  it('renders a global selector and forwards selection changes', () => {
    const setSelectedBranchId = vi.fn();
    branchMocks.useBranchSelection.mockReturnValue({
      availableBranches: [
        { id: 'branch-a', name: 'Casa Central' },
        { id: 'branch-b', name: 'Sucursal Norte' },
      ],
      selectedBranchId: 'branch-a',
      setSelectedBranchId,
      isLoading: false,
      isError: false,
    });

    render(
      <LanguageProvider initialLanguage="es">
        <BranchSwitcher />
      </LanguageProvider>,
    );

    fireEvent.change(screen.getByLabelText('Sucursal activa'), {
      target: { value: 'branch-b' },
    });

    expect(setSelectedBranchId).toHaveBeenCalledWith('branch-b');
  });
});
