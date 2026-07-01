import { createLedgerAccountsCrudConfig } from '../modules/ledger';
import { defineCrudDomain } from './defineCrudDomain';

export const { ConfiguredCrudPage, hasCrudResource, getCrudPageConfig } = defineCrudDomain({
  ledgerAccounts: {
    ...createLedgerAccountsCrudConfig(),
    featureFlags: { tagPills: false, standardMedia: false },
  },
});
