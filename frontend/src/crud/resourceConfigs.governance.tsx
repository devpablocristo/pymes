import {
  createAccountCrudConfig,
  createPartyCrudConfig,
} from '../modules/parties';
import {
  createNexusRolesCrudConfig,
  createProcurementPoliciesCrudConfig,
  createProcurementRequestsCrudConfig,
} from '../modules/nexus-governance';
import { defineCrudDomain } from './defineCrudDomain';
import { formatDate } from './resourceConfigs.shared';
import { PymesSimpleCrudListModeContent } from './PymesSimpleCrudListModeContent';

type Address = {
  street?: string;
  city?: string;
  state?: string;
  zip_code?: string;
  country?: string;
};

type Account = {
  id: string;
  type: string;
  entity_type: string;
  entity_id: string;
  entity_name: string;
  balance: number;
  currency?: string;
  credit_limit: number;
  updated_at: string;
};

type Party = {
  id: string;
  party_type: string;
  display_name: string;
  email?: string;
  phone?: string;
  tax_id?: string;
  notes?: string;
  tags?: string[];
  address?: Address;
  person?: { first_name?: string; last_name?: string };
  organization?: { legal_name?: string; trade_name?: string; tax_condition?: string };
  roles?: Array<{ role: string; is_active: boolean }>;
};

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  procurementRequests: createProcurementRequestsCrudConfig(),
  procurementPolicies: createProcurementPoliciesCrudConfig(),
  accounts: {
    basePath: '/v1/accounts',
    ...createAccountCrudConfig<Account>({
      render: () => <PymesSimpleCrudListModeContent resourceId="accounts" />,
      formatUpdatedAt: (value) => formatDate(String(value ?? '')),
    }),
  },
  roles: createNexusRolesCrudConfig(),
  parties: {
    basePath: '/v1/parties',
    ...createPartyCrudConfig<Party>({
      label: 'entidad',
      labelPlural: 'entidades',
      labelPluralCap: 'Entidades',
      header: 'Entidad',
      render: () => <PymesSimpleCrudListModeContent resourceId="parties" />,
    }),
  },
  employees: {
    basePath: '/v1/parties',
    listQuery: 'role=employee',
    ...createPartyCrudConfig<Party>({
      label: 'empleado',
      labelPlural: 'empleados',
      labelPluralCap: 'Empleados',
      header: 'Empleado',
      render: () => <PymesSimpleCrudListModeContent resourceId="employees" />,
      createLabel: '+ Nuevo empleado',
      searchPlaceholder: 'Buscar...',
      emptyState:
        'No hay entidades con rol empleado. El alta crea una party en /v1/parties con rol employee. Los usuarios con acceso a la consola (miembros de org) se administran aparte.',
      roleEmployee: true,
    }),
  },
});
