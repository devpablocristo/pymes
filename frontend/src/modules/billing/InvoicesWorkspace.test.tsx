import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { LanguageProvider } from '../../lib/i18n';
import { InvoicesWorkspace } from './InvoicesWorkspace';

vi.mock('../../lib/useCrudListCreatedByMerge', () => ({
  useCrudListCreatedByMerge: () => ({}),
}));

function renderInvoicesWorkspace(initialPath = '/modules/invoices/list') {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <LanguageProvider>
        <MemoryRouter initialEntries={[initialPath]} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
          <InvoicesWorkspace />
        </MemoryRouter>
      </LanguageProvider>
    </QueryClientProvider>,
  );
}

describe('InvoicesWorkspace', () => {
  it('renders CSV actions and archived toggle from the reusable CRUD header', async () => {
    renderInvoicesWorkspace();

    expect(await screen.findByRole('button', { name: 'Exportar CSV' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Importar CSV' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ver archivados' })).toBeInTheDocument();
  });
});
