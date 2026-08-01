export interface SDKInvoiceRequest {
  PtoVta: number;
  CbteTipo: number;
  invoices: SDKInvoiceDetail[];
}

export interface SDKInvoiceDetail {
  Concepto: number;
  DocTipo: number;
  DocNro: number;
  CbteDesde: number;
  CbteHasta: number;
  CbteFch: string;
  ImpTotal: number;
  ImpTotConc: number;
  ImpNeto: number;
  ImpOpEx: number;
  ImpTrib: number;
  ImpIVA: number;
  MonId: string;
  MonCotiz: number;
  FchServDesde?: string;
  FchServHasta?: string;
  FchVtoPago?: string;
  Iva?: Array<{ Id: number; BaseImp: number; Importe: number }>;
  CbtesAsoc?: Array<{
    Tipo: number;
    PtoVta: number;
    Nro: number;
    CbteFch?: string;
  }>;
  CondicionIVAReceptorId: number;
  CanMisMonExt?: "S" | "N";
}

export interface SDKError {
  Code: number;
  Msg: string;
}

export interface SDKAuthorizationResponse {
  FeCabResp?: {
    Resultado?: "A" | "R" | "P";
  };
  FeDetResp?: {
    FECAEDetResponse:
      | SDKAuthorizationDetail
      | SDKAuthorizationDetail[];
  };
  Errors?: { Err: SDKError | SDKError[] };
  Events?: { Evt: SDKError | SDKError[] };
}

export interface SDKAuthorizationDetail {
  Resultado: "A" | "R";
  CAE?: string;
  CAEFchVto?: string;
  Observaciones?: { Obs: SDKError | SDKError[] };
}

export interface SDKConsultResponse {
  ResultGet?: {
    PtoVta?: number;
    CbteTipo?: number;
    CbteDesde: number;
    CbteHasta: number;
    DocTipo: number;
    DocNro: number;
    ImpTotal: number;
    ImpNeto: number;
    ImpOpEx: number;
    ImpIVA: number;
    MonId: string;
    MonCotiz: number;
    Resultado: "A" | "R";
    CodAutorizacion?: string;
    FchVto?: string;
    Observaciones?: { Obs: SDKError | SDKError[] };
  };
  Errors?: { Err: SDKError | SDKError[] };
  Events?: { Evt: SDKError | SDKError[] };
}

export interface ExplicitSDKClient {
  authorize(request: SDKInvoiceRequest): Promise<SDKAuthorizationResponse>;
  consult(reference: {
    pointOfSale: number;
    voucherType: number;
    voucherNumber: number;
  }): Promise<SDKConsultResponse>;
}
