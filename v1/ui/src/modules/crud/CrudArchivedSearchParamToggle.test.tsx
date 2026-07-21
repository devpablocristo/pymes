import { fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { CrudArchivedSearchParamToggle } from './CrudArchivedSearchParamToggle';

function LocationProbe() {
  const location = useLocation();
  return <div>{location.pathname}{location.search}</div>;
}

describe('CrudArchivedSearchParamToggle', () => {
  it('toggles the archived query param and button label', async () => {
    render(
      <MemoryRouter initialEntries={['/items/list']} future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Routes>
          <Route
            path="/items/list"
            element={
              <>
                <CrudArchivedSearchParamToggle />
                <LocationProbe />
              </>
            }
          />
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', { name: 'Ver archivadas' })).toBeInTheDocument();
    expect(screen.getByText('/items/list')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Ver archivadas' }));
    expect(await screen.findByRole('button', { name: 'Ver activas' })).toBeInTheDocument();
    expect(screen.getByText('/items/list?archived=1')).toBeInTheDocument();
  });
});
