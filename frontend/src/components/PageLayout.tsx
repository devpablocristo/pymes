import { CrudPageShell } from '@devpablocristo/core-browser/crud';
import { CrudShellHeaderActionsColumn } from '@devpablocristo/modules-crud-ui';
import type { ReactNode } from 'react';
import { usePageSearchShellControl } from './PageSearch';

export type PageLayoutProps = {
  title: ReactNode;
  lead?: ReactNode;
  actions?: ReactNode;
  banner?: ReactNode;
  className?: string;
  searchClearLabel?: string;
  children: ReactNode;
};

function isPrimitiveLead(lead: ReactNode) {
  return typeof lead === 'string' || typeof lead === 'number';
}

export function PageLayout({ title, lead, actions, banner, className, searchClearLabel, children }: PageLayoutProps) {
  const stackClass = ['page-stack', className].filter(Boolean).join(' ');
  const pageSearch = usePageSearchShellControl();
  const hasSearch = pageSearch.visible;
  const primitiveLead = lead != null && lead !== false && isPrimitiveLead(lead) ? lead : undefined;
  const richLead =
    lead != null && lead !== false && !isPrimitiveLead(lead) ? <div className="text-page-lead">{lead}</div> : undefined;

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
    >
      {actions}
    </CrudShellHeaderActionsColumn>
  );

  return (
    <div className={stackClass}>
      <CrudPageShell
        title={title}
        subtitle={primitiveLead}
        headerLeadSlot={richLead}
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
