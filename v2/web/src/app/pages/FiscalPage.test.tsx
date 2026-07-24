import type { HttpClient } from "@devpablocristo/platform-http";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { beforeEach, expect, test, vi } from "vitest";
import type { AuthContextValue } from "../../auth/AuthContext";
import { AppProviders } from "../providers/AppProviders";
import { calendarDate } from "../calendarDate";
import { FiscalPage } from "./FiscalPage";

const organization = {
  id: "11111111-1111-4111-8111-111111111111",
  switchKey: "org_clerk_norte",
  name: "Comercio Norte",
  slug: "comercio-norte",
  role: "owner" as const,
};

function authValue(): AuthContextValue {
  return {
    status: "signed-in",
    sessionId: "sess_test",
    activeOrganizationId: organization.id,
    organizations: [organization],
    user: {
      id: "user_clerk_01",
      email: "ana@example.test",
      displayName: "Ana Pérez",
    },
    getToken: vi.fn(async () => "jwt"),
    setActiveOrganization: vi.fn(async () => undefined),
    signOut: vi.fn(async () => undefined),
    productRole: "owner",
  };
}

function session(permissions: string[]) {
  return {
    user: {
      id: "33333333-3333-4333-8333-333333333333",
      email: "ana@example.test",
      display_name: "Ana Pérez",
    },
    organization: {
      id: organization.id,
      name: organization.name,
      slug: organization.slug,
      status: "active",
      role: "owner",
      switch_key: organization.switchKey,
      sync_status: "synced",
    },
    membership: {
      id: "22222222-2222-4222-8222-222222222222",
      role: "owner",
      status: "active",
    },
    role: "owner",
    permissions,
    session_id: "sess_test",
  };
}

function renderFiscal(
  path: string,
  request: (path: string, options?: Record<string, unknown>) => Promise<unknown>,
  requestResponse: (
    path: string,
    options?: Record<string, unknown>,
  ) => Promise<Response> = async (responsePath, options) => {
    const value = await request(responsePath, options);
    return new Response(JSON.stringify(value), {
      headers: { "Content-Type": "application/json" },
      status: 200,
    });
  },
) {
  window.history.replaceState({}, "", path);
  const requestMock = vi.fn(request);
  const requestResponseMock = vi.fn(requestResponse);
  const apiClient = {
    request: requestMock,
    requestResponse: requestResponseMock,
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue()} apiClient={apiClient}>
      <Routes>
        <Route path="/fiscal/:section?" element={<FiscalPage />} />
      </Routes>
    </AppProviders>,
  );

  return { request: requestMock, requestResponse: requestResponseMock };
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("fiscal view-only keeps reads available and exposes no mutable controls", async () => {
  const { request } = renderFiscal("/fiscal/settings", async (path) => {
    if (path === "/api/v1/session") return session(["fiscal:view"]);
    if (path === "/api/v1/fiscal/settings?environment=homologation") {
      return {
        cuit: "30712345670",
        legal_name: "Comercio Norte SA",
        tax_address: "Av. Siempre Viva 123",
        tax_condition: "registered",
        activity_start_date: "2020-01-02",
        environment: "homologation",
        functional_currency: "ARS",
        version: 1,
        production_ready: false,
      };
    }
    if (path === "/api/v1/fiscal/points-of-sale") {
      return [
        {
          id: "44444444-4444-4444-8444-444444444444",
          number: 3,
          name: "Casa central",
          environment: "homologation",
          active: true,
        },
      ];
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(
    await screen.findByText("Estás viendo la configuración fiscal en modo lectura."),
  ).toBeInTheDocument();
  expect(await screen.findByDisplayValue("Comercio Norte SA")).toBeDisabled();
  expect(screen.getByText("00003 · Casa central")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Guardar perfil" })).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Validar y guardar" }),
  ).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Agregar" })).not.toBeInTheDocument();
  expect(
    request.mock.calls.filter(([, options]) => options && "method" in options),
  ).toHaveLength(0);
});

test("purchase directory filters, lists and registers one exact voucher", async () => {
  const user = userEvent.setup();
  const purchase = {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    environment: "homologation",
    supplier_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    supplier_tax_id: "30712345670",
    supplier_name: "Proveedor Federal SA",
    voucher_type: 1,
    point_of_sale: 12,
    voucher_number: 345,
    issue_date: "2026-07-22",
    currency: "ARS",
    exchange_rate: "1",
    net_amount: "100.00",
    exempt_amount: "0.00",
    non_taxed_amount: "0.00",
    vat_amount: "21.00",
    other_taxes_amount: "0.00",
    withholding_amount: "0.00",
    perception_amount: "0.00",
    total_amount: "121.00",
    version: 1,
    journal_entry_id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
    created_at: "2026-07-22T12:00:00Z",
  };

  const { request } = renderFiscal("/fiscal/purchases", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["fiscal:view", "fiscal:manage"]);
    }
    if (path.startsWith("/api/v1/fiscal/purchase-vouchers?")) {
      return { items: [purchase], page: { total: 1 } };
    }
    if (path === "/api/v1/fiscal/purchase-vouchers" && options?.method === "POST") {
      return purchase;
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(await screen.findByText("Proveedor Federal SA")).toBeInTheDocument();
  expect(screen.getByText("Contabilizado")).toBeInTheDocument();
  expect(
    request.mock.calls.some(([path]) =>
      String(path).includes("period="),
    ),
  ).toBe(true);

  await user.click(screen.getByRole("button", { name: /Registrar compra/ }));
  await user.type(
    screen.getByPlaceholderText("UUID de la compra"),
    "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
  );
  await user.type(
    screen.getByPlaceholderText("UUID del proveedor"),
    "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
  );
  await user.type(screen.getByLabelText("Razón social"), "Proveedor Federal SA");
  await user.type(screen.getByLabelText("CUIT proveedor"), "30712345670");
  await user.selectOptions(screen.getByLabelText("Comprobante"), "3");
  await user.selectOptions(
    screen.getByLabelText("Comprobante original"),
    purchase.id,
  );
  await user.type(screen.getByLabelText("Punto de venta"), "12");
  await user.type(screen.getByLabelText("Número"), "346");
  await user.type(screen.getByLabelText("Detalle"), "Mercadería para reventa");
  fireEvent.change(screen.getByLabelText("Neto"), { target: { value: "100.00" } });
  fireEvent.change(screen.getByLabelText("IVA"), { target: { value: "21.00" } });

  expect(screen.getAllByText("ARS 121,00")).toHaveLength(2);
  await user.click(
    screen.getByRole("button", { name: "Registrar y contabilizar" }),
  );

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/purchase-vouchers" &&
          options?.method === "POST",
      ),
    ).toBe(true);
  });
  const post = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/purchase-vouchers" && options?.method === "POST",
  );
  const body = JSON.parse(String(post?.[1]?.body));
  expect(body.lines).toEqual([
    expect.objectContaining({
      quantity: "1",
      unit_price: "100.00",
      net_amount: "100.00",
      vat_amount: "21.00",
      total_amount: "121.00",
    }),
  ]);
  expect(body.taxes).toEqual([
    expect.objectContaining({
      kind: "vat",
      authority_code: "5",
      taxable_base: "100.00",
      rate: "21",
      amount: "21.00",
      creditable: true,
    }),
  ]);
  expect(body.associated_purchase_voucher_id).toBe(purchase.id);
});

test("sale preview and payload preserve decimal strings beyond IEEE-754 precision", async () => {
  const user = userEvent.setup();
  const point = {
    id: "44444444-4444-4444-8444-444444444444",
    number: 3,
    name: "Casa central",
    environment: "homologation",
    active: true,
  };
  const { request } = renderFiscal("/fiscal/vouchers", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["fiscal:view", "fiscal:manage"]);
    }
    if (path.startsWith("/api/v1/fiscal/vouchers?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path === "/api/v1/fiscal/points-of-sale") return [point];
    if (path === "/api/v1/fiscal/settings?environment=homologation") {
      return {
        cuit: "30712345670",
        legal_name: "Comercio Norte SA",
        tax_address: "Av. Siempre Viva 123",
        tax_condition: "registered",
        activity_start_date: "2020-01-02",
        environment: "homologation",
        functional_currency: "ARS",
        country_code: "AR",
        version: 1,
        production_ready: false,
      };
    }
    if (path === "/api/v1/fiscal/vouchers" && options?.method === "POST") {
      return {
        id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        environment: "homologation",
        source_type: "sale",
        source_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
        state: "queued",
        currency: "ARS",
        total: "10898711098236601.54",
        version: 1,
        created_at: "2026-07-24T12:00:00Z",
        updated_at: "2026-07-24T12:00:00Z",
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await screen.findByText("Todavía no hay comprobantes.");
  await user.click(screen.getByRole("button", { name: /Emitir/ }));
  await user.type(
    screen.getByPlaceholderText("UUID de la venta"),
    "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
  );
  await user.selectOptions(screen.getByLabelText("Punto de venta"), point.id);
  await user.type(screen.getByLabelText("Receptor"), "Cliente Exacto SA");
  await user.type(screen.getByLabelText("Número"), "30712345670");
  await user.type(screen.getByLabelText("Descripción línea 1"), "Servicio exacto");
  fireEvent.change(screen.getByLabelText("Precio neto línea 1"), {
    target: { value: "9007199254740993.01" },
  });

  expect(screen.getAllByText("ARS 10.898.711.098.236.601,54")).not.toHaveLength(0);
  await user.click(screen.getByRole("button", { name: "Solicitar CAE" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
      ),
    ).toBe(true);
  });
  const post = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
  );
  const body = JSON.parse(String(post?.[1]?.body));
  expect(body.lines[0].unit_price).toBe("9007199254740993.01");
  expect(body.lines[0].subtotal).toBe("9007199254740993.01");
  expect(body.lines[0].taxes[0].amount).toBe("1891511843495608.53");
  expect(body.lines[0].cost_confirmed).toBe(false);
  expect(body.lines[0]).not.toHaveProperty("cost_amount");
  expect(body.sale_condition).toBe("cash");
  expect(body.payment_method).toBe("cash");
  expect(body).not.toHaveProperty("party_id");
  expect(body).not.toHaveProperty("accounting_due_date");
  expect(body.environment).toBe("homologation");
});

test("credit sale resets customer-only fields and sends confirmed cost plus identified tributes exactly", async () => {
  const user = userEvent.setup();
  const point = {
    id: "44444444-4444-4444-8444-444444444444",
    number: 3,
    name: "Casa central",
    environment: "homologation",
    active: true,
  };
  const customerID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const saleID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
  const dueDate = calendarDate();
  const { request } = renderFiscal("/fiscal/vouchers", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["fiscal:view", "fiscal:manage"]);
    }
    if (path.startsWith("/api/v1/fiscal/vouchers?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path === "/api/v1/fiscal/points-of-sale") return [point];
    if (path === "/api/v1/fiscal/settings?environment=homologation") {
      return {
        cuit: "30712345670",
        legal_name: "Comercio Norte SA",
        tax_address: "Av. Siempre Viva 123",
        tax_condition: "registered",
        activity_start_date: "2020-01-02",
        environment: "homologation",
        functional_currency: "ARS",
        country_code: "AR",
        version: 1,
        production_ready: false,
      };
    }
    if (path === "/api/v1/fiscal/vouchers" && options?.method === "POST") {
      return {
        id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        environment: "homologation",
        source_type: "sale",
        source_id: saleID,
        state: "queued",
        currency: "ARS",
        total: "124.00",
        version: 1,
        created_at: "2026-07-24T12:00:00Z",
        updated_at: "2026-07-24T12:00:00Z",
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await screen.findByText("Todavía no hay comprobantes.");
  await user.click(screen.getByRole("button", { name: /Emitir/ }));
  await user.type(screen.getByPlaceholderText("UUID de la venta"), saleID);
  await user.selectOptions(screen.getByLabelText("Punto de venta"), point.id);
  await user.type(screen.getByLabelText("Receptor"), "Cliente Federal SA");
  await user.type(screen.getByLabelText("Número"), "30712345670");

  await user.selectOptions(screen.getByLabelText("Condición de venta"), "credit");
  expect(screen.getByLabelText("Cliente contable")).toBeRequired();
  expect(screen.getByLabelText("Vencimiento contable")).toBeRequired();
  const issueForm = screen
    .getByRole("button", { name: "Solicitar CAE" })
    .closest("form");
  expect(issueForm).not.toBeNull();
  fireEvent.submit(issueForm!);
  expect(
    await screen.findByText(
      "Las ventas a crédito requieren un cliente contable y su vencimiento.",
    ),
  ).toHaveAttribute("role", "alert");
  expect(
    request.mock.calls.some(
      ([path, options]) =>
        path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
    ),
  ).toBe(false);
  await user.type(screen.getByLabelText("Cliente contable"), customerID);
  fireEvent.change(screen.getByLabelText("Vencimiento contable"), {
    target: { value: dueDate },
  });
  await user.selectOptions(screen.getByLabelText("Condición de venta"), "cash");
  expect(screen.queryByLabelText("Cliente contable")).not.toBeInTheDocument();
  await user.selectOptions(screen.getByLabelText("Condición de venta"), "credit");
  expect(screen.getByLabelText("Cliente contable")).toHaveValue("");
  expect(screen.getByLabelText("Vencimiento contable")).toHaveValue("");
  await user.type(screen.getByLabelText("Cliente contable"), customerID);
  fireEvent.change(screen.getByLabelText("Vencimiento contable"), {
    target: { value: dueDate },
  });
  await user.selectOptions(
    screen.getByLabelText("Medio de cobro"),
    "bank_transfer",
  );

  await user.type(screen.getByLabelText("Descripción línea 1"), "Producto");
  fireEvent.change(screen.getByLabelText("Precio neto línea 1"), {
    target: { value: "100.00" },
  });
  await user.click(screen.getByLabelText("Costo confirmado línea 1"));
  fireEvent.change(screen.getByLabelText("Costo total línea 1"), {
    target: { value: "60.123456" },
  });
  await user.click(screen.getByRole("button", { name: /Agregar tributo/ }));
  fireEvent.change(screen.getByLabelText("Código ARCA tributo línea 1.1"), {
    target: { value: "99" },
  });
  await user.type(
    screen.getByLabelText("Descripción tributo línea 1.1"),
    "Tasa municipal",
  );
  fireEvent.change(screen.getByLabelText("Base tributo línea 1.1"), {
    target: { value: "100.00" },
  });
  fireEvent.change(screen.getByLabelText("Alícuota tributo línea 1.1"), {
    target: { value: "3" },
  });
  fireEvent.change(screen.getByLabelText("Importe tributo línea 1.1"), {
    target: { value: "3.00" },
  });

  expect(screen.getAllByText("ARS 124,00")).not.toHaveLength(0);
  await user.click(screen.getByRole("button", { name: "Solicitar CAE" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
      ),
    ).toBe(true);
  });
  const post = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
  );
  const body = JSON.parse(String(post?.[1]?.body));
  expect(body).toEqual({
    environment: "homologation",
    source_type: "sale",
    source_id: saleID,
    point_of_sale_id: point.id,
    concept: "products",
    receiver_name: "Cliente Federal SA",
    receiver_document_type: "CUIT",
    receiver_document_number: "30712345670",
    receiver_tax_condition: "responsable_inscripto",
    sale_condition: "credit",
    party_id: customerID,
    payment_method: "bank_transfer",
    accounting_due_date: dueDate,
    currency: "ARS",
    exchange_rate: "1",
    exchange_rate_source: "ARCA",
    lines: [
      {
        description: "Producto",
        quantity: "1",
        unit_price: "100.00",
        subtotal: "100.00",
        cost_amount: "60.123456",
        cost_confirmed: true,
        taxes: [
          {
            kind: "vat",
            rate: "21",
            taxable_base: "100.00",
            amount: "21.00",
          },
          {
            kind: "other_tax",
            authority_code: 99,
            description: "Tasa municipal",
            taxable_base: "100.00",
            rate: "3",
            amount: "3.00",
          },
        ],
      },
    ],
  });
});

test("type C services use final prices, mandatory dates and multiple undiscriminated lines", async () => {
  const user = userEvent.setup();
  const point = {
    id: "44444444-4444-4444-8444-444444444444",
    number: 3,
    name: "Casa central",
    environment: "homologation",
    active: true,
  };
  const { request } = renderFiscal("/fiscal/vouchers", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["fiscal:view", "fiscal:manage"]);
    }
    if (path.startsWith("/api/v1/fiscal/vouchers?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path === "/api/v1/fiscal/points-of-sale") return [point];
    if (path === "/api/v1/fiscal/settings?environment=homologation") {
      return {
        cuit: "20301234567",
        legal_name: "Taller Norte",
        tax_address: "Córdoba 123",
        tax_condition: "monotributo",
        activity_start_date: "2020-01-02",
        environment: "homologation",
        functional_currency: "ARS",
        country_code: "AR",
        version: 1,
        production_ready: false,
      };
    }
    if (path === "/api/v1/fiscal/vouchers" && options?.method === "POST") {
      return {
        id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        environment: "homologation",
        source_type: "sale",
        source_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
        state: "queued",
        concept: "services",
        currency: "ARS",
        total: "150.00",
        created_at: "2026-07-24T12:00:00Z",
        updated_at: "2026-07-24T12:00:00Z",
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await screen.findByText("Todavía no hay comprobantes.");
  await user.click(screen.getByRole("button", { name: /Emitir/ }));
  expect(
    await screen.findByText("Emisión C · precios finales sin IVA discriminado"),
  ).toBeInTheDocument();

  await user.type(
    screen.getByPlaceholderText("UUID de la venta"),
    "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
  );
  await user.selectOptions(screen.getByLabelText("Punto de venta"), point.id);
  await user.selectOptions(screen.getByLabelText("Concepto"), "services");
  await user.type(screen.getByLabelText("Receptor"), "Consumidor final");
  await user.selectOptions(screen.getByLabelText("Documento"), "CONSUMER_FINAL");
  await user.selectOptions(screen.getByLabelText("Condición IVA"), "consumidor_final");
  const today = calendarDate();
  fireEvent.change(screen.getByLabelText("Prestación desde"), {
    target: { value: today },
  });
  fireEvent.change(screen.getByLabelText("Prestación hasta"), {
    target: { value: today },
  });
  fireEvent.change(screen.getByLabelText("Vencimiento de pago"), {
    target: { value: today },
  });
  await user.type(screen.getByLabelText("Descripción línea 1"), "Abono mensual");
  fireEvent.change(screen.getByLabelText("Precio final línea 1"), {
    target: { value: "100.00" },
  });
  await user.click(screen.getByRole("button", { name: /Agregar línea/ }));
  await user.type(screen.getByLabelText("Descripción línea 2"), "Soporte");
  fireEvent.change(screen.getByLabelText("Precio final línea 2"), {
    target: { value: "50.00" },
  });

  expect(screen.getByLabelText("Alícuota línea 1")).toBeDisabled();
  expect(screen.getByLabelText("Vencimiento de pago")).toBeRequired();
  await user.click(screen.getByRole("button", { name: "Solicitar CAE" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
      ),
    ).toBe(true);
  });
  const post = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/vouchers" && options?.method === "POST",
  );
  const body = JSON.parse(String(post?.[1]?.body));
  expect(body).toEqual(
    expect.objectContaining({
      environment: "homologation",
      concept: "services",
      receiver_document_number: "0",
      service_from: today,
      service_to: today,
      payment_due_date: today,
    }),
  );
  expect(body.lines).toHaveLength(2);
  expect(body.lines[0]).toEqual(
    expect.objectContaining({
      unit_price: "100.00",
      subtotal: "100.00",
      taxes: [],
    }),
  );
  expect(body.lines[1].taxes).toEqual([]);
});

test("authorized voucher opens its immutable detail and creates an associated credit note", async () => {
  const user = userEvent.setup();
  const voucher = {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    environment: "homologation",
    state: "authorized",
    kind: "invoice_a",
    source_type: "sale",
    source_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
    concept: "products",
    point_of_sale: 3,
    voucher_number: 45,
    currency: "ARS",
    total: "121.00",
    cae: "71234567890123",
    created_at: "2026-07-24T12:00:00Z",
    updated_at: "2026-07-24T12:01:00Z",
  };
  const detail = {
    ...voucher,
    issue_date: "2026-07-24",
    receiver_name: "Cliente Federal SA",
    receiver_document_type: "CUIT",
    receiver_document_number: "30712345670",
    receiver_tax_condition: "responsable_inscripto",
    exchange_rate: "1",
    exchange_rate_source: "ARCA",
    lines: [
      {
        position: 1,
        description: "Producto",
        quantity: "1",
        unit_price: "100.00",
        tax_treatment: "taxable",
        subtotal: "100.00",
        vat_rate: "21",
        vat_amount: "21.00",
        total: "121.00",
      },
    ],
  };
  const { request } = renderFiscal("/fiscal/vouchers", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["fiscal:view", "fiscal:manage"]);
    }
    if (path.startsWith("/api/v1/fiscal/vouchers?")) {
      return { items: [voucher], page: { total: 1 } };
    }
    if (path === "/api/v1/fiscal/points-of-sale") return [];
    if (path === `/api/v1/fiscal/vouchers/${voucher.id}`) return detail;
    if (path === "/api/v1/fiscal/credit-notes" && options?.method === "POST") {
      return { ...voucher, id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb" };
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(await screen.findByText(/Factura A/i)).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Detalle" }));
  expect(await screen.findByText("Cliente Federal SA")).toBeInTheDocument();
  expect(screen.getByText("snapshot inmutable", { exact: false })).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Crear nota de crédito" }),
  );
  await user.type(screen.getByLabelText("Motivo"), "Devolución parcial");
  await user.click(
    screen.getByRole("button", { name: "Solicitar nota de crédito" }),
  );

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/credit-notes" &&
          options?.method === "POST",
      ),
    ).toBe(true);
  });
  const post = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/credit-notes" && options?.method === "POST",
  );
  expect(JSON.parse(String(post?.[1]?.body))).toEqual(
    expect.objectContaining({
      associated_voucher_id: voucher.id,
      reason: "Devolución parcial",
      lines: [
        expect.objectContaining({
          description: "Producto",
          subtotal: "100.00",
        }),
      ],
    }),
  );
});

test("environment switch ignores stale settings, requires activity date and clears the private key", async () => {
  const user = userEvent.setup();
  let resolveHomologation: ((value: Response) => void) | undefined;
  const homologationResponse = new Promise<Response>((resolve) => {
    resolveHomologation = resolve;
  });
  const productionSettings = {
    cuit: "30712345670",
    legal_name: "Comercio Norte Producción",
    tax_address: "Av. Siempre Viva 123",
    tax_condition: "registered",
    activity_start_date: "2020-01-02",
    environment: "production",
    functional_currency: "ARS",
    country_code: "AR",
    version: 2,
    production_ready: false,
  };
  const { request } = renderFiscal(
    "/fiscal/settings",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["fiscal:view", "fiscal:manage"]);
      }
      if (path === "/api/v1/fiscal/points-of-sale") return [];
      if (path === "/api/v1/fiscal/settings" && options?.method === "PUT") {
        return productionSettings;
      }
      if (path === "/api/v1/fiscal/certificates" && options?.method === "POST") {
        return {
          id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
          environment: "production",
          fingerprint: "abcdef0123456789",
          valid_from: "2026-07-01T00:00:00Z",
          expires_at: "2027-07-01T00:00:00Z",
          active: true,
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
    async (path) => {
      if (path === "/api/v1/fiscal/settings?environment=homologation") {
        return homologationResponse;
      }
      if (path === "/api/v1/fiscal/settings?environment=production") {
        return new Response(JSON.stringify(productionSettings), {
          headers: { "Content-Type": "application/json" },
          status: 200,
        });
      }
      throw new Error(`unexpected response request ${path}`);
    },
  );

  const environmentSelect = screen.getByLabelText("Ambiente");
  await waitFor(() => expect(environmentSelect).not.toBeDisabled());
  await user.selectOptions(environmentSelect, "production");
  expect(
    await screen.findByDisplayValue("Comercio Norte Producción"),
  ).toBeInTheDocument();
  resolveHomologation?.(
    new Response(
      JSON.stringify({
        ...productionSettings,
        legal_name: "Perfil viejo",
        environment: "homologation",
      }),
      { headers: { "Content-Type": "application/json" }, status: 200 },
    ),
  );
  await Promise.resolve();
  expect(screen.queryByDisplayValue("Perfil viejo")).not.toBeInTheDocument();
  expect(screen.getByLabelText("Inicio de actividades")).toBeRequired();

  await user.click(screen.getByRole("button", { name: "Guardar perfil" }));
  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/fiscal/settings" && options?.method === "PUT",
      ),
    ).toBe(true);
  });
  const save = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/fiscal/settings" && options?.method === "PUT",
  );
  expect(JSON.parse(String(save?.[1]?.body))).toEqual(
    expect.objectContaining({
      environment: "production",
      activity_start_date: "2020-01-02",
    }),
  );

  const certificate = screen.getByLabelText("Certificado PEM");
  const privateKey = screen.getByLabelText("Clave privada PEM");
  await user.type(certificate, "-----BEGIN CERTIFICATE-----test-----END CERTIFICATE-----");
  await user.type(privateKey, "-----BEGIN PRIVATE KEY-----secret-----END PRIVATE KEY-----");
  await user.click(screen.getByRole("button", { name: "Validar y guardar" }));
  await waitFor(() => expect(privateKey).toHaveValue(""));
  expect(certificate).toHaveValue("");
});

test("IVA Simple keeps preview downloads separate from the persisted workflow", async () => {
  const user = userEvent.setup();
  const createObjectURL = vi.fn(() => "blob:iva-simple");
  const revokeObjectURL = vi.fn();
  Object.defineProperty(URL, "createObjectURL", {
    configurable: true,
    value: createObjectURL,
  });
  Object.defineProperty(URL, "revokeObjectURL", {
    configurable: true,
    value: revokeObjectURL,
  });
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, "click")
    .mockImplementation(() => undefined);
  renderFiscal(
    "/fiscal/iva",
    async (path) => {
      if (path === "/api/v1/session") return session(["fiscal:view"]);
      if (
        path ===
        `/api/v1/fiscal/iva-simple/${currentTestPeriod()}?environment=production`
      ) {
        return ivaReport();
      }
      throw new Error(`unexpected request ${path}`);
    },
    async (path) => {
      if (path.includes("/workflow?environment=production")) {
        return new Response(null, { status: 404 });
      }
      throw new Error(`unexpected response request ${path}`);
    },
  );

  expect(await screen.findByText("Período consistente")).toBeInTheDocument();
  expect(screen.getByText("Sin preparar")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Vista previa ventas" }));
  await user.click(screen.getByRole("button", { name: "Vista previa compras" }));
  expect(createObjectURL).toHaveBeenCalledTimes(2);
  expect(click).toHaveBeenCalledTimes(2);
  expect(revokeObjectURL).toHaveBeenCalledTimes(2);
  click.mockRestore();
});

test("IVA Simple prepares, closes and keeps actor reasons in versioned commands", async () => {
  const user = userEvent.setup();
  let persisted:
    | {
        id: string;
        period: string;
        environment: "production";
        status: "draft" | "closed";
        opening_balance: string;
        closing_balance?: string;
        report: ReturnType<typeof ivaReport>;
        version: number;
        exports: never[];
        created_at: string;
        updated_at: string;
      }
    | undefined;
  const workflow = (status: "draft" | "closed", version: number) => ({
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    period: currentTestPeriod(),
    environment: "production" as const,
    status,
    opening_balance: "0",
    ...(status === "closed" ? { closing_balance: "-12.60" } : {}),
    report: ivaReport(),
    version,
    exports: [] as never[],
    created_at: "2026-07-24T12:00:00Z",
    updated_at: "2026-07-24T12:00:00Z",
  });
  const { request } = renderFiscal(
    "/fiscal/iva",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["fiscal:view", "fiscal:manage"]);
      }
      if (
        path ===
        `/api/v1/fiscal/iva-simple/${currentTestPeriod()}?environment=production`
      ) {
        return ivaReport();
      }
      if (path.includes("/prepare?environment=production")) {
        persisted = workflow("draft", 1);
        return persisted;
      }
      if (path.includes("/close?environment=production")) {
        expect(JSON.parse(String(options?.body))).toEqual({
          version: 1,
          reason: "Cierre mensual revisado",
        });
        persisted = workflow("closed", 2);
        return persisted;
      }
      throw new Error(`unexpected request ${path}`);
    },
    async (path) => {
      if (!path.includes("/workflow?environment=production")) {
        throw new Error(`unexpected response request ${path}`);
      }
      return persisted
        ? new Response(JSON.stringify(persisted), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          })
        : new Response(null, { status: 404 });
    },
  );

  await user.click(
    await screen.findByRole("button", { name: "Preparar período" }),
  );
  await screen.findByText("Borrador");
  const prepare = request.mock.calls.find(([path]) =>
    path.includes("/prepare?environment=production"),
  );
  expect(JSON.parse(String(prepare?.[1]?.body))).toEqual({});
  expect(prepare?.[1]?.headers).toEqual(
    expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
  );

  await user.type(screen.getByLabelText("Motivo"), "Cierre mensual revisado");
  await user.click(screen.getByRole("button", { name: "Cerrar" }));
  expect(await screen.findByText("Cerrado")).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Exportar ZIP" }),
  ).toBeInTheDocument();
});

test("homologation without evidence is pending, neutral and never promises a CAE request", async () => {
  renderFiscal(
    "/fiscal/homologation",
    async (path) => {
      if (path === "/api/v1/session") return session(["fiscal:view"]);
      throw new Error(`unexpected request ${path}`);
    },
    async (path) => {
      if (path === "/api/v1/fiscal/homologation/latest") {
        return new Response(null, { status: 404 });
      }
      throw new Error(`unexpected response request ${path}`);
    },
  );

  const pending = await screen.findByText("Pendiente");
  expect(pending).toHaveClass("status-pill--pending");
  expect(pending).not.toHaveClass("status-pill--rejected");
  expect(
    screen.getByText(/esta ejecución no solicita CAE/i),
  ).toBeInTheDocument();
  expect(
    await screen.findByText("Sin evidencia registrada"),
  ).toBeInTheDocument();
});

function currentTestPeriod() {
  return calendarDate().slice(0, 7);
}

function ivaReport() {
  return {
    period: currentTestPeriod(),
    sales_net: "100.00",
    output_vat: "21.00",
    purchases_net: "40.00",
    input_vat: "8.40",
    withholdings: "0",
    perceptions: "0",
    balance: "12.60",
    sales_file: "UEs=",
    purchases_file: "UEs=",
    validation_errors: [] as string[],
  };
}
