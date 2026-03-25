import { getTenantProfile } from './tenantProfile';

export function getVisibleWidgetKeys(): Set<string> {
  const profile = getTenantProfile();
  if (!profile) return new Set();

  const vis: Record<string, boolean> = {
    'sales.summary': profile.usesBilling,
    'cashflow.summary': profile.usesBilling,
    'sales.recent': profile.usesBilling,
    'quotes.pipeline': profile.sells === 'products' || profile.sells === 'both',
    'inventory.low_stock': profile.sells === 'products' || profile.sells === 'both',
    'products.top': profile.sells === 'products' || profile.sells === 'both',
    'billing.subscription': true,
    'audit.activity': true,
  };

  return new Set(Object.entries(vis).filter(([, show]) => show).map(([key]) => key));
}

export function getVisibleModuleIds(): Set<string> {
  const profile = getTenantProfile();
  if (!profile) return new Set();

  const visible = new Set<string>();

  const sellsProducts = profile.sells === 'products' || profile.sells === 'both';
  const exploring = profile.sells === 'unsure';

  // Core: always show
  visible.add('customers');

  // Team management: only if not solo
  if (profile.teamSize !== 'solo') {
    visible.add('employees');
    visible.add('roles');
  }

  // Products, inventory, price lists, purchases: only if sells products
  if (sellsProducts || exploring) {
    visible.add('products');
    visible.add('inventory');
    visible.add('inventoryMovements');
    visible.add('priceLists');
    visible.add('suppliers');
    visible.add('purchases');
    // Solicitudes internas + políticas (governance / CEL); backend: /v1/procurement-*
    visible.add('procurementRequests');
    visible.add('procurementPolicies');
  }

  // Quotes: only if sells products (services usually don't quote)
  if (sellsProducts || exploring) {
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

  // Scheduling: only if uses scheduling
  if (profile.usesScheduling) {
    visible.add('appointments');
  }

  // Integrations: only if billing or products
  if (profile.usesBilling || sellsProducts || exploring) {
    visible.add('whatsapp');
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
