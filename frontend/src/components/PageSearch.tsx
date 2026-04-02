/**
 * Buscador de página — arriba a la derecha del área de contenido.
 * Se renderiza desde el Shell. Solo es visible cuando la página activa se registra.
 *
 * Flujo:
 *  1. Shell monta <PageSearchProvider>
 *  2. La página llama usePageSearch() → esto registra la página y devuelve el query
 *  3. PageSearchProvider muestra el input solo si hay una página registrada
 *  4. Al desmontar la página, se des-registra y el input desaparece
 */
import { createContext, useCallback, useContext, useEffect, useRef, useState, type PropsWithChildren } from 'react';
import './PageSearch.css';

type PageSearchContextValue = {
  query: string;
  setQuery: (value: string) => void;
  register: () => () => void;
};

const PageSearchContext = createContext<PageSearchContextValue>({
  query: '',
  setQuery: () => {},
  register: () => () => {},
});

/** true solo dentro de <PageSearchProvider> (Shell); el resto usa búsqueda inline del CRUD. */
export const PageSearchShellContext = createContext(false);

/**
 * Hook que registra la página como consumidora del search y devuelve el query.
 * Al desmontar la página, se des-registra y el input desaparece.
 */
export function usePageSearch(): string {
  const { query, register } = useContext(PageSearchContext);
  useEffect(() => register(), [register]);
  return query;
}

/** Provider + input. Se monta una vez en el Shell. */
export function PageSearchProvider({
  children,
  placeholder = 'Buscar…',
}: PropsWithChildren<{ placeholder?: string }>) {
  const [query, setQuery] = useState('');
  const countRef = useRef(0);
  const [visible, setVisible] = useState(false);

  const register = useCallback(() => {
    countRef.current += 1;
    setVisible(true);
    return () => {
      countRef.current -= 1;
      if (countRef.current <= 0) {
        countRef.current = 0;
        setVisible(false);
        setQuery('');
      }
    };
  }, []);

  return (
    <PageSearchShellContext.Provider value>
      <PageSearchContext.Provider value={{ query, setQuery, register }}>
        {visible && (
          <div className="page-search">
            <input
              type="search"
              className="page-search__input"
              placeholder={placeholder}
              autoComplete="off"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label={placeholder}
            />
            {query.length > 0 && (
              <button
                className="page-search__clear"
                onClick={() => setQuery('')}
                aria-label="Limpiar búsqueda"
                type="button"
              >
                ×
              </button>
            )}
          </div>
        )}
        {children}
      </PageSearchContext.Provider>
    </PageSearchShellContext.Provider>
  );
}
