import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { HttpError } from "@devpablocristo/platform-http";
import { Navigate, NavLink, useParams } from "react-router-dom";
import type { components } from "../../api/schema.generated";
import { useProductApi } from "../../api/ProductApiContext";
import { createIdempotencyKey } from "../../api/idempotency";
import { calendarDate } from "../calendarDate";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";

type CurrentSession = components["schemas"]["CurrentSession"];
type Settings = components["schemas"]["ArgentinaFiscalSettings"];
type Certificate = components["schemas"]["FiscalCertificate"];
type PointOfSale = components["schemas"]["FiscalPointOfSale"];
type Voucher = components["schemas"]["FiscalVoucher"];
type VoucherDetail = components["schemas"]["FiscalVoucherDetail"];
type VoucherList = components["schemas"]["FiscalVoucherList"];
type VoucherLineInput = components["schemas"]["FiscalVoucherLineInput"];
type PurchaseVoucher = components["schemas"]["FiscalPurchaseVoucher"];
type PurchaseVoucherList = components["schemas"]["FiscalPurchaseVoucherList"];
type PurchaseVoucherInput = components["schemas"]["FiscalPurchaseVoucherInput"];
type PurchaseTaxInput = components["schemas"]["FiscalPurchaseTaxInput"];
type IVASimple = components["schemas"]["IVASimpleReport"];
type IVAWorkflow = components["schemas"]["IVASimpleWorkflowPeriod"];
type IVAExport = components["schemas"]["IVASimpleExportArtifact"];
type HomologationRun = components["schemas"]["FiscalHomologationRun"];

const sections = [
  ["vouchers", "Comprobantes"],
  ["purchases", "Compras"],
  ["settings", "Configuración"],
  ["iva", "IVA Simple"],
  ["homologation", "Homologación"],
] as const;

type FiscalSection = (typeof sections)[number][0];

export function FiscalPage() {
  const api = useProductApi();
  const params = useParams<{ section?: string }>();
  const section = (params.section ?? "vouchers") as FiscalSection;
  const [session, setSession] = useState<CurrentSession>();
  const [permissionError, setPermissionError] = useState<string>();

  useEffect(() => {
    const controller = new AbortController();
    api
      .request<CurrentSession>("/api/v1/session", {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then((value) => {
        setSession(value);
        setPermissionError(undefined);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setSession(undefined);
        setPermissionError(
          message(
            cause,
            "No pudimos verificar tus permisos. La sección quedó en modo lectura.",
          ),
        );
      });
    return () => controller.abort();
  }, [api]);

  if (!sections.some(([value]) => value === section)) {
    return <Navigate replace to="/fiscal/vouchers" />;
  }

  const canManage = session?.permissions.includes("fiscal:manage") ?? false;

  return (
    <div className="directory-page fiscal-page">
      <SectionHeader title="Fiscal Argentina" subtitle="ARCA e IVA" />
      <div className="directory-page__content">
        <nav className="directory-tabs finance-tabs" aria-label="Secciones fiscales">
          {sections.map(([value, label]) => (
            <NavLink key={value} to={`/fiscal/${value}`}>
              {label}
            </NavLink>
          ))}
        </nav>
        {permissionError ? (
          <div className="fiscal-permission-state inline-state inline-state--error" role="alert">
            {permissionError}
          </div>
        ) : null}
        {section === "vouchers" ? (
          <VouchersPanel canManage={canManage} />
        ) : section === "purchases" ? (
          <PurchasesPanel canManage={canManage} />
        ) : section === "settings" ? (
          <FiscalSettingsPanel canManage={canManage} />
        ) : section === "iva" ? (
          <IVAPanel canManage={canManage} />
        ) : (
          <HomologationPanel
            canManage={canManage}
            organizationId={session?.organization.id}
          />
        )}
      </div>
    </div>
  );
}

function VouchersPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [items, setItems] = useState<Voucher[]>([]);
  const [points, setPoints] = useState<PointOfSale[]>([]);
  const [environment, setEnvironment] =
    useState<Settings["environment"]>("homologation");
  const [query, setQuery] = useState("");
  const [state, setState] = useState<"all" | Voucher["state"]>("all");
  const [showCreate, setShowCreate] = useState(false);
  const [detail, setDetail] = useState<VoucherDetail>();
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const detailRequest = useRef<AbortController | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const search = new URLSearchParams({ environment, limit: "100" });
      if (query.trim()) search.set("query", query.trim());
      if (state !== "all") search.set("status", state);
      const [vouchers, configuredPoints] = await Promise.all([
        api.request<VoucherList>(`/api/v1/fiscal/vouchers?${search}`),
        api.request<PointOfSale[]>("/api/v1/fiscal/points-of-sale"),
      ]);
      setItems(vouchers.items);
      setPoints(configuredPoints);
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos cargar los comprobantes."));
    } finally {
      setLoading(false);
    }
  }, [api, environment, query, state]);
  useEffect(() => void load(), [load]);
  useEffect(() => {
    if (!canManage) setShowCreate(false);
  }, [canManage]);
  useEffect(() => {
    detailRequest.current?.abort();
    setDetail(undefined);
    setShowCreate(false);
  }, [environment]);
  useEffect(() => () => detailRequest.current?.abort(), []);

  async function openDetail(voucher: Voucher) {
    detailRequest.current?.abort();
    const controller = new AbortController();
    detailRequest.current = controller;
    setDetailLoading(true);
    setError(undefined);
    try {
      const value = await api.request<VoucherDetail>(
        `/api/v1/fiscal/vouchers/${voucher.id}`,
        { signal: controller.signal, skipJSONContentType: true },
      );
      if (value.environment !== environment) {
        throw new Error("El comprobante pertenece a otro ambiente fiscal.");
      }
      setDetail(value);
    } catch (cause) {
      if (controller.signal.aborted) return;
      setError(message(cause, "No pudimos cargar el comprobante."));
    } finally {
      if (!controller.signal.aborted) setDetailLoading(false);
    }
  }

  async function downloadPDF(voucher: Voucher) {
    try {
      const response = await api.requestResponse(
        `/api/v1/fiscal/vouchers/${voucher.id}/pdf`,
      );
      if (!response.ok) throw new Error("El PDF todavía no está disponible.");
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `comprobante-${voucher.point_of_sale ?? 0}-${voucher.voucher_number ?? 0}.pdf`;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (cause) {
      setError(message(cause, "No pudimos descargar el PDF."));
    }
  }

  return (
    <section className="directory-section">
      <div className="finance-toolbar">
        <SectionSearch
          label="Buscar comprobantes"
          placeholder="Buscar receptor, número o CAE…"
          value={query}
          onChange={setQuery}
        />
        <label className="finance-select fiscal-environment-select">
          Ambiente
          <select
            aria-label="Ambiente de comprobantes"
            value={environment}
            onChange={(event) =>
              setEnvironment(event.target.value as Settings["environment"])
            }
          >
            <option value="homologation">Homologación</option>
            <option value="production">Producción</option>
          </select>
        </label>
        <div className="lifecycle-tabs" role="group" aria-label="Estado fiscal">
          {(["all", "queued", "processing", "authorized", "rejected", "uncertain"] as const).map(
            (value) => (
              <button
                className={state === value ? "is-active" : ""}
                key={value}
                onClick={() => setState(value)}
                type="button"
              >
                {fiscalState(value)}
              </button>
            ),
          )}
        </div>
        {canManage ? (
          <button
            className="directory-create-button"
            onClick={() => setShowCreate((value) => !value)}
            type="button"
          >
            <span>＋</span> Emitir
          </button>
        ) : null}
      </div>
      {canManage && showCreate ? (
        <VoucherForm
          environment={environment}
          points={points.filter(
            (point) => point.environment === environment && point.active,
          )}
          onCreated={() => {
            setShowCreate(false);
            void load();
          }}
        />
      ) : null}
      <InlineFeedback error={error} loading={loading} />
      {items.some((item) => item.state === "uncertain") ? (
        <div className="fiscal-warning" role="status">
          <strong>Hay autorizaciones inciertas</strong>
          <span>
            El sistema consulta el mismo número en ARCA antes de cualquier reintento.
          </span>
        </div>
      ) : null}
      <div className="directory-table-wrap">
        <table className="directory-table finance-table">
          <thead>
            <tr>
              <th>Comprobante</th>
              <th>Fecha</th>
              <th>Origen</th>
              <th>Total</th>
              <th>CAE</th>
              <th>Estado</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {items.map((voucher) => (
              <tr key={voucher.id}>
                <td>
                  <strong>{voucherKind(voucher.kind)}</strong>
                  <small className="finance-row-note">
                    {voucher.point_of_sale && voucher.voucher_number
                      ? `${String(voucher.point_of_sale).padStart(5, "0")}-${String(voucher.voucher_number).padStart(8, "0")}`
                      : "Número pendiente"}
                  </small>
                  <small className="finance-row-note">
                    {environmentLabel(voucher.environment)}
                  </small>
                </td>
                <td>{formatDateTime(voucher.created_at)}</td>
                <td>{voucher.source_type}</td>
                <td className="money-cell">
                  {formatMoney(voucher.total, voucher.currency)}
                </td>
                <td className="mono-cell">{voucher.cae ?? "—"}</td>
                <td>
                  <span className={`status-pill status-pill--${voucher.state}`}>
                    {fiscalState(voucher.state)}
                  </span>
                </td>
                <td>
                  <div className="directory-row-actions">
                    <button
                      disabled={voucher.state !== "authorized"}
                      onClick={() => void downloadPDF(voucher)}
                      type="button"
                    >
                      PDF
                    </button>
                    <button onClick={() => void openDetail(voucher)} type="button">
                      Detalle
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {!loading && items.length === 0 ? (
              <EmptyRow columns={7} text="Todavía no hay comprobantes." />
            ) : null}
          </tbody>
        </table>
      </div>
      {detailLoading ? <div className="inline-state">Cargando detalle…</div> : null}
      {detail ? (
        <VoucherDetailPanel
          canManage={canManage}
          detail={detail}
          onClose={() => setDetail(undefined)}
          onCreated={() => {
            setDetail(undefined);
            void load();
          }}
        />
      ) : null}
    </section>
  );
}

function VoucherForm({
  environment,
  points,
  onCreated,
}: {
  environment: Settings["environment"];
  points: PointOfSale[];
  onCreated: () => void;
}) {
  const api = useProductApi();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [settings, setSettings] = useState<Settings>();
  const [settingsLoading, setSettingsLoading] = useState(true);
  const [concept, setConcept] =
    useState<components["schemas"]["FiscalConcept"]>("products");
  const [documentType, setDocumentType] =
    useState<components["schemas"]["FiscalVoucherInput"]["receiver_document_type"]>(
      "CUIT",
    );
  const [saleCondition, setSaleCondition] =
    useState<components["schemas"]["FiscalVoucherInput"]["sale_condition"]>(
      "cash",
    );
  const [paymentMethod, setPaymentMethod] =
    useState<components["schemas"]["FiscalVoucherInput"]["payment_method"]>(
      "cash",
    );
  const [partyID, setPartyID] = useState("");
  const [accountingDueDate, setAccountingDueDate] = useState("");
  const [currency, setCurrency] = useState("ARS");
  const [lines, setLines] = useState<EditableFiscalLine[]>([
    createEditableFiscalLine(),
  ]);
  const typeC =
    settings?.tax_condition === "monotributo" ||
    settings?.tax_condition === "exempt";
  const calculatedLines = lines.map((line) =>
    calculateEditableFiscalLine(line, typeC),
  );
  const net = safeAddDecimals(
    ...calculatedLines.map((line) => line.subtotal),
  );
  const vat = safeAddDecimals(
    ...calculatedLines.map((line) => line.vatAmount),
  );
  const otherTaxes = safeAddDecimals(
    ...calculatedLines.map((line) => line.otherTaxesAmount),
  );
  const total = safeAddDecimals(
    ...calculatedLines.map((line) => line.total),
  );
  const needsServiceDates = concept === "services" || concept === "mixed";

  useEffect(() => {
    const controller = new AbortController();
    setSettings(undefined);
    setSettingsLoading(true);
    setError(undefined);
    api
      .requestResponse(`/api/v1/fiscal/settings?environment=${environment}`, {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then(async (response) => {
        if (response.status === 404) {
          setSettings(undefined);
          return;
        }
        if (!response.ok) throw new Error("No pudimos cargar el perfil fiscal.");
        const value = (await response.json()) as Settings;
        if (value.environment !== environment) {
          throw new Error("El perfil fiscal no coincide con el ambiente elegido.");
        }
        setSettings(value);
      })
      .catch((cause: unknown) => {
        if (controller.signal.aborted) return;
        setError(message(cause, "No pudimos validar el ambiente fiscal."));
      })
      .finally(() => {
        if (!controller.signal.aborted) setSettingsLoading(false);
      });
    return () => controller.abort();
  }, [api, environment]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const serviceFrom = optionalFormString(form, "service_from");
    const serviceTo = optionalFormString(form, "service_to");
    const paymentDueDate = optionalFormString(form, "payment_due_date");
    const normalizedPartyID = partyID.trim();
    if (
      needsServiceDates &&
      (!serviceFrom || !serviceTo || !paymentDueDate)
    ) {
      setError(
        "Servicios y conceptos mixtos requieren período de prestación y vencimiento.",
      );
      return;
    }
    if (serviceFrom && serviceTo && serviceFrom > serviceTo) {
      setError("La prestación no puede terminar antes de comenzar.");
      return;
    }
    if (paymentDueDate && paymentDueDate < calendarDate()) {
      setError("El vencimiento no puede ser anterior a la fecha de emisión.");
      return;
    }
    if (
      saleCondition === "credit" &&
      (!normalizedPartyID || !accountingDueDate)
    ) {
      setError(
        "Las ventas a crédito requieren un cliente contable y su vencimiento.",
      );
      return;
    }
    if (saleCondition === "credit" && accountingDueDate < calendarDate()) {
      setError("El vencimiento contable no puede ser anterior a la emisión.");
      return;
    }
    const invalidCostLine = lines.findIndex(
      (line) =>
        line.costConfirmed &&
        (!isNonNegativeDecimal(line.costAmount) ||
          decimalScale(line.costAmount) > 6),
    );
    if (invalidCostLine >= 0) {
      setError(
        `El costo confirmado de la línea ${invalidCostLine + 1} debe ser un decimal no negativo de hasta seis decimales.`,
      );
      return;
    }
    const invalidTribute = findInvalidFiscalTribute(lines);
    if (invalidTribute) {
      setError(
        `Revisá el tributo ${invalidTribute.tribute} de la línea ${invalidTribute.line}: requiere código ARCA, descripción e importes válidos.`,
      );
      return;
    }
    if (!settings || settings.environment !== environment) {
      setError("Configurá el perfil fiscal del ambiente antes de emitir.");
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      await api.request<Voucher>("/api/v1/fiscal/vouchers", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("fiscal-voucher") },
        body: JSON.stringify({
          environment,
          source_type: "sale",
          source_id: form.get("source_id"),
          point_of_sale_id: form.get("point_of_sale_id"),
          concept,
          receiver_name: form.get("receiver_name"),
          receiver_document_type: documentType,
          receiver_document_number:
            documentType === "CONSUMER_FINAL"
              ? "0"
              : form.get("document_number"),
          receiver_tax_condition: form.get("receiver_tax_condition"),
          sale_condition: saleCondition,
          party_id:
            saleCondition === "credit" ? normalizedPartyID : undefined,
          payment_method: paymentMethod,
          service_from: needsServiceDates ? serviceFrom : undefined,
          service_to: needsServiceDates ? serviceTo : undefined,
          payment_due_date: needsServiceDates ? paymentDueDate : undefined,
          accounting_due_date:
            saleCondition === "credit" ? accountingDueDate : undefined,
          currency,
          exchange_rate: form.get("exchange_rate"),
          exchange_rate_source: "ARCA",
          lines: calculatedLines.map((line) => line.payload),
        }),
      });
      onCreated();
    } catch (cause) {
      setError(message(cause, "No pudimos crear la solicitud fiscal."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      className="finance-form fiscal-issue-form"
      onSubmit={(event) => void submit(event)}
    >
      <div className="fiscal-form-context">
        <span className={`status-pill status-pill--${environment}`}>
          {environmentLabel(environment)}
        </span>
        <strong>
          {settingsLoading
            ? "Validando perfil…"
            : typeC
              ? "Emisión C · precios finales sin IVA discriminado"
              : settings
                ? "Emisión A/B · IVA discriminado según receptor"
                : "Perfil fiscal pendiente"}
        </strong>
      </div>
      <label>
        Documento de venta
        <input name="source_id" placeholder="UUID de la venta" required />
      </label>
      <label>
        Punto de venta
        <select name="point_of_sale_id" required>
          <option value="">Seleccionar</option>
          {points
            .filter((point) => point.active)
            .map((point) => (
              <option key={point.id} value={point.id}>
                {String(point.number).padStart(5, "0")} ·{" "}
                {point.name || point.environment}
              </option>
            ))}
        </select>
      </label>
      <label>
        Concepto
        <select
          name="concept"
          value={concept}
          onChange={(event) =>
            setConcept(
              event.target.value as components["schemas"]["FiscalConcept"],
            )
          }
        >
          <option value="products">Productos</option>
          <option value="services">Servicios</option>
          <option value="mixed">Mixto</option>
        </select>
      </label>
      <label>
        Receptor
        <input name="receiver_name" required />
      </label>
      <label>
        Documento
        <select
          name="document_type"
          value={documentType}
          onChange={(event) =>
            setDocumentType(
              event.target.value as components["schemas"]["FiscalVoucherInput"]["receiver_document_type"],
            )
          }
        >
          <option value="CUIT">CUIT</option>
          <option value="CUIL">CUIL</option>
          <option value="DNI">DNI</option>
          <option value="CONSUMER_FINAL">Consumidor final</option>
        </select>
      </label>
      <label>
        Número
        <input
          disabled={documentType === "CONSUMER_FINAL"}
          name="document_number"
          required={documentType !== "CONSUMER_FINAL"}
        />
      </label>
      <label>
        Condición IVA
        <select name="receiver_tax_condition">
          <option value="responsable_inscripto">Responsable inscripto</option>
          <option value="responsable_monotributo">Monotributo</option>
          <option value="iva_exento">Exento</option>
          <option value="consumidor_final">Consumidor final</option>
        </select>
      </label>
      <fieldset
        aria-describedby="sale-condition-help"
        className="fiscal-sale-terms"
      >
        <legend>Cobro y cuenta corriente</legend>
        <div>
          <label>
            Condición de venta
            <select
              name="sale_condition"
              value={saleCondition}
              onChange={(event) => {
                const value =
                  event.target.value as components["schemas"]["FiscalVoucherInput"]["sale_condition"];
                setSaleCondition(value);
                setError(undefined);
                if (value === "cash") {
                  setPartyID("");
                  setAccountingDueDate("");
                }
              }}
            >
              <option value="cash">Contado</option>
              <option value="credit">Cuenta corriente</option>
            </select>
          </label>
          <label>
            Medio de cobro
            <select
              name="payment_method"
              value={paymentMethod}
              onChange={(event) =>
                setPaymentMethod(
                  event.target.value as components["schemas"]["FiscalVoucherInput"]["payment_method"],
                )
              }
            >
              <option value="cash">Efectivo</option>
              <option value="bank_transfer">Transferencia bancaria</option>
              <option value="card">Tarjeta</option>
              <option value="wallet">Billetera</option>
              <option value="check">Cheque</option>
            </select>
          </label>
          {saleCondition === "credit" ? (
            <>
              <label>
                Cliente contable
                <input
                  name="party_id"
                  onChange={(event) => setPartyID(event.target.value)}
                  pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
                  placeholder="UUID del cliente"
                  required
                  title="Ingresá el UUID completo del cliente"
                  value={partyID}
                />
              </label>
              <label>
                Vencimiento contable
                <input
                  min={calendarDate()}
                  name="accounting_due_date"
                  onChange={(event) => setAccountingDueDate(event.target.value)}
                  required
                  type="date"
                  value={accountingDueDate}
                />
              </label>
            </>
          ) : null}
        </div>
        <small id="sale-condition-help">
          Al contado, el medio define la cuenta de cobro o clearing. A crédito,
          genera una partida por cobrar y conserva ese medio para reintegros.
        </small>
      </fieldset>
      {needsServiceDates ? (
        <>
          <label>
            Prestación desde
            <input name="service_from" required type="date" />
          </label>
          <label>
            Prestación hasta
            <input name="service_to" required type="date" />
          </label>
          <label>
            Vencimiento de pago
            <input
              min={calendarDate()}
              name="payment_due_date"
              required
              type="date"
            />
          </label>
        </>
      ) : null}
      <label>
        Moneda
        <select
          name="currency"
          value={currency}
          onChange={(event) => setCurrency(event.target.value)}
        >
          <option>ARS</option>
          <option>USD</option>
          <option>EUR</option>
        </select>
      </label>
      <label>
        Cotización
        <input
          defaultValue="1"
          min="0.000001"
          name="exchange_rate"
          required
          step="0.000001"
          type="number"
        />
      </label>
      <FiscalLinesEditor
        currency={currency}
        functionalCurrency={settings?.functional_currency ?? "ARS"}
        lines={lines}
        onChange={setLines}
        typeC={typeC}
      />
      <FiscalTotalRail
        currency={currency}
        net={net}
        taxLabel={typeC ? "IVA incluido" : "IVA"}
        tax={vat}
        additionalLabel="Otros tributos"
        additional={isZeroDecimal(otherTaxes) ? undefined : otherTaxes}
        total={total}
      />
      <button
        className="directory-create-button"
        disabled={busy || settingsLoading || !settings || points.length === 0}
        type="submit"
      >
        {busy ? "Encolando…" : "Solicitar CAE"}
      </button>
      {points.length === 0 ? (
        <span className="form-error">
          Configurá un punto de venta activo en {environmentLabel(environment)}.
        </span>
      ) : null}
      {!settingsLoading && !settings ? (
        <span className="form-error">
          Falta el perfil fiscal de {environmentLabel(environment)}.
        </span>
      ) : null}
      {error ? (
        <span className="form-error" role="alert">
          {error}
        </span>
      ) : null}
    </form>
  );
}

type EditableFiscalLine = {
  id: string;
  description: string;
  quantity: string;
  unitPrice: string;
  taxTreatment: "taxable" | "exempt" | "non_taxed";
  vatRate: string;
  costConfirmed: boolean;
  costAmount: string;
  tributes: EditableFiscalTribute[];
};

type EditableFiscalTribute = {
  id: string;
  kind: "other_tax" | "withholding" | "perception";
  authorityCode: string;
  description: string;
  taxableBase: string;
  rate: string;
  amount: string;
};

type CalculatedFiscalLine = {
  subtotal: string;
  vatAmount: string;
  otherTaxesAmount: string;
  total: string;
  payload: VoucherLineInput;
};

function createEditableFiscalLine(
  value: Partial<EditableFiscalLine> = {},
): EditableFiscalLine {
  return {
    id: crypto.randomUUID(),
    description: "",
    quantity: "1",
    unitPrice: "0.00",
    taxTreatment: "taxable",
    vatRate: "21",
    costConfirmed: false,
    costAmount: "0.00",
    ...value,
    tributes: value.tributes ? [...value.tributes] : [],
  };
}

function createEditableFiscalTribute(
  taxableBase: string,
): EditableFiscalTribute {
  return {
    id: crypto.randomUUID(),
    kind: "other_tax",
    authorityCode: "99",
    description: "",
    taxableBase,
    rate: "0",
    amount: "0.00",
  };
}

function calculateEditableFiscalLine(
  line: EditableFiscalLine,
  typeC: boolean,
): CalculatedFiscalLine {
  const subtotal = safeMultiplyDecimals(line.quantity, line.unitPrice, 2);
  const taxTreatment = typeC ? "taxable" : line.taxTreatment;
  const vatRate =
    !typeC && taxTreatment === "taxable" ? line.vatRate : "0";
  const vatAmount =
    !typeC && taxTreatment === "taxable"
      ? safePercentage(subtotal, vatRate, 2)
      : "0.00";
  const fiscalTreatment: VoucherLineInput["taxes"] = typeC
    ? []
    : taxTreatment === "taxable"
      ? [
          {
            kind: "vat",
            rate: vatRate,
            taxable_base: subtotal,
            amount: vatAmount,
          },
        ]
      : [
          {
            kind: taxTreatment,
            rate: "0",
            taxable_base: subtotal,
            amount: "0",
          },
        ];
  const tributeTaxes: VoucherLineInput["taxes"] = line.tributes.map(
    (tribute) => ({
      kind: tribute.kind,
      authority_code: Number.parseInt(tribute.authorityCode, 10),
      description: tribute.description.trim(),
      taxable_base: tribute.taxableBase,
      rate: tribute.rate,
      amount: tribute.amount,
    }),
  );
  const otherTaxesAmount = safeAddDecimals(
    ...line.tributes.map((tribute) => tribute.amount),
  );
  return {
    subtotal,
    vatAmount,
    otherTaxesAmount,
    total: safeAddDecimals(subtotal, vatAmount, otherTaxesAmount),
    payload: {
      description: line.description.trim(),
      quantity: line.quantity,
      unit_price: line.unitPrice,
      subtotal,
      cost_amount: line.costConfirmed ? line.costAmount.trim() : undefined,
      cost_confirmed: line.costConfirmed,
      taxes: [...fiscalTreatment, ...tributeTaxes],
    },
  };
}

function FiscalLinesEditor({
  currency,
  functionalCurrency = "ARS",
  lines,
  onChange,
  typeC,
}: {
  currency: string;
  functionalCurrency?: string;
  lines: EditableFiscalLine[];
  onChange: (lines: EditableFiscalLine[]) => void;
  typeC: boolean;
}) {
  function updateLine<Field extends keyof Omit<EditableFiscalLine, "id">>(
    id: string,
    field: Field,
    value: EditableFiscalLine[Field],
  ) {
    onChange(
      lines.map((line) =>
        line.id === id ? { ...line, [field]: value } : line,
      ),
    );
  }

  function updateTribute<Field extends keyof Omit<EditableFiscalTribute, "id">>(
    lineID: string,
    tributeID: string,
    field: Field,
    value: EditableFiscalTribute[Field],
  ) {
    onChange(
      lines.map((line) =>
        line.id !== lineID
          ? line
          : {
              ...line,
              tributes: line.tributes.map((tribute) =>
                tribute.id === tributeID
                  ? { ...tribute, [field]: value }
                  : tribute,
              ),
            },
      ),
    );
  }

  return (
    <fieldset className="fiscal-lines-editor">
      <legend>Detalle del comprobante</legend>
      <div className="fiscal-lines-editor__head" aria-hidden="true">
        <span>Descripción</span>
        <span>Cantidad</span>
        <span>{typeC ? "Precio final" : "Precio neto"}</span>
        <span>Tratamiento</span>
        <span>Alícuota</span>
        <span>Total</span>
        <span />
      </div>
      {lines.map((line, index) => {
        const calculated = calculateEditableFiscalLine(line, typeC);
        return (
          <div className="fiscal-lines-editor__item" key={line.id}>
            <div className="fiscal-lines-editor__row">
              <input
                aria-label={`Descripción línea ${index + 1}`}
                maxLength={500}
                required
                value={line.description}
                onChange={(event) =>
                  updateLine(line.id, "description", event.target.value)
                }
              />
              <input
                aria-label={`Cantidad línea ${index + 1}`}
                min="0.000001"
                required
                step="0.000001"
                type="number"
                value={line.quantity}
                onChange={(event) =>
                  updateLine(line.id, "quantity", event.target.value)
                }
              />
              <input
                aria-label={`${typeC ? "Precio final" : "Precio neto"} línea ${index + 1}`}
                min="0"
                required
                step="0.01"
                type="number"
                value={line.unitPrice}
                onChange={(event) =>
                  updateLine(line.id, "unitPrice", event.target.value)
                }
              />
              <select
                aria-label={`Tratamiento línea ${index + 1}`}
                disabled={typeC}
                value={typeC ? "taxable" : line.taxTreatment}
                onChange={(event) =>
                  updateLine(
                    line.id,
                    "taxTreatment",
                    event.target.value as EditableFiscalLine["taxTreatment"],
                  )
                }
              >
                <option value="taxable">Gravado</option>
                <option value="exempt">Exento</option>
                <option value="non_taxed">No gravado</option>
              </select>
              <select
                aria-label={`Alícuota línea ${index + 1}`}
                disabled={typeC || line.taxTreatment !== "taxable"}
                value={typeC ? "0" : line.vatRate}
                onChange={(event) =>
                  updateLine(line.id, "vatRate", event.target.value)
                }
              >
                <option value="21">21%</option>
                <option value="10.5">10,5%</option>
                <option value="27">27%</option>
                <option value="5">5%</option>
                <option value="2.5">2,5%</option>
                <option value="0">0%</option>
              </select>
              <output aria-label={`Total línea ${index + 1}`}>
                {formatMoney(calculated.total, currency)}
              </output>
              <button
                aria-label={`Eliminar línea ${index + 1}`}
                disabled={lines.length === 1}
                onClick={() =>
                  onChange(lines.filter((candidate) => candidate.id !== line.id))
                }
                type="button"
              >
                ×
              </button>
            </div>
            <div className="fiscal-line-accounting">
              <label className="fiscal-cost-toggle">
                <input
                  aria-label={`Costo confirmado línea ${index + 1}`}
                  checked={line.costConfirmed}
                  onChange={(event) =>
                    onChange(
                      lines.map((candidate) =>
                        candidate.id === line.id
                          ? {
                              ...candidate,
                              costConfirmed: event.target.checked,
                              costAmount: event.target.checked
                                ? candidate.costAmount
                                : "0.00",
                            }
                          : candidate,
                      ),
                    )
                  }
                  type="checkbox"
                />
                Costo confirmado
              </label>
              <label>
                Costo total ({functionalCurrency})
                <input
                  aria-label={`Costo total línea ${index + 1}`}
                  disabled={!line.costConfirmed}
                  min="0"
                  onChange={(event) =>
                    updateLine(line.id, "costAmount", event.target.value)
                  }
                  required={line.costConfirmed}
                  step="0.000001"
                  type="number"
                  value={line.costAmount}
                />
              </label>
              <button
                className="fiscal-lines-editor__add-tax"
                onClick={() =>
                  updateLine(line.id, "tributes", [
                    ...line.tributes,
                    createEditableFiscalTribute(calculated.subtotal),
                  ])
                }
                type="button"
              >
                ＋ Agregar tributo
              </button>
              <small>
                El costo se usa para inventario/CMV y no integra el comprobante
                fiscal.
              </small>
            </div>
            {line.tributes.map((tribute, tributeIndex) => (
              <fieldset className="fiscal-tribute-editor" key={tribute.id}>
                <legend>
                  Tributo adicional {tributeIndex + 1} · línea {index + 1}
                </legend>
                <label>
                  Tipo
                  <select
                    aria-label={`Tipo tributo línea ${index + 1}.${tributeIndex + 1}`}
                    value={tribute.kind}
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "kind",
                        event.target.value as EditableFiscalTribute["kind"],
                      )
                    }
                  >
                    <option value="other_tax">Otro tributo</option>
                    <option value="withholding">Retención</option>
                    <option value="perception">Percepción</option>
                  </select>
                </label>
                <label>
                  Código ARCA
                  <input
                    aria-label={`Código ARCA tributo línea ${index + 1}.${tributeIndex + 1}`}
                    max="99"
                    min="1"
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "authorityCode",
                        event.target.value,
                      )
                    }
                    required
                    step="1"
                    type="number"
                    value={tribute.authorityCode}
                  />
                </label>
                <label>
                  Descripción
                  <input
                    aria-label={`Descripción tributo línea ${index + 1}.${tributeIndex + 1}`}
                    maxLength={120}
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "description",
                        event.target.value,
                      )
                    }
                    required
                    value={tribute.description}
                  />
                </label>
                <label>
                  Base
                  <input
                    aria-label={`Base tributo línea ${index + 1}.${tributeIndex + 1}`}
                    min="0"
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "taxableBase",
                        event.target.value,
                      )
                    }
                    required
                    step="0.01"
                    type="number"
                    value={tribute.taxableBase}
                  />
                </label>
                <label>
                  Alícuota
                  <input
                    aria-label={`Alícuota tributo línea ${index + 1}.${tributeIndex + 1}`}
                    min="0"
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "rate",
                        event.target.value,
                      )
                    }
                    required
                    step="0.000001"
                    type="number"
                    value={tribute.rate}
                  />
                </label>
                <label>
                  Importe
                  <input
                    aria-label={`Importe tributo línea ${index + 1}.${tributeIndex + 1}`}
                    min="0"
                    onChange={(event) =>
                      updateTribute(
                        line.id,
                        tribute.id,
                        "amount",
                        event.target.value,
                      )
                    }
                    required
                    step="0.01"
                    type="number"
                    value={tribute.amount}
                  />
                </label>
                <button
                  aria-label={`Eliminar tributo línea ${index + 1}.${tributeIndex + 1}`}
                  onClick={() =>
                    updateLine(
                      line.id,
                      "tributes",
                      line.tributes.filter(
                        (candidate) => candidate.id !== tribute.id,
                      ),
                    )
                  }
                  type="button"
                >
                  Eliminar
                </button>
              </fieldset>
            ))}
          </div>
        );
      })}
      <button
        className="fiscal-lines-editor__add"
        disabled={lines.length >= 1000}
        onClick={() => onChange([...lines, createEditableFiscalLine()])}
        type="button"
      >
        ＋ Agregar línea
      </button>
      {typeC ? (
        <small>
          En comprobantes C cada precio ya incluye el impuesto. No se envía una
          alícuota ni un importe de IVA discriminado.
        </small>
      ) : null}
    </fieldset>
  );
}

function VoucherDetailPanel({
  canManage,
  detail,
  onClose,
  onCreated,
}: {
  canManage: boolean;
  detail: VoucherDetail;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [adjustment, setAdjustment] = useState<"credit" | "debit">();
  const canAdjust =
    canManage &&
    detail.state === "authorized" &&
    detail.kind?.startsWith("invoice_");

  return (
    <aside
      aria-label="Detalle del comprobante"
      className="fiscal-voucher-detail"
      role="dialog"
    >
      <header>
        <div>
          <small>
            {environmentLabel(detail.environment)} · snapshot inmutable
          </small>
          <h2>{voucherKind(detail.kind)}</h2>
          <span className="mono-cell">
            {detail.point_of_sale && detail.voucher_number
              ? `${String(detail.point_of_sale).padStart(5, "0")}-${String(detail.voucher_number).padStart(8, "0")}`
              : "Numeración pendiente"}
          </span>
        </div>
        <button aria-label="Cerrar detalle" onClick={onClose} type="button">
          ×
        </button>
      </header>
      <div className="fiscal-voucher-detail__facts">
        <span>
          Emisión <strong>{formatDate(detail.issue_date)}</strong>
        </span>
        <span>
          Receptor <strong>{detail.receiver_name}</strong>
        </span>
        <span>
          Documento{" "}
          <strong>
            {detail.receiver_document_type} {detail.receiver_document_number}
          </strong>
        </span>
        <span>
          Moneda{" "}
          <strong>
            {detail.currency} · {detail.exchange_rate}
          </strong>
        </span>
        {detail.service_from && detail.service_to ? (
          <span>
            Prestación{" "}
            <strong>
              {formatDate(detail.service_from)}–{formatDate(detail.service_to)}
            </strong>
          </span>
        ) : null}
        <span>
          CAE <strong className="mono-cell">{detail.cae ?? "Pendiente"}</strong>
        </span>
      </div>
      <div className="directory-table-wrap">
        <table className="directory-table fiscal-voucher-lines">
          <thead>
            <tr>
              <th>Detalle</th>
              <th>Cantidad</th>
              <th>Unitario</th>
              <th>Tratamiento</th>
              <th>IVA</th>
              <th>Total</th>
            </tr>
          </thead>
          <tbody>
            {detail.lines.map((line) => (
              <tr key={line.position}>
                <td>{line.description}</td>
                <td className="mono-cell">{line.quantity}</td>
                <td className="money-cell">
                  {formatMoney(line.unit_price, detail.currency)}
                </td>
                <td>{taxTreatmentLabel(line.tax_treatment)}</td>
                <td className="money-cell">
                  {formatMoney(line.vat_amount, detail.currency)}
                </td>
                <td className="money-cell">
                  {formatMoney(line.total, detail.currency)}
                </td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <td colSpan={5}>Total fiscal</td>
              <td className="money-cell">
                {formatMoney(detail.total, detail.currency)}
              </td>
            </tr>
          </tfoot>
        </table>
      </div>
      {detail.observations?.length ? (
        <div className="fiscal-warning">
          <strong>Observaciones de ARCA</strong>
          {detail.observations.map((value) => (
            <span key={value}>{value}</span>
          ))}
        </div>
      ) : null}
      {canAdjust ? (
        <div className="fiscal-adjustment-actions">
          <button onClick={() => setAdjustment("credit")} type="button">
            Crear nota de crédito
          </button>
          <button onClick={() => setAdjustment("debit")} type="button">
            Crear nota de débito
          </button>
        </div>
      ) : null}
      {adjustment ? (
        <FiscalAdjustmentForm
          detail={detail}
          key={`${detail.id}-${adjustment}`}
          mode={adjustment}
          onCancel={() => setAdjustment(undefined)}
          onCreated={onCreated}
        />
      ) : null}
    </aside>
  );
}

function FiscalAdjustmentForm({
  detail,
  mode,
  onCancel,
  onCreated,
}: {
  detail: VoucherDetail;
  mode: "credit" | "debit";
  onCancel: () => void;
  onCreated: () => void;
}) {
  const api = useProductApi();
  const typeC = detail.kind?.endsWith("_c") ?? false;
  const [lines, setLines] = useState<EditableFiscalLine[]>(() =>
    detail.lines.map((line) =>
      createEditableFiscalLine({
        description: line.description,
        quantity: line.quantity,
        unitPrice: line.unit_price,
        taxTreatment: line.tax_treatment,
        vatRate: line.vat_rate,
      }),
    ),
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy(true);
    setError(undefined);
    try {
      await api.request<Voucher>(
        `/api/v1/fiscal/${mode === "credit" ? "credit-notes" : "debit-notes"}`,
        {
          method: "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey(`fiscal-${mode}-note`),
          },
          body: JSON.stringify({
            associated_voucher_id: detail.id,
            reason: formString(form, "reason"),
            lines: lines.map(
              (line) => calculateEditableFiscalLine(line, typeC).payload,
            ),
          }),
        },
      );
      onCreated();
    } catch (cause) {
      setError(
        message(
          cause,
          `No pudimos crear la nota de ${mode === "credit" ? "crédito" : "débito"}.`,
        ),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      className="finance-form fiscal-adjustment-form"
      onSubmit={(event) => void submit(event)}
    >
      <h2>
        Nueva nota de {mode === "credit" ? "crédito" : "débito"} asociada
      </h2>
      <label className="finance-form__wide">
        Motivo
        <input maxLength={500} name="reason" required />
      </label>
      <FiscalLinesEditor
        currency={detail.currency}
        lines={lines}
        onChange={setLines}
        typeC={typeC}
      />
      <div className="fiscal-adjustment-form__actions">
        <button disabled={busy} onClick={onCancel} type="button">
          Cancelar
        </button>
        <button className="directory-create-button" disabled={busy} type="submit">
          {busy
            ? "Encolando…"
            : `Solicitar nota de ${mode === "credit" ? "crédito" : "débito"}`}
        </button>
      </div>
      {error ? (
        <span className="form-error" role="alert">
          {error}
        </span>
      ) : null}
    </form>
  );
}

function PurchasesPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [items, setItems] = useState<PurchaseVoucher[]>([]);
  const [query, setQuery] = useState("");
  const [period, setPeriod] = useState(currentPeriod());
  const [showCreate, setShowCreate] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const search = new URLSearchParams({ limit: "100", period });
      if (query.trim()) search.set("query", query.trim());
      const response = await api.request<PurchaseVoucherList>(
        `/api/v1/fiscal/purchase-vouchers?${search}`,
      );
      setItems(response.items);
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos cargar los comprobantes de compra."));
    } finally {
      setLoading(false);
    }
  }, [api, period, query]);
  useEffect(() => void load(), [load]);
  useEffect(() => {
    if (!canManage) setShowCreate(false);
  }, [canManage]);

  return (
    <section className="directory-section">
      <div className="finance-toolbar fiscal-purchase-toolbar">
        <SectionSearch
          label="Buscar compras"
          placeholder="Buscar proveedor, CUIT o número…"
          value={query}
          onChange={setQuery}
        />
        <label className="finance-select">
          Período
          <input
            aria-label="Período de compras"
            type="month"
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
          />
        </label>
        {canManage ? (
          <button
            className="directory-create-button"
            onClick={() => setShowCreate((value) => !value)}
            type="button"
          >
            <span>＋</span> Registrar compra
          </button>
        ) : null}
      </div>
      {canManage && showCreate ? (
        <PurchaseForm
          purchases={items}
          onCreated={() => {
            setShowCreate(false);
            void load();
          }}
        />
      ) : null}
      <InlineFeedback error={error} loading={loading} />
      <div className="directory-table-wrap">
        <table className="directory-table finance-table fiscal-purchase-table">
          <thead>
            <tr>
              <th>Comprobante</th>
              <th>Fecha</th>
              <th>Proveedor</th>
              <th>Neto</th>
              <th>IVA</th>
              <th>Tributos</th>
              <th>Total</th>
              <th>Asiento</th>
            </tr>
          </thead>
          <tbody>
            {items.map((purchase) => (
              <tr key={purchase.id}>
                <td>
                  <strong>{purchaseVoucherKind(purchase.voucher_type)}</strong>
                  <small className="finance-row-note">
                    {String(purchase.point_of_sale).padStart(5, "0")}-
                    {String(purchase.voucher_number).padStart(8, "0")}
                  </small>
                </td>
                <td>{formatDate(purchase.issue_date)}</td>
                <td>
                  <strong>{purchase.supplier_name}</strong>
                  <small className="finance-row-note">{purchase.supplier_tax_id}</small>
                </td>
                <td className="money-cell">
                  {formatMoney(purchase.net_amount, purchase.currency)}
                </td>
                <td className="money-cell">
                  {formatMoney(purchase.vat_amount, purchase.currency)}
                </td>
                <td className="money-cell">
                  {formatMoney(
                    safeAddDecimals(
                      purchase.other_taxes_amount,
                      purchase.perception_amount,
                      purchase.withholding_amount,
                    ),
                    purchase.currency,
                  )}
                </td>
                <td className="money-cell">
                  {formatMoney(purchase.total_amount, purchase.currency)}
                </td>
                <td>
                  {purchase.journal_entry_id ? (
                    <span className="status-pill status-pill--completed">Contabilizado</span>
                  ) : (
                    <span className="status-pill status-pill--queued">Pendiente</span>
                  )}
                </td>
              </tr>
            ))}
            {!loading && items.length === 0 ? (
              <EmptyRow
                columns={8}
                text="No hay comprobantes de compra para este período."
              />
            ) : null}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function PurchaseForm({
  purchases,
  onCreated,
}: {
  purchases: PurchaseVoucher[];
  onCreated: () => void;
}) {
  const api = useProductApi();
  const today = calendarDate();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [currency, setCurrency] = useState("ARS");
  const [environment, setEnvironment] =
    useState<Settings["environment"]>("homologation");
  const [voucherType, setVoucherType] = useState("1");
  const [associationQuery, setAssociationQuery] = useState("");
  const [associatedPurchaseID, setAssociatedPurchaseID] = useState("");
  const [taxTreatment, setTaxTreatment] =
    useState<components["schemas"]["FiscalPurchaseLineInput"]["tax_treatment"]>(
      "taxable",
    );
  const [net, setNet] = useState("0.00");
  const [vat, setVAT] = useState("0.00");
  const [additionalKind, setAdditionalKind] = useState<
    "none" | PurchaseTaxInput["kind"]
  >("none");
  const [additionalTax, setAdditionalTax] = useState("0.00");
  const lineTotal = safeAddDecimals(net, vat);
  const voucherTotal =
    additionalKind === "withholding"
      ? lineTotal
      : safeAddDecimals(lineTotal, additionalTax);
  const needsAssociation = purchaseAdjustmentTypes.has(voucherType);
  const originalType = purchaseInvoiceType(voucherType);
  const eligibleOriginals = purchases.filter((purchase) => {
    if (
      purchase.environment !== environment ||
      purchase.voucher_type !== originalType
    ) {
      return false;
    }
    const needle = associationQuery.trim().toLocaleLowerCase("es");
    if (!needle) return true;
    return [
      purchase.supplier_name,
      purchase.supplier_tax_id,
      `${purchase.point_of_sale}-${purchase.voucher_number}`,
    ].some((value) => String(value).toLocaleLowerCase("es").includes(needle));
  });

  useEffect(() => setAssociatedPurchaseID(""), [environment, voucherType]);

  function changeTaxTreatment(
    value: components["schemas"]["FiscalPurchaseLineInput"]["tax_treatment"],
  ) {
    setTaxTreatment(value);
    if (value !== "taxable") setVAT("0.00");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const vatRate = taxTreatment === "taxable" ? formString(form, "vat_rate") : "0";
    const taxes: PurchaseTaxInput[] = [];
    if (taxTreatment === "taxable") {
      taxes.push({
        kind: "vat",
        authority_code: vatAuthorityCode(vatRate),
        description: `IVA ${vatRate}%`,
        taxable_base: net,
        rate: vatRate,
        amount: vat,
        creditable: true,
      });
    }
    if (additionalKind !== "none" && !isZeroDecimal(additionalTax)) {
      taxes.push({
        kind: additionalKind,
        authority_code: formString(form, "additional_authority_code"),
        jurisdiction: optionalFormString(form, "additional_jurisdiction"),
        description: formString(form, "additional_description"),
        taxable_base: net,
        rate: formString(form, "additional_rate"),
        amount: additionalTax,
        creditable: false,
      });
    }

    const body: PurchaseVoucherInput = {
      environment,
      source_id: formString(form, "source_id"),
      supplier_id: formString(form, "supplier_id"),
      supplier_tax_id: formString(form, "supplier_tax_id"),
      supplier_name: formString(form, "supplier_name"),
      voucher_type: parseInteger(voucherType) as PurchaseVoucherInput["voucher_type"],
      point_of_sale: parseInteger(formString(form, "point_of_sale")),
      voucher_number: parseInteger(formString(form, "voucher_number")),
      issue_date: formString(form, "issue_date"),
      due_date: optionalFormString(form, "due_date"),
      currency,
      exchange_rate: formString(form, "exchange_rate"),
      exchange_rate_date: formString(form, "exchange_rate_date"),
      exchange_rate_source: formString(form, "exchange_rate_source"),
      source_reference: optionalFormString(form, "source_reference"),
      associated_purchase_voucher_id: needsAssociation
        ? associatedPurchaseID
        : undefined,
      lines: [
        {
          description: formString(form, "description"),
          quantity: "1",
          unit_of_measure: formString(form, "unit_of_measure"),
          unit_price: net,
          discount_amount: "0",
          tax_treatment: taxTreatment,
          vat_rate: vatRate,
          net_amount: net,
          vat_amount: vat,
          total_amount: lineTotal,
          inventory: form.get("inventory") === "on",
        },
      ],
      taxes,
    };

    setBusy(true);
    setError(undefined);
    try {
      await api.request<PurchaseVoucher>("/api/v1/fiscal/purchase-vouchers", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("fiscal-purchase") },
        body: JSON.stringify(body),
      });
      onCreated();
    } catch (cause) {
      setError(message(cause, "No pudimos registrar el comprobante de compra."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form
      className="finance-form fiscal-purchase-form"
      onSubmit={(event) => void submit(event)}
    >
      <h2>Registrar comprobante de proveedor</h2>
      <label>
        Documento de compra
        <input name="source_id" placeholder="UUID de la compra" required />
      </label>
      <label>
        Proveedor
        <input name="supplier_id" placeholder="UUID del proveedor" required />
      </label>
      <label>
        Razón social
        <input name="supplier_name" required />
      </label>
      <label>
        CUIT proveedor
        <input inputMode="numeric" name="supplier_tax_id" pattern="[0-9]{11}" required />
      </label>
      <label>
        Ambiente
        <select
          name="environment"
          value={environment}
          onChange={(event) =>
            setEnvironment(event.target.value as Settings["environment"])
          }
        >
          <option value="homologation">Homologación</option>
          <option value="production">Producción</option>
        </select>
      </label>
      <label>
        Comprobante
        <select
          name="voucher_type"
          value={voucherType}
          onChange={(event) => setVoucherType(event.target.value)}
        >
          <option value="1">Factura A</option>
          <option value="2">Nota de débito A</option>
          <option value="3">Nota de crédito A</option>
          <option value="6">Factura B</option>
          <option value="7">Nota de débito B</option>
          <option value="8">Nota de crédito B</option>
          <option value="11">Factura C</option>
          <option value="12">Nota de débito C</option>
          <option value="13">Nota de crédito C</option>
        </select>
      </label>
      {needsAssociation ? (
        <div className="fiscal-purchase-association finance-form__wide">
          <label>
            Buscar comprobante original
            <input
              placeholder="Proveedor, CUIT o número"
              type="search"
              value={associationQuery}
              onChange={(event) => setAssociationQuery(event.target.value)}
            />
          </label>
          <label>
            Comprobante original
            <select
              name="associated_purchase_voucher_id"
              required
              value={associatedPurchaseID}
              onChange={(event) => setAssociatedPurchaseID(event.target.value)}
            >
              <option value="">Seleccionar factura asociada</option>
              {eligibleOriginals.map((purchase) => (
                <option key={purchase.id} value={purchase.id}>
                  {purchase.supplier_name} ·{" "}
                  {String(purchase.point_of_sale).padStart(5, "0")}-
                  {String(purchase.voucher_number).padStart(8, "0")}
                </option>
              ))}
            </select>
          </label>
          {eligibleOriginals.length === 0 ? (
            <small>
              No hay una factura compatible cargada en este período y ambiente.
            </small>
          ) : null}
        </div>
      ) : null}
      <label>
        Punto de venta
        <input max="99999" min="1" name="point_of_sale" required type="number" />
      </label>
      <label>
        Número
        <input min="1" name="voucher_number" required type="number" />
      </label>
      <label>
        Emisión
        <input defaultValue={today} name="issue_date" required type="date" />
      </label>
      <label>
        Vencimiento
        <input name="due_date" type="date" />
      </label>
      <label className="finance-form__wide">
        Referencia
        <input name="source_reference" placeholder="Orden, remito o referencia interna" />
      </label>
      <label>
        Moneda
        <select value={currency} onChange={(event) => setCurrency(event.target.value)}>
          <option>ARS</option>
          <option>USD</option>
          <option>EUR</option>
        </select>
      </label>
      <label>
        Cotización
        <input
          defaultValue="1"
          min="0.000001"
          name="exchange_rate"
          required
          step="0.000001"
          type="number"
        />
      </label>
      <label>
        Fecha cotización
        <input defaultValue={today} name="exchange_rate_date" required type="date" />
      </label>
      <label>
        Fuente cotización
        <input defaultValue="ARCA" name="exchange_rate_source" required />
      </label>
      <label className="finance-form__wide">
        Detalle
        <input name="description" required />
      </label>
      <label>
        Unidad
        <input defaultValue="unidad" name="unit_of_measure" required />
      </label>
      <label>
        Tratamiento
        <select
          value={taxTreatment}
          onChange={(event) =>
            changeTaxTreatment(
              event.target.value as components["schemas"]["FiscalPurchaseLineInput"]["tax_treatment"],
            )
          }
        >
          <option value="taxable">Gravado</option>
          <option value="exempt">Exento</option>
          <option value="non_taxed">No gravado</option>
        </select>
      </label>
      <label>
        Neto
        <input
          min="0.01"
          name="net_amount"
          required
          step="0.01"
          type="number"
          value={net}
          onChange={(event) => setNet(event.target.value)}
        />
      </label>
      <label>
        Alícuota IVA
        <select disabled={taxTreatment !== "taxable"} name="vat_rate" defaultValue="21">
          <option value="21">21%</option>
          <option value="10.5">10,5%</option>
          <option value="27">27%</option>
          <option value="5">5%</option>
          <option value="2.5">2,5%</option>
          <option value="0">0%</option>
        </select>
      </label>
      <label>
        IVA
        <input
          disabled={taxTreatment !== "taxable"}
          min="0"
          name="vat_amount"
          required
          step="0.01"
          type="number"
          value={vat}
          onChange={(event) => setVAT(event.target.value)}
        />
      </label>
      <label className="finance-check">
        <input name="inventory" type="checkbox" />
        Mercadería inventariable
      </label>
      <label>
        Tributo adicional
        <select
          value={additionalKind}
          onChange={(event) =>
            setAdditionalKind(
              event.target.value as "none" | PurchaseTaxInput["kind"],
            )
          }
        >
          <option value="none">Sin tributo</option>
          <option value="other_tax">Otro tributo</option>
          <option value="perception">Percepción</option>
          <option value="withholding">Retención</option>
        </select>
      </label>
      <label>
        Código
        <input
          defaultValue="99"
          disabled={additionalKind === "none"}
          name="additional_authority_code"
          required={additionalKind !== "none"}
        />
      </label>
      <label>
        Descripción
        <input
          defaultValue="Otro tributo"
          disabled={additionalKind === "none"}
          name="additional_description"
          required={additionalKind !== "none"}
        />
      </label>
      <label>
        Jurisdicción
        <input
          disabled={additionalKind === "none"}
          name="additional_jurisdiction"
          placeholder="AR-B"
        />
      </label>
      <label>
        Alícuota
        <input
          defaultValue="0"
          disabled={additionalKind === "none"}
          min="0"
          name="additional_rate"
          required={additionalKind !== "none"}
          step="0.000001"
          type="number"
        />
      </label>
      <label>
        Importe tributo
        <input
          disabled={additionalKind === "none"}
          min="0"
          name="additional_amount"
          required={additionalKind !== "none"}
          step="0.01"
          type="number"
          value={additionalTax}
          onChange={(event) => setAdditionalTax(event.target.value)}
        />
      </label>
      <FiscalTotalRail
        currency={currency}
        net={net}
        taxLabel="IVA"
        tax={vat}
        additionalLabel={
          additionalKind === "none"
            ? undefined
            : additionalKind === "withholding"
              ? "Retención (no suma)"
              : "Otros tributos"
        }
        additional={additionalKind === "none" ? undefined : additionalTax}
        additionalOperator={additionalKind === "withholding" ? "·" : "＋"}
        total={voucherTotal}
      />
      <button className="directory-create-button" disabled={busy} type="submit">
        {busy ? "Registrando…" : "Registrar y contabilizar"}
      </button>
      {error ? (
        <span className="form-error" role="alert">
          {error}
        </span>
      ) : null}
    </form>
  );
}

function FiscalTotalRail({
  currency,
  net,
  taxLabel,
  tax,
  additionalLabel,
  additional,
  additionalOperator = "＋",
  total,
}: {
  currency: string;
  net: string;
  taxLabel: string;
  tax: string;
  additionalLabel?: string;
  additional?: string;
  additionalOperator?: "＋" | "·";
  total: string;
}) {
  return (
    <div className="fiscal-total-rail" aria-label="Totales exactos">
      <span>
        Neto <strong>{formatMoney(net, currency)}</strong>
      </span>
      <i aria-hidden="true">＋</i>
      <span>
        {taxLabel} <strong>{formatMoney(tax, currency)}</strong>
      </span>
      {additionalLabel && additional ? (
        <>
          <i aria-hidden="true">{additionalOperator}</i>
          <span>
            {additionalLabel} <strong>{formatMoney(additional, currency)}</strong>
          </span>
        </>
      ) : null}
      <i aria-hidden="true">＝</i>
      <span className="is-total">
        Total <strong>{formatMoney(total, currency)}</strong>
      </span>
    </div>
  );
}

function FiscalSettingsPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [settings, setSettings] = useState<Settings>();
  const [points, setPoints] = useState<PointOfSale[]>([]);
  const [environment, setEnvironment] =
    useState<Settings["environment"]>("homologation");
  const [error, setError] = useState<string>();
  const [saved, setSaved] = useState(false);
  const [loading, setLoading] = useState(true);
  const currentEnvironment = useRef(environment);
  currentEnvironment.current = environment;
  const load = useCallback(async (
    targetEnvironment: Settings["environment"],
    signal?: AbortSignal,
  ) => {
    setLoading(true);
    try {
      const configuredPoints = await api.request<PointOfSale[]>(
        "/api/v1/fiscal/points-of-sale",
        { signal, skipJSONContentType: true },
      );
      setPoints(configuredPoints);
      const response = await api.requestResponse(
        `/api/v1/fiscal/settings?environment=${targetEnvironment}`,
        { signal, skipJSONContentType: true },
      );
      if (signal?.aborted || currentEnvironment.current !== targetEnvironment) {
        return;
      }
      const value = (await response.json()) as Settings;
      if (value.environment !== targetEnvironment) {
        throw new Error("El perfil fiscal recibido pertenece a otro ambiente.");
      }
      setSettings(value);
      setError(undefined);
    } catch (cause) {
      if (signal?.aborted) return;
      if (isNotFound(cause)) {
        setSettings(undefined);
        setError(undefined);
        return;
      }
      setError(message(cause, "Configurá el perfil fiscal para comenzar."));
    } finally {
      if (!signal?.aborted && currentEnvironment.current === targetEnvironment) {
        setLoading(false);
      }
    }
  }, [api]);
  useEffect(() => {
    const controller = new AbortController();
    setSettings(undefined);
    setPoints([]);
    setSaved(false);
    setError(undefined);
    void load(environment, controller.signal);
    return () => controller.abort();
  }, [environment, load]);

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canManage) return;
    const form = new FormData(event.currentTarget);
    const targetEnvironment = environment;
    try {
      const value = await api.request<Settings>("/api/v1/fiscal/settings", {
        method: "PUT",
        headers: { "Idempotency-Key": createIdempotencyKey("fiscal-settings") },
        body: JSON.stringify({
          cuit: form.get("cuit"),
          legal_name: form.get("legal_name"),
          tax_address: form.get("tax_address"),
          tax_condition: form.get("tax_condition"),
          activity_start_date: form.get("activity_start_date"),
          environment: targetEnvironment,
          functional_currency: "ARS",
          version: settings?.version ?? 0,
        }),
      });
      if (
        currentEnvironment.current !== targetEnvironment ||
        value.environment !== targetEnvironment
      ) {
        return;
      }
      setSettings(value);
      setSaved(true);
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos guardar la configuración fiscal."));
    }
  }

  async function addPoint(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canManage) return;
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const targetEnvironment = environment;
    try {
      await api.request<PointOfSale>("/api/v1/fiscal/points-of-sale", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("fiscal-pos") },
        body: JSON.stringify({
          number: parseInteger(formString(form, "number")),
          name: form.get("name"),
          environment: targetEnvironment,
        }),
      });
      formElement.reset();
      if (currentEnvironment.current === targetEnvironment) {
        await load(targetEnvironment);
      }
    } catch (cause) {
      setError(message(cause, "No pudimos agregar el punto de venta."));
    }
  }

  async function enableProduction() {
    if (!canManage || environment !== "production" || !settings) return;
    try {
      const value = await api.request<Settings>(
        "/api/v1/fiscal/production/enable",
        {
          method: "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey("fiscal-production-enable"),
          },
          body: JSON.stringify({
            version: settings.version,
            reason: "Habilitación productiva confirmada desde la configuración fiscal",
          }),
        },
      );
      setSettings(value);
      setSaved(true);
      setError(undefined);
    } catch (cause) {
      setError(
        message(
          cause,
          "Producción sigue bloqueada: revisá homologación, certificado y punto de venta.",
        ),
      );
    }
  }

  return (
    <section className="directory-section fiscal-settings-grid">
      {!canManage ? (
        <div className="fiscal-readonly-note" role="status">
          Estás viendo la configuración fiscal en modo lectura.
        </div>
      ) : null}
      <div className="fiscal-readiness">
        <div>
          <span className={settings ? "is-ready" : ""} />
          Perfil fiscal
        </div>
        <div>
          <span className={settings?.certificate_expires_at ? "is-ready" : ""} />
          Certificado
        </div>
        <div>
          <span
            className={
              points.some(
                (point) => point.environment === environment && point.active,
              )
                ? "is-ready"
                : ""
            }
          />
          Punto de venta
        </div>
        <div>
          <span className={settings?.production_ready ? "is-ready" : ""} />
          Producción
        </div>
      </div>
      <form
        className="finance-form fiscal-profile-form"
        key={`profile-${environment}`}
        onSubmit={(event) => void save(event)}
      >
        <h2>Identidad fiscal</h2>
        <label>
          CUIT
          <input
            defaultValue={settings?.cuit}
            disabled={!canManage}
            key={`cuit-${settings?.cuit}`}
            name="cuit"
            pattern="[0-9]{11}"
            required
          />
        </label>
        <label>
          Razón social
          <input
            defaultValue={settings?.legal_name}
            disabled={!canManage}
            key={`name-${settings?.legal_name}`}
            name="legal_name"
            required
          />
        </label>
        <label className="finance-form__wide">
          Domicilio fiscal
          <input
            defaultValue={settings?.tax_address}
            disabled={!canManage}
            key={`address-${settings?.tax_address}`}
            name="tax_address"
            required
          />
        </label>
        <label>
          Condición IVA
          <select
            defaultValue={settings?.tax_condition ?? "registered"}
            disabled={!canManage}
            key={`condition-${settings?.tax_condition}`}
            name="tax_condition"
          >
            <option value="registered">Responsable inscripto</option>
            <option value="monotributo">Monotributo</option>
            <option value="exempt">Exento</option>
          </select>
        </label>
        <label>
          Inicio de actividades
          <input
            defaultValue={settings?.activity_start_date}
            disabled={!canManage}
            key={`activity-${settings?.activity_start_date}`}
            name="activity_start_date"
            required
            type="date"
          />
        </label>
        <label>
          Ambiente
          <select
            disabled={!canManage}
            name="environment"
            value={environment}
            onChange={(event) => {
              const next = event.target.value as Settings["environment"];
              if (next !== environment) {
                setSettings(undefined);
                setPoints([]);
                setError(undefined);
                setSaved(false);
                setEnvironment(next);
              }
            }}
          >
            <option value="homologation">Homologación</option>
            <option value="production">Producción</option>
          </select>
        </label>
        {canManage ? (
          <button className="directory-create-button" type="submit">
            Guardar perfil
          </button>
        ) : null}
        {saved ? <span className="form-success">Perfil fiscal guardado.</span> : null}
      </form>
      {canManage ? (
        <CertificateForm
          key={environment}
          onSaved={(savedEnvironment) => {
            if (currentEnvironment.current === savedEnvironment) {
              void load(savedEnvironment);
            }
          }}
          environment={environment}
        />
      ) : null}
      <form
        className="finance-form"
        key={`points-${environment}`}
        onSubmit={(event) => void addPoint(event)}
      >
        <h2>Puntos de venta</h2>
        {canManage ? (
          <>
            <label>
              Número
              <input min="1" name="number" required type="number" />
            </label>
            <label>
              Nombre
              <input name="name" placeholder="Casa central" />
            </label>
            <button className="directory-create-button" type="submit">
              Agregar
            </button>
          </>
        ) : null}
        <div className="fiscal-point-list">
          {points
            .filter((point) => point.environment === environment)
            .map((point) => (
              <span key={point.id}>
                {String(point.number).padStart(5, "0")} ·{" "}
                {point.name || point.environment}
              </span>
            ))}
          {!points.some((point) => point.environment === environment) ? (
            <span>Sin puntos de venta configurados</span>
          ) : null}
        </div>
      </form>
      {canManage && environment === "production" && settings ? (
        <div className="fiscal-production-gate">
          <div>
            <strong>
              {settings.production_ready
                ? "Producción habilitada"
                : "Producción bloqueada"}
            </strong>
            <small>
              Exige homologación técnica vigente, certificado productivo y
              punto de venta activo.
            </small>
          </div>
          {!settings.production_ready ? (
            <button
              className="directory-create-button"
              onClick={() => void enableProduction()}
              type="button"
            >
              Habilitar producción
            </button>
          ) : null}
        </div>
      ) : null}
      <InlineFeedback error={error} loading={loading} />
    </section>
  );
}

function CertificateForm({
  environment,
  onSaved,
}: {
  environment: Settings["environment"];
  onSaved: (environment: Settings["environment"]) => void;
}) {
  const api = useProductApi();
  const [certificate, setCertificate] = useState<Certificate>();
  const [error, setError] = useState<string>();

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      const value = await api.request<Certificate>("/api/v1/fiscal/certificates", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("fiscal-cert") },
        body: JSON.stringify({
          environment,
          certificate_pem: form.get("certificate_pem"),
          private_key_pem: form.get("private_key_pem"),
        }),
      });
      formElement.reset();
      setCertificate(value);
      setError(undefined);
      onSaved(environment);
    } catch (cause) {
      setError(message(cause, "El certificado o la clave no son válidos."));
    }
  }

  return (
    <form
      className="finance-form fiscal-certificate-form"
      onSubmit={(event) => void submit(event)}
    >
      <h2>Certificado ARCA</h2>
      <label className="finance-form__wide">
        Certificado PEM
        <textarea name="certificate_pem" required rows={4} />
      </label>
      <label className="finance-form__wide">
        Clave privada PEM
        <textarea autoComplete="off" name="private_key_pem" required rows={4} />
      </label>
      <button className="directory-create-button" type="submit">
        Validar y guardar
      </button>
      {certificate ? (
        <span className="form-success">
          Válido hasta {formatDateTime(certificate.expires_at)} ·{" "}
          {certificate.fingerprint.slice(0, 12)}
        </span>
      ) : null}
      {error ? (
        <span className="form-error" role="alert">
          {error}
        </span>
      ) : null}
    </form>
  );
}

function IVAPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [period, setPeriod] = useState(currentPeriod());
  const [environment, setEnvironment] =
    useState<Settings["environment"]>("production");
  const [report, setReport] = useState<IVASimple>();
  const [workflow, setWorkflow] = useState<IVAWorkflow>();
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async (signal?: AbortSignal) => {
    setLoading(true);
    try {
      const preview = await api.request<IVASimple>(
        `/api/v1/fiscal/iva-simple/${period}?environment=${environment}`,
        { signal, skipJSONContentType: true },
      );
      const persistedResponse = await api.requestResponse(
        `/api/v1/fiscal/iva-simple/${period}/workflow?environment=${environment}`,
        { signal, skipJSONContentType: true },
      );
      if (signal?.aborted) return;
      if (!persistedResponse.ok && persistedResponse.status !== 404) {
        throw new Error("No pudimos consultar el cierre de IVA.");
      }
      const persisted =
        persistedResponse.status === 404
          ? undefined
          : ((await persistedResponse.json()) as IVAWorkflow);
      setWorkflow(persisted);
      setReport(
        persisted
          ? {
              ...persisted.report,
              sales_file: preview.sales_file,
              purchases_file: preview.purchases_file,
            }
          : preview,
      );
      setError(undefined);
    } catch (cause) {
      if (signal?.aborted) return;
      setWorkflow(undefined);
      setReport(undefined);
      setError(message(cause, "No pudimos preparar IVA Simple."));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }, [api, environment, period]);

  useEffect(() => {
    const controller = new AbortController();
    setReport(undefined);
    setWorkflow(undefined);
    setError(undefined);
    setReason("");
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  function downloadPreview(kind: "sales" | "purchases") {
    const encoded =
      kind === "sales" ? report?.sales_file : report?.purchases_file;
    if (!encoded) return;
    try {
      downloadBase64File(
        encoded,
        `iva-simple-${period}-${kind === "sales" ? "ventas" : "compras"}-${environment}.zip`,
        "application/zip",
      );
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos preparar el archivo para descargar."));
    }
  }

  async function prepareWorkflow() {
    if (!canManage) return;
    setSaving(true);
    try {
      const value = await api.request<IVAWorkflow>(
        `/api/v1/fiscal/iva-simple/${period}/prepare?environment=${environment}`,
        {
          method: "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey("iva-simple-prepare"),
          },
          body: JSON.stringify(
            workflow ? { version: workflow.version } : {},
          ),
        },
      );
      setWorkflow(value);
      setReport((current) => ({
        ...value.report,
        sales_file: current?.sales_file,
        purchases_file: current?.purchases_file,
      }));
      setError(undefined);
    } catch (cause) {
      setError(
        message(
          cause,
          "No pudimos tomar el snapshot de comprobantes y compras.",
        ),
      );
    } finally {
      setSaving(false);
    }
  }

  async function transition(action: "close" | "export" | "reopen") {
    if (!canManage || !workflow) return;
    const normalizedReason = reason.trim();
    if (!normalizedReason) {
      setError("Indicá el motivo para dejar trazabilidad del cambio.");
      return;
    }
    setSaving(true);
    try {
      if (action === "export") {
        const artifact = await api.request<IVAExport>(
          `/api/v1/fiscal/iva-simple/${period}/export?environment=${environment}`,
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey("iva-simple-export"),
            },
            body: JSON.stringify({
              version: workflow.version,
              reason: normalizedReason,
            }),
          },
        );
        downloadBase64File(
          artifact.content_base64,
          artifact.filename,
          artifact.media_type,
        );
      } else {
        await api.request<IVAWorkflow>(
          `/api/v1/fiscal/iva-simple/${period}/${action}?environment=${environment}`,
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey(
                `iva-simple-${action}`,
              ),
            },
            body: JSON.stringify({
              version: workflow.version,
              reason: normalizedReason,
            }),
          },
        );
      }
      setReason("");
      await load();
    } catch (cause) {
      setError(
        message(
          cause,
          action === "close"
            ? "El período no concilia con comprobantes y asientos."
            : "No pudimos completar la transición de IVA Simple.",
        ),
      );
    } finally {
      setSaving(false);
    }
  }

  async function downloadPersisted(item: IVAWorkflow["exports"][number]) {
    try {
      const artifact = await api.request<IVAExport>(
        `/api/v1/fiscal/iva-simple/${period}/exports/${item.id}?environment=${environment}`,
        { skipJSONContentType: true },
      );
      downloadBase64File(
        artifact.content_base64,
        artifact.filename,
        artifact.media_type,
      );
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos descargar la exportación guardada."));
    }
  }

  return (
    <section className="directory-section">
      <div className="finance-toolbar">
        <label className="finance-select">
          Período
          <input
            type="month"
            value={period}
            onChange={(event) => setPeriod(event.target.value)}
          />
        </label>
        <label className="finance-select">
          Ambiente
          <select
            value={environment}
            onChange={(event) =>
              setEnvironment(event.target.value as Settings["environment"])
            }
          >
            <option value="production">Producción</option>
            <option value="homologation">Homologación</option>
          </select>
        </label>
        <div className="iva-workflow-state">
          <span>Estado</span>
          <strong>
            {workflow
              ? ivaWorkflowStatusLabel(workflow.status)
              : "Sin preparar"}
          </strong>
        </div>
        <div className="iva-balance">
          <span>Saldo del período</span>
          <strong>{formatMoney(report?.balance ?? "0", "ARS")}</strong>
        </div>
      </div>
      <InlineFeedback error={error} loading={loading} />
      {report ? (
        <>
          <div className="iva-grid">
            <Metric label="Ventas netas" value={report.sales_net} />
            <Metric label="IVA débito" value={report.output_vat} />
            <Metric label="Compras netas" value={report.purchases_net} />
            <Metric label="IVA crédito" value={report.input_vat} />
            <Metric label="Retenciones" value={report.withholdings} />
            <Metric label="Percepciones" value={report.perceptions} />
          </div>
          {report.validation_errors.length ? (
            <div className="fiscal-warning">
              <strong>Revisiones pendientes</strong>
              {report.validation_errors.map((value) => (
                <span key={value}>{value}</span>
              ))}
            </div>
          ) : (
            <div className="fiscal-ready">
              <strong>Período consistente</strong>
              <span>
                El snapshot fiscal está consistente. El cierre valida además
                los vínculos y las cuentas contables de IVA.
              </span>
            </div>
          )}
          <div className="iva-workflow">
            <div className="iva-workflow__copy">
              <strong>Cierre y exportación auditables</strong>
              <small>
                Preparar toma un snapshot. Cerrar exige conciliación contable y
                exportar conserva un ZIP inmutable con hash.
              </small>
            </div>
            {canManage ? (
              <>
                <button
                  disabled={saving || (workflow?.status ?? "draft") !== "draft"}
                  onClick={() => void prepareWorkflow()}
                  type="button"
                >
                  {workflow ? "Actualizar snapshot" : "Preparar período"}
                </button>
                {workflow ? (
                  <label className="iva-workflow__reason">
                    Motivo
                    <input
                      maxLength={500}
                      onChange={(event) => setReason(event.target.value)}
                      placeholder="Ej.: cierre mensual revisado"
                      value={reason}
                    />
                  </label>
                ) : null}
                {workflow?.status === "draft" ? (
                  <button
                    disabled={saving}
                    onClick={() => void transition("close")}
                    type="button"
                  >
                    Cerrar
                  </button>
                ) : null}
                {workflow?.status === "closed" ? (
                  <button
                    disabled={saving}
                    onClick={() => void transition("export")}
                    type="button"
                  >
                    Exportar ZIP
                  </button>
                ) : null}
                {workflow &&
                (workflow.status === "closed" ||
                  workflow.status === "exported") ? (
                  <button
                    className="button-secondary"
                    disabled={saving}
                    onClick={() => void transition("reopen")}
                    type="button"
                  >
                    Reabrir
                  </button>
                ) : null}
              </>
            ) : (
              <span className="fiscal-readonly-note">
                Sólo lectura. La gestión requiere permiso fiscal.
              </span>
            )}
          </div>
          <div className="iva-downloads">
            <div>
              <strong>Vista previa</strong>
              <small>
                Estos archivos no quedan cerrados hasta exportar el período.
              </small>
            </div>
            <button
              disabled={!report.sales_file}
              onClick={() => downloadPreview("sales")}
              type="button"
            >
              Vista previa ventas
            </button>
            <button
              disabled={!report.purchases_file}
              onClick={() => downloadPreview("purchases")}
              type="button"
            >
              Vista previa compras
            </button>
          </div>
          {workflow?.exports.length ? (
            <div className="iva-export-history">
              <strong>Exportaciones inmutables</strong>
              {workflow.exports.map((item) => (
                <button
                  key={item.id}
                  onClick={() => void downloadPersisted(item)}
                  type="button"
                >
                  {item.filename} · v{item.export_version} ·{" "}
                  {item.sha256.slice(0, 12)}
                </button>
              ))}
            </div>
          ) : null}
        </>
      ) : null}
    </section>
  );
}

function HomologationPanel({
  canManage,
  organizationId,
}: {
  canManage: boolean;
  organizationId?: string;
}) {
  const api = useProductApi();
  const [run, setRun] = useState<HomologationRun>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [showGuide, setShowGuide] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await api.requestResponse(
        "/api/v1/fiscal/homologation/latest",
      );
      if (response.status === 404) {
        setRun(undefined);
        setError(undefined);
        return;
      }
      if (!response.ok) throw new Error("No pudimos consultar la homologación.");
      setRun((await response.json()) as HomologationRun);
      setError(undefined);
    } catch (cause) {
      setError(message(cause, "No pudimos consultar la evidencia técnica."));
    } finally {
      setLoading(false);
    }
  }, [api]);

  useEffect(() => void load(), [load]);

  return (
    <section className="directory-section">
      <div className="homologation-card">
        <header>
          <div>
            <small>Puerta de producción</small>
            <h2>Verificar antes de habilitar</h2>
          </div>
          <span
            className={`status-pill status-pill--${
              run?.status === "succeeded"
                ? "authorized"
                : run?.status === "failed"
                  ? "rejected"
                  : "pending"
            }`}
          >
            {run?.status === "succeeded"
              ? "Evidencia completa"
              : run?.status === "failed"
                ? "Con fallas"
                : "Pendiente"}
          </span>
        </header>
        <InlineFeedback error={error} loading={loading} />
        <ol>
          <li>
            <span>1</span>
            <div>
              <strong>Identidad y certificado</strong>
              <small>CUIT, par criptográfico y vigencia.</small>
            </div>
          </li>
          <li>
            <span>2</span>
            <div>
              <strong>Conexión WSAA</strong>
              <small>Ticket separado para homologación.</small>
            </div>
          </li>
          <li>
            <span>3</span>
            <div>
              <strong>Interoperabilidad WSFEv1</strong>
              <small>
                Consultas de numeración y matriz local A/B/C, notas, servicios y
                multimoneda.
              </small>
            </div>
          </li>
          <li>
            <span>4</span>
            <div>
              <strong>Artefactos locales</strong>
              <small>
                QR y PDF se validan con datos de prueba; esta ejecución no solicita
                CAE.
              </small>
            </div>
          </li>
        </ol>
        {run ? (
          <div className="homologation-evidence" role="status">
            <strong>
              {run.success_count}/{run.check_count} verificaciones correctas
            </strong>
            <span>
              {formatDateTime(run.completed_at)} · SHA-256{" "}
              <code>{run.evidence_sha256.slice(0, 16)}…</code>
            </span>
            <small>{run.evidence_note}</small>
            <details>
              <summary>Ver verificaciones</summary>
              <ul>
                {run.checks.map((check) => (
                  <li key={`${check.ordinal}-${check.name}`}>
                    <span
                      className={`status-pill status-pill--${
                        check.status === "succeeded" ? "authorized" : "rejected"
                      }`}
                    >
                      {check.status === "succeeded" ? "OK" : "Falló"}
                    </span>
                    <span>
                      <strong>{check.name}</strong>
                      <small>{check.detail}</small>
                    </span>
                  </li>
                ))}
              </ul>
            </details>
          </div>
        ) : !loading ? (
          <div className="fiscal-warning">
            <strong>Sin evidencia registrada</strong>
            <span>La ejecución es manual, explícita y no emite comprobantes.</span>
          </div>
        ) : null}
        {showGuide ? (
          <div className="homologation-guide">
            <strong>Ejecución opt-in desde el entorno operativo</strong>
            <code>
              make fiscal-homologation ORG_ID=
              {organizationId ?? "&lt;uuid-organización&gt;"}
            </code>
            <small>
              Sólo usa WSAA, consultas de última numeración y pruebas locales.
              Nunca llama a FECAESolicitar.
            </small>
          </div>
        ) : null}
        <footer>
          {canManage ? (
            <button
              className="directory-create-button"
              onClick={() => setShowGuide((value) => !value)}
              type="button"
            >
              {showGuide ? "Ocultar guía" : "Cómo ejecutar"}
            </button>
          ) : null}
          <button onClick={() => void load()} type="button">
            Actualizar evidencia
          </button>
          <p>
            Producción permanece bloqueada hasta completar toda la evidencia y
            habilitarla explícitamente.
          </p>
        </footer>
      </div>
    </section>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article>
      <span>{label}</span>
      <strong>{formatMoney(value, "ARS")}</strong>
    </article>
  );
}

function InlineFeedback({ error, loading }: { error?: string; loading?: boolean }) {
  if (error) {
    return (
      <div className="inline-state inline-state--error" role="alert">
        {error}
      </div>
    );
  }
  if (loading) return <div className="inline-state">Cargando…</div>;
  return null;
}

function EmptyRow({ columns, text }: { columns: number; text: string }) {
  return (
    <tr>
      <td className="directory-empty" colSpan={columns}>
        <strong>{text}</strong>
        <span>La trazabilidad fiscal aparecerá acá.</span>
      </td>
    </tr>
  );
}

function fiscalState(value: "all" | Voucher["state"]) {
  return (
    {
      all: "Todos",
      queued: "En cola",
      processing: "Autorizando",
      authorized: "Autorizado",
      rejected: "Rechazado",
      uncertain: "A conciliar",
    } as const
  )[value];
}

function voucherKind(value?: Voucher["kind"]) {
  if (!value) return "Comprobante pendiente";
  return value
    .replaceAll("_", " ")
    .replace("invoice", "Factura")
    .replace("credit note", "Nota de crédito")
    .replace("debit note", "Nota de débito")
    .toUpperCase();
}

function purchaseVoucherKind(value: number) {
  return (
    {
      1: "Factura A",
      2: "Nota de débito A",
      3: "Nota de crédito A",
      6: "Factura B",
      7: "Nota de débito B",
      8: "Nota de crédito B",
      11: "Factura C",
      12: "Nota de débito C",
      13: "Nota de crédito C",
    } as Record<number, string>
  )[value] ?? `Tipo ${value}`;
}

const purchaseAdjustmentTypes = new Set(["2", "3", "7", "8", "12", "13"]);

function purchaseInvoiceType(value: string) {
  return (
    {
      "2": 1,
      "3": 1,
      "7": 6,
      "8": 6,
      "12": 11,
      "13": 11,
    } as Record<string, number>
  )[value];
}

type DecimalParts = {
  coefficient: bigint;
  scale: number;
};

function parseExactDecimal(value: string): DecimalParts | undefined {
  const normalized = value.trim().replace(",", ".");
  const match = /^([+-]?)(\d+)(?:\.(\d+))?$/.exec(normalized);
  if (!match) return undefined;
  const fraction = match[3] ?? "";
  const sign = match[1] === "-" ? -1n : 1n;
  return {
    coefficient: sign * BigInt(`${match[2]}${fraction}`),
    scale: fraction.length,
  };
}

function isNonNegativeDecimal(value: string) {
  const parsed = parseExactDecimal(value);
  return parsed !== undefined && parsed.coefficient >= 0n;
}

function decimalScale(value: string) {
  return parseExactDecimal(value)?.scale ?? Number.POSITIVE_INFINITY;
}

function findInvalidFiscalTribute(lines: EditableFiscalLine[]) {
  for (const [lineIndex, line] of lines.entries()) {
    for (const [tributeIndex, tribute] of line.tributes.entries()) {
      const authorityCode = tribute.authorityCode.trim();
      const parsedAuthorityCode = Number.parseInt(authorityCode, 10);
      const decimals = [
        tribute.taxableBase,
        tribute.rate,
        tribute.amount,
      ];
      if (
        !/^\d{1,2}$/.test(authorityCode) ||
        parsedAuthorityCode < 1 ||
        parsedAuthorityCode > 99 ||
        !tribute.description.trim() ||
        decimals.some(
          (value) =>
            !isNonNegativeDecimal(value) || decimalScale(value) > 6,
        )
      ) {
        return { line: lineIndex + 1, tribute: tributeIndex + 1 };
      }
    }
  }
  return undefined;
}

function safeAddDecimals(...values: string[]) {
  if (values.length === 0) return "0.00";
  const parsed = values.map(parseExactDecimal);
  if (parsed.some((value) => !value)) return "0.00";
  const parts = parsed as DecimalParts[];
  const scale = Math.max(2, ...parts.map((value) => value.scale));
  const coefficient = parts.reduce(
    (sum, value) =>
      sum + value.coefficient * 10n ** BigInt(scale - value.scale),
    0n,
  );
  return renderExactDecimal({ coefficient, scale });
}

function safeMultiplyDecimals(left: string, right: string, scale: number) {
  const leftParts = parseExactDecimal(left);
  const rightParts = parseExactDecimal(right);
  if (!leftParts || !rightParts) return renderExactDecimal({ coefficient: 0n, scale });
  return quantizeExactDecimal(
    {
      coefficient: leftParts.coefficient * rightParts.coefficient,
      scale: leftParts.scale + rightParts.scale,
    },
    scale,
  );
}

function safePercentage(value: string, rate: string, scale: number) {
  const multiplied = safeMultiplyDecimals(value, rate, scale + 2);
  const parts = parseExactDecimal(multiplied);
  if (!parts) return renderExactDecimal({ coefficient: 0n, scale });
  return quantizeExactDecimal(
    { coefficient: parts.coefficient, scale: parts.scale + 2 },
    scale,
  );
}

function quantizeExactDecimal(value: DecimalParts, scale: number) {
  if (value.scale <= scale) {
    return renderExactDecimal({
      coefficient: value.coefficient * 10n ** BigInt(scale - value.scale),
      scale,
    });
  }
  const divisor = 10n ** BigInt(value.scale - scale);
  const negative = value.coefficient < 0n;
  const absolute = negative ? -value.coefficient : value.coefficient;
  let rounded = absolute / divisor;
  if ((absolute % divisor) * 2n >= divisor) rounded += 1n;
  return renderExactDecimal({
    coefficient: negative ? -rounded : rounded,
    scale,
  });
}

function renderExactDecimal(value: DecimalParts) {
  const negative = value.coefficient < 0n;
  const absolute = (negative ? -value.coefficient : value.coefficient)
    .toString()
    .padStart(value.scale + 1, "0");
  const integer =
    value.scale === 0 ? absolute : absolute.slice(0, absolute.length - value.scale);
  const fraction = value.scale === 0 ? "" : absolute.slice(-value.scale);
  return `${negative ? "-" : ""}${integer}${fraction ? `.${fraction}` : ""}`;
}

function isZeroDecimal(value: string) {
  return parseExactDecimal(value)?.coefficient === 0n;
}

function formatMoney(value: string, currency: string) {
  const parsed = parseExactDecimal(value);
  if (!parsed) return `${currency} ${value}`;
  const exact = renderExactDecimal({
    coefficient: parsed.coefficient * 10n ** BigInt(Math.max(0, 2 - parsed.scale)),
    scale: Math.max(2, parsed.scale),
  });
  const negative = exact.startsWith("-");
  const [rawInteger, fraction = ""] = (negative ? exact.slice(1) : exact).split(".");
  const grouped = rawInteger.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  return `${currency} ${negative ? "-" : ""}${grouped},${fraction}`;
}

function vatAuthorityCode(rate: string) {
  return (
    {
      "0": "3",
      "10.5": "4",
      "21": "5",
      "27": "6",
      "5": "8",
      "2.5": "9",
    } as Record<string, string>
  )[rate] ?? "99";
}

function environmentLabel(environment: Settings["environment"]) {
  return environment === "production" ? "Producción" : "Homologación";
}

function ivaWorkflowStatusLabel(status: IVAWorkflow["status"]) {
  return (
    {
      draft: "Borrador",
      closed: "Cerrado",
      exported: "Exportado",
    } as const
  )[status];
}

function taxTreatmentLabel(
  value: components["schemas"]["FiscalVoucherSnapshotLine"]["tax_treatment"],
) {
  return (
    {
      taxable: "Gravado",
      exempt: "Exento",
      non_taxed: "No gravado",
    } as const
  )[value];
}

function downloadBase64File(
  encoded: string,
  filename: string,
  contentType: string,
) {
  const binary = window.atob(encoded);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  const url = URL.createObjectURL(new Blob([bytes], { type: contentType }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

function formString(form: FormData, name: string) {
  return String(form.get(name) ?? "").trim();
}

function optionalFormString(form: FormData, name: string) {
  const value = formString(form, name);
  return value || undefined;
}

function parseInteger(value: string) {
  return Number.parseInt(value, 10);
}

function currentPeriod() {
  return calendarDate().slice(0, 7);
}

function formatDate(value: string) {
  const [year, month, day] = value.slice(0, 10).split("-");
  return year && month && day ? `${day}/${month}/${year}` : value;
}

function formatDateTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat("es-AR", { dateStyle: "medium" }).format(date);
}

function message(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}

function isNotFound(cause: unknown) {
  return cause instanceof HttpError && cause.status === 404;
}
