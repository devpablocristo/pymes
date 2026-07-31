export interface FiscalRuntimeMetrics {
  authorized: number;
  rejected: number;
  uncertain: number;
  not_found: number;
}

export interface FiscalRuntimeObserver {
  ping(): Promise<void>;
  metrics(): Promise<FiscalRuntimeMetrics>;
}
