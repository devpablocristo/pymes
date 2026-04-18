import { describe, expect, it } from 'vitest';

import {
  accountFormToBody,
  buildAccountSearchText,
  buildCustomerSearchText,
  buildPartySearchText,
  buildSupplierSearchText,
  customerFormToBody,
  formatActivePartyRoles,
  formatPartyAddress,
  parsePartyPermissionInputs,
  parsePartyTagCsv,
  partyFormToBody,
  roleEmployeeBody,
  supplierFormToBody,
} from './partiesHelpers';

describe('partiesHelpers', () => {
  it('normaliza tags y direccion para customer-like records', () => {
    expect(parsePartyTagCsv(' vip, mora , , mayorista ')).toEqual(['vip', 'mora', 'mayorista']);
    expect(formatPartyAddress({ street: 'San Martin 1', city: 'Tucuman', country: 'AR' })).toBe('San Martin 1, Tucuman, AR');
    expect(
      buildCustomerSearchText({
        name: 'Cliente Demo',
        email: 'demo@test.local',
        tags: ['vip'],
        address: { city: 'Tucuman' },
      }),
    ).toContain('Cliente Demo');
  });

  it('arma body de customer y supplier de forma consistente', () => {
    expect(
      customerFormToBody({
        type: 'person',
        name: 'Cliente',
        tax_id: '20-1',
        email: 'cliente@test.local',
        phone: '123',
        tags: 'vip, mora',
        address_street: 'A',
        address_city: 'B',
        address_state: 'C',
        address_country: 'AR',
        notes: 'nota',
      }),
    ).toMatchObject({
      type: 'person',
      name: 'Cliente',
      tags: ['vip', 'mora'],
      address: { street: 'A', city: 'B', state: 'C', country: 'AR' },
    });

    expect(
      supplierFormToBody({
        name: 'Proveedor',
        contact_name: 'Ana',
        tax_id: '30-1',
        email: 'prove@test.local',
        phone: '456',
        tags: 'importado, logistica',
        notes: 'ok',
      }),
    ).toMatchObject({
      name: 'Proveedor',
      contact_name: 'Ana',
      tags: ['importado', 'logistica'],
    });
    expect(buildSupplierSearchText({ name: 'Proveedor', contact_name: 'Ana' })).toContain('Ana');
  });

  it('arma body de parties y employees', () => {
    const partyBody = partyFormToBody({
      party_type: 'organization',
      display_name: 'ACME',
      org_legal_name: 'ACME SA',
      org_trade_name: 'ACME',
      org_tax_condition: 'RI',
      tags: 'cliente',
    });
    expect(partyBody).toMatchObject({
      party_type: 'organization',
      display_name: 'ACME',
      tags: ['cliente'],
      organization: { legal_name: 'ACME SA', trade_name: 'ACME', tax_condition: 'RI' },
    });

    expect(
      roleEmployeeBody({
        party_type: 'person',
        display_name: 'Operario',
      }),
    ).toMatchObject({
      display_name: 'Operario',
      roles: [{ role: 'employee' }],
    });
    expect(buildPartySearchText({ display_name: 'Operario', roles: [{ role: 'employee', is_active: true }] })).toContain(
      'employee',
    );
    expect(formatActivePartyRoles([{ role: 'employee', is_active: true }, { role: 'supplier', is_active: false }])).toBe(
      'employee',
    );
  });

  it('parsea permisos y cuentas', () => {
    expect(parsePartyPermissionInputs('[{"resource":"customers","action":"read"}]')).toEqual([
      { resource: 'customers', action: 'read' },
    ]);

    expect(
      accountFormToBody({
        type: 'receivable',
        entity_type: 'customer',
        entity_id: 'abc',
        entity_name: 'Cliente Demo',
        amount: '100',
        currency: 'ARS',
        credit_limit: '200',
        description: 'Inicial',
      }),
    ).toMatchObject({
      type: 'receivable',
      entity_type: 'customer',
      entity_id: 'abc',
      entity_name: 'Cliente Demo',
      amount: 100,
      credit_limit: 200,
    });
    expect(buildAccountSearchText({ entity_name: 'Cliente Demo', type: 'receivable' })).toContain('Cliente Demo');
  });
});
