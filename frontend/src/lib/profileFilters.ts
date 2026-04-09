import { getTenantProfile } from './tenantProfile';

export function getVisibleModuleIds(): Set<string> {
  const profile = getTenantProfile();
  if (!profile) return new Set();

  const visible = new Set<string>();

  const sellsProducts = profile.sells === 'products' || profile.sells === 'both';
  const sellsServices = profile.sells === 'services' || profile.sells === 'both';
  const sellsCatalog = sellsProducts || sellsServices;
  const exploring = profile.sells === 'unsure';

  // Core: always show
  visible.add('customers');

  // Team management: only if not solo
  if (profile.teamSize !== 'solo') {
    visible.add('employees');
    visible.add('roles');
  }

  // Inventory remains product-only.
  if (sellsProducts || exploring) {
    visible.add('products');
    visible.add('inventory');
    visible.add('inventoryMovements');
  }

  if (sellsServices || exploring) {
    visible.add('services');
  }

  // Commercial catalog modules apply to both products and services.
  if (sellsCatalog || exploring) {
    visible.add('priceLists');
    visible.add('suppliers');
    visible.add('purchases');
    // Solicitudes internas + políticas (governance / CEL); backend: /v1/procurement-*
    visible.add('procurementRequests');
    visible.add('procurementPolicies');
    visible.add('quotes');
  }

  // Billing & finance: only if wants payment tracking
  if (profile.usesBilling || exploring) {
    visible.add('sales');
    visible.add('payments');
    visible.add('cashflow');
    visible.add('accounts');
  }

  // Heavier finance: products + billing
  if ((sellsProducts && profile.usesBilling) || exploring) {
    visible.add('suppliers');
    visible.add('returns');
    visible.add('creditNotes');
    visible.add('recurring');
  }

  // Scheduling: operación interna en `/agenda`. El flujo público cliente
  // se sirve desde su URL real, no embebido en consola.

  // Integrations: only if billing or products
  if (profile.usesBilling || sellsProducts || exploring) {
    visible.add('paymentGateway');
  }

  // Documents & timeline: show if billing or products (operational tools)
  if (profile.usesBilling || sellsProducts || exploring) {
    visible.add('timeline');
    visible.add('documents');
    visible.add('attachments');
  }

  // Parties: show if medium+ team (advanced entity model)
  if (profile.teamSize === 'medium' || profile.teamSize === 'large' || exploring) {
    visible.add('parties');
  }

  // Advanced/admin: only if medium+ team or exploring
  if (profile.teamSize === 'medium' || profile.teamSize === 'large' || exploring) {
    visible.add('audit');
    visible.add('reports');
    visible.add('dataIO');
    visible.add('webhooks');
  }

  // Reports: show if billing (even solo users want to see reports)
  if (profile.usesBilling) {
    visible.add('reports');
  }

  return visible;
}
