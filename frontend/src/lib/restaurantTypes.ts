export type RestaurantDiningArea = {
  id: string;
  org_id: string;
  name: string;
  sort_order: number;
  is_favorite?: boolean;
  tags?: string[];
  created_at: string;
  updated_at: string;
};

export type RestaurantDiningTable = {
  id: string;
  org_id: string;
  area_id: string;
  code: string;
  label: string;
  capacity: number;
  status: string;
  notes: string;
  is_favorite?: boolean;
  tags?: string[];
  created_at: string;
  updated_at: string;
};

export type RestaurantTableSession = {
  id: string;
  org_id: string;
  table_id: string;
  table_code?: string;
  area_name?: string;
  guest_count: number;
  party_label: string;
  notes: string;
  opened_at: string;
  closed_at?: string | null;
  created_at: string;
  updated_at: string;
};
