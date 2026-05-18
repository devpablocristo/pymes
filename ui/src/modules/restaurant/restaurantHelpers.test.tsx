import { describe, expect, it } from 'vitest';
import {
  createRestaurantDiningAreasCrudConfig,
  createRestaurantDiningTablesCrudConfig,
  renderRestaurantTableStatusBadge,
} from './restaurantHelpers';

describe('restaurantHelpers', () => {
  it('builds dining area and dining table configs', () => {
    expect(createRestaurantDiningAreasCrudConfig().labelPlural).toBe('zonas del salón');
    expect(createRestaurantDiningTablesCrudConfig().labelPlural).toBe('mesas');
  });

  it('renders a status badge for tables', () => {
    const badge = renderRestaurantTableStatusBadge('occupied');
    expect(badge.props.className).toContain('badge-warning');
    expect(badge.props.children).toBe('occupied');
  });
});
