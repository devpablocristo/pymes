import { createContext, useContext } from 'react';

export type HeaderMenuItem = {
  label: string;
  href: string;
  onSelect?: () => void;
};

const HeaderMenuItemsContext = createContext<HeaderMenuItem[]>([]);

export function HeaderMenuItemsProvider({
  items,
  children,
}: {
  items: HeaderMenuItem[];
  children: React.ReactNode;
}) {
  return <HeaderMenuItemsContext.Provider value={items}>{children}</HeaderMenuItemsContext.Provider>;
}

export function useHeaderMenuItems() {
  return useContext(HeaderMenuItemsContext);
}
