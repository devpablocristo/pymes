export type BeautyStaffMember = {
  id: string;
  org_id: string;
  display_name: string;
  role: string;
  color: string;
  is_active: boolean;
  notes: string;
  created_at: string;
  updated_at: string;
};

export type BeautySalonService = {
  id: string;
  org_id: string;
  code: string;
  name: string;
  description: string;
  category: string;
  duration_minutes: number;
  base_price: number;
  currency: string;
  tax_rate: number;
  linked_product_id?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};
