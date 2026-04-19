import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Branch } from '@devpablocristo/modules-scheduling/next';
import { LanguageProvider } from '../lib/i18n';
import { BranchContext, type BranchContextValue } from '../lib/branchSelectionContext';

import { BranchSwitcher } from './BranchSwitcher';

function buildBranch(id: string, name: string): Branch {
  return {
    id,
    org_id: 'org-demo',
    code: id,
    name,
    timezone: 'America/Argentina/Tucuman',
    address: `${name} 123`,
    active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  };
}

function buildBranchContextValue(overrides: Partial<BranchContextValue> = {}): BranchContextValue {
  return {
    orgId: 'org-demo',
    branches: [buildBranch('branch-a', 'Casa Central')],
    availableBranches: [buildBranch('branch-a', 'Casa Central')],
    selectedBranchId: 'branch-a',
    selectedBranch: buildBranch('branch-a', 'Casa Central'),
    isLoading: false,
    isError: false,
    error: null,
    setSelectedBranchId: vi.fn(),
    ...overrides,
  };
}

describe('BranchSwitcher', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('stays hidden when there is zero or one available branch', () => {
    render(
      <LanguageProvider initialLanguage="es">
        <BranchContext.Provider
          value={buildBranchContextValue({
            branches: [buildBranch('branch-a', 'Casa Central')],
            availableBranches: [buildBranch('branch-a', 'Casa Central')],
          })}
        >
          <BranchSwitcher />
        </BranchContext.Provider>
      </LanguageProvider>,
    );

    expect(screen.queryByLabelText('Sucursal activa')).toBeNull();
  });

  it('renders a global selector and forwards selection changes', () => {
    const setSelectedBranchId = vi.fn();

    render(
      <LanguageProvider initialLanguage="es">
        <BranchContext.Provider
          value={buildBranchContextValue({
            branches: [
              buildBranch('branch-a', 'Casa Central'),
              buildBranch('branch-b', 'Sucursal Norte'),
            ],
            availableBranches: [
              buildBranch('branch-a', 'Casa Central'),
              buildBranch('branch-b', 'Sucursal Norte'),
            ],
            selectedBranchId: 'branch-a',
            selectedBranch: buildBranch('branch-a', 'Casa Central'),
            setSelectedBranchId,
          })}
        >
          <BranchSwitcher />
        </BranchContext.Provider>
      </LanguageProvider>,
    );

    fireEvent.change(screen.getByLabelText('Sucursal activa'), {
      target: { value: 'branch-b' },
    });

    expect(setSelectedBranchId).toHaveBeenCalledWith('branch-b');
  });

  it('can stay visible with a single branch when explicitly requested', () => {
    render(
      <LanguageProvider initialLanguage="es">
        <BranchContext.Provider value={buildBranchContextValue()}>
          <BranchSwitcher hideWhenSingle={false} />
        </BranchContext.Provider>
      </LanguageProvider>,
    );

    expect(screen.getByLabelText('Sucursal activa')).toBeInTheDocument();
  });

  it('renders compact header variant without visible label text', () => {
    render(
      <LanguageProvider initialLanguage="es">
        <BranchContext.Provider value={buildBranchContextValue()}>
          <BranchSwitcher hideWhenSingle={false} variant="header" />
        </BranchContext.Provider>
      </LanguageProvider>,
    );

    expect(screen.getByLabelText('Sucursal activa')).toBeInTheDocument();
    expect(screen.queryByText('Sucursal')).toBeNull();
  });
});
