import type { HttpClient } from "@devpablocristo/platform-http";
import { render, screen, waitFor, within } from "@testing-library/react";
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
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return {
        items: [
          {
            id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
            code: "1.1.01",
            name: "Caja",
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

  renderAccounting("/accounting/accounts", async (path) => {
    if (path === "/api/v1/session") return session(["accounting:view"]);
    if (path === "/api/v1/accounting/account-mappings") return [];
    if (path.startsWith("/api/v1/accounting/accounts?")) {
      return {
        items: [
          { id: activoID, code: "1", name: "Activo", account_type: "asset", normal_balance: "debit", monetary_classification: "not_applicable", postable: false, lifecycle_state: "active", version: 1 },
          { id: disponibilidadesID, code: "1.1", name: "Disponibilidades", parent_id: activoID, account_type: "asset", normal_balance: "debit", monetary_classification: "not_applicable", postable: false, lifecycle_state: "active", version: 1 },
          { id: cajaID, code: "1.1.01", name: "Caja", parent_id: disponibilidadesID, account_type: "asset", normal_balance: "debit", monetary_classification: "monetary", postable: true, lifecycle_state: "active", version: 1 },
          { id: bancosID, code: "1.1.02", name: "Bancos", parent_id: disponibilidadesID, account_type: "asset", normal_balance: "debit", monetary_classification: "monetary", postable: true, lifecycle_state: "active", version: 1 },
        ],
        page: { total: 4 },
      };
    }
    throw new Error(`unexpected request ${path}`);
  });

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

test("edits the account tree, updates mappings and follows the account cursor", async () => {
  const user = userEvent.setup();
  const rootAccount = {
    id: "10000000-0000-4000-8000-000000000000",
    code: "1",
    name: "Activo",
    account_type: "asset",
    normal_balance: "debit",
    monetary_classification: "monetary",
    parent_id: null,
    postable: false,
    lifecycle_state: "active",
    version: 1,
  };
  const cashAccount = {
    ...rootAccount,
    id: "11000000-0000-4000-8000-000000000000",
    code: "1.1.01",
    name: "Caja",
    parent_id: rootAccount.id,
    postable: true,
    version: 3,
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
        options?.method === "PUT"
      ) {
        const body = JSON.parse(String(options.body));
        return { ...cashAccount, ...body, version: 4 };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        if (path.includes("cursor=")) {
          return { items: [], page: { total: allAccounts.length } };
        }
        return {
          items: allAccounts,
          page: { total: allAccounts.length, next_cursor: "cursor-two" },
        };
      }
      throw new Error(`unexpected request ${path}`);
    },
  );

  const accountTable = await screen.findByRole("table");
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
  await user.click(screen.getByRole("button", { name: "Editar" }));
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
  await user.selectOptions(
    screen.getByLabelText("Cuenta del mapping cash"),
    bankAccount.id,
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

  await user.click(screen.getByRole("button", { name: "Siguiente" }));
  await waitFor(() =>
    expect(
      request.mock.calls.some(([path]) => String(path).includes("cursor=cursor-two")),
    ).toBe(true),
  );
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
  await user.selectOptions(screen.getByLabelText("Cuenta línea 1"), debitAccount.id);
  await user.type(screen.getByLabelText("Debe línea 1"), "100.00");
  await user.selectOptions(screen.getByLabelText("Cuenta línea 2"), creditAccount.id);
  await user.type(screen.getByLabelText("Haber línea 2"), "60.00");
  await user.click(screen.getByRole("button", { name: "＋ Agregar línea" }));
  await user.selectOptions(screen.getByLabelText("Cuenta línea 3"), creditAccount.id);
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
  const { request } = renderAccounting(
    "/accounting/journal",
    async (path) => {
      if (path === "/api/v1/session") return session(["accounting:view"]);
      if (path.startsWith("/api/v1/accounting/journal-entries?")) {
        return { items: [entry], page: { total: 1 } };
      }
      if (path.startsWith("/api/v1/accounting/drafts?")) {
        return { items: [], page: { total: 0 } };
      }
      if (path.startsWith("/api/v1/accounting/accounts?")) {
        return { items: [account], page: { total: 1 } };
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

test("exports the selected report as PDF with the active date filters", async () => {
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
      if (path.startsWith("/api/v1/accounting/reports/trial-balance?")) {
        return {
          report: "trial-balance",
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
    /^\/api\/v1\/accounting\/reports\/trial-balance\/export\?/,
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
      if (path.startsWith("/api/v1/accounting/reports/trial-balance?")) {
        return {
          report: "trial-balance",
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
