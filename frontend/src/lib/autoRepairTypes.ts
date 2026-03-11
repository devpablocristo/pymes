export type AutoRepairVehicle = {
  id: string;
  org_id: string;
  customer_id?: string;
  customer_name: string;
  license_plate: string;
  vin: string;
  make: string;
  model: string;
  year: number;
  kilometers: number;
  color: string;
  notes: string;
  created_at: string;
  updated_at: string;
};

export type AutoRepairService = {
  id: string;
  org_id: string;
  code: string;
  name: string;
  description: string;
  category: string;
  estimated_hours: number;
  base_price: number;
  currency: string;
  tax_rate: number;
  linked_product_id?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type AutoRepairWorkOrderItem = {
  id?: string;
  item_type: 'service' | 'part';
  service_id?: string;
  product_id?: string;
  description: string;
  quantity: number;
  unit_price: number;
  tax_rate: number;
  sort_order?: number;
  metadata?: Record<string, unknown>;
};

export type AutoRepairWorkOrder = {
  id: string;
  org_id: string;
  number: string;
  vehicle_id: string;
  vehicle_plate: string;
  customer_id?: string;
  customer_name: string;
  appointment_id?: string;
  quote_id?: string;
  sale_id?: string;
  status: 'received' | 'diagnosis' | 'in_progress' | 'ready' | 'delivered' | 'invoiced' | 'cancelled';
  requested_work: string;
  diagnosis: string;
  notes: string;
  internal_notes: string;
  currency: string;
  subtotal_services: number;
  subtotal_parts: number;
  tax_total: number;
  total: number;
  opened_at: string;
  promised_at?: string;
  ready_at?: string;
  delivered_at?: string;
  created_by: string;
  created_at: string;
  updated_at: string;
  items: AutoRepairWorkOrderItem[];
};

export type AutoRepairAppointment = {
  id: string;
  customer_name: string;
  title: string;
  status?: string;
  start_at?: string;
  end_at?: string;
};

export type AutoRepairPaymentLink = {
  id: string;
  provider: string;
  reference_type: string;
  reference_id: string;
  status: string;
  amount: number;
  payment_url?: string;
  qr_data?: string;
  expires_at: string;
  created_at: string;
};

export type WorkshopVehicle = AutoRepairVehicle;
export type WorkshopService = AutoRepairService;
export type WorkOrderItem = AutoRepairWorkOrderItem;
export type WorkOrder = AutoRepairWorkOrder;
export type WorkshopAppointment = AutoRepairAppointment;
export type WorkshopPaymentLink = AutoRepairPaymentLink;
