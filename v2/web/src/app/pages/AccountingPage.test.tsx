import {
  HttpError,
  type HttpClient,
} from "@devpablocristo/platform-http";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { beforeEach, expect, test, vi } from "vitest";
import type { AuthContextValue } from "../../auth/AuthContext";
import { AppProviders } from "../providers/AppProviders";
import { AccountingPage } from "./AccountingPage";

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

const editableAccountCapabilities = {
  can_edit_name: true,
  can_edit_structure: true,
  can_archive: true,
  can_trash: true,
  can_restore: false,
  can_duplicate: true,
  edit_blockers: [],
  archive_blockers: [],
  trash_blockers: [],
  restore_blockers: [],
};

function accountSummaryFixture(
  overrides: Record<string, unknown> = {},
) {
  return {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    code: "1.1.01",
    name: "Caja",
    node_type: "posting",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    parent_id: null,
    postable: true,
    lifecycle_state: "active",
    version: 1,
    depth: 0,
    path: [],
    has_children: false,
    used: false,
    mapped: false,
    system_managed: false,
    context_only: false,
    capabilities: editableAccountCapabilities,
    ...overrides,
  };
}

function trialBalanceFixture(overrides: Record<string, unknown> = {}) {
  return {
    currency: "ARS",
    from: "2026-01-01",
    to: "2026-07-24",
    items: [
      {
        account_id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
        code: "1.1.01",
        name: "Caja",
        account_class: "asset",
        normal_balance: "debit",
        lifecycle_state: "active",
        path: ["Activo", "Disponibilidades", "Caja"],
        opening_balance: { amount: "1250.00", side: "debit" },
        debit: "500.50",
        credit: "250.00",
        closing_balance: { amount: "1500.50", side: "debit" },
      },
    ],
    totals: {
      opening_debit: "1250.00",
      opening_credit: "0.00",
      movement_debit: "500.50",
      movement_credit: "250.00",
      closing_debit: "1500.50",
      closing_credit: "0.00",
    },
    controls: {
      opening_difference: "0.00",
      movement_difference: "0.00",
      closing_difference: "0.00",
    },
    page: { total: 1, next_cursor: null },
    ...overrides,
  };
}

function fiscalYearPeriodsFixture(
  states: Array<"open" | "soft_closed" | "locked"> = Array(12).fill("open"),
) {
  return states.map((state, index) => {
    const start = new Date(Date.UTC(2026, index, 1));
    const end = new Date(Date.UTC(2026, index + 1, 0));
    return {
      id: `${String(index + 1).padStart(8, "0")}-0000-4000-8000-000000000000`,
      code: `2026-${String(index + 1).padStart(2, "0")}`,
      sequence: index + 1,
      start_date: start.toISOString().slice(0, 10),
      end_date: end.toISOString().slice(0, 10),
      state,
      version: 1,
      is_legacy: false,
      close_readiness: {
        status: index === 0 ? "ready" : "not_evaluated",
        blocking_count: 0,
        evaluated_at:
          index === 0 ? "2026-02-01T13:30:00.000Z" : null,
        checks:
          index === 0
            ? [
                {
                  code: "unposted_documents",
                  label: "Documentos sin asiento",
                  status: "passed",
                  count: 0,
                },
              ]
            : [],
      },
      capabilities: {
        can_soft_close: state === "open",
        can_lock: state === "soft_closed",
        can_reopen_to_soft_closed: state === "locked",
        can_reopen_to_open: state === "soft_closed",
        blockers: [],
      },
    };
  });
}

function fiscalYearFixture<T extends Record<string, unknown>>(
  overrides: T = {} as T,
) {
  return {
    id: "f0000000-0000-4000-8000-000000000001",
    code: "2026",
    start_date: "2026-01-01",
    end_date: "2026-12-31",
    is_legacy: false,
    state: "open" as const,
    version: 1,
    period_counts: { open: 12, soft_closed: 0, locked: 0 },
    annual_close_status: "not_started" as const,
    annual_closing_draft_id: null,
    annual_closing_entry_id: null,
    ...overrides,
  };
}

function fiscalYearDetailFixture<T extends Record<string, unknown>>(
  overrides: T = {} as T,
) {
  return fiscalYearFixture({
    periods: fiscalYearPeriodsFixture(),
    recent_events: [
      {
        id: "e0000000-0000-4000-8000-000000000001",
        event_type: "period_transition",
        from_status: "open",
        to_status: "soft_closed",
        actor: "Ana Pérez",
        reason: "Cierre de control",
        occurred_at: "2026-02-01T13:30:00.000Z",
      },
    ],
    capabilities: {
      can_prepare_annual_close: false,
      can_reopen: false,
      blockers: ["previous_period_not_locked"],
    },
    ...overrides,
  });
}

function renderAccounting(
  path: string,
  request: (path: string, options?: Record<string, unknown>) => Promise<unknown>,
  requestResponse: (
    path: string,
    options?: Record<string, unknown>,
  ) => Promise<Response> = async () => new Response(),
) {
  window.history.replaceState({}, "", path);
  const requestMock = vi.fn(request);
  const responseMock = vi.fn(requestResponse);
  const apiClient = {
    request: requestMock,
    requestResponse: responseMock,
  } as unknown as HttpClient;

  render(
    <AppProviders authValue={authValue()} apiClient={apiClient}>
      <Routes>
        <Route path="/accounting/:section?" element={<AccountingPage />} />
      </Routes>
    </AppProviders>,
  );

  return { request: requestMock, requestResponse: responseMock };
}

beforeEach(() => {
  window.localStorage.clear();
  window.history.replaceState({}, "", "/");
});

test("accounting view-only can inspect accounts but has no mutation controls", async () => {
  renderAccounting("/accounting/accounts", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path === "/api/v1/accounting/account-mappings") return [];
    if (path === "/api/v1/accounting/account-mapping-definitions") return [];
    if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
      return {
        items: [
          {
            id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            code: "1.1.01",
            name: "Caja",
            account_type: "asset",
            normal_balance: "debit",
            monetary_classification: "monetary",
            node_type: "posting",
            depth: 0,
            path: [],
            has_children: false,
            used: false,
            mapped: false,
            system_managed: false,
            context_only: false,
            capabilities: {
              can_edit_name: true,
              can_edit_structure: true,
              can_archive: true,
              can_trash: true,
              can_restore: false,
              can_duplicate: true,
              edit_blockers: [],
              archive_blockers: [],
              trash_blockers: [],
              restore_blockers: [],
            },
            postable: true,
            lifecycle_state: "active",
            version: 1,
          },
        ],
        totals: { active: 1, archived: 0, trashed: 0 },
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(await screen.findByText("Caja")).toBeInTheDocument();
  expect(
    await screen.findByText(/modo lectura/i),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: /Nueva cuenta/ }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Archivar" }),
  ).not.toBeInTheDocument();
  const accountRow = screen.getByText("Caja").closest("tr");
  expect(accountRow).not.toBeNull();
  expect(
    within(accountRow as HTMLTableRowElement).queryByRole("button"),
  ).not.toBeInTheDocument();
});

test("collapsing an account group hides every descendant and expands it again", async () => {
  const user = userEvent.setup();
  const activoID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const disponibilidadesID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const cajaID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
  const bancosID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
  const capabilities = {
    can_edit_name: true,
    can_edit_structure: true,
    can_archive: true,
    can_trash: true,
    can_restore: false,
    can_duplicate: true,
    edit_blockers: [],
    archive_blockers: [],
    trash_blockers: [],
    restore_blockers: [],
  };

  renderAccounting("/accounting/accounts", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path === "/api/v1/accounting/account-mappings") return [];
    if (path === "/api/v1/accounting/account-mapping-definitions") return [];
    if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
      return {
        items: [
          { id: activoID, code: "1", name: "Activo", node_type: "group", depth: 0, path: [], has_children: true, used: false, mapped: false, system_managed: true, context_only: false, capabilities, account_type: "asset", normal_balance: "debit", monetary_classification: "not_applicable", postable: false, lifecycle_state: "active", version: 1 },
          { id: disponibilidadesID, code: "1.1", name: "Disponibilidades", node_type: "group", depth: 1, path: [activoID], has_children: true, used: false, mapped: false, system_managed: false, context_only: false, capabilities, parent_id: activoID, account_type: "asset", normal_balance: "debit", monetary_classification: "not_applicable", postable: false, lifecycle_state: "active", version: 1 },
          { id: cajaID, code: "1.1.01", name: "Caja", node_type: "posting", depth: 2, path: [activoID, disponibilidadesID], has_children: false, used: false, mapped: false, system_managed: false, context_only: false, capabilities, parent_id: disponibilidadesID, account_type: "asset", normal_balance: "debit", monetary_classification: "monetary", postable: true, lifecycle_state: "active", version: 1 },
          { id: bancosID, code: "1.1.02", name: "Bancos", node_type: "posting", depth: 2, path: [activoID, disponibilidadesID], has_children: false, used: false, mapped: false, system_managed: false, context_only: false, capabilities, parent_id: disponibilidadesID, account_type: "asset", normal_balance: "debit", monetary_classification: "monetary", postable: true, lifecycle_state: "active", version: 1 },
        ],
        totals: { active: 4, archived: 0, trashed: 0 },
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  const expandActivo = await screen.findByRole("button", { name: "Expandir Activo" });
  expect(screen.queryByText("Caja")).not.toBeInTheDocument();
  await user.click(expandActivo);
  await user.click(screen.getByRole("button", { name: "Expandir Disponibilidades" }));
  await screen.findByText("Caja");
  await user.click(screen.getByRole("button", { name: "Contraer Disponibilidades" }));

  await waitFor(() => {
    expect(screen.queryByText("Caja")).not.toBeInTheDocument();
    expect(screen.queryByText("Bancos")).not.toBeInTheDocument();
  });
  expect(screen.getByRole("button", { name: "Expandir Disponibilidades" })).toHaveAttribute("aria-expanded", "false");

  await user.click(screen.getByRole("button", { name: "Expandir Disponibilidades" }));
  expect(await screen.findByText("Caja")).toBeInTheDocument();
  expect(screen.getByText("Bancos")).toBeInTheDocument();
});

test("checkboxes only build a multi-selection and never open the account form", async () => {
  const user = userEvent.setup();
  const asset = accountSummaryFixture({
    id: "10000000-0000-4000-8000-000000000000",
    code: "1",
    name: "Activo",
    node_type: "group",
    postable: false,
    monetary_classification: "not_applicable",
    has_children: false,
  });
  const liability = accountSummaryFixture({
    id: "20000000-0000-4000-8000-000000000000",
    code: "2",
    name: "Pasivo",
    node_type: "group",
    postable: false,
    account_type: "liability",
    normal_balance: "credit",
    monetary_classification: "not_applicable",
    has_children: false,
  });

  renderAccounting("/accounting/accounts", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/account-mappings") return [];
    if (path === "/api/v1/accounting/account-mapping-definitions") return [];
    if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
      return {
        items: [asset, liability],
        totals: { active: 2, archived: 0, trashed: 0 },
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(
    await screen.findByRole("checkbox", { name: "Seleccionar 1 Activo" }),
  );
  await user.click(
    screen.getByRole("checkbox", { name: "Seleccionar 2 Pasivo" }),
  );

  expect(screen.getByText("2 seleccionados")).toBeInTheDocument();
  expect(screen.queryByLabelText("Nombre")).not.toBeInTheDocument();
  expect(
    screen.getByRole("checkbox", { name: "Seleccionar 1 Activo" }),
  ).toBeChecked();
  expect(
    screen.getByRole("checkbox", { name: "Seleccionar 2 Pasivo" }),
  ).toBeChecked();
});

test("keeps only lifecycle conflicts selected after a partial bulk result", async () => {
  const user = userEvent.setup();
  const cash = accountSummaryFixture();
  const bank = accountSummaryFixture({
    id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    code: "1.1.02",
    name: "Banco",
  });
  const { request } = renderAccounting(
    "/accounting/accounts",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/account-mappings") return [];
      if (path === "/api/v1/accounting/account-mapping-definitions") return [];
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: [cash, bank],
          totals: { active: 2, archived: 0, trashed: 0 },
        };
      }
      if (
        path === `/api/v1/accounting/accounts/${cash.id}/archive` &&
        options?.method === "POST"
      ) {
        return { ...cash, lifecycle_state: "archived", version: 2 };
      }
      if (
        path === `/api/v1/accounting/accounts/${bank.id}/archive` &&
        options?.method === "POST"
      ) {
        throw new Error("version conflict");
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("checkbox", {
      name: "Seleccionar todo el resultado",
    }),
  );
  await user.click(screen.getByRole("button", { name: "Archivar" }));

  expect(
    await screen.findByText(/1 actualizadas y 1 con conflictos/),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("checkbox", { name: "Seleccionar 1.1.01 Caja" }),
  ).not.toBeChecked();
  expect(
    screen.getByRole("checkbox", { name: "Seleccionar 1.1.02 Banco" }),
  ).toBeChecked();
  expect(
    request.mock.calls.filter(
      ([path]) => String(path).endsWith("/archive"),
    ),
  ).toHaveLength(2);
});

test("separates new groups from posting accounts and derives class from any parent group", async () => {
  const user = userEvent.setup();
  const asset = accountSummaryFixture({
    id: "10000000-0000-4000-8000-000000000000",
    code: "1",
    name: "Activo",
    node_type: "group",
    postable: false,
    monetary_classification: "not_applicable",
  });
  const liability = accountSummaryFixture({
    id: "20000000-0000-4000-8000-000000000000",
    code: "2",
    name: "Pasivo",
    node_type: "group",
    postable: false,
    account_type: "liability",
    normal_balance: "credit",
    monetary_classification: "not_applicable",
  });
  const { request } = renderAccounting(
    "/accounting/accounts",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/account-mappings") return [];
      if (path === "/api/v1/accounting/account-mapping-definitions") return [];
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: [asset, liability],
          totals: { active: 2, archived: 0, trashed: 0 },
        };
      }
      if (
        path === "/api/v1/accounting/accounts" &&
        options?.method === "POST"
      ) {
        return {
          ...accountSummaryFixture(),
          ...JSON.parse(String(options.body)),
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: "Nuevo rubro" }));
  expect(screen.getByRole("heading", { name: "Nuevo rubro" })).toBeInTheDocument();
  expect(screen.getByLabelText("Clasificación")).toHaveValue("not_applicable");
  await user.click(screen.getByRole("button", { name: "Cancelar" }));

  await user.click(screen.getByRole("button", { name: "Nueva cuenta" }));
  const parent = screen.getByRole("combobox", { name: "Rubro superior" });
  await user.click(parent);
  await user.click(screen.getByRole("option", { name: "2Pasivo" }));
  expect(screen.getByLabelText("Clase")).toHaveValue("liability");
  expect(screen.getByLabelText("Clase")).toBeDisabled();
  await user.type(screen.getByLabelText("Código"), "2.1.99");
  await user.type(screen.getByLabelText("Nombre"), "Proveedor exterior");
  await user.click(screen.getByRole("button", { name: "Guardar cuenta" }));

  await waitFor(() => {
    const create = request.mock.calls.find(
      ([path, options]) =>
        path === "/api/v1/accounting/accounts" && options?.method === "POST",
    );
    expect(JSON.parse(String(create?.[1]?.body))).toEqual(
      expect.objectContaining({
        node_type: "posting",
        postable: true,
        account_type: "liability",
        normal_balance: "credit",
        parent_id: liability.id,
      }),
    );
  });
});

test("starts with roots collapsed and expands the returned ancestor path when searching", async () => {
  const user = userEvent.setup();
  const root = accountSummaryFixture({
    id: "10000000-0000-4000-8000-000000000000",
    code: "1",
    name: "Activo",
    node_type: "group",
    postable: false,
    monetary_classification: "not_applicable",
    has_children: true,
  });
  const cash = accountSummaryFixture({
    id: "11000000-0000-4000-8000-000000000000",
    parent_id: root.id,
    depth: 1,
    path: [root.id],
  });
  const { request } = renderAccounting("/accounting/accounts", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/account-mappings") return [];
    if (path === "/api/v1/accounting/account-mapping-definitions") return [];
    if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
      const searching = path.includes("query=Caja");
      return {
        items: searching
          ? [{ ...root, context_only: true }, cash]
          : [root, cash],
        totals: { active: 2, archived: 0, trashed: 0 },
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(
    await screen.findByRole("button", { name: "Expandir Activo" }),
  ).toBeInTheDocument();
  expect(screen.queryByText("Caja")).not.toBeInTheDocument();
  await user.type(screen.getByRole("searchbox", { name: "Buscar cuentas" }), "Caja");

  expect(await screen.findByText("Caja")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Contraer Activo" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  expect(
    request.mock.calls.some(([path]) => String(path).includes("query=Caja")),
  ).toBe(true);
});

test("imports an extract, loads suggestions and saves the selected match", async () => {
  const user = userEvent.setup();
  const financialAccount = {
    id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
    ledger_account_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    ledger_account_code: "1.1.02",
    ledger_account_name: "Banco Nación",
    account_type: "bank",
    name: "BNA cuenta corriente",
    currency: "ARS",
    institution_name: "Banco Nación",
    archived: false,
    version: 1,
  };
  const movement = {
    id: "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
    booked_at: "2026-07-22",
    value_at: "2026-07-22",
    description: "Transferencia cliente",
    reference: "TRX-42",
    amount: "121.00",
    currency: "ARS",
    fingerprint: "abcdef1234567890",
  };
  const suggestion = {
    statement_movement_id: movement.id,
    journal_line_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
    amount: "121.00",
    score: 98,
    reasons: ["importe exacto", "misma fecha"],
  };
  const { request } = renderAccounting(
    "/accounting/reconciliation",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/financial-accounts?")) {
        return [financialAccount];
      }
      if (path.startsWith("/api/v1/accounting/reconciliations?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return {
          items: [
            {
              id: financialAccount.ledger_account_id,
              code: financialAccount.ledger_account_code,
              name: financialAccount.ledger_account_name,
              account_type: "asset",
              normal_balance: "debit",
              monetary_classification: "monetary",
              postable: true,
              lifecycle_state: "active",
              version: 1,
            },
          ],
          page: { total: 1 },
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return {
          items: [
            {
              id: "abababab-abab-4bab-8bab-abababababab",
              entry_number: 42,
              accounting_date: "2026-07-22",
              description: "Cobro transferencia cliente",
              currency: "ARS",
              source_type: "receipt",
              lines: [
                {
                  id: suggestion.journal_line_id,
                  line_number: 1,
                  account_id: financialAccount.ledger_account_id,
                  debit: "121.00",
                  credit: "0",
                  memo: "TRX-42",
                },
              ],
              total_debit: "121.00",
              total_credit: "121.00",
              created_at: "2026-07-22T15:00:00Z",
            },
          ],
          page: { total: 1 },
        };
      }
      if (
        path === "/api/v1/accounting/statement-imports" &&
        options?.method === "POST"
      ) {
        return {
          id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee",
          financial_account_id: financialAccount.id,
          file_name: "julio.csv",
          format: "csv",
          sha256: "1234567890abcdef",
          currency: "ARS",
          imported_at: "2026-07-24T12:00:00Z",
          movements: [movement],
        };
      }
      if (path.includes("/suggestions?")) return [suggestion];
      if (
        path === "/api/v1/accounting/reconciliations" &&
        options?.method === "POST"
      ) {
        const body = JSON.parse(String(options.body));
        return {
          id: "ffffffff-ffff-4fff-8fff-ffffffffffff",
          account_id: financialAccount.id,
          period_start: body.period_start,
          statement_date: body.statement_date,
          opening_balance: body.opening_balance,
          statement_balance: body.statement_balance,
          ledger_balance: "121.00",
          difference: "0.00",
          currency: "ARS",
          state: "draft",
          version: 1,
          matches: body.matches.map(
            (match: Record<string, string>, index: number) => ({
              ...match,
              id: `match-${index}`,
              created_at: "2026-07-24T12:00:00Z",
            }),
          ),
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const input = await screen.findByLabelText("Archivo de extracto");
  await user.upload(
    input,
    new File(["fecha,detalle,importe"], "julio.csv", { type: "text/csv" }),
  );

  expect(await screen.findByText("Transferencia cliente")).toBeInTheDocument();
  expect(await screen.findByText(/Coincidencia 98/)).toBeInTheDocument();
  expect(
    await screen.findByText(/Asiento 42 · .*Banco Nación/),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Vincular" }));
  await user.click(screen.getByRole("button", { name: "Crear conciliación" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path, options]) =>
          path === "/api/v1/accounting/reconciliations" &&
          options?.method === "POST",
      ),
    ).toBe(true);
  });
  const importCall = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/accounting/statement-imports" &&
      options?.method === "POST",
  );
  expect(JSON.parse(String(importCall?.[1]?.body))).toEqual(
    expect.objectContaining({
      financial_account_id: financialAccount.id,
      file_name: "julio.csv",
      format: "csv",
      content_base64: expect.any(String),
    }),
  );
  const saveCall = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/accounting/reconciliations" &&
      options?.method === "POST",
  );
  expect(JSON.parse(String(saveCall?.[1]?.body)).matches).toEqual([
    {
      statement_movement_id: movement.id,
      journal_line_id: suggestion.journal_line_id,
      statement_amount: "121.00",
      ledger_amount: "121.00",
    },
  ]);
});

test("opens an exact account row, edits its name and updates a canonical mapping", async () => {
  const user = userEvent.setup();
  const capabilities = {
    can_edit_name: true,
    can_edit_structure: true,
    can_archive: true,
    can_trash: true,
    can_restore: false,
    can_duplicate: true,
    edit_blockers: [],
    archive_blockers: [],
    trash_blockers: [],
    restore_blockers: [],
  };
  const rootAccount = {
    id: "10000000-0000-4000-8000-000000000000",
    code: "1",
    name: "Activo",
    node_type: "group",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "not_applicable",
    parent_id: null,
    postable: false,
    lifecycle_state: "active",
    version: 1,
    depth: 0,
    path: [],
    has_children: true,
    used: false,
    mapped: false,
    system_managed: true,
    context_only: false,
    capabilities: { ...capabilities, can_edit_name: false, can_edit_structure: false },
  };
  const cashAccount = {
    ...rootAccount,
    id: "11000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    parent_id: rootAccount.id,
    node_type: "posting",
    monetary_classification: "monetary",
    postable: true,
    version: 3,
    depth: 1,
    path: [rootAccount.id],
    has_children: false,
    mapped: true,
    system_managed: false,
    capabilities,
  };
  const bankAccount = {
    ...cashAccount,
    id: "12000000-0000-4000-8000-000000000000",
    code: "1.1.02",
    name: "Banco Nación",
    version: 1,
  };
  const allAccounts = [rootAccount, cashAccount, bankAccount];

  const { request } = renderAccounting(
    "/accounting/accounts",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/account-mappings" && !options?.method) {
        return [
          {
            role: "cash",
            account_id: cashAccount.id,
            account_code: cashAccount.code,
            account_name: cashAccount.name,
            version: 2,
          },
        ];
      }
      if (path === "/api/v1/accounting/account-mapping-definitions") {
        return [
          {
            role: "cash",
            label_es: "Caja",
            label_en: "Cash",
            description_es: "Cuenta usada para cobros en efectivo.",
            description_en: "Cash postings.",
            required: true,
            compatible_account_types: ["asset"],
            compatible_normal_balances: ["debit"],
            compatible_monetary_classifications: ["monetary"],
            is_alias: false,
            display_order: 1,
          },
          {
            role: "cash_account",
            canonical_role: "cash",
            label_es: "Caja heredada",
            label_en: "Legacy cash",
            description_es: "Alias heredado.",
            description_en: "Legacy alias.",
            required: false,
            compatible_account_types: ["asset"],
            compatible_normal_balances: ["debit"],
            compatible_monetary_classifications: ["monetary"],
            is_alias: true,
            display_order: 2,
          },
        ];
      }
      if (
        path === "/api/v1/accounting/account-mappings" &&
        options?.method === "PUT"
      ) {
        return [
          {
            role: "cash",
            account_id: bankAccount.id,
            account_code: bankAccount.code,
            account_name: bankAccount.name,
            version: 3,
          },
        ];
      }
      if (
        path === `/api/v1/accounting/accounts/${cashAccount.id}` &&
        !options?.method
      ) {
        return {
          ...cashAccount,
          usage: {
            used: false,
            journal_lines: 0,
            draft_lines: 0,
            mappings: 1,
            children: 0,
            financial_accounts: 0,
            open_items: 0,
            inflation_lines: 0,
            revaluation_lines: 0,
            total_dependencies: 1,
            active_children: 0,
            active_financial_accounts: 0,
          },
          mapped_roles: ["cash"],
        };
      }
      if (
        path === `/api/v1/accounting/accounts/${cashAccount.id}` &&
        options?.method === "PUT"
      ) {
        const body = JSON.parse(String(options.body));
        return { ...cashAccount, ...body, version: 4 };
      }
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: allAccounts,
          totals: { active: allAccounts.length, archived: 0, trashed: 0 },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const accountTable = await screen.findByRole("table");
  await user.click(await within(accountTable).findByRole("button", { name: "Expandir Activo" }));
  const cashLabel = await within(accountTable).findByText("Caja", {
    selector: "span",
  });
  const cashRow = cashLabel.closest("tr");
  expect(cashRow).not.toBeNull();
  await user.click(
    within(cashRow as HTMLTableRowElement).getByRole("checkbox", {
      name: "Seleccionar 1.1.01 Caja",
    }),
  );
  expect(screen.queryByLabelText("Nombre")).not.toBeInTheDocument();
  await user.click(cashRow as HTMLTableRowElement);
  const nameInput = screen.getByLabelText("Nombre");
  await user.clear(nameInput);
  await user.type(nameInput, "Caja central");
  await user.click(screen.getByRole("button", { name: "Guardar cuenta" }));

  await waitFor(() => {
    const update = request.mock.calls.find(
      ([path, options]) =>
        path === `/api/v1/accounting/accounts/${cashAccount.id}` &&
        options?.method === "PUT",
    );
    expect(JSON.parse(String(update?.[1]?.body))).toEqual(
      expect.objectContaining({ name: "Caja central", version: 3 }),
    );
  });

  await user.click(screen.getByRole("button", { name: "Editar mappings" }));
  const mappingAccount = screen.getByRole("combobox", {
    name: "Cuenta para Caja",
  });
  await user.clear(mappingAccount);
  await user.type(mappingAccount, "Banco");
  await user.click(
    screen.getByRole("option", { name: /1\.1\.02Banco Nación/ }),
  );
  await user.click(screen.getByRole("button", { name: "Guardar mappings" }));
  await waitFor(() => {
    const update = request.mock.calls.find(
      ([path, options]) =>
        path === "/api/v1/accounting/account-mappings" &&
        options?.method === "PUT",
    );
    expect(JSON.parse(String(update?.[1]?.body))).toEqual([
      { role: "cash", account_id: bankAccount.id, version: 2 },
    ]);
  });
  expect(screen.getByText(/cash_account/)).toBeInTheDocument();
});

test("saves a versioned multiline draft before posting it", async () => {
  const user = userEvent.setup();
  const debitAccount = {
    id: "21000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    parent_id: null,
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const creditAccount = {
    ...debitAccount,
    id: "41000000-0000-4000-8000-000000000000",
    code: "4.1.01",
    name: "Ventas",
    account_type: "income",
    normal_balance: "credit",
  };
  let savedDraft: Record<string, unknown> | undefined;

  const { request } = renderAccounting(
    "/accounting/journal",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return {
          items: savedDraft ? [savedDraft] : [],
          page: { total: savedDraft ? 1 : 0 },
        };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return {
          items: [debitAccount, creditAccount],
          page: { total: 2 },
        };
      }
      if (
        path === "/api/v1/accounting/drafts" &&
        options?.method === "POST"
      ) {
        const body = JSON.parse(String(options.body));
        savedDraft = {
          id: "30000000-0000-4000-8000-000000000000",
          ...body,
          lines: body.lines.map(
            (line: Record<string, string>, index: number) => ({
              ...line,
              id: `31000000-0000-4000-8000-00000000000${index}`,
              line_number: index + 1,
            }),
          ),
          total_debit: "100.00",
          total_credit: "100.00",
          version: 1,
        };
        return savedDraft;
      }
      if (
        path ===
          "/api/v1/accounting/drafts/30000000-0000-4000-8000-000000000000/post" &&
        options?.method === "POST"
      ) {
        return {
          id: "32000000-0000-4000-8000-000000000000",
          entry_number: 1,
          accounting_date: "2026-07-24",
          description: "Venta del día",
          currency: "ARS",
          source_type: "manual",
          lines: (savedDraft?.lines as unknown[]) ?? [],
          total_debit: "100.00",
          total_credit: "100.00",
          created_at: "2026-07-24T12:00:00Z",
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: /Nuevo asiento/ }));
  await user.type(screen.getByLabelText("Detalle"), "Venta del día");
  await user.click(screen.getByLabelText("Cuenta línea 1"));
  await user.click(screen.getByRole("option", { name: /1\.1\.01.*Caja/ }));
  await user.type(screen.getByLabelText("Debe línea 1"), "100.00");
  await user.click(screen.getByLabelText("Cuenta línea 2"));
  await user.click(screen.getByRole("option", { name: /4\.1\.01.*Ventas/ }));
  await user.type(screen.getByLabelText("Haber línea 2"), "160.00");
  const balanceRail = screen.getByText("Diferencia").closest(
    ".journal-balance-rail",
  );
  expect(balanceRail).not.toBeNull();
  expect(
    within(balanceRail as HTMLElement).getByText("ARS 60,00"),
  ).toBeInTheDocument();
  await user.clear(screen.getByLabelText("Haber línea 2"));
  await user.type(screen.getByLabelText("Haber línea 2"), "60.00");
  await user.click(screen.getByRole("button", { name: "＋ Agregar línea" }));
  await user.click(screen.getByLabelText("Cuenta línea 3"));
  await user.click(screen.getByRole("option", { name: /4\.1\.01.*Ventas/ }));
  await user.type(screen.getByLabelText("Haber línea 3"), "40.00");

  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));
  await waitFor(() =>
    expect(
      request.mock.calls.filter(
        ([path, options]) =>
          path === "/api/v1/accounting/drafts" && options?.method === "POST",
      ),
    ).toHaveLength(1),
  );
  expect(
    request.mock.calls.some(([path]) => String(path).endsWith("/post")),
  ).toBe(false);
  const createCall = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/accounting/drafts" && options?.method === "POST",
  );
  expect(JSON.parse(String(createCall?.[1]?.body)).lines).toHaveLength(3);

  await user.click(await screen.findByRole("button", { name: "Contabilizar" }));
  const confirmation = screen.getByRole("dialog", {
    name: "Contabilizar asiento",
  });
  expect(within(confirmation).getByText("Detalle")).toBeInTheDocument();
  expect(within(confirmation).getByText("Venta del día")).toBeInTheDocument();
  expect(within(confirmation).getByText("Diferencia")).toBeInTheDocument();
  expect(within(confirmation).getByText("ARS 0,00")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Contabilizar asiento" }),
  );
  await waitFor(() => {
    const postCall = request.mock.calls.find(
      ([path, options]) =>
        String(path).endsWith("/post") && options?.method === "POST",
    );
    expect(JSON.parse(String(postCall?.[1]?.body))).toEqual({
      version: 1,
      reason: "Asiento manual",
    });
  });
});

test("sends the Diario search, date range, and real payment source to the server", async () => {
  const user = userEvent.setup();
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await screen.findByRole("button", { name: /Nuevo asiento/ });
  await user.type(screen.getByLabelText("Buscar asientos"), "proveedor");
  await user.click(screen.getByRole("button", { name: "Filtros" }));
  fireEvent.change(screen.getByLabelText("Desde"), {
    target: { value: "2026-07-01" },
  });
  fireEvent.change(screen.getByLabelText("Hasta"), {
    target: { value: "2026-07-31" },
  });
  await user.selectOptions(screen.getByLabelText("Origen"), "supplier_payment");

  await waitFor(() => {
    const journalURL = request.mock.calls
      .map(([path]) => String(path))
      .find(
        (path) =>
          path.startsWith("/api/v1/accounting/journal-entries?") &&
          path.includes("query=proveedor") &&
          path.includes("from=2026-07-01") &&
          path.includes("to=2026-07-31") &&
          path.includes("source_type=supplier_payment"),
      );
    expect(journalURL).toBeDefined();
  });
});

test("saves an empty draft but keeps posting disabled", async () => {
  const user = userEvent.setup();
  let saved = false;
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return {
          items: saved
            ? [
                {
                  id: "70000000-0000-4000-8000-000000000000",
                  accounting_date: "2026-07-24",
                  description: "",
                  currency: "ARS",
                  functional_currency: "ARS",
                  line_count: 0,
                  total_debit: "0",
                  total_credit: "0",
                  posting_status: {
                    state: "incomplete",
                    difference: "0",
                    issues: [
                      "description_required",
                      "minimum_lines",
                      "zero_total",
                    ],
                  },
                  updated_at: "2026-07-24T12:00:00Z",
                  version: 1,
                },
              ]
            : [],
          page: { total: saved ? 1 : 0 },
        };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (
        path === "/api/v1/accounting/drafts" &&
        options?.method === "POST"
      ) {
        saved = true;
        const body = JSON.parse(String(options.body));
        return {
          id: "70000000-0000-4000-8000-000000000000",
          ...body,
          functional_currency: "ARS",
          lines: [],
          total_debit: "0",
          total_credit: "0",
          posting_status: {
            state: "incomplete",
            difference: "0",
            issues: [
              "description_required",
              "minimum_lines",
              "zero_total",
            ],
          },
          version: 1,
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: /Nuevo asiento/ }));
  const search = screen.getByLabelText("Buscar asientos");
  const filters = screen.getByRole("button", { name: "Filtros" });
  expect(
    search.compareDocumentPosition(filters) & Node.DOCUMENT_POSITION_FOLLOWING,
  ).not.toBe(0);
  expect(screen.getByRole("button", { name: "Guardar borrador" })).toBeEnabled();
  expect(screen.getByRole("button", { name: "Contabilizar" })).toBeDisabled();
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));

  await waitFor(() => {
    const create = request.mock.calls.find(
      ([path, options]) =>
        path === "/api/v1/accounting/drafts" && options?.method === "POST",
    );
    expect(JSON.parse(String(create?.[1]?.body))).toEqual(
      expect.objectContaining({ description: "", lines: [] }),
    );
  });
  expect(screen.getByRole("button", { name: "Contabilizar" })).toBeDisabled();
  expect(
    screen.getAllByText("Incompleto").length,
  ).toBeGreaterThanOrEqual(1);
});

test("reuses the draft idempotency key when retrying the same save", async () => {
  const user = userEvent.setup();
  const keys: string[] = [];
  let saveAttempts = 0;
  renderAccounting("/accounting/journal", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (
      path === "/api/v1/accounting/drafts" &&
      options?.method === "POST"
    ) {
      saveAttempts += 1;
      keys.push(
        String(
          (options.headers as Record<string, string>)?.[
            "Idempotency-Key"
          ],
        ),
      );
      if (saveAttempts === 1) {
        throw new TypeError("Failed to fetch");
      }
      const body = JSON.parse(String(options.body));
      return {
        id: "70500000-0000-4000-8000-000000000000",
        ...body,
        functional_currency: "ARS",
        total_debit: "0",
        total_credit: "0",
        posting_status: {
          state: "incomplete",
          difference: "0",
          issues: ["description_required", "minimum_lines", "zero_total"],
        },
        version: 1,
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("button", { name: /Nuevo asiento/ }));
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));
  expect(
    await screen.findByText(
      "No pudimos conectarnos con Pymes. Revisá la conexión y reintentá.",
    ),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));
  expect(
    await screen.findByText("Borrador · v1"),
  ).toBeInTheDocument();
  expect(keys).toHaveLength(2);
  expect(keys[0]).toBeTruthy();
  expect(keys[1]).toBe(keys[0]);
});

test("resets the journal cursor when changing tabs", async () => {
  const user = userEvent.setup();
  const entryQueries: string[] = [];
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      entryQueries.push(path);
      const cursor = new URL(path, "http://pymes.test").searchParams.get(
        "cursor",
      );
      return {
        items: [],
        page: {
          total: 31,
          next_cursor: cursor ? undefined : "entry-next",
        },
      };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [], page: { total: 0 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  const next = await screen.findByRole("button", { name: "Siguiente" });
  await waitFor(() => expect(next).toBeEnabled());
  await user.click(next);
  await waitFor(() =>
    expect(entryQueries.at(-1)).toContain("cursor=entry-next"),
  );
  await user.click(screen.getByRole("tab", { name: /Borradores/ }));
  await waitFor(() => {
    expect(entryQueries.length).toBeGreaterThanOrEqual(3);
    expect(entryQueries.at(-1)).not.toContain("cursor=");
  });
});

test("selects and discards drafts without an actions column", async () => {
  const user = userEvent.setup();
  let discarded = false;
  const draft = {
    id: "71000000-0000-4000-8000-000000000000",
    accounting_date: "2026-07-24",
    reference: "AJ-24",
    description: "Ajuste pendiente",
    currency: "ARS",
    functional_currency: "ARS",
    line_count: 0,
    total_debit: "0",
    total_credit: "0",
    posting_status: {
      state: "incomplete",
      difference: "0",
      issues: ["minimum_lines", "zero_total"],
    },
    updated_at: "2026-07-24T12:00:00Z",
    version: 3,
  };
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return {
          items: discarded ? [] : [draft],
          page: { total: discarded ? 0 : 1 },
        };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (
        path === `/api/v1/accounting/drafts/${draft.id}/discard` &&
        options?.method === "POST"
      ) {
        discarded = true;
        return undefined;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("tab", { name: /Borradores/ }),
  );
  expect(screen.queryByRole("columnheader", { name: "Gestión" })).not.toBeInTheDocument();
  await user.click(
    screen.getByRole("checkbox", {
      name: "Seleccionar todos los borradores de la página",
    }),
  );
  expect(
    screen.getByRole("checkbox", {
      name: "Seleccionar borrador Ajuste pendiente",
    }),
  ).toBeChecked();
  expect(screen.getByText("1 seleccionado")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Descartar" }));
  await user.click(
    screen.getByRole("button", { name: "Descartar borradores" }),
  );

  await waitFor(() => {
    const discard = request.mock.calls.find(
      ([path, options]) =>
        path === `/api/v1/accounting/drafts/${draft.id}/discard` &&
        options?.method === "POST",
    );
    expect(JSON.parse(String(discard?.[1]?.body))).toEqual({
      version: 3,
      reason: "Descartado desde el Diario",
    });
    expect(discard?.[1]?.headers).toEqual(
      expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
    );
  });
  expect(await screen.findByText("No hay borradores pendientes.")).toBeInTheDocument();
});

test("opens a draft as read-only for users without accounting management", async () => {
  const user = userEvent.setup();
  const draftID = "71500000-0000-4000-8000-000000000000";
  const summary = {
    id: draftID,
    accounting_date: "2026-07-24",
    reference: "AJ-LECTURA",
    description: "Borrador visible",
    currency: "ARS",
    functional_currency: "ARS",
    line_count: 2,
    total_debit: "2500",
    total_credit: "2500",
    posting_status: {
      state: "ready",
      difference: "0",
      issues: [],
    },
    updated_at: "2026-07-24T12:00:00Z",
    updated_by: "Ana Pérez",
    version: 4,
  };
  const detail = {
    ...summary,
    lines: [
      {
        id: "line-read-1",
        account_id: "account-cash",
        account_code: "1.1.01",
        account_name: "Caja",
        debit: "2500",
        credit: "0",
        memo: "Ingreso",
      },
      {
        id: "line-read-2",
        account_id: "account-sales",
        account_code: "4.1",
        account_name: "Ventas de mercaderías",
        debit: "0",
        credit: "2500",
      },
    ],
  };
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [summary], page: { total: 1 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path === `/api/v1/accounting/drafts/${draftID}`) {
        return detail;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("tab", { name: /Borradores/ }));
  expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  await user.click(
    screen.getByRole("row", { name: /Abrir borrador Borrador visible/ }),
  );

  const drawer = await screen.findByRole("dialog", {
    name: "Borrador visible",
  });
  expect(within(drawer).getByText("AJ-LECTURA")).toBeInTheDocument();
  expect(within(drawer).getByText("1.1.01 · Caja")).toBeInTheDocument();
  expect(
    within(drawer).getByText("4.1 · Ventas de mercaderías"),
  ).toBeInTheDocument();
  expect(within(drawer).getByText("Ana Pérez")).toBeInTheDocument();
  expect(
    within(drawer).queryByRole("button", { name: "Guardar borrador" }),
  ).not.toBeInTheDocument();
  expect(
    within(drawer).queryByRole("button", { name: "Contabilizar" }),
  ).not.toBeInTheDocument();
  expect(
    within(drawer).queryByRole("button", { name: /Descartar/ }),
  ).not.toBeInTheDocument();
  expect(request).toHaveBeenCalledWith(
    `/api/v1/accounting/drafts/${draftID}`,
    expect.objectContaining({ skipJSONContentType: true }),
  );
});

test("opens the exact journal draft referenced by draft_id", async () => {
  const draftID = "71600000-0000-4000-8000-000000000000";
  const detail = {
    id: draftID,
    accounting_date: "2026-12-31",
    reference: "CIERRE-2026",
    description: "Cierre anual enlazado",
    currency: "ARS",
    functional_currency: "ARS",
    line_count: 0,
    lines: [],
    total_debit: "0",
    total_credit: "0",
    posting_status: {
      state: "incomplete",
      difference: "0",
      issues: ["minimum_lines", "zero_total"],
    },
    updated_at: "2026-12-31T23:00:00Z",
    version: 1,
  };
  const { request } = renderAccounting(
    `/accounting/journal?draft_id=${draftID}`,
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path === `/api/v1/accounting/drafts/${draftID}`) return detail;
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(await screen.findByDisplayValue("Cierre anual enlazado")).toBeInTheDocument();
  expect(screen.getByRole("tab", { name: /Borradores/ })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  expect(request).toHaveBeenCalledWith(
    `/api/v1/accounting/drafts/${draftID}`,
    expect.objectContaining({ skipJSONContentType: true }),
  );
});

test("opens the exact posted entry referenced by entry_id", async () => {
  const entryID = "71700000-0000-4000-8000-000000000000";
  const entry = {
    id: entryID,
    entry_number: 98,
    accounting_date: "2026-12-31",
    reference: "CIERRE-2026",
    description: "Asiento de cierre enlazado",
    currency: "ARS",
    functional_currency: "ARS",
    source_type: "annual_closing",
    lines: [],
    total_debit: "500",
    total_credit: "500",
    created_at: "2026-12-31T23:30:00Z",
  };
  const { request } = renderAccounting(
    `/accounting/journal?entry_id=${entryID}`,
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path === `/api/v1/accounting/journal-entries/${entryID}`) {
        return entry;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(
    await screen.findByRole("dialog", { name: "Asiento de cierre enlazado" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Asiento Nº 98")).toBeInTheDocument();
  expect(request).toHaveBeenCalledWith(
    `/api/v1/accounting/journal-entries/${entryID}`,
    expect.objectContaining({ skipJSONContentType: true }),
  );
});

test("resolves a draft version conflict by reloading or saving a copy", async () => {
  const user = userEvent.setup();
  const draftID = "72000000-0000-4000-8000-000000000000";
  let detailReads = 0;
  let copiedBody: Record<string, unknown> | undefined;
  const summary = {
    id: draftID,
    accounting_date: "2026-07-24",
    description: "Versión inicial",
    functional_currency: "ARS",
    currency: "ARS",
    exchange_rate: "1",
    kind: "manual",
    line_count: 0,
    total_debit: "0",
    total_credit: "0",
    posting_status: {
      state: "incomplete",
      difference: "0",
      issues: ["minimum_lines", "zero_total"],
    },
    updated_by: "Ana",
    updated_at: "2026-07-24T12:00:00Z",
    version: 1,
  };
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [summary], page: { total: 1 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path === `/api/v1/accounting/drafts/${draftID}` && !options?.method) {
        detailReads += 1;
        return {
          ...summary,
          description:
            detailReads === 1 ? "Versión inicial" : "Versión actualizada",
          lines: [],
          created_by: "Ana",
          created_at: "2026-07-24T11:00:00Z",
          version: detailReads === 1 ? 1 : 2,
        };
      }
      if (
        path === `/api/v1/accounting/drafts/${draftID}` &&
        options?.method === "PUT"
      ) {
        throw new HttpError(
          "version conflict",
          409,
          JSON.stringify({
            error: {
              code: "VERSION_CONFLICT",
              message: "version conflict",
            },
          }),
        );
      }
      if (
        path === "/api/v1/accounting/drafts" &&
        options?.method === "POST"
      ) {
        copiedBody = JSON.parse(String(options.body));
        return {
          ...summary,
          ...copiedBody,
          id: "73000000-0000-4000-8000-000000000000",
          lines: [],
          created_by: "Ana",
          created_at: "2026-07-24T13:00:00Z",
          updated_at: "2026-07-24T13:00:00Z",
          version: 1,
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("tab", { name: /Borradores/ }));
  await user.click(
    screen.getByRole("row", { name: "Abrir borrador Versión inicial" }),
  );
  const detail = await screen.findByLabelText("Detalle");
  await user.clear(detail);
  await user.type(detail, "Mi cambio local");
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));

  expect(
    await screen.findByRole("heading", {
      name: "Este borrador cambió en otra sesión",
    }),
  ).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Recargar última versión" }),
  );
  await waitFor(() =>
    expect(screen.getByLabelText("Detalle")).toHaveValue(
      "Versión actualizada",
    ),
  );

  await user.clear(screen.getByLabelText("Detalle"));
  await user.type(screen.getByLabelText("Detalle"), "Copia local");
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));
  await user.click(
    await screen.findByRole("button", { name: "Guardar como nuevo" }),
  );
  await waitFor(() =>
    expect(copiedBody).toEqual(
      expect.objectContaining({ description: "Copia local" }),
    ),
  );
  expect(
    request.mock.calls.filter(
      ([path, options]) =>
        path === "/api/v1/accounting/drafts" && options?.method === "POST",
    ),
  ).toHaveLength(1);
});

test("selects a journal account with the keyboard", async () => {
  const user = userEvent.setup();
  const cash = {
    id: "74000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const bank = {
    ...cash,
    id: "74500000-0000-4000-8000-000000000000",
    code: "1.1.02",
    name: "Bancos",
  };
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [cash, bank], page: { total: 2 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("button", { name: /Nuevo asiento/ }));
  const account = screen.getByLabelText("Cuenta línea 1");
  await user.click(account);
  await user.keyboard("{ArrowDown}{Enter}");
  expect(account).toHaveValue("1.1.01 · Caja");
  expect(account).toHaveAttribute("aria-expanded", "false");
});

test("keeps a remotely searched account available for journal validation", async () => {
  const user = userEvent.setup();
  const remoteCash = {
    id: "74700000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja remota",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const sales = {
    ...remoteCash,
    id: "74800000-0000-4000-8000-000000000000",
    code: "4.1.01",
    name: "Ventas",
    account_type: "income",
    normal_balance: "credit",
  };
  const accountQueries: string[] = [];
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      accountQueries.push(path);
      const search = new URL(path, "http://pymes.test").searchParams;
      return search.has("query")
        ? { items: [remoteCash], page: { total: 1 } }
        : { items: [sales], page: { total: 1 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("button", { name: /Nuevo asiento/ }));
  await user.type(screen.getByLabelText("Detalle"), "Venta en caja");
  const firstAccount = screen.getByLabelText("Cuenta línea 1");
  await user.type(firstAccount, "Caja remota");
  await user.click(
    await screen.findByRole("option", { name: /1\.1\.01.*Caja remota/ }),
  );
  const secondAccount = screen.getByLabelText("Cuenta línea 2");
  await user.click(secondAccount);
  await user.click(
    screen.getByRole("option", { name: /4\.1\.01.*Ventas/ }),
  );
  await user.type(screen.getByLabelText("Debe línea 1"), "100");
  await user.type(screen.getByLabelText("Haber línea 2"), "100");

  expect(firstAccount).toHaveValue("1.1.01 · Caja remota");
  expect(
    screen.queryByText(
      "Esta cuenta ya no está disponible para contabilizar.",
    ),
  ).not.toBeInTheDocument();
  expect(screen.getByText("Listo para contabilizar")).toBeInTheDocument();
  expect(
    accountQueries.some((path) =>
      new URL(path, "http://pymes.test").searchParams.has("query"),
    ),
  ).toBe(true);
});

test("resolves an existing draft account outside the initial account page", async () => {
  const user = userEvent.setup();
  const draftID = "74900000-0000-4000-8000-000000000000";
  const remoteCash = {
    id: "74900000-0000-4000-8000-000000000001",
    code: "1.1.01",
    name: "Caja fuera de página",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "active",
    version: 2,
  };
  const sales = {
    ...remoteCash,
    id: "74900000-0000-4000-8000-000000000002",
    code: "4.1.01",
    name: "Ventas",
    account_type: "income",
    normal_balance: "credit",
  };
  const summary = {
    id: draftID,
    accounting_date: "2026-07-24",
    description: "Borrador con cuenta remota",
    currency: "ARS",
    functional_currency: "ARS",
    line_count: 2,
    total_debit: "100",
    total_credit: "100",
    posting_status: {
      state: "ready",
      difference: "0",
      issues: [],
    },
    updated_at: "2026-07-24T12:00:00Z",
    version: 3,
  };
  const detail = {
    ...summary,
    lines: [
      {
        id: "74900000-0000-4000-8000-000000000011",
        account_id: remoteCash.id,
        account_code: remoteCash.code,
        account_name: remoteCash.name,
        debit: "100",
        credit: "0",
      },
      {
        id: "74900000-0000-4000-8000-000000000012",
        account_id: sales.id,
        account_code: sales.code,
        account_name: sales.name,
        debit: "0",
        credit: "100",
      },
    ],
  };
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [summary], page: { total: 1 } };
      }
      if (path === `/api/v1/accounting/drafts/${draftID}`) {
        return detail;
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [sales], page: { total: 1 } };
      }
      if (path === `/api/v1/accounting/accounts/${remoteCash.id}`) {
        return remoteCash;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("tab", { name: /Borradores/ }));
  await user.click(
    screen.getByRole("row", {
      name: "Abrir borrador Borrador con cuenta remota",
    }),
  );
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      `/api/v1/accounting/accounts/${remoteCash.id}`,
      expect.objectContaining({ skipJSONContentType: true }),
    ),
  );
  await user.type(screen.getByLabelText("Detalle"), " actualizado");
  const editor = document.querySelector(".journal-editor");
  expect(editor).not.toBeNull();
  await waitFor(() =>
    expect(
      within(editor as HTMLElement).getByText("Listo para contabilizar"),
    ).toBeInTheDocument(),
  );
  expect(
    within(editor as HTMLElement).queryByText(
      "Esta cuenta ya no está disponible para contabilizar.",
    ),
  ).not.toBeInTheDocument();
});

test("keeps archived and non-postable draft accounts blocked after resolution", async () => {
  const user = userEvent.setup();
  const draftID = "74a00000-0000-4000-8000-000000000000";
  const archived = {
    id: "74a00000-0000-4000-8000-000000000001",
    code: "1.1.09",
    name: "Caja archivada",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "archived",
    version: 4,
  };
  const heading = {
    id: "74a00000-0000-4000-8000-000000000002",
    code: "4",
    name: "Ingresos",
    account_type: "income",
    normal_balance: "credit",
    monetary_classification: "non_monetary",
    postable: false,
    lifecycle_state: "active",
    version: 1,
  };
  const summary = {
    id: draftID,
    accounting_date: "2026-07-24",
    description: "Borrador bloqueado",
    currency: "ARS",
    functional_currency: "ARS",
    line_count: 2,
    total_debit: "100",
    total_credit: "100",
    posting_status: {
      state: "blocked",
      difference: "0",
      issues: ["account_archived", "account_not_postable"],
    },
    updated_at: "2026-07-24T12:00:00Z",
    version: 2,
  };
  const detail = {
    ...summary,
    lines: [
      {
        id: "74a00000-0000-4000-8000-000000000011",
        account_id: archived.id,
        account_code: archived.code,
        account_name: archived.name,
        debit: "100",
        credit: "0",
      },
      {
        id: "74a00000-0000-4000-8000-000000000012",
        account_id: heading.id,
        account_code: heading.code,
        account_name: heading.name,
        debit: "0",
        credit: "100",
      },
    ],
  };
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [summary], page: { total: 1 } };
    }
    if (path === `/api/v1/accounting/drafts/${draftID}`) {
      return detail;
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path === `/api/v1/accounting/accounts/${archived.id}`) {
      return archived;
    }
    if (path === `/api/v1/accounting/accounts/${heading.id}`) {
      return heading;
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("tab", { name: /Borradores/ }));
  await user.click(
    screen.getByRole("row", { name: "Abrir borrador Borrador bloqueado" }),
  );
  expect(
    await screen.findByText(
      "Esta cuenta ya no está disponible para contabilizar.",
    ),
  ).toBeInTheDocument();
  expect(
    await screen.findByText(
      "Esta cuenta es un rubro y no admite imputaciones.",
    ),
  ).toBeInTheDocument();
  await user.type(screen.getByLabelText("Detalle"), " actualizado");
  const editor = document.querySelector(".journal-editor");
  expect(editor).not.toBeNull();
  expect(
    within(editor as HTMLElement).getByText("Bloqueado"),
  ).toBeInTheDocument();
  expect(
    within(editor as HTMLElement).getByText(
      "Una de las cuentas fue archivada. Reemplazala antes de contabilizar.",
    ),
  ).toBeInTheDocument();
});

test("copies a manual entry without carrying its reference or source", async () => {
  const user = userEvent.setup();
  const cash = {
    id: "75000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const sales = {
    ...cash,
    id: "76000000-0000-4000-8000-000000000000",
    code: "4.1.01",
    name: "Ventas",
    account_type: "income",
    normal_balance: "credit",
  };
  const entry = {
    id: "77000000-0000-4000-8000-000000000000",
    entry_number: 91,
    accounting_date: "2026-07-23",
    reference: "FAC-0001",
    description: "Venta mostrador",
    functional_currency: "ARS",
    currency: "ARS",
    exchange_rate: "1",
    kind: "manual",
    posting_kind: "primary",
    created_by: "Ana",
    source_type: "manual_draft",
    source_id: "78000000-0000-4000-8000-000000000000",
    lines: [
      {
        id: "79000000-0000-4000-8000-000000000000",
        line_number: 1,
        account_id: cash.id,
        account_code: cash.code,
        account_name: cash.name,
        debit: "100",
        credit: "0",
      },
      {
        id: "7a000000-0000-4000-8000-000000000000",
        line_number: 2,
        account_id: sales.id,
        account_code: sales.code,
        account_name: sales.name,
        debit: "0",
        credit: "100",
      },
    ],
    total_debit: "100",
    total_credit: "100",
    created_at: "2026-07-23T12:00:00Z",
  };
  let copiedBody: Record<string, unknown> | undefined;
  renderAccounting("/accounting/journal", async (path, options) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [entry], page: { total: 1 } };
    }
    if (path === `/api/v1/accounting/journal-entries/${entry.id}`) {
      return entry;
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [cash, sales], page: { total: 2 } };
    }
    if (
      path === "/api/v1/accounting/drafts" &&
      options?.method === "POST"
    ) {
      copiedBody = JSON.parse(String(options.body));
      return {
        id: "7b000000-0000-4000-8000-000000000000",
        ...copiedBody,
        functional_currency: "ARS",
        exchange_rate: "1",
        kind: "manual",
        lines: entry.lines,
        total_debit: "100",
        total_credit: "100",
        posting_status: {
          state: "ready",
          difference: "0",
          issues: [],
        },
        created_by: "Ana",
        updated_by: "Ana",
        created_at: "2026-07-24T12:00:00Z",
        updated_at: "2026-07-24T12:00:00Z",
        version: 1,
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(
    await screen.findByRole("row", { name: "Abrir asiento 91" }),
  );
  await user.click(
    await screen.findByRole("button", { name: "Copiar como nuevo" }),
  );
  expect(screen.getByLabelText("Referencia")).toHaveValue("");
  expect(screen.getByLabelText("Detalle")).toHaveValue("Venta mostrador");
  await user.click(screen.getByRole("button", { name: "Guardar borrador" }));
  await waitFor(() => expect(copiedBody).toBeDefined());
  expect(copiedBody).not.toHaveProperty("reference");
  expect(copiedBody).not.toHaveProperty("source_type");
  expect(copiedBody).not.toHaveProperty("source_id");
});

test("reopens a foreign-currency draft using original transaction amounts", async () => {
  const user = userEvent.setup();
  const cash = {
    id: "7c000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const sales = {
    ...cash,
    id: "7d000000-0000-4000-8000-000000000000",
    code: "4.1.01",
    name: "Ventas",
    account_type: "income",
    normal_balance: "credit",
  };
  const summary = {
    id: "7e000000-0000-4000-8000-000000000000",
    accounting_date: "2026-07-24",
    description: "Ajuste USD",
    functional_currency: "ARS",
    currency: "USD",
    exchange_rate: "1000",
    exchange_rate_date: "2026-07-24",
    exchange_rate_source: "BNA",
    kind: "manual",
    line_count: 2,
    total_debit: "100000",
    total_credit: "100000",
    posting_status: { state: "ready", difference: "0", issues: [] },
    updated_by: "Ana",
    updated_at: "2026-07-24T12:00:00Z",
    version: 1,
  };
  const detail = {
    ...summary,
    lines: [
      {
        id: "7f000000-0000-4000-8000-000000000000",
        line_number: 1,
        account_id: cash.id,
        account_code: cash.code,
        account_name: cash.name,
        debit: "100000",
        credit: "0",
        transaction_currency: "USD",
        transaction_amount: "100",
        exchange_rate: "1000",
      },
      {
        id: "80000000-0000-4000-8000-000000000000",
        line_number: 2,
        account_id: sales.id,
        account_code: sales.code,
        account_name: sales.name,
        debit: "0",
        credit: "100000",
        transaction_currency: "USD",
        transaction_amount: "100",
        exchange_rate: "1000",
      },
    ],
    created_by: "Ana",
    created_at: "2026-07-24T11:00:00Z",
  };
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [summary], page: { total: 1 } };
    }
    if (path === `/api/v1/accounting/drafts/${summary.id}`) return detail;
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [cash, sales], page: { total: 2 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("tab", { name: /Borradores/ }));
  await user.click(
    screen.getByRole("row", { name: "Abrir borrador Ajuste USD" }),
  );
  expect(await screen.findByLabelText("Debe línea 1")).toHaveValue("100");
  expect(screen.getByLabelText("Haber línea 2")).toHaveValue("100");
  const rail = screen.getByText("Diferencia").closest(".journal-balance-rail");
  expect(rail).not.toBeNull();
  expect(within(rail as HTMLElement).getAllByText("USD 100,00")).toHaveLength(2);
  expect(
    within(rail as HTMLElement).getByText(/Equivalente funcional/),
  ).toHaveTextContent("ARS 100.000,00");
});

test("opens a journal entry from the keyboard and shows account names", async () => {
  const user = userEvent.setup();
  const account = {
    id: "51000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    parent_id: null,
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const entry = {
    id: "52000000-0000-4000-8000-000000000000",
    entry_number: 27,
    accounting_date: "2026-07-24",
    description: "Apertura de caja",
    currency: "ARS",
    source_type: "manual",
    lines: [
      {
        id: "53000000-0000-4000-8000-000000000000",
        line_number: 1,
        account_id: account.id,
        debit: "250.00",
        credit: "0",
        memo: "Turno mañana",
      },
    ],
    total_debit: "250.00",
    total_credit: "250.00",
    created_at: "2026-07-24T12:00:00Z",
  };
  const reversal = {
    ...entry,
    id: "54000000-0000-4000-8000-000000000000",
    entry_number: 28,
    description: "Reversa de apertura de caja",
    kind: "reversal",
    posting_kind: "reversal",
    source_type: "journal_entry",
    source_id: entry.id,
    reverses_entry_id: entry.id,
    reversal_reason: "Corrección de apertura",
    lines: entry.lines.map((line) => ({
      ...line,
      id: "55000000-0000-4000-8000-000000000000",
      debit: line.credit,
      credit: line.debit,
    })),
    created_at: "2026-07-24T13:00:00Z",
  };
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
        };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [entry], page: { total: 1 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [account], page: { total: 1 } };
      }
      if (
        path === `/api/v1/accounting/journal-entries/${entry.id}/reverse` &&
        options?.method === "POST"
      ) {
        return reversal;
      }
      if (path === `/api/v1/accounting/journal-entries/${reversal.id}`) {
        return reversal;
      }
      if (path === `/api/v1/accounting/journal-entries/${entry.id}`) {
        return entry;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const row = await screen.findByRole("row", { name: "Abrir asiento 27" });
  row.focus();
  await user.keyboard("{Enter}");
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  expect(screen.getByText("1.1.01 · Caja")).toBeInTheDocument();
  expect(screen.getByText("Turno mañana")).toBeInTheDocument();
  expect(
    request.mock.calls.some(
      ([path]) => path === `/api/v1/accounting/journal-entries/${entry.id}`,
    ),
  ).toBe(true);

  await user.click(screen.getByRole("button", { name: "Crear reversa" }));
  expect(
    screen.getByRole("heading", { name: "Crear reversa" }),
  ).toBeInTheDocument();
  expect(screen.getByLabelText("Fecha de reversa")).toHaveAttribute(
    "min",
    entry.accounting_date,
  );
  await user.type(
    screen.getByLabelText("Motivo"),
    "Corrección de apertura",
  );
  await user.click(screen.getByRole("button", { name: "Confirmar reversa" }));
  await waitFor(() => {
    const reverse = request.mock.calls.find(
      ([path, options]) =>
        path === `/api/v1/accounting/journal-entries/${entry.id}/reverse` &&
        options?.method === "POST",
    );
    expect(JSON.parse(String(reverse?.[1]?.body))).toEqual({
      accounting_date: expect.any(String),
      reason: "Corrección de apertura",
    });
  });
  expect(
    await screen.findByRole("heading", {
      name: "Reversa de apertura de caja",
    }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "Crear reversa" }),
  ).toBeInTheDocument();
});

test("groups original journal totals by transaction currency", async () => {
  const user = userEvent.setup();
  const entry = {
    id: "55500000-0000-4000-8000-000000000000",
    entry_number: 31,
    accounting_date: "2026-07-24",
    description: "Asiento multimoneda",
    currency: "USD",
    functional_currency: "ARS",
    source_type: "manual",
    lines: [
      {
        id: "55500000-0000-4000-8000-000000000001",
        line_number: 1,
        account_id: "55500000-0000-4000-8000-000000000011",
        account_code: "1.1.01",
        account_name: "Caja",
        debit: "200000",
        credit: "0",
        transaction_amount: "200",
        transaction_currency: "USD",
      },
      {
        id: "55500000-0000-4000-8000-000000000002",
        line_number: 2,
        account_id: "55500000-0000-4000-8000-000000000012",
        account_code: "4.1.01",
        account_name: "Ventas USD",
        debit: "0",
        credit: "100000",
        transaction_amount: "100",
        transaction_currency: "USD",
      },
      {
        id: "55500000-0000-4000-8000-000000000003",
        line_number: 3,
        account_id: "55500000-0000-4000-8000-000000000013",
        account_code: "4.3",
        account_name: "Diferencia de cambio",
        debit: "0",
        credit: "100000",
        transaction_amount: "100000",
        transaction_currency: "ARS",
      },
    ],
    total_debit: "200000",
    total_credit: "200000",
    created_at: "2026-07-24T12:00:00Z",
  };
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [entry], page: { total: 1 } };
    }
    if (path === `/api/v1/accounting/journal-entries/${entry.id}`) {
      return entry;
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [], page: { total: 0 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("row", { name: "Abrir asiento 31" }));
  const drawer = await screen.findByRole("dialog", {
    name: "Asiento multimoneda",
  });
  const footer = drawer.querySelector("footer");
  expect(footer).not.toBeNull();
  expect(
    within(footer as HTMLElement).getByText(/Debe original \(USD\)/),
  ).toHaveTextContent("USD 200,00");
  expect(
    within(footer as HTMLElement).getByText(/Haber original \(USD\)/),
  ).toHaveTextContent("USD 100,00");
  expect(
    within(footer as HTMLElement).getByText(/Debe original \(ARS\)/),
  ).toHaveTextContent("ARS 0,00");
  expect(
    within(footer as HTMLElement).getByText(/Haber original \(ARS\)/),
  ).toHaveTextContent("ARS 100.000,00");
});

test("does not offer a Diario reversal for an entry from a sale", async () => {
  const user = userEvent.setup();
  const entry = {
    id: "56000000-0000-4000-8000-000000000000",
    entry_number: 29,
    accounting_date: "2026-07-24",
    reference: "FC-A 0001-00000042",
    description: "Venta facturada",
    currency: "ARS",
    functional_currency: "ARS",
    source_type: "sale",
    source_id: "57000000-0000-4000-8000-000000000000",
    posting_kind: "primary",
    lines: [
      {
        id: "58000000-0000-4000-8000-000000000000",
        line_number: 1,
        account_id: "59000000-0000-4000-8000-000000000000",
        account_code: "1.2.01",
        account_name: "Clientes",
        debit: "1210",
        credit: "0",
      },
      {
        id: "5a000000-0000-4000-8000-000000000000",
        line_number: 2,
        account_id: "5b000000-0000-4000-8000-000000000000",
        account_code: "4.1",
        account_name: "Ventas",
        debit: "0",
        credit: "1210",
      },
    ],
    total_debit: "1210",
    total_credit: "1210",
    created_at: "2026-07-24T12:00:00Z",
  };
  renderAccounting("/accounting/journal", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      return {
        country_code: "AR",
        functional_currency: "ARS",
        timezone: "America/Argentina/Buenos_Aires",
      };
    }
    if (path.startsWith("/api/v1/accounting/journal-entries?")) {
      return { items: [entry], page: { total: 1 } };
    }
    if (path === `/api/v1/accounting/journal-entries/${entry.id}`) {
      return entry;
    }
    if (path.startsWith("/api/v1/accounting/drafts?")) {
      return { items: [], page: { total: 0 } };
    }
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return { items: [], page: { total: 0 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  await user.click(await screen.findByRole("row", { name: "Abrir asiento 29" }));
  const drawer = await screen.findByRole("dialog", {
    name: "Venta facturada",
  });
  expect(
    within(drawer).queryByRole("button", { name: "Crear reversa" }),
  ).not.toBeInTheDocument();
  expect(
    within(drawer).queryByRole("button", { name: "Copiar como nuevo" }),
  ).not.toBeInTheDocument();
});

test("a closed reconciliation is named, keyboard accessible and locked", async () => {
  const user = userEvent.setup();
  const financialAccount = {
    id: "61000000-0000-4000-8000-000000000000",
    ledger_account_id: "62000000-0000-4000-8000-000000000000",
    ledger_account_code: "1.1.02",
    ledger_account_name: "Banco Nación",
    account_type: "bank",
    name: "BNA cuenta corriente",
    currency: "ARS",
    archived: false,
    version: 1,
  };
  const ledgerAccount = {
    id: financialAccount.ledger_account_id,
    code: financialAccount.ledger_account_code,
    name: financialAccount.ledger_account_name,
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    parent_id: null,
    postable: true,
    lifecycle_state: "active",
    version: 1,
  };
  const lineID = "63000000-0000-4000-8000-000000000000";
  const reconciliation = {
    id: "64000000-0000-4000-8000-000000000000",
    account_id: financialAccount.id,
    period_start: "2026-07-01",
    statement_date: "2026-07-24",
    opening_balance: "0",
    statement_balance: "121.00",
    ledger_balance: "121.00",
    difference: "0",
    currency: "ARS",
    state: "completed",
    version: 2,
    matches: [
      {
        id: "65000000-0000-4000-8000-000000000000",
        statement_movement_id: "66000000-0000-4000-8000-000000000000",
        journal_line_id: lineID,
        statement_amount: "121.00",
        ledger_amount: "121.00",
        created_at: "2026-07-24T12:00:00Z",
      },
    ],
  };

  renderAccounting(
    "/accounting/reconciliation",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path.startsWith("/api/v1/accounting/financial-accounts?")) {
        return [financialAccount];
      }
      if (path.startsWith("/api/v1/accounting/reconciliations?")) {
        return { items: [reconciliation], page: { total: 1 } };
      }
      if (path === `/api/v1/accounting/reconciliations/${reconciliation.id}`) {
        return reconciliation;
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [ledgerAccount], page: { total: 1 } };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return {
          items: [
            {
              id: "67000000-0000-4000-8000-000000000000",
              entry_number: 52,
              accounting_date: "2026-07-24",
              description: "Cobro de cliente",
              currency: "ARS",
              source_type: "receipt",
              lines: [
                {
                  id: lineID,
                  line_number: 1,
                  account_id: ledgerAccount.id,
                  debit: "121.00",
                  credit: "0",
                  memo: "Transferencia",
                },
              ],
              total_debit: "121.00",
              total_credit: "121.00",
              created_at: "2026-07-24T12:00:00Z",
            },
          ],
          page: { total: 1 },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const row = await screen.findByRole("row", {
    name: "Abrir conciliación de BNA cuenta corriente",
  });
  expect(within(row).getByText("BNA cuenta corriente")).toBeInTheDocument();
  row.focus();
  await user.keyboard(" ");

  expect(await screen.findByText("Conciliación cerrada")).toBeInTheDocument();
  expect(screen.getByLabelText("Saldo inicial")).toBeDisabled();
  expect(screen.getByRole("button", { name: /Importar CSV/ })).toBeDisabled();
  expect(
    screen.queryByRole("button", { name: "Guardar cambios" }),
  ).not.toBeInTheDocument();
  expect(await screen.findByText(/Asiento 52 · .*Banco Nación/)).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Reabrir" })).toBeInTheDocument();
});

test("keeps the ledger account immutable when editing a financial account", async () => {
  const user = userEvent.setup();
  const financialAccount = {
    id: "68000000-0000-4000-8000-000000000000",
    ledger_account_id: "69000000-0000-4000-8000-000000000000",
    ledger_account_code: "1.1.02",
    ledger_account_name: "Banco Nación",
    account_type: "bank",
    name: "BNA cuenta corriente",
    currency: "ARS",
    institution_name: "Banco Nación",
    archived: false,
    version: 3,
  };
  const ledgerAccounts = [
    {
      id: financialAccount.ledger_account_id,
      code: financialAccount.ledger_account_code,
      name: financialAccount.ledger_account_name,
      account_type: "asset",
      normal_balance: "debit",
      monetary_classification: "monetary",
      postable: true,
      lifecycle_state: "active",
      version: 1,
    },
    {
      id: "6a000000-0000-4000-8000-000000000000",
      code: "1.1.03",
      name: "Otra cuenta bancaria",
      account_type: "asset",
      normal_balance: "debit",
      monetary_classification: "monetary",
      postable: true,
      lifecycle_state: "active",
      version: 1,
    },
  ];
  let savedBody: Record<string, unknown> | undefined;
  const { request } = renderAccounting(
    "/accounting/reconciliation",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path.startsWith("/api/v1/accounting/financial-accounts?")) {
        return [financialAccount];
      }
      if (
        path ===
          `/api/v1/accounting/financial-accounts/${financialAccount.id}` &&
        options?.method === "PUT"
      ) {
        const body = JSON.parse(String(options.body)) as Record<
          string,
          unknown
        >;
        savedBody = body;
        return {
          ...financialAccount,
          name: String(body.name),
          version: 4,
        };
      }
      if (path.startsWith("/api/v1/accounting/reconciliations?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: ledgerAccounts, page: { total: 2 } };
      }
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [], page: { total: 0 } };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: "Editar" }));
  const ledgerAccount = screen.getByLabelText("Cuenta contable");
  expect(ledgerAccount).toBeDisabled();
  expect(ledgerAccount).toHaveValue(financialAccount.ledger_account_id);
  expect(
    screen.getByText(
      "La cuenta contable no cambia; para usar otra, archivá ésta y creá una nueva.",
    ),
  ).toBeInTheDocument();
  await user.clear(screen.getByLabelText("Nombre"));
  await user.type(screen.getByLabelText("Nombre"), "BNA principal");
  await user.click(screen.getByRole("button", { name: "Guardar cuenta" }));

  await waitFor(() => {
    expect(savedBody).toEqual(
      expect.objectContaining({
        ledger_account_id: financialAccount.ledger_account_id,
        name: "BNA principal",
        version: 3,
      }),
    );
  });
  expect(
    request.mock.calls.find(
      ([path, options]) =>
        path ===
          `/api/v1/accounting/financial-accounts/${financialAccount.id}` &&
        options?.method === "PUT",
    )?.[1]?.headers,
  ).toEqual(
    expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
  );

  await user.click(
    await screen.findByRole("button", { name: /Cuenta financiera/ }),
  );
  expect(screen.getByLabelText("Cuenta contable")).toBeEnabled();
  expect(
    screen.queryByText(
      "La cuenta contable no cambia; para usar otra, archivá ésta y creá una nueva.",
    ),
  ).not.toBeInTheDocument();
});

test("shows a focused Libro Mayor and opens the exact journal entry", async () => {
  const user = userEvent.setup();
  const cash = accountSummaryFixture({
    parent_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
  });
  const entryID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
  const lineID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
  const { request } = renderAccounting(
    `/accounting/ledger?account_id=${cash.id}&from=2026-03-01&to=2026-04-30`,
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view"]);
      }
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: [cash],
          totals: { active: 1, archived: 0, trashed: 0 },
        };
      }
      if (path.startsWith("/api/v1/accounting/general-ledger?")) {
        return {
          account: cash,
          currency: "ARS",
          from: "2026-01-01",
          to: "2026-07-24",
          opening_balance: { amount: "1250.00", side: "debit" },
          closing_balance: { amount: "1500.50", side: "debit" },
          total_debit: "500.50",
          total_credit: "250.00",
          items: [
            {
              entry_id: entryID,
              line_id: lineID,
              entry_number: 42,
              line_number: 1,
              accounting_date: "2026-07-20",
              reference: "FAC-A-0001-00000042",
              origin: "sale",
              description: "Venta mostrador",
              memo: "Cobro por transferencia",
              debit: "500.50",
              credit: "0",
              balance: { amount: "1750.50", side: "debit" },
            },
          ],
          page: { total: 1, next_cursor: null },
        };
      }
      if (path === `/api/v1/accounting/journal-entries/${entryID}`) {
        return {
          id: entryID,
          entry_number: 42,
          accounting_date: "2026-07-20",
          description: "Venta mostrador",
          reference: "FAC-A-0001-00000042",
          currency: "ARS",
          functional_currency: "ARS",
          source_type: "sale",
          posting_kind: "primary",
          lines: [
            {
              id: lineID,
              account_id: cash.id,
              account_code: cash.code,
              account_name: cash.name,
              debit: "500.50",
              credit: "0",
              memo: "Cobro por transferencia",
            },
          ],
          total_debit: "500.50",
          total_credit: "500.50",
          created_at: "2026-07-20T14:30:00Z",
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(await screen.findByText("FAC-A-0001-00000042")).toBeInTheDocument();
  expect(screen.getByText("Activo · Naturaleza deudora")).toBeInTheDocument();
  const summary = screen.getByLabelText("Resumen del Mayor");
  expect(within(summary).getByText("Saldo inicial")).toBeInTheDocument();
  expect(within(summary).getByText("ARS 1.250,00")).toBeInTheDocument();
  expect(within(summary).getByText("Saldo final")).toBeInTheDocument();
  expect(within(summary).getByText("ARS 1.500,50")).toBeInTheDocument();
  expect(
    request.mock.calls.some(
      ([path]) =>
        String(path).startsWith("/api/v1/accounting/general-ledger?") &&
        String(path).includes(`account_id=${cash.id}`) &&
        String(path).includes("from=2026-03-01") &&
        String(path).includes("to=2026-04-30"),
    ),
  ).toBe(true);

  await user.click(
    screen.getByRole("row", { name: "Abrir asiento número 42" }),
  );
  expect(
    await screen.findByRole("dialog", { name: "Venta mostrador" }),
  ).toBeInTheDocument();
  expect(screen.getByText("Asiento Nº 42")).toBeInTheDocument();
});

test("keeps Mayor and Sumas y saldos out of Informes and places Ejercicios fifth", async () => {
  renderAccounting("/accounting/reports", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/financial-accounts?")) return [];
    if (path.startsWith("/api/v1/accounting/reports/journal?")) {
      return {
        report: "journal",
        from: "2026-01-01",
        to: "2026-07-24",
        currency: "ARS",
        rows: [],
        total_debit: "0",
        total_credit: "0",
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

  const navigation = screen.getByRole("navigation", {
    name: "Secciones contables",
  });
  expect(
    within(navigation)
      .getAllByRole("link")
      .map((link) => link.textContent),
  ).toEqual([
    "Plan de cuentas",
    "Diario",
    "Mayor",
    "Sumas y saldos",
    "Ejercicios",
    "Cobros y pagos",
    "Informes",
    "Conciliación",
    "Inflación",
  ]);
  expect(await screen.findByLabelText("Informe")).toBeInTheDocument();
  expect(
    screen.queryByRole("option", { name: "Libro Mayor" }),
  ).not.toBeInTheDocument();
  expect(
    screen.queryByRole("option", { name: "Balance de comprobación" }),
  ).not.toBeInTheDocument();
  expect(screen.getByLabelText("Informe")).toHaveValue("journal");
});

test("shows Ejercicios as a searchable table and expands its periods independently", async () => {
  const user = userEvent.setup();
  const summary = fiscalYearFixture();
  const detail = fiscalYearDetailFixture();
  const { request } = renderAccounting(
    "/accounting/fiscal-years",
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 1,
          version: 1,
          can_change_fiscal_year_start: true,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        return {
          items: [summary],
          page: { total: 1, next_cursor: null },
        };
      }
      if (path === `/api/v1/accounting/fiscal-years/${summary.id}`) {
        return detail;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(
    await screen.findByRole("heading", {
      name: "Ejercicios y períodos contables",
    }),
  ).toBeInTheDocument();
  const navigation = screen.getByRole("navigation", {
    name: "Secciones contables",
  });
  expect(
    within(navigation)
      .getAllByRole("link")
      .map((link) => link.textContent)
      .slice(0, 5),
  ).toEqual([
    "Plan de cuentas",
    "Diario",
    "Mayor",
    "Sumas y saldos",
    "Ejercicios",
  ]);
  expect(
    screen.getByRole("img", {
      name: /Calendario del ejercicio 2026: 0 bloqueados, 0 en cierre y 12 abiertos/,
    }),
  ).toBeInTheDocument();
  expect(screen.getByText("1 ejercicios")).toBeInTheDocument();

  await user.click(
    screen.getByRole("button", {
      name: "Desplegar períodos de 2026",
    }),
  );
  expect(await screen.findByText("Doce períodos")).toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

  await user.click(
    screen.getByRole("row", { name: "Abrir ejercicio 2026" }),
  );
  const drawer = await screen.findByRole("dialog", { name: "2026" });
  expect(within(drawer).getByText("Historial reciente")).toBeInTheDocument();
  expect(within(drawer).getByText("Cierre de control")).toBeInTheDocument();
  expect(within(drawer).getAllByText("Sin evaluar").length).toBeGreaterThan(0);
  expect(
    within(drawer).queryByRole("button", {
      name: "Iniciar cierre preliminar",
    }),
  ).not.toBeInTheDocument();

  await user.click(
    screen.getByRole("button", { name: "Cerrar ficha del ejercicio" }),
  );
  await user.click(
    screen.getByRole("button", { name: "En cierre" }),
  );
  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path]) =>
          String(path).startsWith("/api/v1/accounting/fiscal-years?") &&
          String(path).includes("state=closing"),
      ),
    ).toBe(true);
  });
});

test("does not let an old fiscal-year cursor response overwrite new filters", async () => {
  const user = userEvent.setup();
  const firstPage = fiscalYearFixture({ code: "2026" });
  const stalePage = fiscalYearFixture({
    id: "f0000000-0000-4000-8000-000000000002",
    code: "2025",
  });
  const filteredPage = fiscalYearFixture({
    id: "f0000000-0000-4000-8000-000000000003",
    code: "2024",
    state: "closing",
    period_counts: { open: 0, soft_closed: 12, locked: 0 },
  });
  let resolveStale:
    | ((value: {
        items: ReturnType<typeof fiscalYearFixture>[];
        page: { total: number; next_cursor: null };
      }) => void)
    | undefined;

  const { request } = renderAccounting(
    "/accounting/fiscal-years",
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 1,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        const search = new URL(path, "http://pymes.test").searchParams;
        if (search.get("cursor") === "older") {
          return new Promise((resolve) => {
            resolveStale = resolve;
          });
        }
        if (search.get("state") === "closing") {
          return {
            items: [filteredPage],
            page: { total: 1, next_cursor: null },
          };
        }
        return {
          items: [firstPage],
          page: { total: 2, next_cursor: "older" },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(
    await screen.findByRole("row", { name: "Abrir ejercicio 2026" }),
  ).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Siguiente" }));
  await waitFor(() => expect(resolveStale).toBeDefined());
  await user.click(screen.getByRole("button", { name: "En cierre" }));
  expect(
    await screen.findByRole("row", { name: "Abrir ejercicio 2024" }),
  ).toBeInTheDocument();

  await act(async () => {
    resolveStale?.({
      items: [stalePage],
      page: { total: 1, next_cursor: null },
    });
    await Promise.resolve();
  });

  expect(
    screen.getByRole("row", { name: "Abrir ejercicio 2024" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("row", { name: "Abrir ejercicio 2025" }),
  ).not.toBeInTheDocument();
  const filteredRequest = request.mock.calls
    .map(([path]) => String(path))
    .find((path) => path.includes("state=closing"));
  expect(filteredRequest).toBeDefined();
  expect(filteredRequest).not.toContain("cursor=");
});

test("keeps fiscal calendar mutations disabled when settings cannot load", async () => {
  renderAccounting("/accounting/fiscal-years", async (path) => {
    if (path === "/api/v1/session") {
      return session(["accounting:view", "accounting:manage"]);
    }
    if (path === "/api/v1/accounting/settings") {
      throw new Error("settings unavailable");
    }
    if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
      return { items: [], page: { total: 0, next_cursor: null } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(
    await screen.findByText(/Las altas y los cambios de calendario quedan deshabilitados/i),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("button", { name: "+ Nuevo ejercicio" }),
  ).toBeDisabled();
  expect(
    screen.queryByRole("button", { name: "Configurar" }),
  ).not.toBeInTheDocument();
  expect(screen.getByText(/Inicio fiscal · No disponible/i)).toBeInTheDocument();
});

test("presents legacy periods, checklist links and nested dialogs accessibly", async () => {
  const user = userEvent.setup();
  const legacyPeriod = {
    ...fiscalYearPeriodsFixture()[0],
    code: "LEGACY-2026",
    end_date: "2026-12-31",
    is_legacy: true,
    close_readiness: {
      status: "ready" as const,
      blocking_count: 0,
      evaluated_at: "2027-01-02T12:00:00Z",
      checks: [
        {
          code: "pending_drafts",
          label: "Borradores contables pendientes",
          status: "warning" as const,
          count: 2,
          target_path: "/accounting/journal?status=drafts",
        },
      ],
    },
  };
  const detail = fiscalYearDetailFixture({
    is_legacy: true,
    period_counts: { open: 1, soft_closed: 0, locked: 0 },
    annual_close_status: "not_ready",
    periods: [legacyPeriod],
    recent_events: [
      {
        id: "e0000000-0000-4000-8000-000000000009",
        event_type: "annual_close_draft",
        from_status: "ready",
        to_status: "draft",
        actor: "Ana Pérez",
        occurred_at: "2027-01-02T12:00:00Z",
      },
    ],
  });

  renderAccounting(
    "/accounting/fiscal-years",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 1,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        return {
          items: [detail],
          page: { total: 1, next_cursor: null },
        };
      }
      if (path === `/api/v1/accounting/fiscal-years/${detail.id}`) {
        return detail;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  expect(await screen.findByText("Heredado")).toBeInTheDocument();
  const expand = screen.getByRole("button", {
    name: "Desplegar períodos de 2026",
  });
  expand.focus();
  await user.keyboard("{Enter}");
  expect(await screen.findByText("Período heredado")).toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

  await user.click(
    screen.getByRole("row", { name: "Abrir ejercicio 2026" }),
  );
  const drawer = await screen.findByRole("dialog", { name: "2026" });
  expect(within(drawer).getByText("Calendario heredado")).toBeInTheDocument();
  expect(
    within(drawer).getByText(
      "Cierre anual listo → Borrador de cierre preparado",
    ),
  ).toBeInTheDocument();
  await user.click(
    within(drawer).getByRole("button", {
      name: "Mostrar controles de Enero 2026",
    }),
  );
  expect(
    within(drawer).getByRole("link", {
      name: "Borradores contables pendientes: revisar",
    }),
  ).toHaveAttribute("href", "/accounting/journal?status=drafts");

  await user.click(
    within(drawer).getByRole("button", {
      name: "Iniciar cierre preliminar",
    }),
  );
  expect(
    await screen.findByRole("dialog", {
      name: "Iniciar cierre de Enero 2026",
    }),
  ).toBeInTheDocument();
  await user.keyboard("{Escape}");
  expect(
    screen.queryByRole("dialog", {
      name: "Iniciar cierre de Enero 2026",
    }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("dialog", { name: "2026" })).toBeInTheDocument();
});

test("previews twelve monthly periods before creating an exercise", async () => {
  const user = userEvent.setup();
  const detail = fiscalYearDetailFixture();
  const { request } = renderAccounting(
    "/accounting/fiscal-years",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 4,
          version: 2,
          can_change_fiscal_year_start: true,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        return { items: [], page: { total: 0, next_cursor: null } };
      }
      if (
        path === "/api/v1/accounting/fiscal-years" &&
        options?.method === "POST"
      ) {
        return detail;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("button", { name: "+ Nuevo ejercicio" }),
  );
  await user.clear(screen.getByLabelText("Año de inicio"));
  await user.type(screen.getByLabelText("Año de inicio"), "2027");

  const preview = screen.getByLabelText("Vista previa de los doce períodos");
  expect(preview.children).toHaveLength(12);
  expect(screen.getByText("2027/2028")).toBeInTheDocument();
  expect(within(preview).getByText("ABR")).toBeInTheDocument();
  expect(within(preview).getByText("MAR")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Crear ejercicio" }));
  await waitFor(() => {
    const createCall = request.mock.calls.find(
      ([path, options]) =>
        path === "/api/v1/accounting/fiscal-years" &&
        options?.method === "POST",
    );
    expect(createCall).toBeDefined();
    expect(JSON.parse(String(createCall?.[1]?.body))).toEqual({
      start_year: 2027,
    });
  });
  expect(
    await screen.findByText(
      "El ejercicio y sus doce períodos quedaron preparados.",
    ),
  ).toBeInTheDocument();
});

test("changes a period state through an accessible audited dialog", async () => {
  const user = userEvent.setup();
  const summary = fiscalYearFixture();
  const detail = fiscalYearDetailFixture();
  const prompt = vi.spyOn(window, "prompt");
  const { request } = renderAccounting(
    "/accounting/fiscal-years",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 1,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        return {
          items: [summary],
          page: { total: 1, next_cursor: null },
        };
      }
      if (path === `/api/v1/accounting/fiscal-years/${summary.id}`) {
        return detail;
      }
      if (
        path ===
          `/api/v1/accounting/periods/${detail.periods[0].id}/transition` &&
        options?.method === "POST"
      ) {
        return {
          ...detail.periods[0],
          state: "soft_closed",
          version: 2,
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("row", { name: "Abrir ejercicio 2026" }),
  );
  await user.click(
    screen.getByRole("button", {
      name: "Mostrar controles de Enero 2026",
    }),
  );
  await user.click(
    screen.getByRole("button", { name: "Iniciar cierre preliminar" }),
  );

  const dialog = await screen.findByRole("dialog", {
    name: "Iniciar cierre de Enero 2026",
  });
  expect(prompt).not.toHaveBeenCalled();
  expect(
    within(dialog).getByText(/sólo podrán contabilizarse ajustes autorizados/i),
  ).toBeInTheDocument();
  await user.type(within(dialog).getByLabelText("Motivo"), "Cierre mensual");
  await user.click(
    within(dialog).getByRole("button", { name: "Confirmar cambio" }),
  );

  await waitFor(() => {
    const transitionCall = request.mock.calls.find(
      ([path, options]) =>
        path ===
          `/api/v1/accounting/periods/${detail.periods[0].id}/transition` &&
        options?.method === "POST",
    );
    expect(transitionCall).toBeDefined();
    expect(JSON.parse(String(transitionCall?.[1]?.body))).toEqual({
      desired_state: "soft_closed",
      version: 1,
      reason: "Cierre mensual",
    });
  });
});

test("handles an annual close that does not require a journal entry", async () => {
  const user = userEvent.setup();
  const periods = fiscalYearPeriodsFixture([
    ...Array(11).fill("locked"),
    "soft_closed",
  ]);
  const detail = fiscalYearDetailFixture({
    state: "closing",
    period_counts: { open: 0, soft_closed: 1, locked: 11 },
    annual_close_status: "ready",
    periods,
    capabilities: {
      can_prepare_annual_close: true,
      can_reopen: false,
      blockers: [],
    },
  });

  renderAccounting(
    "/accounting/fiscal-years",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path === "/api/v1/accounting/settings") {
        return {
          country_code: "AR",
          functional_currency: "ARS",
          timezone: "America/Argentina/Buenos_Aires",
          fiscal_year_start_month: 1,
        };
      }
      if (path.startsWith("/api/v1/accounting/fiscal-years?")) {
        return {
          items: [detail],
          page: { total: 1, next_cursor: null },
        };
      }
      if (path === `/api/v1/accounting/fiscal-years/${detail.id}`) {
        return detail;
      }
      if (
        path ===
          `/api/v1/accounting/fiscal-years/${detail.id}/annual-close-draft` &&
        options?.method === "POST"
      ) {
        return {
          fiscal_year: {
            ...detail,
            annual_close_status: "not_required",
          },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("row", { name: "Abrir ejercicio 2026" }),
  );
  await user.click(
    screen.getByRole("button", { name: "Preparar cierre anual" }),
  );
  const dialog = await screen.findByRole("dialog", {
    name: "Preparar cierre 2026",
  });
  await user.click(
    within(dialog).getByRole("button", { name: "Generar borrador" }),
  );

  expect(
    await screen.findByText(
      "El ejercicio no tiene saldos temporales: no se requiere asiento de cierre.",
    ),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("link", { name: "Abrir en Diario" }),
  ).not.toBeInTheDocument();
});

test("filters, paginates and exports the selected Libro Mayor", async () => {
  const user = userEvent.setup();
  const cash = accountSummaryFixture();
  const createObjectURL = vi
    .spyOn(URL, "createObjectURL")
    .mockReturnValue("blob:ledger");
  const revokeObjectURL = vi
    .spyOn(URL, "revokeObjectURL")
    .mockImplementation(() => undefined);
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, "click")
    .mockImplementation(() => undefined);
  const { request, requestResponse } = renderAccounting(
    `/accounting/ledger?account_id=${cash.id}`,
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: [cash],
          totals: { active: 1, archived: 0, trashed: 0 },
        };
      }
      if (path.startsWith("/api/v1/accounting/general-ledger?")) {
        const nextPage = path.includes("cursor=next-page");
        return {
          account: cash,
          currency: "ARS",
          from: "2026-01-01",
          to: "2026-07-24",
          opening_balance: { amount: "0", side: "zero" },
          closing_balance: { amount: "0", side: "zero" },
          total_debit: "0",
          total_credit: "0",
          items: [],
          page: {
            total: 51,
            next_cursor: nextPage ? null : "next-page",
          },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
    async () =>
      new Response("ledger", {
        status: 200,
        headers: {
          "content-type": "text/csv",
          "content-disposition": 'attachment; filename="mayor-caja.csv"',
        },
      }),
  );

  await screen.findByText("51 movimientos · página 1");
  await user.selectOptions(screen.getByLabelText("Origen del Mayor"), "sale");
  await user.type(
    screen.getByLabelText("Buscar movimientos del Mayor"),
    "factura 42",
  );
  await user.click(screen.getByRole("button", { name: "Siguiente" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path]) =>
          String(path).startsWith("/api/v1/accounting/general-ledger?") &&
          String(path).includes("origin=sale") &&
          String(path).includes("query=factura+42") &&
          String(path).includes("cursor=next-page"),
      ),
    ).toBe(true);
  });
  expect(await screen.findByText("51 movimientos · página 2")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "CSV" }));
  await waitFor(() => expect(requestResponse).toHaveBeenCalledTimes(1));
  const exportPath = String(requestResponse.mock.calls[0]?.[0]);
  expect(exportPath).toContain("/api/v1/accounting/general-ledger/export?");
  expect(exportPath).toContain(`account_id=${cash.id}`);
  expect(exportPath).toContain("origin=sale");
  expect(exportPath).toContain("query=factura+42");
  expect(exportPath).toContain("format=csv");
  expect(exportPath).not.toContain("cursor=");
  expect(click).toHaveBeenCalled();
  createObjectURL.mockRestore();
  revokeObjectURL.mockRestore();
  click.mockRestore();
});

test("shows Balance de sumas y saldos with its six debit and credit columns", async () => {
  const user = userEvent.setup();
  const archivedIncome = {
    account_id: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
    code: "4.1",
    name: "Ventas de mercaderías",
    account_class: "income",
    normal_balance: "credit",
    lifecycle_state: "archived",
    path: ["Ingresos", "Ventas de mercaderías"],
    opening_balance: { amount: "0.00", side: "zero" },
    debit: "0.00",
    credit: "950.00",
    closing_balance: { amount: "950.00", side: "credit" },
  };
  const { request } = renderAccounting("/accounting/trial-balance", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/trial-balance?")) {
      return trialBalanceFixture({
        items: [trialBalanceFixture().items[0], archivedIncome],
        page: { total: 2, next_cursor: null },
      });
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(
    await screen.findByRole("heading", { name: "Balance de sumas y saldos" }),
  ).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Código" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Cuenta" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Saldo inicial" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Movimientos" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Saldo final" })).toBeInTheDocument();
  expect(screen.getAllByRole("columnheader", { name: "Deudor" })).toHaveLength(2);
  expect(screen.getAllByRole("columnheader", { name: "Acreedor" })).toHaveLength(2);
  expect(screen.getByRole("columnheader", { name: "Debe" })).toBeInTheDocument();
  expect(screen.getByRole("columnheader", { name: "Haber" })).toBeInTheDocument();
  expect(screen.getByText("Activo / Disponibilidades")).toBeInTheDocument();
  expect(screen.getByText("Archivada")).toBeInTheDocument();
  expect(screen.getAllByText("Equilibrado")).toHaveLength(3);
  expect(
    request.mock.calls.some(
      ([path]) =>
        String(path).startsWith("/api/v1/accounting/trial-balance?") &&
        String(path).includes("include_zero=false"),
    ),
  ).toBe(true);

  await user.type(
    screen.getByRole("searchbox", { name: "Buscar cuentas en Sumas y saldos" }),
    "Caja",
  );
  expect(await screen.findAllByText("Vista filtrada")).toHaveLength(3);
  await user.selectOptions(screen.getByLabelText("Clase de cuenta"), "income");
  await user.click(screen.getByRole("button", { name: "Todas" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path]) =>
          String(path).startsWith("/api/v1/accounting/trial-balance?") &&
          String(path).includes("query=Caja") &&
          String(path).includes("account_class=income") &&
          String(path).includes("include_zero=true"),
      ),
    ).toBe(true);
  });
});

test("marks the inconsistent control in Sumas y saldos without hiding its difference", async () => {
  renderAccounting("/accounting/trial-balance", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/trial-balance?")) {
      return trialBalanceFixture({
        controls: {
          opening_difference: "0.00",
          movement_difference: "1.25",
          closing_difference: "0.00",
        },
      });
    }
    throw new Error(`unexpected request ${path}`);
  });

  const control = await screen.findByLabelText("Control de sumas y saldos");
  const movements = within(control)
    .getByText("Movimientos")
    .closest("section");
  expect(movements).not.toBeNull();
  expect(within(movements as HTMLElement).getByText("Diferencia")).toBeInTheDocument();
  expect(within(movements as HTMLElement).getByText("ARS 1,25")).toBeInTheDocument();
  expect(within(control).getAllByText("Equilibrado")).toHaveLength(2);
});

test("localizes an error from Sumas y saldos instead of exposing the backend message", async () => {
  renderAccounting("/accounting/trial-balance", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/trial-balance?")) {
      throw new HttpError(
        "invalid period",
        400,
        JSON.stringify({
          error: {
            code: "ACCOUNTING_INVALID_PERIOD",
            message: "invalid period",
          },
        }),
      );
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(await screen.findByRole("alert")).toHaveTextContent(
    "El rango de fechas no es válido. Revisá Desde y Hasta.",
  );
  expect(screen.queryByText("invalid period")).not.toBeInTheDocument();
});

test("distinguishes a period without entries from filters without matches", async () => {
  const user = userEvent.setup();
  renderAccounting("/accounting/trial-balance", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/trial-balance?")) {
      return trialBalanceFixture({
        items: [],
        page: { total: 0, next_cursor: null },
      });
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(
    await screen.findByText(
      "No hay saldos ni movimientos contabilizados en el período.",
    ),
  ).toBeInTheDocument();

  await user.type(
    screen.getByRole("searchbox", { name: "Buscar cuentas en Sumas y saldos" }),
    "inexistente",
  );
  expect(
    await screen.findByText("No hay cuentas que coincidan con los filtros."),
  ).toBeInTheDocument();
});

test("paginates, exports and opens the selected account in Mayor with its period", async () => {
  const user = userEvent.setup();
  const cash = accountSummaryFixture();
  const createObjectURL = vi
    .spyOn(URL, "createObjectURL")
    .mockReturnValue("blob:trial-balance");
  const revokeObjectURL = vi
    .spyOn(URL, "revokeObjectURL")
    .mockImplementation(() => undefined);
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, "click")
    .mockImplementation(() => undefined);
  const { request, requestResponse } = renderAccounting(
    "/accounting/trial-balance",
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path.startsWith("/api/v1/accounting/trial-balance?")) {
        const nextPage = path.includes("cursor=next-page");
        return trialBalanceFixture({
          page: { total: 51, next_cursor: nextPage ? null : "next-page" },
        });
      }
      if (path.startsWith("/api/v1/accounting/accounts/tree?")) {
        return {
          items: [cash],
          totals: { active: 1, archived: 0, trashed: 0 },
        };
      }
      if (path.startsWith("/api/v1/accounting/general-ledger?")) {
        return {
          account: cash,
          currency: "ARS",
          from: "2026-02-01",
          to: "2026-05-31",
          opening_balance: { amount: "0", side: "zero" },
          closing_balance: { amount: "0", side: "zero" },
          total_debit: "0",
          total_credit: "0",
          items: [],
          page: { total: 0, next_cursor: null },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
    async () =>
      new Response("trial-balance", {
        status: 200,
        headers: {
          "content-type": "text/csv",
          "content-disposition": 'attachment; filename="sumas-y-saldos.csv"',
        },
      }),
  );

  await screen.findByText("51 cuentas · página 1");
  fireEvent.change(screen.getByLabelText("Sumas y saldos desde"), {
    target: { value: "2026-02-01" },
  });
  fireEvent.change(screen.getByLabelText("Sumas y saldos hasta"), {
    target: { value: "2026-05-31" },
  });
  await user.click(screen.getByRole("button", { name: "Siguiente" }));

  await waitFor(() => {
    expect(
      request.mock.calls.some(
        ([path]) =>
          String(path).startsWith("/api/v1/accounting/trial-balance?") &&
          String(path).includes("from=2026-02-01") &&
          String(path).includes("to=2026-05-31") &&
          String(path).includes("cursor=next-page"),
      ),
    ).toBe(true);
  });
  expect(await screen.findByText("51 cuentas · página 2")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "CSV" }));
  await waitFor(() => expect(requestResponse).toHaveBeenCalledTimes(1));
  const exportPath = String(requestResponse.mock.calls[0]?.[0]);
  expect(exportPath).toContain("/api/v1/accounting/trial-balance/export?");
  expect(exportPath).toContain("from=2026-02-01");
  expect(exportPath).toContain("to=2026-05-31");
  expect(exportPath).toContain("include_zero=false");
  expect(exportPath).not.toContain("cursor=");
  expect(click).toHaveBeenCalled();

  fireEvent.keyDown(
    screen.getByRole("row", { name: "Abrir Mayor de 1.1.01 Caja" }),
    { key: "Enter" },
  );
  await waitFor(() => {
    expect(window.location.pathname).toBe("/accounting/ledger");
    expect(window.location.search).toContain(`account_id=${cash.id}`);
    expect(window.location.search).toContain("from=2026-02-01");
    expect(window.location.search).toContain("to=2026-05-31");
  });
  createObjectURL.mockRestore();
  revokeObjectURL.mockRestore();
  click.mockRestore();
});

test("exports Libro Diario by default with the active date filters", async () => {
  const user = userEvent.setup();
  const createObjectURL = vi
    .spyOn(URL, "createObjectURL")
    .mockReturnValue("blob:report");
  const revokeObjectURL = vi
    .spyOn(URL, "revokeObjectURL")
    .mockImplementation(() => undefined);
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, "click")
    .mockImplementation(() => undefined);
  const { requestResponse } = renderAccounting(
    "/accounting/reports",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/financial-accounts?")) {
        return [];
      }
      if (path.startsWith("/api/v1/accounting/reports/journal?")) {
        return {
          report: "journal",
          from: "2026-01-01",
          to: "2026-07-24",
          currency: "ARS",
          rows: [
            {
              key: "cash",
              label: "Caja",
              debit: "9007199254740993.01",
              credit: "0.00",
              balance: "9007199254740993.01",
            },
          ],
          total_debit: "9007199254740993.01",
          total_credit: "0.00",
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
    async () =>
      new Response("pdf", {
        status: 200,
        headers: {
          "content-type": "application/pdf",
          "content-disposition": 'attachment; filename="balance.pdf"',
        },
      }),
  );

  expect(
    await screen.findAllByText("ARS 9.007.199.254.740.993,01"),
  ).toHaveLength(3);
  await user.click(screen.getByRole("button", { name: "PDF" }));

  await waitFor(() => expect(requestResponse).toHaveBeenCalledTimes(1));
  expect(String(requestResponse.mock.calls[0]?.[0])).toMatch(
    /^\/api\/v1\/accounting\/reports\/journal\/export\?/,
  );
  expect(String(requestResponse.mock.calls[0]?.[0])).toContain("format=pdf");
  expect(String(requestResponse.mock.calls[0]?.[0])).toContain("from=");
  expect(String(requestResponse.mock.calls[0]?.[0])).toContain("to=");
  await waitFor(() => expect(createObjectURL).toHaveBeenCalled());
  expect(click).toHaveBeenCalled();
  createObjectURL.mockRestore();
  revokeObjectURL.mockRestore();
  click.mockRestore();
});

test("filters the financial activity report by the selected cash or bank account", async () => {
  const user = userEvent.setup();
  const financialAccount = {
    id: "70000000-0000-4000-8000-000000000000",
    ledger_account_id: "70100000-0000-4000-8000-000000000000",
    ledger_account_code: "1.1.02",
    ledger_account_name: "Banco Nación",
    account_type: "bank",
    name: "BNA cuenta corriente",
    currency: "ARS",
    archived: false,
    version: 1,
  };
  const { request } = renderAccounting(
    "/accounting/reports",
    async (path) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view"]);
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/financial-accounts?")) {
        return [financialAccount];
      }
      if (path.startsWith("/api/v1/accounting/reports/journal?")) {
        return {
          report: "journal",
          from: "2026-01-01",
          to: "2026-07-24",
          currency: "ARS",
          rows: [],
          total_debit: "0",
          total_credit: "0",
        };
      }
      if (path.startsWith("/api/v1/accounting/reports/financial-activity?")) {
        return {
          report: "financial-activity",
          from: "2026-01-01",
          to: "2026-07-24",
          currency: "ARS",
          rows: [
            {
              key: "closing",
              label: "Saldo Banco Nación",
              debit: "1500.00",
              credit: "250.00",
              balance: "1250.00",
            },
          ],
          total_debit: "1500.00",
          total_credit: "250.00",
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.selectOptions(
    await screen.findByLabelText("Informe"),
    "financial-activity",
  );
  expect(
    await screen.findByLabelText("Cuenta financiera del informe"),
  ).toHaveValue(financialAccount.id);
  expect(await screen.findByText("Saldo Banco Nación")).toBeInTheDocument();
  expect(
    request.mock.calls.some(
      ([path]) =>
        String(path).includes("/reports/financial-activity?") &&
        String(path).includes(
          `financial_account_id=${financialAccount.id}`,
        ),
    ),
  ).toBe(true);
});

test("lists open items in read-only mode without settlement actions", async () => {
  const openItem = {
    id: "71000000-0000-4000-8000-000000000000",
    item_type: "receivable",
    party_id: "72000000-0000-4000-8000-000000000000",
    account_id: "73000000-0000-4000-8000-000000000000",
    origin_entry_id: "74000000-0000-4000-8000-000000000000",
    source_type: "fiscal_voucher",
    source_id: "75000000-0000-4000-8000-000000000000",
    issued_at: "2026-07-01",
    due_date: "2026-07-31",
    currency: "ARS",
    original_amount: "1000.00",
    original_functional_amount: "1000.00",
    open_amount: "625.50",
    open_functional_amount: "625.50",
  };

  renderAccounting("/accounting/open-items", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path.startsWith("/api/v1/accounting/open-items?")) {
      return { items: [openItem], page: { total: 1 } };
    }
    throw new Error(`unexpected request ${path}`);
  });

  expect(await screen.findAllByText("ARS 625,50")).not.toHaveLength(0);
  expect(screen.getByText("Comprobante fiscal · 75000000")).toBeInTheDocument();
  expect(screen.getByText(/modo lectura/i)).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Registrar cobro" }),
  ).not.toBeInTheDocument();
});

test("registers an exact receipt with a durable idempotency key", async () => {
  const user = userEvent.setup();
  const openItem = {
    id: "81000000-0000-4000-8000-000000000000",
    item_type: "receivable",
    party_id: "82000000-0000-4000-8000-000000000000",
    account_id: "83000000-0000-4000-8000-000000000000",
    origin_entry_id: "84000000-0000-4000-8000-000000000000",
    source_type: "sale",
    source_id: "85000000-0000-4000-8000-000000000000",
    issued_at: "2026-07-01",
    due_date: "2026-07-31",
    currency: "ARS",
    original_amount: "9007199254740993.01",
    original_functional_amount: "9007199254740993.01",
    open_amount: "9007199254740993.01",
    open_functional_amount: "9007199254740993.01",
  };
  let settled = false;
  const entry = {
    id: "86000000-0000-4000-8000-000000000000",
    entry_number: 84,
    accounting_date: "2026-07-24",
    description: "Cobro de cliente",
    currency: "ARS",
    source_type: "receipt",
    lines: [],
    total_debit: "1250.50",
    total_credit: "1250.50",
    created_at: "2026-07-24T12:00:00Z",
  };
  const { request } = renderAccounting(
    "/accounting/open-items",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path.startsWith("/api/v1/accounting/open-items?")) {
        return {
          items: settled ? [] : [openItem],
          page: { total: settled ? 0 : 1 },
        };
      }
      if (
        path === "/api/v1/accounting/receipts" &&
        options?.method === "POST"
      ) {
        settled = true;
        return entry;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(
    await screen.findByRole("button", { name: "Registrar cobro" }),
  );
  const amount = screen.getByLabelText("Importe del movimiento");
  await user.clear(amount);
  await user.type(amount, "1250.50");
  await user.selectOptions(
    screen.getByLabelText("Medio del movimiento"),
    "cash",
  );
  await user.clear(screen.getByLabelText("Fecha contable del movimiento"));
  await user.type(
    screen.getByLabelText("Fecha contable del movimiento"),
    "2026-07-24",
  );
  await user.click(screen.getByRole("button", { name: "Contabilizar cobro" }));

  expect(await screen.findByText("Cobro contabilizado")).toBeInTheDocument();
  expect(screen.getByText(/Asiento Nº 84/)).toBeInTheDocument();
  const receiptCall = request.mock.calls.find(
    ([path, options]) =>
      path === "/api/v1/accounting/receipts" && options?.method === "POST",
  );
  expect(receiptCall?.[1]?.headers).toEqual({
    "Idempotency-Key": expect.stringMatching(/^accounting-receipt-/),
  });
  expect(JSON.parse(String(receiptCall?.[1]?.body))).toEqual({
    open_item_id: openItem.id,
    accounting_date: "2026-07-24",
    payment_method: "cash",
    amount: "1250.50",
    exchange_rate: "1",
    exchange_rate_date: "2026-07-24",
    exchange_rate_source: "moneda funcional",
  });
});

test("retries a supplier payment with the same key and exposes replay state", async () => {
  const user = userEvent.setup();
  const openItem = {
    id: "91000000-0000-4000-8000-000000000000",
    item_type: "payable",
    party_id: "92000000-0000-4000-8000-000000000000",
    account_id: "93000000-0000-4000-8000-000000000000",
    origin_entry_id: "94000000-0000-4000-8000-000000000000",
    source_type: "fiscal_purchase",
    source_id: "95000000-0000-4000-8000-000000000000",
    issued_at: "2026-07-01",
    due_date: null,
    currency: "USD",
    original_amount: "10.25",
    original_functional_amount: "12300.00",
    open_amount: "10.25",
    open_functional_amount: "12300.00",
  };
  const entry = {
    id: "96000000-0000-4000-8000-000000000000",
    entry_number: 96,
    accounting_date: "2026-07-24",
    description: "Pago a proveedor",
    currency: "ARS",
    source_type: "supplier_payment",
    lines: [],
    total_debit: "12300.00",
    total_credit: "12300.00",
    created_at: "2026-07-24T12:00:00Z",
  };
  let attempts = 0;
  const { request } = renderAccounting(
    "/accounting/open-items",
    async (path, options) => {
      if (path === "/api/v1/session") {
        return session(["accounting:view", "accounting:manage"]);
      }
      if (path.startsWith("/api/v1/accounting/open-items?")) {
        return path.includes("item_type=payable")
          ? { items: [openItem], page: { total: 1 } }
          : { items: [], page: { total: 0 } };
      }
      if (
        path === "/api/v1/accounting/supplier-payments" &&
        options?.method === "POST"
      ) {
        attempts += 1;
        if (attempts === 1) throw new Error("La respuesta no llegó.");
        return entry;
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  await user.click(await screen.findByRole("button", { name: "A pagar" }));
  await user.click(
    await screen.findByRole("button", { name: "Registrar pago" }),
  );
  await user.type(screen.getByLabelText("Cotización del movimiento"), "1200");
  await user.type(screen.getByLabelText("Fuente de la cotización"), "BNA");
  await user.click(screen.getByRole("button", { name: "Contabilizar pago" }));

  expect(await screen.findByText("La respuesta no llegó.")).toBeInTheDocument();
  await user.click(
    screen.getByRole("button", { name: "Reintentar sin duplicar" }),
  );

  expect(await screen.findByText("Reintento idempotente")).toBeInTheDocument();
  expect(
    screen.getByText(/recuperada sin generar un asiento duplicado/i),
  ).toBeInTheDocument();
  const paymentCalls = request.mock.calls.filter(
    ([path, options]) =>
      path === "/api/v1/accounting/supplier-payments" &&
      options?.method === "POST",
  );
  expect(paymentCalls).toHaveLength(2);
  expect(paymentCalls[0]?.[1]?.headers).toEqual(paymentCalls[1]?.[1]?.headers);
});
