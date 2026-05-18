import { createContext } from 'react';

export type HeaderMenuItem = {
  label: string;
  href: string;
  onSelect?: () => void;
};

export const HeaderMenuItemsContext = createContext<HeaderMenuItem[]>([]);
