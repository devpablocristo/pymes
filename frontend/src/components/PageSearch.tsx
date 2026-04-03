/* eslint-disable react-refresh/only-export-components -- hooks acoplados al Context del mismo archivo */
/**
 * Buscador de página — vive en el Shell, pero se incrusta en la cabecera (`PageLayout`)
 * para quedar alineado con el título principal cuando la página activa se registra.
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
  visible: boolean;
  placeholder: string;
};

const PageSearchContext = createContext<PageSearchContextValue>({
  query: '',
  setQuery: () => {},
  register: () => () => {},
  visible: false,
  placeholder: 'Buscar...',
});

/** true solo dentro de <PageSearchProvider> (Shell); el resto usa búsqueda inline del CRUD. */
const PageSearchShellContext = createContext(false);

/**
 * Hook que registra la página como consumidora del search y devuelve el query.
 * Al desmontar la página, se des-registra y el input desaparece.
 */
export function usePageSearch(): string {
  const { query, register } = useContext(PageSearchContext);
  useEffect(() => register(), [register]);
  return query;
}

export function usePageSearchShellControl() {
  const { query, setQuery, visible, placeholder } = useContext(PageSearchContext);
  return {
    query,
    visible,
    placeholder,
    setQuery,
    clear: () => setQuery(''),
  };
}

/** Provider del buscador global. Se monta una vez en el Shell. */
export function PageSearchProvider({
  children,
  placeholder = 'Buscar...',
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
      <PageSearchContext.Provider value={{ query, setQuery, register, visible, placeholder }}>
        {children}
      </PageSearchContext.Provider>
    </PageSearchShellContext.Provider>
  );
}
