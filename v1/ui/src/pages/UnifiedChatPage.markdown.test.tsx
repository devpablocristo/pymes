import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { AssistantMarkdown } from './UnifiedChatPage';

describe('AssistantMarkdown', () => {
  it('renders assistant markdown formatting', () => {
    render(
      <AssistantMarkdown
        text={[
          'Semana con **oportunidades** claras.',
          '',
          '* **Cliente Demo Uno:** deuda pendiente.',
          '* Mercado Plaza: seguimiento.',
          '',
          '1. Contactar clientes.',
          '2. Revisar pagos.',
        ].join('\n')}
      />,
    );

    expect(screen.getByText('oportunidades')).toBeInTheDocument();
    expect(screen.getByText('oportunidades').tagName).toBe('STRONG');

    const lists = screen.getAllByRole('list');
    expect(lists).toHaveLength(2);
    expect(within(lists[0]).getAllByRole('listitem')).toHaveLength(2);
    expect(within(lists[1]).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByText('Cliente Demo Uno:').tagName).toBe('STRONG');
  });
});
