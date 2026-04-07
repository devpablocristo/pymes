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
  archived_at?: string | null;
  created_at: string;
  updated_at: string;
};

// Alias neutro mantenido por compatibilidad con resourceConfigs.workshops.tsx.
export type WorkshopVehicle = AutoRepairVehicle;
