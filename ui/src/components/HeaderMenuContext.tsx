import { HeaderMenuItemsContext, type HeaderMenuItem } from './HeaderMenuContext.model';

export type { HeaderMenuItem } from './HeaderMenuContext.model';

export function HeaderMenuItemsProvider({
  items,
  children,
}: {
  items: HeaderMenuItem[];
  children: React.ReactNode;
}) {
  return <HeaderMenuItemsContext.Provider value={items}>{children}</HeaderMenuItemsContext.Provider>;
}
