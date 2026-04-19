import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { CrudShellHeaderActionsColumn } from '@devpablocristo/modules-crud-ui';
import type { ReactNode } from 'react';
import { usePageSearchShellControl } from './PageSearch';
import { HeaderMenu } from './HeaderMenu';
import { useHeaderMenuItems } from './HeaderMenuContext';

export type PageLayoutProps = {
  title: ReactNode;
  lead?: ReactNode;
  actions?: ReactNode;
  inlineActions?: ReactNode;
  menuItems?: Array<{ label: string; href: string }>;
  banner?: ReactNode;
  className?: string;
  children: ReactNode;
};

export function PageLayout({ title, lead, actions, inlineActions, menuItems, banner, className, children }: PageLayoutProps) {
  const contextualMenuItems = useHeaderMenuItems();
  const stackClass = ['page-stack', className].filter(Boolean).join(' ');
  const pageSearch = usePageSearchShellControl();
  const hasSearch = pageSearch.visible;
  void lead;

  const headerActions = (
    <CrudShellHeaderActionsColumn
      search={
        hasSearch
          ? {
              value: pageSearch.query,
              onChange: pageSearch.setQuery,
              placeholder: pageSearch.placeholder,
              inputClassName: 'page-search__input m-kanban__search crud-resource-shell-header__search',
            }
          : null
      }
      searchInlineActions={inlineActions}
    >
      {actions}
    </CrudShellHeaderActionsColumn>
  );

  return (
    <div className={stackClass}>
      <div className="page-layout__header-top-row">
        <HeaderMenu items={[...contextualMenuItems, ...(menuItems ?? [])]} />
      </div>
      <CrudPageShell
        title={title}
        search={undefined}
        headerActions={headerActions}
      >
        <>
          {banner}
          {children}
        </>
      </CrudPageShell>
    </div>
  );
}
