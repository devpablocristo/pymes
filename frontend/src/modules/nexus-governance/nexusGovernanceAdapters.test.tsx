import { describe, expect, it } from 'vitest';
import {
  createNexusRolesCrudConfig,
  createProcurementPoliciesCrudConfig,
  createProcurementRequestsCrudConfig,
  getNexusGovernanceNotice,
} from './nexusGovernanceAdapters';

describe('nexusGovernanceAdapters', () => {
  it('builds procurement and role configs as thin adapters', () => {
    expect(createProcurementRequestsCrudConfig().labelPlural).toBe('solicitudes de compra internas');
    expect(createProcurementPoliciesCrudConfig().labelPluralCap).toContain('Nexus Governance');
    expect(createNexusRolesCrudConfig().labelPlural).toBe('roles');
  });

  it('exposes the governance ownership notice', () => {
    expect(getNexusGovernanceNotice()).toContain('Nexus');
  });
});
