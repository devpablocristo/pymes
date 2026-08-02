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
    Concepto: number;
    PtoVta?: number;
    CbteTipo?: number;
    CbteDesde: number;
    CbteHasta: number;
    CbteFch: string;
    DocTipo: number;
    DocNro: number;
    ImpTotal: number;
    ImpTotConc: number;
    ImpNeto: number;
    ImpOpEx: number;
    ImpTrib: number;
    ImpIVA: number;
    FchServDesde: string;
    FchServHasta: string;
    FchVtoPago: string;
    MonId: string;
    MonCotiz: number;
    Resultado: "A" | "R";
    CodAutorizacion?: string;
    EmisionTipo: string;
    FchVto?: string;
    CondicionIVAReceptorId?: number;
    CanMisMonExt?: string;
    Iva?: {
      AlicIva:
        | { Id: number; BaseImp: number; Importe: number }
        | Array<{ Id: number; BaseImp: number; Importe: number }>;
    };
    CbtesAsoc?: {
      CbteAsoc:
        | {
            Tipo: number;
            PtoVta: number;
            Nro: number;
            Cuit?: string | number;
            CbteFch?: string;
          }
        | Array<{
            Tipo: number;
            PtoVta: number;
            Nro: number;
            Cuit?: string | number;
            CbteFch?: string;
          }>;
    };
    Observaciones?: { Obs: SDKError | SDKError[] };
  };
  Errors?: { Err: SDKError | SDKError[] };
  Events?: { Evt: SDKError | SDKError[] };
}

export interface SDKPointOfSale {
  number: number;
  emissionType: string;
  blocked: boolean;
  deactivatedOn?: string;
}

export interface SDKVoucherSequenceReference {
  pointOfSale: number;
  voucherType: number;
}

export interface SDKLastAuthorizedVoucher
  extends SDKVoucherSequenceReference {
  voucherNumber: number;
}

export interface ExplicitSDKBaseClient {
  authorize(request: SDKInvoiceRequest): Promise<SDKAuthorizationResponse>;
  consult(reference: {
    pointOfSale: number;
    voucherType: number;
    voucherNumber: number;
  }): Promise<SDKConsultResponse>;
  listPointsOfSale(): Promise<SDKPointOfSale[]>;
}

export interface ExplicitSDKSequenceClient {
  lastAuthorizedVoucher(
    reference: SDKVoucherSequenceReference,
  ): Promise<SDKLastAuthorizedVoucher>;
}

export interface ExplicitSDKClient
  extends ExplicitSDKBaseClient, ExplicitSDKSequenceClient {}
