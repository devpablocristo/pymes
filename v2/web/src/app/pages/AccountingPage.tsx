import {
  type CSSProperties,
  type ChangeEvent,
  type FormEvent,
  type ReactNode,
  type RefObject,
  Fragment,
  useId,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { normalizeHttpError } from "@devpablocristo/platform-http";
import {
  NavLink,
  Navigate,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import type { components } from "../../api/schema.generated";
import { useProductApi } from "../../api/ProductApiContext";
import { createIdempotencyKey } from "../../api/idempotency";
import { calendarDate } from "../calendarDate";
import { ChevronRightIcon } from "../icons";
import {
  EntityLifecycleTabs,
  EntitySelectionToolbar,
  type EntityLifecycleState,
  type EntitySelectionAction,
  toApiLifecycleState,
} from "../components/EntityLifecycle";
import { SectionHeader, SectionSearch } from "../shell/SectionChrome";

type Account = components["schemas"]["AccountingAccount"];
type AccountList = components["schemas"]["AccountingAccountList"];
type AccountMapping = components["schemas"]["AccountingMapping"];
type AccountNodeType = "group" | "posting";
type AccountTypeFilter = "all" | AccountNodeType;
type AccountCapabilities = {
  can_edit_name: boolean;
  can_edit_structure: boolean;
  can_archive: boolean;
  can_trash: boolean;
  can_restore: boolean;
  can_duplicate: boolean;
  edit_blockers: string[];
  archive_blockers: string[];
  trash_blockers: string[];
  restore_blockers: string[];
};
type AccountUsage = {
  used: boolean;
  journal_lines: number;
  draft_lines: number;
  mappings: number;
  children: number;
  financial_accounts: number;
  open_items: number;
  inflation_lines: number;
  revaluation_lines: number;
  total_dependencies: number;
  active_children: number;
  active_financial_accounts: number;
};
type AccountingAccountView = Account & {
  node_type: AccountNodeType;
  depth: number;
  path: string[];
  has_children: boolean;
  used: boolean;
  mapped: boolean;
  system_managed: boolean;
  context_only: boolean;
  capabilities: AccountCapabilities;
  usage?: AccountUsage;
  mapped_roles?: string[];
};
type AccountingAccountTree = {
  items: AccountingAccountView[];
  totals: {
    active: number;
    archived: number;
    trashed: number;
  };
};
type AccountingMappingDefinition = {
  role: string;
  label_es: string;
  label_en: string;
  description_es: string;
  description_en: string;
  required: boolean;
  compatible_account_types: Account["account_type"][];
  compatible_normal_balances: Account["normal_balance"][];
  compatible_monetary_classifications: Account["monetary_classification"][];
  canonical_role?: string | null;
  is_alias: boolean;
  display_order?: number;
};
type AccountingMappingDefinitions =
  | AccountingMappingDefinition[]
  | { items: AccountingMappingDefinition[] };
type CurrentSession = components["schemas"]["CurrentSession"];
type Draft = components["schemas"]["JournalDraft"];
type DraftList = components["schemas"]["JournalDraftList"];
type DraftSummary = components["schemas"]["JournalDraftSummary"];
type Entry = components["schemas"]["JournalEntry"];
type EntryList = components["schemas"]["JournalEntryList"];
type EntrySummary = components["schemas"]["JournalEntrySummary"];
type JournalLine = components["schemas"]["JournalLine"];
type OpenItem = components["schemas"]["AccountingOpenItem"];
type OpenItemList = components["schemas"]["AccountingOpenItemList"];
type OpenItemType = components["schemas"]["AccountingOpenItemType"];
type PaymentMethod = components["schemas"]["AccountingPaymentMethod"];
type SettlementInput = components["schemas"]["AccountingSettlementInput"];
type PageInfo = components["schemas"]["PageInfo"];
type Period = components["schemas"]["AccountingPeriod"];
type FinancialAccount = components["schemas"]["FinancialAccount"];
type StatementImport = components["schemas"]["StatementImport"];
type Reconciliation = components["schemas"]["Reconciliation"];
type ReconciliationList = components["schemas"]["ReconciliationList"];
type ReconciliationMatchInput = components["schemas"]["ReconciliationMatchInput"];
type ReconciliationSuggestion = components["schemas"]["ReconciliationSuggestion"];
type Report = components["schemas"]["AccountingReport"];
type InflationAdjustment = components["schemas"]["InflationAdjustment"];
type CurrencyRevaluation = components["schemas"]["CurrencyRevaluation"];
type ClosingExchangeRateInput = components["schemas"]["ClosingExchangeRateInput"];

type JournalPostingState = "incomplete" | "unbalanced" | "blocked" | "ready";
type JournalPostingIssue =
  | "description_required"
  | "minimum_lines"
  | "line_account_required"
  | "line_side_invalid"
  | "unbalanced"
  | "zero_total"
  | "period_closed"
  | "account_archived"
  | "account_not_postable";

type JournalPostingStatusView = {
  state: JournalPostingState;
  difference: string;
  issues: JournalPostingIssue[];
};

type JournalLineView = JournalLine & {
  account_code?: string;
  account_name?: string;
};

type JournalDraftView = Partial<Draft & DraftSummary> &
  Pick<
    Draft,
    | "id"
    | "accounting_date"
    | "description"
    | "currency"
    | "total_debit"
    | "total_credit"
    | "version"
  > & {
  reference?: string | null;
  functional_currency?: string;
  exchange_rate?: string;
  exchange_rate_date?: string | null;
  exchange_rate_source?: string | null;
  lines?: JournalLineView[];
  line_count?: number;
  posting_status?: JournalPostingStatusView;
  updated_at?: string;
  updated_by?: string;
  };

type JournalEntryView = Partial<Entry & EntrySummary> &
  Pick<
    Entry,
    | "id"
    | "entry_number"
    | "accounting_date"
    | "description"
    | "currency"
    | "total_debit"
    | "total_credit"
    | "created_at"
  > & {
  reference?: string | null;
  functional_currency?: string;
  exchange_rate?: string;
  exchange_rate_date?: string | null;
  exchange_rate_source?: string | null;
  lines?: JournalLineView[];
  line_count?: number;
  reverses_entry_number?: number | null;
  reversed_by_entry_number?: number | null;
  kind?: string;
  posting_kind?: string;
  created_by?: string;
  reversed_by_entry_id?: string | null;
  };

type JournalDraftPage = {
  items: JournalDraftView[];
  page: PageInfo;
};

type JournalEntryPage = {
  items: JournalEntryView[];
  page: PageInfo;
};

type GeneralLedgerBalanceView =
  components["schemas"]["GeneralLedgerBalance"];
type GeneralLedgerView = components["schemas"]["GeneralLedger"];

type TrialBalanceBalance = components["schemas"]["TrialBalanceBalance"];
type TrialBalanceItem = components["schemas"]["TrialBalanceItem"];
type TrialBalanceView = components["schemas"]["TrialBalance"];

type AccountingSettingsView = {
  country_code: string;
  functional_currency: string;
  timezone: string;
};

type LogicalOperationKey = {
  signature: string;
  value: string;
};

const sections = [
  ["accounts", "Plan de cuentas"],
  ["journal", "Diario"],
  ["ledger", "Mayor"],
  ["trial-balance", "Sumas y saldos"],
  ["open-items", "Cobros y pagos"],
  ["reports", "Informes"],
  ["reconciliation", "Conciliación"],
  ["periods", "Cierre"],
  ["inflation", "Inflación"],
] as const;

type AccountingSection = (typeof sections)[number][0];

export function AccountingPage() {
  const api = useProductApi();
  const params = useParams<{ section?: string }>();
  const section = (params.section ?? "accounts") as AccountingSection;
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
    return <Navigate replace to="/accounting/accounts" />;
  }

  const canManage = session?.permissions.includes("accounting:manage") ?? false;

  return (
    <div className="directory-page finance-page">
      <SectionHeader title="Contabilidad" subtitle="Libros y control" />
      <div className="directory-page__content">
        <nav className="directory-tabs finance-tabs" aria-label="Secciones contables">
          {sections.map(([value, label]) => (
            <NavLink key={value} to={`/accounting/${value}`}>
              {label}
            </NavLink>
          ))}
        </nav>
        {permissionError ? (
          <div className="finance-permission-state inline-state inline-state--error" role="alert">
            {permissionError}
          </div>
        ) : null}
        <AccountingSectionView canManage={canManage} section={section} />
      </div>
    </div>
  );
}

function AccountingSectionView({
  canManage,
  section,
}: {
  canManage: boolean;
  section: AccountingSection;
}) {
  if (section === "accounts") return <AccountsPanel canManage={canManage} />;
  if (section === "journal") return <JournalPanel canManage={canManage} />;
  if (section === "ledger") return <LedgerPanel />;
  if (section === "trial-balance") return <TrialBalancePanel />;
  if (section === "open-items") return <OpenItemsPanel canManage={canManage} />;
  if (section === "reports") return <ReportsPanel />;
  if (section === "reconciliation") {
    return <ReconciliationPanel canManage={canManage} />;
  }
  if (section === "periods") return <PeriodsPanel canManage={canManage} />;
  return <InflationPanel canManage={canManage} />;
}

type AccountEditorMode = "create-group" | "create-posting" | "edit" | "duplicate";
type AccountLifecycleAction = "archive" | "unarchive" | "trash" | "restore";

function AccountsPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [accounts, setAccounts] = useState<AccountingAccountView[]>([]);
  const [supportAccounts, setSupportAccounts] = useState<AccountingAccountView[]>([]);
  const [mappings, setMappings] = useState<AccountMapping[]>([]);
  const [mappingDefinitions, setMappingDefinitions] = useState<
    AccountingMappingDefinition[]
  >([]);
  const [query, setQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState<AccountTypeFilter>("all");
  const [lifecycle, setLifecycle] = useState<EntityLifecycleState>("active");
  const [totals, setTotals] = useState({ active: 0, archived: 0, trashed: 0 });
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [bulkNotice, setBulkNotice] = useState<string>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [editorMode, setEditorMode] = useState<AccountEditorMode>();
  const [editorAccount, setEditorAccount] = useState<AccountingAccountView>();
  const [editorDirty, setEditorDirty] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const search = new URLSearchParams({
        lifecycle_state: toApiLifecycleState(lifecycle),
      });
      if (query.trim()) search.set("query", query.trim());
      if (typeFilter !== "all") search.set("node_type", typeFilter);
      const response = await api.request<AccountingAccountTree>(
        `/api/v1/accounting/accounts/tree?${search}`,
        { skipJSONContentType: true },
      );
      const items = (response.items ?? []).map(normalizeAccountView);
      setAccounts(items);
      setTotals(response.totals ?? countAccountLifecycles(items));
      setCollapsed(
        query.trim() || typeFilter !== "all"
          ? new Set()
          : new Set(
              items
                .filter((account) => account.node_type === "group" && account.has_children)
                .map((account) => account.id),
            ),
      );
    } catch (cause) {
      setError(
        accountErrorMessage(cause, "No pudimos cargar el plan de cuentas."),
      );
    } finally {
      setLoading(false);
    }
  }, [api, lifecycle, query, typeFilter]);

  const loadSupportData = useCallback(async () => {
    try {
      const [currentMappings, definitions, activeTree] = await Promise.all([
        api.request<AccountMapping[]>("/api/v1/accounting/account-mappings", {
          skipJSONContentType: true,
        }),
        api.request<AccountingMappingDefinitions>(
          "/api/v1/accounting/account-mapping-definitions",
          { skipJSONContentType: true },
        ),
        api.request<AccountingAccountTree>(
          "/api/v1/accounting/accounts/tree?lifecycle_state=active",
          { skipJSONContentType: true },
        ),
      ]);
      setMappings(currentMappings);
      setMappingDefinitions(
        Array.isArray(definitions) ? definitions : definitions.items ?? [],
      );
      setSupportAccounts((activeTree.items ?? []).map(normalizeAccountView));
    } catch (cause) {
      setError(
        accountErrorMessage(
          cause,
          "No pudimos cargar la configuración de posteo automático.",
        ),
      );
    }
  }, [api]);

  useEffect(() => void load(), [load]);
  useEffect(() => void loadSupportData(), [loadSupportData]);
  useEffect(() => {
    setSelectedIDs((current) =>
      current.filter((id) => accounts.some((account) => account.id === id)),
    );
  }, [accounts]);

  function confirmEditorChange() {
    return (
      !editorDirty ||
      window.confirm(
        "Hay cambios sin guardar. Si continuás, se perderán.",
      )
    );
  }

  function closeEditor() {
    if (!confirmEditorChange()) return;
    setEditorMode(undefined);
    setEditorAccount(undefined);
    setEditorDirty(false);
  }

  function startCreate(mode: "create-group" | "create-posting") {
    if (!confirmEditorChange()) return;
    setEditorAccount(undefined);
    setEditorMode(mode);
    setEditorDirty(false);
  }

  async function openAccount(account: AccountingAccountView) {
    if (!confirmEditorChange()) return;
    setDetailLoading(true);
    setError(undefined);
    try {
      const detail = await api.request<AccountingAccountView>(
        `/api/v1/accounting/accounts/${account.id}`,
        { skipJSONContentType: true },
      );
      setEditorAccount(normalizeAccountView(detail));
      setEditorMode("edit");
      setEditorDirty(false);
    } catch (cause) {
      setError(
        accountErrorMessage(cause, "No pudimos abrir la ficha de la cuenta."),
      );
    } finally {
      setDetailLoading(false);
    }
  }

  function changeLifecycle(value: EntityLifecycleState) {
    if (!confirmEditorChange()) return;
    setLifecycle(value);
    setSelectedIDs([]);
    setEditorMode(undefined);
    setEditorAccount(undefined);
    setEditorDirty(false);
  }

  async function transition(
    selected: AccountingAccountView[],
    action: AccountLifecycleAction,
  ) {
    if (selected.length === 0) return;
    setBulkBusy(true);
    setError(undefined);
    setBulkNotice(undefined);
    const orderedDepths = [...new Set(selected.map((account) => account.depth))].sort(
      (left, right) =>
        action === "archive" || action === "trash"
          ? right - left
          : left - right,
    );
    const failedIDs: string[] = [];
    let succeeded = 0;
    for (const depth of orderedDepths) {
      const level = selected.filter((account) => account.depth === depth);
      const results = await Promise.allSettled(
        level.map((account) =>
          api.request<Account>(
            `/api/v1/accounting/accounts/${account.id}/${action}`,
            {
              method: "POST",
              headers: {
                "Idempotency-Key": createIdempotencyKey(
                  `account-${action}-${account.id}`,
                ),
              },
              body: JSON.stringify({
                version: account.version,
                reason: "Cambio desde plan de cuentas",
              }),
            },
          ),
        ),
      );
      results.forEach((result, index) => {
        if (result.status === "fulfilled") succeeded += 1;
        else if (level[index]) failedIDs.push(level[index].id);
      });
    }
    setSelectedIDs(failedIDs);
    setBulkNotice(
      failedIDs.length === 0
        ? `${succeeded} ${
            succeeded === 1 ? "cuenta actualizada" : "cuentas actualizadas"
          }.`
        : `${succeeded} actualizadas y ${failedIDs.length} con conflictos. Las cuentas pendientes siguen seleccionadas.`,
    );
    if (failedIDs.length > 0) {
      setError(
        "Algunas cuentas no pudieron cambiar de estado. Abrí su ficha para revisar los bloqueos.",
      );
    }
    await Promise.all([load(), loadSupportData()]);
    setBulkBusy(false);
  }

  const treeRows = accountTreeRows(accounts, collapsed);
  const selectableRows = accounts.filter((account) => !account.context_only);
  const selectedAccounts = accounts.filter((account) =>
    selectedIDs.includes(account.id),
  );
  const allVisibleSelected =
    selectableRows.length > 0 &&
    selectableRows.every((account) => selectedIDs.includes(account.id));
  const someVisibleSelected = selectableRows.some((account) =>
    selectedIDs.includes(account.id),
  );
  const action =
    lifecycle === "active"
      ? undefined
      : lifecycle === "archived"
        ? "unarchive"
        : "restore";

  return (
    <section className="directory-section accounts-directory">
      <div className="accounts-toolbar">
        {canManage ? (
          <div className="accounts-toolbar__actions">
            <div className="accounts-toolbar__buttons">
              {lifecycle === "active" ? (
                <>
                  <button
                    className="directory-create-button"
                    onClick={() => startCreate("create-group")}
                    type="button"
                  >
                    <span aria-hidden="true">＋</span>
                    Nuevo rubro
                  </button>
                  <button
                    className="directory-create-button"
                    onClick={() => startCreate("create-posting")}
                    type="button"
                  >
                    <span aria-hidden="true">＋</span>
                    Nueva cuenta
                  </button>
                </>
              ) : null}
              <span className="accounts-toolbar__separator" aria-hidden="true" />
              <button
                disabled={bulkBusy || selectedIDs.length === 0}
                onClick={() => setSelectedIDs([])}
                type="button"
              >
                Limpiar
              </button>
              {lifecycle === "active" ? (
                <>
                  <button
                    disabled={
                      bulkBusy ||
                      selectedAccounts.length === 0 ||
                      selectedAccounts.every(
                        (account) => !account.capabilities.can_archive,
                      )
                    }
                    onClick={() =>
                      void transition(selectedAccounts, "archive")
                    }
                    type="button"
                  >
                    Archivar
                  </button>
                  <button
                    className="is-danger"
                    disabled={
                      bulkBusy ||
                      selectedAccounts.length === 0 ||
                      selectedAccounts.every(
                        (account) => !account.capabilities.can_trash,
                      )
                    }
                    onClick={() => void transition(selectedAccounts, "trash")}
                    type="button"
                  >
                    Papelera
                  </button>
                </>
              ) : (
                <button
                  disabled={
                    bulkBusy ||
                    selectedAccounts.length === 0 ||
                    selectedAccounts.every(
                      (account) => !account.capabilities.can_restore,
                    )
                  }
                  onClick={() =>
                    void transition(selectedAccounts, action ?? "restore")
                  }
                  type="button"
                >
                  {lifecycle === "archived" ? "Desarchivar" : "Restaurar"}
                </button>
              )}
            </div>
            <span className="accounts-toolbar__count">
              {selectedIDs.length}{" "}
              {selectedIDs.length === 1 ? "seleccionado" : "seleccionados"}
            </span>
          </div>
        ) : (
          <span />
        )}
        <div className="accounts-toolbar__filters">
          <SectionSearch
            label="Buscar cuentas"
            placeholder="Buscar por código o nombre…"
            value={query}
            onChange={setQuery}
          />
          <AccountNodeTypeTabs onChange={setTypeFilter} value={typeFilter} />
          <AccountLifecycleTabs
            onChange={changeLifecycle}
            state={lifecycle}
            totals={totals}
          />
        </div>
      </div>
      {!canManage ? <ReadOnlyNote /> : null}
      {editorMode ? (
        <AccountForm
          account={editorAccount}
          accounts={supportAccounts}
          canManage={canManage}
          key={`${editorMode}-${editorAccount?.id ?? "new"}`}
          mode={editorMode}
          onCancel={closeEditor}
          onDirtyChange={setEditorDirty}
          onDuplicate={(account) => {
            setEditorAccount(account);
            setEditorMode("duplicate");
            setEditorDirty(false);
          }}
          onReload={(account) => {
            setEditorAccount(account);
            setEditorMode("edit");
            setEditorDirty(false);
          }}
          onSaved={() => {
            setEditorMode(undefined);
            setEditorAccount(undefined);
            setEditorDirty(false);
            void Promise.all([load(), loadSupportData()]);
          }}
        />
      ) : null}
      {bulkNotice ? (
        <div className="inline-state accounts-bulk-notice" role="status">
          {bulkNotice}
        </div>
      ) : null}
      <InlineFeedback error={error} loading={loading || detailLoading} />
      <div className="directory-table-wrap accounts-tree-wrap">
        <table className="directory-table finance-table accounts-tree-table">
          <thead>
            <tr>
              {canManage ? (
                <th className="entity-select-cell">
                  <PageSelectionCheckbox
                    checked={allVisibleSelected}
                    indeterminate={!allVisibleSelected && someVisibleSelected}
                    onChange={(checked) =>
                      setSelectedIDs(
                        checked
                          ? selectableRows.map((account) => account.id)
                          : [],
                      )
                    }
                  />
                </th>
              ) : null}
              <th>Código</th>
              <th>Cuenta</th>
              <th>Tipo</th>
              <th>Rubro</th>
              <th>Naturaleza</th>
              <th>Clasificación</th>
              <th>Estado de uso</th>
            </tr>
          </thead>
          <tbody>
            {treeRows.map(({ account, depth, hasChildren }) => (
              <tr
                aria-label={`Abrir ${account.code} ${account.name}`}
                className={`${account.node_type === "group" ? "is-account-group" : "is-posting-account"} ${
                  depth === 0 ? "is-account-root" : ""
                } ${account.context_only ? "is-context-only" : ""}`}
                key={account.id}
                onClick={() => void openAccount(account)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    void openAccount(account);
                  }
                }}
                tabIndex={0}
              >
                {canManage ? (
                  <td
                    className="entity-select-cell"
                    onClick={(event) => event.stopPropagation()}
                  >
                    <input
                      aria-label={`Seleccionar ${account.code} ${account.name}`}
                      checked={selectedIDs.includes(account.id)}
                      disabled={account.context_only}
                      onChange={(event) =>
                        setSelectedIDs((current) =>
                          event.target.checked
                            ? [...new Set([...current, account.id])]
                            : current.filter((id) => id !== account.id),
                        )
                      }
                      type="checkbox"
                    />
                  </td>
                ) : null}
                <td>{account.code}</td>
                <td className="directory-table__primary">
                  <div
                    className="account-tree-cell"
                    style={{ "--account-depth": depth } as CSSProperties}
                  >
                    {hasChildren ? (
                      <button
                        aria-label={`${
                          collapsed.has(account.id) ? "Expandir" : "Contraer"
                        } ${account.name}`}
                        aria-expanded={!collapsed.has(account.id)}
                        className="account-tree-toggle"
                        onClick={(event) => {
                          event.stopPropagation();
                          setCollapsed((previous) => {
                            const next = new Set(previous);
                            if (next.has(account.id)) next.delete(account.id);
                            else next.add(account.id);
                            return next;
                          });
                        }}
                        type="button"
                      >
                        <ChevronRightIcon
                          aria-hidden="true"
                          className={collapsed.has(account.id) ? undefined : "is-expanded"}
                        />
                      </button>
                    ) : (
                      <span className="account-tree-leaf" aria-hidden="true" />
                    )}
                    <span>{account.name}</span>
                  </div>
                </td>
                <td>
                  {account.node_type === "group" ? "Rubro" : "Imputable"}
                </td>
                <td>{accountTypeLabel(account.account_type)}</td>
                <td>
                  {account.normal_balance === "debit"
                    ? "Deudora"
                    : "Acreedora"}
                </td>
                <td>
                  {monetaryClassificationLabel(
                    account.monetary_classification,
                  )}
                </td>
                <td>
                  <div className="account-usage-cell">
                    <span>{accountUsageLabel(account)}</span>
                    {account.node_type === "posting" ? (
                      <NavLink
                        onClick={(event) => event.stopPropagation()}
                        to={`/accounting/ledger?account_id=${account.id}`}
                      >
                        Ver mayor
                      </NavLink>
                    ) : null}
                  </div>
                </td>
              </tr>
            ))}
            {!loading && accounts.length === 0 ? (
              <EmptyRow
                columns={canManage ? 8 : 7}
                text="No hay cuentas que coincidan."
              />
            ) : null}
          </tbody>
        </table>
      </div>
      <div className="accounts-tree-summary">
        <span>{accounts.length} elementos en el resultado</span>
        <span>El árbol se construye por rubro superior.</span>
      </div>
      <AccountMappingsPanel
        accounts={supportAccounts.filter(
          (account) =>
            account.node_type === "posting" &&
            account.lifecycle_state === "active",
        )}
        canManage={canManage}
        definitions={mappingDefinitions}
        mappings={mappings}
        onSaved={(saved) => setMappings(saved)}
      />
    </section>
  );
}

function PageSelectionCheckbox({
  checked,
  indeterminate,
  onChange,
}: {
  checked: boolean;
  indeterminate: boolean;
  onChange: (checked: boolean) => void;
}) {
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (ref.current) ref.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      aria-label="Seleccionar todo el resultado"
      checked={checked}
      onChange={(event) => onChange(event.target.checked)}
      ref={ref}
      type="checkbox"
    />
  );
}

function AccountNodeTypeTabs({
  onChange,
  value,
}: {
  onChange: (value: AccountTypeFilter) => void;
  value: AccountTypeFilter;
}) {
  return (
    <div className="account-filter-tabs" role="tablist" aria-label="Tipo de cuenta">
      {(
        [
          ["all", "Todos"],
          ["group", "Rubros"],
          ["posting", "Imputables"],
        ] as const
      ).map(([candidate, label]) => (
        <button
          aria-selected={candidate === value}
          className={candidate === value ? "is-active" : ""}
          key={candidate}
          onClick={() => onChange(candidate)}
          role="tab"
          type="button"
        >
          {label}
        </button>
      ))}
    </div>
  );
}

function AccountLifecycleTabs({
  onChange,
  state,
  totals,
}: {
  onChange: (state: EntityLifecycleState) => void;
  state: EntityLifecycleState;
  totals: { active: number; archived: number; trashed: number };
}) {
  const tabs: Array<[EntityLifecycleState, string, number]> = [
    ["active", "Activos", totals.active],
    ["archived", "Archivados", totals.archived],
    ["trash", "Papelera", totals.trashed],
  ];
  return (
    <div className="lifecycle-tabs account-lifecycle-tabs" role="tablist" aria-label="Estado de cuentas">
      {tabs.map(([candidate, label, count]) => (
        <button
          aria-selected={candidate === state}
          className={candidate === state ? "is-active" : ""}
          key={candidate}
          onClick={() => onChange(candidate)}
          role="tab"
          type="button"
        >
          <span>{label}</span>
          <small>{count}</small>
        </button>
      ))}
    </div>
  );
}

type AccountFormState = {
  code: string;
  name: string;
  accountType: Account["account_type"];
  normalBalance: Account["normal_balance"];
  monetaryClassification: Account["monetary_classification"];
  parentID: string;
};

function AccountForm({
  account,
  accounts,
  canManage,
  mode,
  onCancel,
  onDirtyChange,
  onDuplicate,
  onReload,
  onSaved,
}: {
  account?: AccountingAccountView;
  accounts: AccountingAccountView[];
  canManage: boolean;
  mode: AccountEditorMode;
  onCancel: () => void;
  onDirtyChange: (dirty: boolean) => void;
  onDuplicate: (account: AccountingAccountView) => void;
  onReload: (account: AccountingAccountView) => void;
  onSaved: () => void;
}) {
  const api = useProductApi();
  const nodeType: AccountNodeType =
    mode === "create-group"
      ? "group"
      : mode === "create-posting"
        ? "posting"
        : account?.node_type ?? (account?.postable ? "posting" : "group");
  const initial = accountFormInitialState(account, mode, nodeType);
  const [value, setValue] = useState<AccountFormState>(initial);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [conflict, setConflict] = useState(false);
  const dirty = JSON.stringify(value) !== JSON.stringify(initial);
  const lifecycleReadOnly =
    mode === "edit" && account?.lifecycle_state !== "active";
  const systemReadOnly = mode === "edit" && Boolean(account?.system_managed);
  const formReadOnly = !canManage || lifecycleReadOnly || systemReadOnly;
  const structuralReadOnly =
    formReadOnly ||
    (mode === "edit" && !Boolean(account?.capabilities.can_edit_structure));
  const canSubmit =
    !formReadOnly &&
    value.code.trim().length > 0 &&
    value.name.trim().length > 0 &&
    (nodeType === "group" || Boolean(value.parentID));
  const groupAccounts = accounts.filter(
    (candidate) =>
      candidate.node_type === "group" &&
      candidate.lifecycle_state === "active" &&
      candidate.id !== account?.id &&
      (mode === "create-posting" ||
        candidate.account_type === value.accountType),
  );

  useEffect(() => onDirtyChange(dirty), [dirty, onDirtyChange]);
  useEffect(() => {
    if (!dirty) return;
    const preventUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", preventUnload);
    return () => window.removeEventListener("beforeunload", preventUnload);
  }, [dirty]);

  function update(next: Partial<AccountFormState>) {
    setConflict(false);
    setValue((current) => ({ ...current, ...next }));
  }

  async function reloadLatest() {
    if (!account) return;
    setBusy(true);
    setError(undefined);
    try {
      const detail = await api.request<AccountingAccountView>(
        `/api/v1/accounting/accounts/${account.id}`,
        { skipJSONContentType: true },
      );
      onReload(normalizeAccountView(detail));
    } catch (cause) {
      setError(
        accountErrorMessage(cause, "No pudimos recargar la última versión."),
      );
    } finally {
      setBusy(false);
    }
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!canSubmit) return;
    setBusy(true);
    setError(undefined);
    setConflict(false);
    const editing = mode === "edit" && account;
    try {
      await api.request<AccountingAccountView>(
        editing
          ? `/api/v1/accounting/accounts/${account.id}`
          : "/api/v1/accounting/accounts",
        {
          method: editing ? "PUT" : "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey(
              editing ? "account-update" : `account-${nodeType}`,
            ),
          },
          body: JSON.stringify({
            code: value.code.trim(),
            name: value.name.trim(),
            node_type: nodeType,
            postable: nodeType === "posting",
            account_type: value.accountType,
            normal_balance:
              nodeType === "group"
                ? normalBalanceForType(value.accountType)
                : value.normalBalance,
            monetary_classification:
              nodeType === "group"
                ? "not_applicable"
                : value.monetaryClassification,
            parent_id: value.parentID || null,
            ...(editing ? { version: account.version } : {}),
          }),
        },
      );
      onSaved();
    } catch (cause) {
      const normalized = normalizeHttpError(cause, {
        fallbackMessage: "No pudimos guardar la cuenta.",
      });
      setConflict(normalized.code === "VERSION_CONFLICT");
      setError(accountErrorMessage(cause, "No pudimos guardar la cuenta."));
    } finally {
      setBusy(false);
    }
  }

  const title =
    mode === "create-group"
      ? "Nuevo rubro"
      : mode === "create-posting"
        ? "Nueva cuenta imputable"
        : mode === "duplicate"
          ? `Duplicar ${account?.name ?? "cuenta"}`
          : `${nodeType === "group" ? "Rubro" : "Cuenta"} ${account?.code ?? ""}`;

  return (
    <form
      className={`finance-form finance-form--account account-editor ${
        formReadOnly ? "is-readonly" : ""
      }`}
      onSubmit={(event) => void submit(event)}
    >
      <header className="account-editor__header">
        <div>
          <small>{nodeType === "group" ? "Rubro estructural" : "Cuenta imputable"}</small>
          <h2>{title}</h2>
          {account?.parent_id ? <span>Depende de un rubro superior.</span> : null}
        </div>
        <div>
          {mode === "edit" &&
          account?.capabilities.can_duplicate &&
          canManage ? (
            <button onClick={() => onDuplicate(account)} type="button">
              Duplicar como nueva
            </button>
          ) : null}
          <button aria-label="Cerrar ficha" onClick={onCancel} type="button">
            ×
          </button>
        </div>
      </header>
      <label>
        Código
        <input
          disabled={structuralReadOnly}
          maxLength={32}
          onChange={(event) => update({ code: event.target.value })}
          required
          value={value.code}
        />
      </label>
      <label>
        Nombre
        <input
          disabled={
            formReadOnly ||
            (mode === "edit" && !account?.capabilities.can_edit_name)
          }
          maxLength={160}
          onChange={(event) => update({ name: event.target.value })}
          required
          value={value.name}
        />
      </label>
      <label>
        Clase
        <select
          disabled={structuralReadOnly || nodeType === "posting"}
          onChange={(event) => {
            const accountType = event.target.value as Account["account_type"];
            update({
              accountType,
              normalBalance: normalBalanceForType(accountType),
              parentID: "",
            });
          }}
          value={value.accountType}
        >
          <option value="asset">Activo</option>
          <option value="liability">Pasivo</option>
          <option value="equity">Patrimonio</option>
          <option value="income">Ingresos</option>
          <option value="cost">Costos</option>
          <option value="expense">Gastos</option>
        </select>
      </label>
      <label>
        Naturaleza
        <select
          disabled={structuralReadOnly || nodeType === "group"}
          onChange={(event) =>
            update({
              normalBalance: event.target.value as Account["normal_balance"],
            })
          }
          value={
            nodeType === "group"
              ? normalBalanceForType(value.accountType)
              : value.normalBalance
          }
        >
          <option value="debit">Deudora</option>
          <option value="credit">Acreedora</option>
        </select>
      </label>
      <label>
        Clasificación
        <select
          disabled={structuralReadOnly || nodeType === "group"}
          onChange={(event) =>
            update({
              monetaryClassification: event.target
                .value as Account["monetary_classification"],
            })
          }
          value={
            nodeType === "group"
              ? "not_applicable"
              : value.monetaryClassification
          }
        >
          {nodeType === "group" ? (
            <option value="not_applicable">No aplicable</option>
          ) : (
            <>
              <option value="monetary">Monetaria</option>
              <option value="non_monetary">No monetaria</option>
            </>
          )}
        </select>
      </label>
      <label className="account-editor__parent">
        Rubro superior {nodeType === "posting" ? "(obligatorio)" : "(opcional)"}
        <AccountLookupCombobox
          accounts={groupAccounts}
          allowEmpty={nodeType === "group"}
          disabled={structuralReadOnly}
          emptyText="Sin rubro superior"
          label="Rubro superior"
          onSelect={(selected) => {
            if (!selected) {
              update({ parentID: "" });
              return;
            }
            update({
              parentID: selected.id,
              accountType: selected.account_type,
              normalBalance: normalBalanceForType(selected.account_type),
            });
          }}
          selectedID={value.parentID}
        />
      </label>
      <div className="account-editor__status">
        <span className={`account-node-badge is-${nodeType}`}>
          {nodeType === "group" ? "Rubro · no imputable" : "Cuenta imputable"}
        </span>
        {account?.used ? <span>En uso</span> : <span>Sin uso</span>}
        {account?.mapped ? <span>Usada en posteo automático</span> : null}
        {account?.system_managed ? <span>Protegida por el sistema</span> : null}
      </div>
      {account?.usage ? <AccountUsageSummary usage={account.usage} /> : null}
      {account && accountCapabilityBlockers(account).length ? (
        <div className="account-editor__blockers">
          <strong>Bloqueos actuales</strong>
          <ul>
            {accountCapabilityBlockers(account).map((blocker) => (
              <li key={blocker}>{accountBlockerLabel(blocker)}</li>
            ))}
          </ul>
        </div>
      ) : null}
      <div className="finance-form__actions account-editor__actions">
        <button onClick={onCancel} type="button">
          {formReadOnly ? "Cerrar" : "Cancelar"}
        </button>
        {!formReadOnly ? (
          <button
            className="directory-create-button"
            disabled={busy || !canSubmit}
            type="submit"
          >
            {busy
              ? "Guardando…"
              : nodeType === "group"
                ? "Guardar rubro"
                : "Guardar cuenta"}
          </button>
        ) : null}
      </div>
      {error ? (
        <div className="form-error account-editor__error" role="alert">
          <span>{error}</span>
          {conflict ? (
            <div>
              <button disabled={busy} onClick={() => void reloadLatest()} type="button">
                Recargar última versión
              </button>
              {account ? (
                <button
                  onClick={() =>
                    onDuplicate(
                      accountFromFormState(account, value, nodeType),
                    )
                  }
                  type="button"
                >
                  Guardar como nueva
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}
    </form>
  );
}

function AccountUsageSummary({ usage }: { usage: AccountUsage }) {
  const items: Array<[string, number]> = [
    ["Asientos", usage.journal_lines],
    ["Borradores", usage.draft_lines],
    ["Mappings", usage.mappings],
    ["Hijas", usage.children],
    ["Cuentas financieras", usage.financial_accounts],
    ["Partidas abiertas", usage.open_items],
    ["Ajustes por inflación", usage.inflation_lines],
    ["Revaluaciones", usage.revaluation_lines],
  ];
  return (
    <dl className="account-usage-summary">
      {items.map(([label, count]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{count}</dd>
        </div>
      ))}
    </dl>
  );
}

function AccountLookupCombobox({
  accounts,
  allowEmpty = true,
  disabled = false,
  emptyText = "Sin cuenta",
  label,
  onSelect,
  selectedID,
}: {
  accounts: AccountingAccountView[];
  allowEmpty?: boolean;
  disabled?: boolean;
  emptyText?: string;
  label: string;
  onSelect: (account?: AccountingAccountView) => void;
  selectedID: string;
}) {
  const listID = useId();
  const selected = accounts.find((account) => account.id === selectedID);
  const selectedLabel = selected ? `${selected.code} · ${selected.name}` : "";
  const [query, setQuery] = useState(selectedLabel);
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);

  useEffect(() => {
    if (!open) setQuery(selectedLabel);
  }, [open, selectedLabel]);

  const normalized = query.trim().toLocaleLowerCase("es");
  const options = accounts
    .filter(
      (account) =>
        !normalized ||
        query === selectedLabel ||
        `${account.code} ${account.name}`
          .toLocaleLowerCase("es")
          .includes(normalized),
    )
    .sort(compareAccountsByCode)
    .slice(0, 40);

  function choose(account?: AccountingAccountView) {
    onSelect(account);
    setQuery(account ? `${account.code} · ${account.name}` : "");
    setOpen(false);
    setActiveIndex(-1);
  }

  return (
    <div className="journal-account-combobox account-lookup-combobox">
      <input
        aria-activedescendant={
          open && activeIndex >= 0 && options[activeIndex]
            ? `${listID}-${options[activeIndex].id}`
            : undefined
        }
        aria-autocomplete="list"
        aria-controls={listID}
        aria-expanded={open}
        aria-label={label}
        autoComplete="off"
        disabled={disabled}
        onBlur={() => window.setTimeout(() => setOpen(false), 0)}
        onChange={(event) => {
          setQuery(event.target.value);
          if (selectedID) onSelect(undefined);
          setActiveIndex(-1);
          setOpen(true);
        }}
        onFocus={() => {
          setOpen(true);
          setActiveIndex(-1);
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown") {
            event.preventDefault();
            setOpen(true);
            setActiveIndex((current) =>
              Math.min(current + 1, options.length - 1),
            );
          } else if (event.key === "ArrowUp") {
            event.preventDefault();
            setActiveIndex((current) => Math.max(current - 1, 0));
          } else if (event.key === "Enter" && open) {
            event.preventDefault();
            const option = options[activeIndex];
            if (option) choose(option);
          } else if (event.key === "Escape") {
            setOpen(false);
          }
        }}
        placeholder="Buscar por código o nombre…"
        role="combobox"
        value={query}
      />
      {open ? (
        <div className="journal-account-combobox__options" id={listID} role="listbox">
          {allowEmpty ? (
            <button
              aria-selected={!selectedID}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => choose(undefined)}
              role="option"
              type="button"
            >
              <strong>—</strong>
              <span>{emptyText}</span>
            </button>
          ) : null}
          {options.map((account, index) => (
            <button
              aria-selected={selectedID === account.id}
              className={index === activeIndex ? "is-active" : ""}
              id={`${listID}-${account.id}`}
              key={account.id}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => choose(account)}
              role="option"
              type="button"
            >
              <strong>{account.code}</strong>
              <span>{account.name}</span>
            </button>
          ))}
          {options.length === 0 ? (
            <span>No hay cuentas que coincidan.</span>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

type EditableMapping = {
  definition: AccountingMappingDefinition;
  accountID: string;
  version?: number;
};

function AccountMappingsPanel({
  accounts,
  canManage,
  definitions,
  mappings,
  onSaved,
}: {
  accounts: AccountingAccountView[];
  canManage: boolean;
  definitions: AccountingMappingDefinition[];
  mappings: AccountMapping[];
  onSaved: (mappings: AccountMapping[]) => void;
}) {
  const api = useProductApi();
  const [rows, setRows] = useState<EditableMapping[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const canonicalDefinitions = definitions
    .filter((definition) => !definition.is_alias)
    .sort(
      (left, right) =>
        (left.display_order ?? 0) - (right.display_order ?? 0) ||
        left.label_es.localeCompare(right.label_es, "es"),
    );
  const aliasDefinitions = definitions
    .filter((definition) => definition.is_alias)
    .sort(
      (left, right) =>
        (left.display_order ?? 0) - (right.display_order ?? 0) ||
        left.role.localeCompare(right.role, "es"),
    );
  const required = canonicalDefinitions.filter((definition) => definition.required);
  const configuredRequired = required.filter((definition) =>
    mappings.some((mapping) => mapping.role === definition.role),
  ).length;
  const canonicalRoles = new Set(
    canonicalDefinitions.map((definition) => definition.role),
  );

  function rowsFromMappings() {
    return canonicalDefinitions.map((definition) => {
      const mapping = mappings.find(
        (candidate) => candidate.role === definition.role,
      );
      return {
        definition,
        accountID: mapping?.account_id ?? "",
        version: mapping?.version,
      };
    });
  }

  useEffect(() => {
    setRows(rowsFromMappings());
    // The definitions endpoint is authoritative; mapping changes reset the
    // working copy so cancel never leaks edits into the next session.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [definitions, mappings]);

  const mappingGroups = ["1", "2", "3", "4", "5", "6"].map((code) => ({
    code,
    label: accountGroupLabel(code),
    mappings: mappings
      .filter(
        (mapping) =>
          canonicalRoles.has(mapping.role) &&
          mapping.account_code.split(".")[0] === code,
      )
      .sort((left, right) =>
        left.account_code.localeCompare(right.account_code, "es", {
          numeric: true,
        }),
      ),
  }));

  async function save() {
    const missingRequired = rows.filter(
      (row) => row.definition.required && !row.accountID,
    );
    if (missingRequired.length > 0) {
      setError(
        `Completá ${
          missingRequired.length === 1
            ? "el mapping requerido"
            : "los mappings requeridos"
        } antes de guardar.`,
      );
      return;
    }
    const commands = rows.filter((row) => row.accountID);
    setBusy(true);
    setError(undefined);
    try {
      const saved = await api.request<AccountMapping[]>(
        "/api/v1/accounting/account-mappings",
        {
          method: "PUT",
          headers: {
            "Idempotency-Key": createIdempotencyKey("account-mappings"),
          },
          body: JSON.stringify(
            commands.map((row) => ({
              role: row.definition.role,
              account_id: row.accountID,
              ...(row.version ? { version: row.version } : {}),
            })),
          ),
        },
      );
      onSaved(saved);
      setEditing(false);
    } catch (cause) {
      setError(
        accountErrorMessage(cause, "No pudimos guardar los mappings."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="account-mappings-card">
      <header>
        <div>
          <small>Posteo automático</small>
          <strong>
            Mappings funcionales · {configuredRequired}/{required.length} requeridos
          </strong>
          <span>
            Definen qué cuenta usa cada operación automática. Si cambiás un mapping,
            los próximos posteos usarán la nueva cuenta.
          </span>
        </div>
        <div className="account-mappings-card__actions">
          <button
            aria-expanded={expanded || editing}
            className="account-mappings-card__toggle"
            onClick={() => setExpanded((value) => !value)}
            type="button"
          >
            {expanded || editing ? "Ocultar información" : "Desplegar información"}
          </button>
          {canManage ? (
            <button
              className="account-mappings-card__edit"
              onClick={() => {
                setExpanded(true);
                setEditing((value) => {
                  if (value) setRows(rowsFromMappings());
                  return !value;
                });
                setError(undefined);
              }}
              type="button"
            >
              {editing ? "Cancelar edición" : "Editar mappings"}
            </button>
          ) : null}
        </div>
      </header>
      {editing ? (
        <div className="account-mapping-editor">
          {rows.map((row, index) => {
            const compatibleAccounts = accounts.filter((account) =>
              accountMatchesDefinition(account, row.definition),
            );
            return (
              <div key={row.definition.role}>
                <div className="account-mapping-editor__definition">
                  <strong>
                    {row.definition.label_es}
                    {row.definition.required ? " · Requerido" : ""}
                  </strong>
                  <span>{row.definition.description_es}</span>
                  <code>{row.definition.role}</code>
                </div>
                <label>
                  Cuenta para {row.definition.label_es}
                  <AccountLookupCombobox
                    accounts={compatibleAccounts}
                    allowEmpty={!row.definition.required}
                    emptyText="Sin configurar"
                    label={`Cuenta para ${row.definition.label_es}`}
                    onSelect={(selected) =>
                      setRows((previous) =>
                        previous.map((candidate, candidateIndex) =>
                          candidateIndex === index
                            ? {
                                ...candidate,
                                accountID: selected?.id ?? "",
                              }
                            : candidate,
                        ),
                      )
                    }
                    selectedID={row.accountID}
                  />
                </label>
              </div>
            );
          })}
          <footer>
            <span>
              Los roles canónicos están definidos por el sistema y no se pueden crear
              manualmente.
            </span>
            <button
              className="directory-create-button"
              disabled={busy}
              onClick={() => void save()}
              type="button"
            >
              {busy ? "Guardando…" : "Guardar mappings"}
            </button>
          </footer>
          {error ? (
            <span className="form-error" role="alert">
              {error}
            </span>
          ) : null}
        </div>
      ) : expanded ? (
        <>
          <div className="account-mapping-grid">
            {mappingGroups.map((group) => (
              <section key={group.code}>
                <header>
                  <strong>{group.code}</strong>
                  <span>{group.label}</span>
                </header>
                <div>
                  {group.mappings.map((mapping) => {
                    const definition = definitions.find(
                      (candidate) => candidate.role === mapping.role,
                    );
                    return (
                      <article key={mapping.role}>
                        <span>
                          {definition?.label_es ?? mappingRoleLabel(mapping.role)}
                        </span>
                        <strong>
                          {mapping.account_code} · {mapping.account_name}
                        </strong>
                      </article>
                    );
                  })}
                  {group.mappings.length === 0 ? <p>Sin mappings.</p> : null}
                </div>
              </section>
            ))}
          </div>
          {aliasDefinitions.length ? (
            <div className="account-mapping-aliases">
              <strong>Aliases heredados</strong>
              {aliasDefinitions.map((alias) => (
                <span key={alias.role}>
                  <code>{alias.role}</code>
                  {alias.canonical_role ? (
                    <>
                      {" → "}
                      <code>{alias.canonical_role}</code>
                    </>
                  ) : (
                    " · sin reemplazo canónico"
                  )}
                </span>
              ))}
            </div>
          ) : null}
        </>
      ) : null}
    </article>
  );
}

function JournalPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [entries, setEntries] = useState<JournalEntryView[]>([]);
  const [drafts, setDrafts] = useState<JournalDraftView[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [query, setQuery] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");
  const [sourceType, setSourceType] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [editingDraft, setEditingDraft] = useState<JournalDraftView>();
  const [copySeed, setCopySeed] = useState<JournalDraftView>();
  const [editorDirty, setEditorDirty] = useState(false);
  const [selectedDraftIDs, setSelectedDraftIDs] = useState<string[]>([]);
  const [discardRequested, setDiscardRequested] = useState(false);
  const [discardBusy, setDiscardBusy] = useState(false);
  const [draftDetailBusy, setDraftDetailBusy] = useState<string>();
  const [selectedEntry, setSelectedEntry] = useState<JournalEntryView>();
  const [reverseEntry, setReverseEntry] = useState<JournalEntryView>();
  const [entryDetailBusy, setEntryDetailBusy] = useState(false);
  const [mode, setMode] = useState<"posted" | "drafts">("posted");
  const [entryCursor, setEntryCursor] = useState<string>();
  const [entryCursorTrail, setEntryCursorTrail] = useState<string[]>([]);
  const [draftCursor, setDraftCursor] = useState<string>();
  const [draftCursorTrail, setDraftCursorTrail] = useState<string[]>([]);
  const [entryPage, setEntryPage] = useState<PageInfo>({ total: 0 });
  const [draftPage, setDraftPage] = useState<PageInfo>({ total: 0 });
  const [functionalCurrency, setFunctionalCurrency] = useState("");
  const discardIdempotencyKeys = useRef(
    new Map<string, LogicalOperationKey>(),
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const entrySearch = new URLSearchParams({
        include_lines: "false",
        limit: "30",
      });
      if (query.trim()) entrySearch.set("query", query.trim());
      if (dateFrom) entrySearch.set("from", dateFrom);
      if (dateTo) entrySearch.set("to", dateTo);
      if (sourceType) entrySearch.set("source_type", sourceType);
      if (entryCursor) entrySearch.set("cursor", entryCursor);
      const draftSearch = new URLSearchParams({ limit: "30" });
      if (query.trim()) draftSearch.set("query", query.trim());
      if (dateFrom) draftSearch.set("from", dateFrom);
      if (dateTo) draftSearch.set("to", dateTo);
      if (draftCursor) draftSearch.set("cursor", draftCursor);
      const [journal, pendingDrafts, chart, settings] = await Promise.all([
        api.request<JournalEntryPage>(
          `/api/v1/accounting/journal-entries?${entrySearch}`,
        ),
        api.request<JournalDraftPage>(`/api/v1/accounting/drafts?${draftSearch}`),
        api.request<AccountList>(
          "/api/v1/accounting/accounts?lifecycle_state=active&postable=true&limit=100",
        ),
        api.request<AccountingSettingsView>("/api/v1/accounting/settings", {
          skipJSONContentType: true,
        }),
      ]);
      const entryItems = Array.isArray(journal?.items) ? journal.items : [];
      const draftItems = Array.isArray(pendingDrafts?.items)
        ? pendingDrafts.items
        : [];
      setEntries(entryItems);
      setEntryPage(journal?.page ?? { total: 0 });
      setDrafts(draftItems);
      setDraftPage(pendingDrafts?.page ?? { total: 0 });
      setSelectedDraftIDs((current) =>
        current.filter((id) => draftItems.some((draft) => draft.id === id)),
      );
      setAccounts(
        (Array.isArray(chart?.items) ? chart.items : []).filter(
          (item) => item.postable && item.lifecycle_state === "active",
        ),
      );
      setFunctionalCurrency(settings.functional_currency);
    } catch (cause) {
      setError(journalErrorMessage(cause, "No pudimos cargar el Diario."));
    } finally {
      setLoading(false);
    }
  }, [
    api,
    dateFrom,
    dateTo,
    draftCursor,
    entryCursor,
    query,
    sourceType,
  ]);
  useEffect(() => void load(), [load]);
  useEffect(() => {
    if (!canManage) {
      setShowCreate(false);
      setEditingDraft(undefined);
      setCopySeed(undefined);
      setSelectedDraftIDs([]);
    }
  }, [canManage]);

  function resetPagination() {
    setEntryCursor(undefined);
    setEntryCursorTrail([]);
    setDraftCursor(undefined);
    setDraftCursorTrail([]);
  }

  function changeQuery(value: string) {
    setQuery(value);
    resetPagination();
  }

  function changeMode(next: "posted" | "drafts") {
    setMode(next);
    if (next === "drafts") setSourceType("");
    setSelectedDraftIDs([]);
    resetPagination();
  }

  async function openEntry(entry: JournalEntryView | string) {
    setEntryDetailBusy(true);
    setError(undefined);
    try {
      const id = typeof entry === "string" ? entry : entry.id;
      setSelectedEntry(
        await api.request<JournalEntryView>(
          `/api/v1/accounting/journal-entries/${id}`,
          { skipJSONContentType: true },
        ),
      );
    } catch (cause) {
      setError(journalErrorMessage(cause, "No pudimos abrir el asiento."));
    } finally {
      setEntryDetailBusy(false);
    }
  }

  async function openDraft(draft: JournalDraftView) {
    if (
      editorDirty &&
      !window.confirm(
        "Hay cambios sin guardar. ¿Querés descartarlos y abrir otro borrador?",
      )
    ) {
      return;
    }
    setDraftDetailBusy(draft.id);
    setError(undefined);
    try {
      let detail = draft;
      try {
        detail = await api.request<JournalDraftView>(
          `/api/v1/accounting/drafts/${draft.id}`,
          { skipJSONContentType: true },
        );
      } catch (cause) {
        const normalized = normalizeHttpError(cause);
        if (normalized.status !== 404 || !draft.lines) throw cause;
      }
      setShowCreate(false);
      setCopySeed(undefined);
      setEditingDraft(detail);
    } catch (cause) {
      setError(journalErrorMessage(cause, "No pudimos abrir el borrador."));
    } finally {
      setDraftDetailBusy(undefined);
    }
  }

  async function discardSelectedDrafts() {
    const selected = drafts.filter((draft) =>
      selectedDraftIDs.includes(draft.id),
    );
    if (selected.length === 0) return;
    setDiscardBusy(true);
    setError(undefined);
    const results = await Promise.allSettled(
      selected.map((draft) => {
        const signature = `${draft.id}:${draft.version}`;
        const operationKey = logicalOperationKey(
          discardIdempotencyKeys.current.get(draft.id),
          "journal-draft-discard",
          signature,
        );
        discardIdempotencyKeys.current.set(draft.id, operationKey);
        return api.request<void>(
          `/api/v1/accounting/drafts/${draft.id}/discard`,
          {
            method: "POST",
            headers: {
              "Idempotency-Key": operationKey.value,
            },
            body: JSON.stringify({
              version: draft.version,
              reason: "Descartado desde el Diario",
            }),
          },
        );
      }),
    );
    const failedIDs = selected
      .filter((_, index) => results[index]?.status === "rejected")
      .map((draft) => draft.id);
    const discardedCount = selected.length - failedIDs.length;
    selected.forEach((draft, index) => {
      if (results[index]?.status === "fulfilled") {
        discardIdempotencyKeys.current.delete(draft.id);
      }
    });
    setSelectedDraftIDs(failedIDs);
    setDiscardRequested(false);
    setDiscardBusy(false);
    if (failedIDs.length > 0) {
      const firstFailure = results.find(
        (result): result is PromiseRejectedResult =>
          result.status === "rejected",
      );
      setError(
        `${discardedCount} ${
          discardedCount === 1 ? "borrador descartado" : "borradores descartados"
        }. ${journalErrorMessage(
          firstFailure?.reason,
          `No pudimos descartar ${failedIDs.length} ${
            failedIDs.length === 1 ? "borrador" : "borradores"
          }.`,
        )}`,
      );
    }
    if (editingDraft && selectedDraftIDs.includes(editingDraft.id) && !failedIDs.includes(editingDraft.id)) {
      setEditingDraft(undefined);
    }
    await load();
  }

  function copyEntry(entry: JournalEntryView) {
    if (
      editorDirty &&
      !window.confirm(
        "Hay cambios sin guardar. ¿Querés descartarlos y preparar esta copia?",
      )
    ) {
      return;
    }
    const lines = entry.lines ?? [];
    setSelectedEntry(undefined);
    setEditingDraft(undefined);
    setShowCreate(false);
    setCopySeed({
      id: "",
      accounting_date: calendarDate(),
      description: entry.description,
      currency: entry.currency,
      functional_currency: entry.functional_currency ?? functionalCurrency,
      exchange_rate: entry.exchange_rate ?? "1",
      exchange_rate_date: entry.exchange_rate_date,
      exchange_rate_source: entry.exchange_rate_source,
      reference: "",
      lines,
      total_debit: entry.total_debit,
      total_credit: entry.total_credit,
      version: 0,
    });
  }

  function startNew() {
    setEditingDraft(undefined);
    setCopySeed(undefined);
    setShowCreate(true);
  }

  function clearFilters() {
    setDateFrom("");
    setDateTo("");
    setSourceType("");
    resetPagination();
  }

  const filterCount =
    Number(Boolean(dateFrom)) + Number(Boolean(dateTo)) + Number(Boolean(sourceType));
  const pageDraftIDs = drafts.map((draft) => draft.id);
  const allPageDraftsSelected =
    pageDraftIDs.length > 0 &&
    pageDraftIDs.every((id) => selectedDraftIDs.includes(id));
  const somePageDraftsSelected =
    !allPageDraftsSelected &&
    pageDraftIDs.some((id) => selectedDraftIDs.includes(id));
  const draftSelectionActions: EntitySelectionAction[] = [
    {
      id: "discard",
      label: "Descartar",
      danger: true,
      onClick: () => setDiscardRequested(true),
    },
  ];

  return (
    <section className="directory-section journal-section">
      <div className="finance-toolbar journal-toolbar">
        {canManage ? (
          <button
            className="directory-create-button finance-toolbar__create"
            disabled={
              !functionalCurrency ||
              showCreate ||
              Boolean(editingDraft) ||
              Boolean(copySeed)
            }
            onClick={startNew}
            type="button"
          >
            <span>＋</span> Nuevo asiento
          </button>
        ) : null}
        <div className="finance-toolbar__start journal-toolbar__controls">
          <SectionSearch
            label="Buscar asientos"
            placeholder="Número, referencia, detalle o cuenta…"
            value={query}
            onChange={changeQuery}
          />
          <button
            aria-expanded={filtersOpen}
            className={`journal-filter-button ${filterCount ? "has-filters" : ""}`}
            onClick={() => setFiltersOpen((value) => !value)}
            type="button"
          >
            Filtros{filterCount ? ` · ${filterCount}` : ""}
          </button>
          <div className="lifecycle-tabs journal-mode-tabs" role="tablist" aria-label="Vista del Diario">
            <button
              aria-selected={mode === "posted"}
              className={mode === "posted" ? "is-active" : ""}
              onClick={() => changeMode("posted")}
              role="tab"
              type="button"
            >
              Contabilizados <span>{entryPage.total}</span>
            </button>
            <button
              aria-selected={mode === "drafts"}
              className={mode === "drafts" ? "is-active" : ""}
              onClick={() => changeMode("drafts")}
              role="tab"
              type="button"
            >
              Borradores <span>{draftPage.total}</span>
            </button>
          </div>
        </div>
      </div>
      {filtersOpen ? (
        <div className="journal-filters" aria-label="Filtros del Diario">
          <label>
            Desde
            <input
              type="date"
              value={dateFrom}
              onChange={(event) => {
                setDateFrom(event.target.value);
                resetPagination();
              }}
            />
          </label>
          <label>
            Hasta
            <input
              type="date"
              value={dateTo}
              onChange={(event) => {
                setDateTo(event.target.value);
                resetPagination();
              }}
            />
          </label>
          {mode === "posted" ? (
            <label>
              Origen
              <select
                value={sourceType}
                onChange={(event) => {
                  setSourceType(event.target.value);
                  resetPagination();
                }}
              >
                <option value="">Todos</option>
                <option value="manual_draft">Manual</option>
                <option value="sale">Ventas</option>
                <option value="purchase">Compras</option>
                <option value="receipt">Cobros</option>
                <option value="supplier_payment">Pagos</option>
                <option value="customer_credit_note">Devoluciones</option>
                <option value="journal_entry">Otro asiento</option>
              </select>
            </label>
          ) : null}
          <button
            disabled={filterCount === 0}
            onClick={clearFilters}
            type="button"
          >
            Limpiar filtros
          </button>
        </div>
      ) : null}
      {!canManage ? <ReadOnlyNote /> : null}
      {mode === "drafts" && canManage ? (
        <div className="journal-draft-selection">
          <EntitySelectionToolbar
            actions={draftSelectionActions}
            busy={discardBusy}
            onClear={() => setSelectedDraftIDs([])}
            selectedCount={selectedDraftIDs.length}
          />
        </div>
      ) : null}
      {canManage && (showCreate || editingDraft || copySeed) ? (
        <JournalForm
          accounts={accounts}
          draft={editingDraft ?? copySeed}
          functionalCurrency={functionalCurrency}
          isCopy={Boolean(copySeed)}
          key={
            editingDraft?.id ??
            (copySeed ? `copy-${copySeed.accounting_date}` : "new")
          }
          onCancel={() => {
            setEditorDirty(false);
            setEditingDraft(undefined);
            setCopySeed(undefined);
            setShowCreate(false);
          }}
          onPosted={(entry) => {
            setEditorDirty(false);
            setEditingDraft(undefined);
            setCopySeed(undefined);
            setShowCreate(false);
            setMode("posted");
            void openEntry(entry.id);
            void load();
          }}
          onSaved={(saved) => {
            setEditingDraft(saved);
            setCopySeed(undefined);
            setMode("drafts");
            void load();
          }}
          onDirtyChange={setEditorDirty}
        />
      ) : null}
      {!canManage && editingDraft ? (
        <JournalDraftDrawer
          draft={editingDraft}
          onClose={() => setEditingDraft(undefined)}
        />
      ) : null}
      <InlineFeedback error={error} loading={loading} />
      {mode === "posted" ? (
        <>
          <div className="directory-table-wrap">
            <table className="directory-table finance-table finance-table--interactive journal-table">
              <thead>
                <tr>
                  <th>Nº</th>
                  <th>Fecha</th>
                  <th>Referencia / detalle</th>
                  <th>Origen</th>
                  <th>Debe</th>
                  <th>Haber</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((entry) => (
                  <tr
                    aria-label={`Abrir asiento ${entry.entry_number}`}
                    key={entry.id}
                    onClick={() => void openEntry(entry)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        void openEntry(entry);
                      }
                    }}
                    tabIndex={0}
                  >
                    <td className="mono-cell">{entry.entry_number}</td>
                    <td>{formatDate(entry.accounting_date)}</td>
                    <td className="directory-table__primary">
                      {entry.reference ? <small>{entry.reference}</small> : null}
                      <strong>{entry.description}</strong>
                      {entry.reverses_entry_id ? (
                        <small className="finance-row-note">Reversa</small>
                      ) : null}
                    </td>
                    <td>{journalSourceLabel(entry.source_type)}</td>
                    <td className="money-cell">
                      {formatMoney(
                        entry.total_debit,
                        entry.functional_currency ?? entry.currency,
                      )}
                    </td>
                    <td className="money-cell">
                      {formatMoney(
                        entry.total_credit,
                        entry.functional_currency ?? entry.currency,
                      )}
                    </td>
                  </tr>
                ))}
                {!loading && entries.length === 0 ? (
                  <EmptyRow
                    columns={6}
                    text={
                      query || filterCount
                        ? "No encontramos asientos con esos filtros."
                        : "Todavía no hay asientos contabilizados."
                    }
                  />
                ) : null}
              </tbody>
            </table>
          </div>
          <CursorPagination
            currentPage={entryCursorTrail.length + 1}
            hasNext={Boolean(entryPage.next_cursor)}
            hasPrevious={entryCursorTrail.length > 0}
            onNext={() => {
              if (!entryPage.next_cursor) return;
              setEntryCursorTrail((previous) => [...previous, entryCursor ?? ""]);
              setEntryCursor(entryPage.next_cursor ?? undefined);
            }}
            onPrevious={() => {
              const previous = entryCursorTrail.at(-1) ?? "";
              setEntryCursorTrail((items) => items.slice(0, -1));
              setEntryCursor(previous || undefined);
            }}
            total={entryPage.total}
          />
        </>
      ) : (
        <>
          <div className="directory-table-wrap">
            <table className="directory-table finance-table finance-table--interactive journal-table journal-table--drafts">
              <thead>
                <tr>
                  {canManage ? (
                    <th className="directory-table__select">
                      <JournalPageSelectionCheckbox
                        checked={allPageDraftsSelected}
                        indeterminate={somePageDraftsSelected}
                        onChange={(checked) =>
                          setSelectedDraftIDs((current) =>
                            checked
                              ? [...new Set([...current, ...pageDraftIDs])]
                              : current.filter(
                                  (id) => !pageDraftIDs.includes(id),
                                ),
                          )
                        }
                      />
                    </th>
                  ) : null}
                  <th>Fecha</th>
                  <th>Referencia / detalle</th>
                  <th>Líneas</th>
                  <th>Debe</th>
                  <th>Haber</th>
                  <th>Preparación</th>
                  <th>Actualizado</th>
                </tr>
              </thead>
              <tbody>
                {drafts.map((draft) => {
                  const status = draftPostingStatus(draft);
                  return (
                    <tr
                      aria-label={`Abrir borrador ${draft.description || "sin detalle"}`}
                      aria-busy={draftDetailBusy === draft.id}
                      key={draft.id}
                      onClick={() => void openDraft(draft)}
                      onKeyDown={(event) => {
                        if (event.key === "Enter") {
                          event.preventDefault();
                          void openDraft(draft);
                        }
                      }}
                      tabIndex={0}
                    >
                      {canManage ? (
                        <td
                          className="directory-table__select"
                          onClick={(event) => event.stopPropagation()}
                        >
                          <input
                            aria-label={`Seleccionar borrador ${draft.description || "sin detalle"}`}
                            checked={selectedDraftIDs.includes(draft.id)}
                            onChange={(event) =>
                              setSelectedDraftIDs((current) =>
                                event.target.checked
                                  ? [...new Set([...current, draft.id])]
                                  : current.filter((id) => id !== draft.id),
                              )
                            }
                            type="checkbox"
                          />
                        </td>
                      ) : null}
                      <td>{formatDate(draft.accounting_date)}</td>
                      <td className="directory-table__primary">
                        {draft.reference ? <small>{draft.reference}</small> : null}
                        <strong>{draft.description || "Sin detalle"}</strong>
                      </td>
                      <td className="mono-cell">
                        {draft.line_count ?? draft.lines?.length ?? 0}
                      </td>
                      <td className="money-cell">
                        {formatMoney(
                          draft.total_debit,
                          draft.functional_currency ?? draft.currency,
                        )}
                      </td>
                      <td className="money-cell">
                        {formatMoney(
                          draft.total_credit,
                          draft.functional_currency ?? draft.currency,
                        )}
                      </td>
                      <td>
                        <JournalStatusBadge status={status.state} />
                      </td>
                      <td>
                        {draft.updated_at
                          ? formatDateTime(draft.updated_at)
                          : `v${draft.version}`}
                      </td>
                    </tr>
                  );
                })}
                {!loading && drafts.length === 0 ? (
                  <EmptyRow
                    columns={canManage ? 8 : 7}
                    text={
                      query || filterCount
                        ? "No encontramos borradores con esos filtros."
                        : "No hay borradores pendientes."
                    }
                  />
                ) : null}
              </tbody>
            </table>
          </div>
          <CursorPagination
            currentPage={draftCursorTrail.length + 1}
            hasNext={Boolean(draftPage.next_cursor)}
            hasPrevious={draftCursorTrail.length > 0}
            onNext={() => {
              if (!draftPage.next_cursor) return;
              setDraftCursorTrail((previous) => [...previous, draftCursor ?? ""]);
              setDraftCursor(draftPage.next_cursor ?? undefined);
            }}
            onPrevious={() => {
              const previous = draftCursorTrail.at(-1) ?? "";
              setDraftCursorTrail((items) => items.slice(0, -1));
              setDraftCursor(previous || undefined);
            }}
            total={draftPage.total}
          />
        </>
      )}
      {entryDetailBusy ? <div className="inline-state">Abriendo asiento…</div> : null}
      {selectedEntry ? (
        <EntryDrawer
          accounts={accounts}
          canManage={canManage}
          entry={selectedEntry}
          onClose={() => setSelectedEntry(undefined)}
          onCopy={() => copyEntry(selectedEntry)}
          onOpenRelated={(id) => void openEntry(id)}
          onReverse={() => setReverseEntry(selectedEntry)}
        />
      ) : null}
      {reverseEntry ? (
        <JournalReversalDialog
          entry={reverseEntry}
          onClose={() => setReverseEntry(undefined)}
          onCompleted={(created) => {
            setReverseEntry(undefined);
            void openEntry(created.id);
            void load();
          }}
        />
      ) : null}
      {discardRequested ? (
        <JournalModal
          eyebrow="Borradores"
          onClose={() => {
            if (!discardBusy) setDiscardRequested(false);
          }}
          title={`Descartar ${
            selectedDraftIDs.length === 1
              ? "el borrador seleccionado"
              : `${selectedDraftIDs.length} borradores`
          }`}
        >
          <p>
            Los borradores descartados dejan de estar disponibles para
            contabilizar. Esta acción no modifica ningún asiento confirmado.
          </p>
          <div className="journal-modal__actions">
            <button
              disabled={discardBusy}
              onClick={() => setDiscardRequested(false)}
              type="button"
            >
              Cancelar
            </button>
            <button
              className="journal-danger-button"
              disabled={discardBusy}
              onClick={() => void discardSelectedDrafts()}
              type="button"
            >
              {discardBusy ? "Descartando…" : "Descartar borradores"}
            </button>
          </div>
        </JournalModal>
      ) : null}
    </section>
  );
}

type EditableJournalLine = {
  localID: string;
  accountID: string;
  accountCode?: string;
  accountName?: string;
  debit: string;
  credit: string;
  memo: string;
};

function JournalForm({
  accounts,
  draft,
  functionalCurrency,
  isCopy = false,
  onCancel,
  onDirtyChange,
  onPosted,
  onSaved,
}: {
  accounts: Account[];
  draft?: JournalDraftView;
  functionalCurrency: string;
  isCopy?: boolean;
  onCancel: () => void;
  onDirtyChange: (dirty: boolean) => void;
  onPosted: (entry: JournalEntryView) => void;
  onSaved: (draft: JournalDraftView) => void;
}) {
  const api = useProductApi();
  const [busy, setBusy] = useState<"save" | "post" | "reload">();
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [persisted, setPersisted] = useState<JournalDraftView | undefined>(
    isCopy ? undefined : draft,
  );
  const [dirty, setDirty] = useState(false);
  const [date, setDate] = useState(draft?.accounting_date ?? calendarDate());
  const [reference, setReference] = useState(
    isCopy ? "" : draft?.reference ?? "",
  );
  const [description, setDescription] = useState(draft?.description ?? "");
  const [currency, setCurrency] = useState(
    draft?.currency ?? functionalCurrency,
  );
  const [exchangeRate, setExchangeRate] = useState(
    draft?.exchange_rate ?? "1",
  );
  const [exchangeRateDate, setExchangeRateDate] = useState(
    draft?.exchange_rate_date ?? draft?.accounting_date ?? calendarDate(),
  );
  const [exchangeRateSource, setExchangeRateSource] = useState(
    draft?.exchange_rate_source ?? "",
  );
  const [lines, setLines] = useState<EditableJournalLine[]>(() => {
    const existing = draft?.lines?.map(editableJournalLine) ?? [];
    return existing.length >= 2
      ? existing
      : [...existing, ...Array.from({ length: 2 - existing.length }, blankJournalLine)];
  });
  const [resolvedAccounts, setResolvedAccounts] = useState<Account[]>([]);
  const requestedAccountIDs = useRef(new Set<string>());
  const [postConfirmationOpen, setPostConfirmationOpen] = useState(false);
  const [closeConfirmationOpen, setCloseConfirmationOpen] = useState(false);
  const [conflictOpen, setConflictOpen] = useState(false);
  const saveIdempotencyKey = useRef<LogicalOperationKey | undefined>(
    undefined,
  );
  const postIdempotencyKey = useRef<LogicalOperationKey | undefined>(
    undefined,
  );

  const totalDebit = sumDecimalStrings(lines.map((line) => line.debit || "0"));
  const totalCredit = sumDecimalStrings(lines.map((line) => line.credit || "0"));
  const difference = subtractDecimalStrings(totalDebit, totalCredit);
  const structureIssue = journalDraftStructureIssue(lines);
  const availableAccounts = [
    ...accounts,
    ...resolvedAccounts.filter(
      (resolved) => !accounts.some((account) => account.id === resolved.id),
    ),
  ];
  const localStatus = journalFormPostingStatus({
    accounts: availableAccounts,
    description,
    difference,
    lines,
    totalCredit,
    totalDebit,
  });
  const postingStatus =
    !dirty && persisted?.posting_status
      ? persisted.posting_status
      : localStatus;
  const readyToPost =
    Boolean(persisted) &&
    !dirty &&
    postingStatus.state === "ready" &&
    busy === undefined;
  const foreignCurrency = currency !== functionalCurrency;
  const lineAccountSignature = [
    ...new Set(lines.map((line) => line.accountID).filter(Boolean)),
  ]
    .sort()
    .join(",");

  function markDirty() {
    setDirty(true);
    setNotice(undefined);
    setError(undefined);
  }

  function mutateLine(
    index: number,
    field: keyof EditableJournalLine,
    value: string,
  ) {
    setLines((previous) =>
      previous.map((line, itemIndex) =>
        itemIndex === index ? { ...line, [field]: value } : line,
      ),
    );
    markDirty();
  }

  function selectAccount(index: number, account?: Account) {
    if (account) {
      setResolvedAccounts((current) => [
        ...current.filter((item) => item.id !== account.id),
        account,
      ]);
    }
    setLines((previous) =>
      previous.map((line, itemIndex) =>
        itemIndex === index
          ? {
              ...line,
              accountID: account?.id ?? "",
              accountCode: account?.code,
              accountName: account?.name,
            }
          : line,
      ),
    );
    markDirty();
  }

  useEffect(() => {
    const knownAccountIDs = new Set(
      [...accounts, ...resolvedAccounts].map((account) => account.id),
    );
    const missingAccountIDs = lineAccountSignature
      .split(",")
      .filter(
        (id) =>
          id &&
          !knownAccountIDs.has(id) &&
          !requestedAccountIDs.current.has(id),
      );
    if (missingAccountIDs.length === 0) return;

    const controller = new AbortController();
    let active = true;
    missingAccountIDs.forEach((id) => requestedAccountIDs.current.add(id));
    void Promise.allSettled(
      missingAccountIDs.map((id) =>
        api.request<Account>(`/api/v1/accounting/accounts/${id}`, {
          signal: controller.signal,
          skipJSONContentType: true,
        }),
      ),
    ).then((results) => {
      if (!active) return;
      const found = results.flatMap((result) =>
        result.status === "fulfilled" ? [result.value] : [],
      );
      if (found.length === 0) return;
      setResolvedAccounts((current) => {
        const byID = new Map(
          current.map((account) => [account.id, account]),
        );
        found.forEach((account) => byID.set(account.id, account));
        return [...byID.values()];
      });
    });

    return () => {
      active = false;
      controller.abort();
      missingAccountIDs.forEach((id) =>
        requestedAccountIDs.current.delete(id),
      );
    };
  }, [
    accounts,
    api,
    lineAccountSignature,
    resolvedAccounts,
  ]);

  function payload() {
    return {
      accounting_date: date,
      ...(reference.trim() ? { reference: reference.trim() } : {}),
      description: description.trim(),
      currency: currency.trim().toUpperCase(),
      ...(foreignCurrency
        ? {
            exchange_rate: exchangeRate,
            exchange_rate_date: exchangeRateDate,
            exchange_rate_source: exchangeRateSource.trim(),
          }
        : {}),
      lines: lines
        .filter((line) => !journalLineIsBlank(line))
        .map((line) => ({
          account_id: line.accountID,
          debit: line.debit || "0",
          credit: line.credit || "0",
          ...(line.memo.trim() ? { memo: line.memo.trim() } : {}),
        })),
    };
  }

  async function saveDraft(options: { asNew?: boolean } = {}) {
    if (!date || !currency.trim()) {
      setError("Indicá la fecha contable y la moneda.");
      return;
    }
    if (structureIssue) {
      setError(structureIssue);
      return;
    }
    if (
      foreignCurrency &&
      (!decimalValueIsPositive(exchangeRate) ||
        !exchangeRateDate ||
        !exchangeRateSource.trim())
    ) {
      setError(
        "Para una moneda extranjera indicá cotización, fecha y fuente.",
      );
      return;
    }
    const updating = Boolean(persisted) && !options.asNew;
    const requestBody = {
      ...payload(),
      ...(updating ? { version: persisted?.version } : {}),
    };
    const requestPath = updating
      ? `/api/v1/accounting/drafts/${persisted?.id}`
      : "/api/v1/accounting/drafts";
    const saveSignature = `${updating ? "update" : "create"}:${requestPath}:${JSON.stringify(
      requestBody,
    )}`;
    saveIdempotencyKey.current = logicalOperationKey(
      saveIdempotencyKey.current,
      updating ? "journal-draft-update" : "journal-draft",
      saveSignature,
    );
    setBusy("save");
    setError(undefined);
    setNotice(undefined);
    try {
      const saved = await api.request<JournalDraftView>(
        requestPath,
        {
          method: updating ? "PUT" : "POST",
          headers: {
            "Idempotency-Key": saveIdempotencyKey.current.value,
          },
          body: JSON.stringify(requestBody),
        },
      );
      saveIdempotencyKey.current = undefined;
      postIdempotencyKey.current = undefined;
      setPersisted(saved);
      setDirty(false);
      setConflictOpen(false);
      setNotice(`Borrador guardado · versión ${saved.version}`);
      onSaved(saved);
    } catch (cause) {
      const normalized = normalizeHttpError(cause);
      if (
        normalized.code === "VERSION_CONFLICT" ||
        (normalized.status === 409 && !normalized.code)
      ) {
        setConflictOpen(true);
      } else {
        setError(
          journalErrorMessage(cause, "No pudimos guardar el borrador."),
        );
      }
    } finally {
      setBusy(undefined);
    }
  }

  async function reloadLatest() {
    if (!persisted) return;
    setBusy("reload");
    setError(undefined);
    try {
      const latest = await api.request<JournalDraftView>(
        `/api/v1/accounting/drafts/${persisted.id}`,
        { skipJSONContentType: true },
      );
      requestedAccountIDs.current.clear();
      setResolvedAccounts([]);
      hydrateJournalDraft(latest, {
        setCurrency,
        setDate,
        setDescription,
        setExchangeRate,
        setExchangeRateDate,
        setExchangeRateSource,
        setLines,
        setReference,
      });
      setPersisted(latest);
      saveIdempotencyKey.current = undefined;
      postIdempotencyKey.current = undefined;
      setDirty(false);
      setConflictOpen(false);
      setNotice(`Borrador actualizado · versión ${latest.version}`);
      onSaved(latest);
    } catch (cause) {
      setError(
        journalErrorMessage(cause, "No pudimos recargar el borrador."),
      );
    } finally {
      setBusy(undefined);
    }
  }

  async function post() {
    if (!persisted || !readyToPost) return;
    const requestBody = {
      version: persisted.version,
      reason: "Asiento manual",
    };
    postIdempotencyKey.current = logicalOperationKey(
      postIdempotencyKey.current,
      "journal-post",
      `${persisted.id}:${JSON.stringify(requestBody)}`,
    );
    setBusy("post");
    setError(undefined);
    try {
      const entry = await api.request<JournalEntryView>(
        `/api/v1/accounting/drafts/${persisted.id}/post`,
        {
          method: "POST",
          headers: {
            "Idempotency-Key": postIdempotencyKey.current.value,
          },
          body: JSON.stringify(requestBody),
        },
      );
      postIdempotencyKey.current = undefined;
      setPostConfirmationOpen(false);
      onPosted(entry);
    } catch (cause) {
      const normalized = normalizeHttpError(cause);
      if (
        normalized.code === "VERSION_CONFLICT" ||
        (normalized.status === 409 && !normalized.code)
      ) {
        setPostConfirmationOpen(false);
        setConflictOpen(true);
      } else {
        setError(
          journalErrorMessage(cause, "No pudimos contabilizar el asiento."),
        );
      }
    } finally {
      setBusy(undefined);
    }
  }

  function requestClose() {
    if (dirty) {
      setCloseConfirmationOpen(true);
      return;
    }
    onCancel();
  }

  useEffect(() => {
    onDirtyChange(dirty);
  }, [dirty, onDirtyChange]);

  useEffect(() => {
    if (!dirty) return;
    function handleBeforeUnload(event: BeforeUnloadEvent) {
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => window.removeEventListener("beforeunload", handleBeforeUnload);
  }, [dirty]);

  useEffect(() => {
    function handleSaveShortcut(event: KeyboardEvent) {
      if (
        (event.ctrlKey || event.metaKey) &&
        event.key.toLocaleLowerCase("es") === "s"
      ) {
        event.preventDefault();
        if (!busy) void saveDraft();
      }
    }
    window.addEventListener("keydown", handleSaveShortcut);
    return () => window.removeEventListener("keydown", handleSaveShortcut);
  });

  const blockerCopy = postingIssueCopy(postingStatus.issues[0]);

  return (
    <>
      <form
        className="journal-editor"
        onSubmit={(event) => {
          event.preventDefault();
          void saveDraft();
        }}
      >
        <header>
          <div>
            <small>
              {persisted
                ? `Borrador · v${persisted.version}`
                : isCopy
                  ? "Copia de asiento"
                  : "Asiento manual"}
            </small>
            <strong>
              {persisted
                ? "Editar borrador"
                : isCopy
                  ? "Preparar una copia"
                  : "Preparar borrador"}
            </strong>
          </div>
          <button
            aria-label="Cerrar editor de asiento"
            onClick={requestClose}
            type="button"
          >
            ×
          </button>
        </header>
        <div className="journal-editor__meta">
          <label>
            Fecha
            <input
              required
              type="date"
              value={date}
              onChange={(event) => {
                setDate(event.target.value);
                markDirty();
              }}
            />
          </label>
          <label>
            Referencia
            <input
              maxLength={160}
              placeholder="Comprobante u origen"
              value={reference}
              onChange={(event) => {
                setReference(event.target.value);
                markDirty();
              }}
            />
          </label>
          <label className="finance-form__wide journal-editor__description">
            Detalle
            <input
              maxLength={500}
              placeholder="Descripción del asiento"
              value={description}
              onChange={(event) => {
                setDescription(event.target.value);
                markDirty();
              }}
            />
          </label>
          <label>
            Moneda
            <select
              value={currency}
              onChange={(event) => {
                const nextCurrency = event.target.value;
                setCurrency(nextCurrency);
                if (nextCurrency === functionalCurrency) {
                  setExchangeRate("1");
                }
                markDirty();
              }}
            >
              {[functionalCurrency, "USD", "EUR", "BRL", "UYU"]
                .filter((value, index, values) => values.indexOf(value) === index)
                .map((value) => (
                  <option key={value} value={value}>
                    {value}
                  </option>
                ))}
            </select>
          </label>
          {foreignCurrency ? (
            <div className="journal-editor__fx">
              <label>
                Cotización
                <input
                  inputMode="decimal"
                  placeholder="0.000000"
                  value={exchangeRate}
                  onChange={(event) => {
                    setExchangeRate(event.target.value);
                    markDirty();
                  }}
                />
              </label>
              <label>
                Fecha de cotización
                <input
                  type="date"
                  value={exchangeRateDate}
                  onChange={(event) => {
                    setExchangeRateDate(event.target.value);
                    markDirty();
                  }}
                />
              </label>
              <label>
                Fuente
                <input
                  placeholder="BNA, comprobante…"
                  value={exchangeRateSource}
                  onChange={(event) => {
                    setExchangeRateSource(event.target.value);
                    markDirty();
                  }}
                />
              </label>
            </div>
          ) : null}
        </div>
        <div className="journal-line-editor">
          <div className="journal-line-editor__head">
            <span>Cuenta</span>
            <span>Debe</span>
            <span>Haber</span>
            <span>Detalle de línea</span>
            <span />
          </div>
          {lines.map((line, index) => (
            <div className="journal-line-editor__row" key={line.localID}>
              <JournalAccountCombobox
                accounts={availableAccounts}
                fallbackCode={line.accountCode}
                fallbackName={line.accountName}
                index={index}
                onSelect={(account) => selectAccount(index, account)}
                selectedID={line.accountID}
              />
              <label>
                <span className="visually-hidden">Debe línea {index + 1}</span>
                <input
                  aria-label={`Debe línea ${index + 1}`}
                  inputMode="decimal"
                  placeholder="0.00"
                  value={line.debit}
                  onChange={(event) =>
                    mutateLine(index, "debit", event.target.value)
                  }
                />
              </label>
              <label>
                <span className="visually-hidden">Haber línea {index + 1}</span>
                <input
                  aria-label={`Haber línea ${index + 1}`}
                  inputMode="decimal"
                  placeholder="0.00"
                  value={line.credit}
                  onChange={(event) =>
                    mutateLine(index, "credit", event.target.value)
                  }
                />
              </label>
              <label>
                <span className="visually-hidden">Detalle línea {index + 1}</span>
                <input
                  aria-label={`Detalle línea ${index + 1}`}
                  maxLength={500}
                  placeholder="Opcional"
                  value={line.memo}
                  onChange={(event) =>
                    mutateLine(index, "memo", event.target.value)
                  }
                />
              </label>
              <button
                aria-label={`Quitar línea ${index + 1}`}
                disabled={lines.length <= 2}
                onClick={() => {
                  setLines((previous) =>
                    previous.filter((_, itemIndex) => itemIndex !== index),
                  );
                  markDirty();
                }}
                type="button"
              >
                ×
              </button>
            </div>
          ))}
        </div>
        <footer>
          <button
            onClick={() => {
              setLines((previous) => [...previous, blankJournalLine()]);
              markDirty();
            }}
            type="button"
          >
            ＋ Agregar línea
          </button>
          <div
            aria-live="polite"
            className={`journal-balance-rail is-${postingStatus.state}`}
          >
            <span>
              Debe <strong>{formatMoney(totalDebit, currency)}</strong>
            </span>
            <span>
              Haber <strong>{formatMoney(totalCredit, currency)}</strong>
            </span>
            <span>
              Diferencia <strong>{formatMoney(difference, currency)}</strong>
            </span>
            <JournalStatusBadge status={postingStatus.state} />
            {foreignCurrency && decimalValueIsPositive(exchangeRate) ? (
              <small className="journal-balance-rail__functional">
                Equivalente funcional · Debe{" "}
                {formatMoney(
                  multiplyDecimalStrings(totalDebit, exchangeRate),
                  functionalCurrency,
                )}{" "}
                · Haber{" "}
                {formatMoney(
                  multiplyDecimalStrings(totalCredit, exchangeRate),
                  functionalCurrency,
                )}
              </small>
            ) : null}
          </div>
          <div className="journal-editor__actions">
            <button
              disabled={busy !== undefined}
              onClick={requestClose}
              type="button"
            >
              Cancelar
            </button>
            <button
              disabled={Boolean(structureIssue) || busy !== undefined}
              type="submit"
            >
              {busy === "save" ? "Guardando…" : "Guardar borrador"}
            </button>
            <button
              className="directory-create-button"
              disabled={!readyToPost}
              onClick={() => setPostConfirmationOpen(true)}
              type="button"
            >
              {busy === "post" ? "Contabilizando…" : "Contabilizar"}
            </button>
          </div>
        </footer>
        {dirty && persisted ? (
          <span className="journal-editor__hint">
            Guardá los cambios antes de contabilizar.
          </span>
        ) : null}
        {postingStatus.state !== "ready" ? (
          <span className="journal-editor__posting-hint">
            {blockerCopy ??
              "Para contabilizar, Debe y Haber deben coincidir."}
          </span>
        ) : null}
        {notice ? (
          <span className="form-success" role="status">
            {notice}
          </span>
        ) : null}
        {error ? (
          <span className="form-error" role="alert">
            {error}
          </span>
        ) : null}
      </form>
      {postConfirmationOpen ? (
        <JournalModal
          eyebrow="Confirmación"
          onClose={() => {
            if (!busy) setPostConfirmationOpen(false);
          }}
          title="Contabilizar asiento"
        >
          <div className="journal-confirmation-summary">
            <span>
              Fecha<strong>{formatDate(date)}</strong>
            </span>
            <span>
              Período<strong>{date.slice(0, 7)}</strong>
            </span>
            <span>
              Referencia<strong>{reference.trim() || "Sin referencia"}</strong>
            </span>
            <span>
              Detalle<strong>{description.trim() || "Sin detalle"}</strong>
            </span>
            <span>
              Líneas
              <strong>
                {lines.filter((line) => !journalLineIsBlank(line)).length}
              </strong>
            </span>
            <span>
              Debe<strong>{formatMoney(totalDebit, currency)}</strong>
            </span>
            <span>
              Haber<strong>{formatMoney(totalCredit, currency)}</strong>
            </span>
            <span>
              Diferencia<strong>{formatMoney(difference, currency)}</strong>
            </span>
          </div>
          <p className="journal-modal__warning">
            Al contabilizar se asignará el número definitivo. El asiento no
            podrá editarse ni eliminarse; cualquier corrección se hará mediante
            una reversa.
          </p>
          <div className="journal-modal__actions">
            <button
              disabled={busy !== undefined}
              onClick={() => setPostConfirmationOpen(false)}
              type="button"
            >
              Volver
            </button>
            <button
              className="directory-create-button"
              disabled={busy !== undefined}
              onClick={() => void post()}
              type="button"
            >
              {busy === "post" ? "Contabilizando…" : "Contabilizar asiento"}
            </button>
          </div>
        </JournalModal>
      ) : null}
      {closeConfirmationOpen ? (
        <JournalModal
          eyebrow="Cambios sin guardar"
          onClose={() => setCloseConfirmationOpen(false)}
          title="¿Cerrar el borrador?"
        >
          <p>Los cambios realizados desde el último guardado se perderán.</p>
          <div className="journal-modal__actions">
            <button
              onClick={() => setCloseConfirmationOpen(false)}
              type="button"
            >
              Seguir editando
            </button>
            <button
              className="journal-danger-button"
              onClick={onCancel}
              type="button"
            >
              Cerrar sin guardar
            </button>
          </div>
        </JournalModal>
      ) : null}
      {conflictOpen ? (
        <JournalModal
          eyebrow="Conflicto de versión"
          onClose={() => setConflictOpen(false)}
          title="Este borrador cambió en otra sesión"
        >
          <p>
            No sobrescribimos la versión más reciente. Podés recargarla o
            guardar tu contenido actual como un borrador nuevo.
          </p>
          <div className="journal-modal__actions">
            <button
              disabled={busy !== undefined}
              onClick={() => void reloadLatest()}
              type="button"
            >
              {busy === "reload" ? "Recargando…" : "Recargar última versión"}
            </button>
            <button
              className="directory-create-button"
              disabled={busy !== undefined}
              onClick={() => void saveDraft({ asNew: true })}
              type="button"
            >
              Guardar como nuevo
            </button>
          </div>
        </JournalModal>
      ) : null}
    </>
  );
}

function JournalAccountCombobox({
  accounts,
  fallbackCode,
  fallbackName,
  index,
  onSelect,
  selectedID,
}: {
  accounts: Account[];
  fallbackCode?: string;
  fallbackName?: string;
  index: number;
  onSelect: (account?: Account) => void;
  selectedID: string;
}) {
  const api = useProductApi();
  const listID = useId();
  const selected = accounts.find((account) => account.id === selectedID);
  const selectedLabel = selected
    ? `${selected.code} · ${selected.name}`
    : selectedID && (fallbackCode || fallbackName)
      ? `${fallbackCode ?? ""}${fallbackCode && fallbackName ? " · " : ""}${
          fallbackName ?? ""
        }`
      : "";
  const [query, setQuery] = useState(selectedLabel);
  const [open, setOpen] = useState(false);
  const [remoteOptions, setRemoteOptions] = useState<Account[]>();
  const [activeIndex, setActiveIndex] = useState(-1);

  useEffect(() => {
    if (!open || !query.trim() || query === selectedLabel) {
      setRemoteOptions(undefined);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      const search = new URLSearchParams({
        lifecycle_state: "active",
        postable: "true",
        limit: "20",
        query: query.trim(),
      });
      api
        .request<AccountList>(`/api/v1/accounting/accounts?${search}`, {
          signal: controller.signal,
          skipJSONContentType: true,
        })
        .then((response) =>
          setRemoteOptions(
            (Array.isArray(response?.items) ? response.items : []).filter(
              (account) =>
                account.postable && account.lifecycle_state === "active",
            ),
          ),
        )
        .catch(() => {
          if (!controller.signal.aborted) setRemoteOptions(undefined);
        });
    }, 180);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [api, open, query, selectedLabel]);

  useEffect(() => {
    if (!open) setQuery(selectedLabel);
  }, [open, selectedLabel]);

  const normalized = query.trim().toLocaleLowerCase("es");
  const visibleOptions = (remoteOptions ?? accounts)
    .filter((account) => {
      if (!account.postable || account.lifecycle_state !== "active") {
        return false;
      }
      if (!normalized || query === selectedLabel) return true;
      return `${account.code} ${account.name}`
        .toLocaleLowerCase("es")
        .includes(normalized);
    })
    .slice(0, 20);

  function choose(account?: Account) {
    onSelect(account);
    setQuery(account ? `${account.code} · ${account.name}` : "");
    setOpen(false);
  }

  return (
    <div className="journal-account-combobox">
      <label>
        <span className="visually-hidden">Cuenta línea {index + 1}</span>
        <input
          aria-activedescendant={
            open && activeIndex >= 0 && visibleOptions[activeIndex]
              ? `${listID}-${visibleOptions[activeIndex]?.id}`
              : undefined
          }
          aria-autocomplete="list"
          aria-controls={listID}
          aria-expanded={open}
          aria-label={`Cuenta línea ${index + 1}`}
          autoComplete="off"
          onBlur={() => window.setTimeout(() => setOpen(false), 0)}
          onChange={(event) => {
            setQuery(event.target.value);
            if (selectedID) onSelect(undefined);
            setActiveIndex(-1);
            setOpen(true);
          }}
          onFocus={() => {
            setOpen(true);
            setActiveIndex(-1);
          }}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              setOpen(true);
              setActiveIndex((current) =>
                Math.min(current + 1, visibleOptions.length - 1),
              );
            } else if (event.key === "ArrowUp") {
              event.preventDefault();
              setActiveIndex((current) => Math.max(current - 1, 0));
            } else if (event.key === "Enter" && open) {
              event.preventDefault();
              const option = visibleOptions[activeIndex];
              if (option) choose(option);
            } else if (event.key === "Escape") {
              setOpen(false);
            }
          }}
          placeholder="Buscar por código o nombre…"
          role="combobox"
          value={query}
        />
      </label>
      {open ? (
        <div className="journal-account-combobox__options" id={listID} role="listbox">
          {visibleOptions.map((account, optionIndex) => (
            <button
              aria-selected={selectedID === account.id}
              className={optionIndex === activeIndex ? "is-active" : ""}
              id={`${listID}-${account.id}`}
              key={account.id}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => choose(account)}
              role="option"
              type="button"
            >
              <strong>{account.code}</strong>
              <span>{account.name}</span>
            </button>
          ))}
          {visibleOptions.length === 0 ? (
            <span>No hay cuentas imputables que coincidan.</span>
          ) : null}
        </div>
      ) : null}
      {selectedID &&
      (!selected || selected.lifecycle_state !== "active") ? (
        <small className="journal-account-combobox__warning">
          Esta cuenta ya no está disponible para contabilizar.
        </small>
      ) : selectedID && selected && !selected.postable ? (
        <small className="journal-account-combobox__warning">
          Esta cuenta es un rubro y no admite imputaciones.
        </small>
      ) : null}
    </div>
  );
}

function JournalStatusBadge({ status }: { status: JournalPostingState }) {
  const labels: Record<JournalPostingState, string> = {
    incomplete: "Incompleto",
    unbalanced: "Desbalanceado",
    blocked: "Bloqueado",
    ready: "Listo para contabilizar",
  };
  return (
    <span className={`journal-status-badge is-${status}`}>{labels[status]}</span>
  );
}

function JournalPageSelectionCheckbox({
  checked,
  indeterminate,
  onChange,
}: {
  checked: boolean;
  indeterminate: boolean;
  onChange: (checked: boolean) => void;
}) {
  const checkbox = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (checkbox.current) checkbox.current.indeterminate = indeterminate;
  }, [indeterminate]);
  return (
    <input
      aria-label={
        checked
          ? "Deseleccionar todos los borradores de la página"
          : "Seleccionar todos los borradores de la página"
      }
      checked={checked}
      onChange={(event) => onChange(event.target.checked)}
      ref={checkbox}
      type="checkbox"
    />
  );
}

function JournalDraftDrawer({
  draft,
  onClose,
}: {
  draft: JournalDraftView;
  onClose: () => void;
}) {
  const drawer = useRef<HTMLElement>(null);
  useDialogFocus(drawer, onClose);
  const status = draftPostingStatus(draft);
  const originalTotals = journalOriginalTotals(draft);

  return (
    <div
      className="finance-drawer-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <aside
        aria-labelledby="draft-drawer-title"
        aria-modal="true"
        className="finance-drawer journal-entry-drawer journal-draft-drawer"
        ref={drawer}
        role="dialog"
        tabIndex={-1}
      >
        <header>
          <div>
            <small>Borrador · versión {draft.version}</small>
            <h2 id="draft-drawer-title">
              {draft.description || "Sin detalle"}
            </h2>
            <span>
              {formatDate(draft.accounting_date)} · {draft.currency}
            </span>
          </div>
          <button
            aria-label="Cerrar detalle del borrador"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </header>
        <div className="finance-drawer__facts">
          <span>
            Referencia<strong>{draft.reference || "Sin referencia"}</strong>
          </span>
          <span>
            Preparación
            <strong>
              <JournalStatusBadge status={status.state} />
            </strong>
          </span>
          {draft.updated_at ? (
            <span>
              Última modificación
              <strong>{formatDateTime(draft.updated_at)}</strong>
            </span>
          ) : null}
          {draft.updated_by ? (
            <span>
              Modificado por<strong>{draft.updated_by}</strong>
            </span>
          ) : null}
          {draft.exchange_rate &&
          draft.currency !== draft.functional_currency ? (
            <span>
              Cotización
              <strong>
                {draft.exchange_rate} ·{" "}
                {draft.exchange_rate_source || "Sin fuente"}
              </strong>
            </span>
          ) : null}
        </div>
        {status.issues.length > 0 ? (
          <div className="journal-draft-drawer__issues">
            <strong>Antes de contabilizar</strong>
            <ul>
              {status.issues.map((issue) => (
                <li key={issue}>{postingIssueCopy(issue)}</li>
              ))}
            </ul>
          </div>
        ) : null}
        <div className="finance-drawer__lines">
          <div>
            <span>Cuenta</span>
            <span>Debe</span>
            <span>Haber</span>
          </div>
          {(draft.lines ?? []).map((line, index) => {
            const accountLabel =
              line.account_code || line.account_name
                ? `${line.account_code ?? ""}${
                    line.account_code && line.account_name ? " · " : ""
                  }${line.account_name ?? ""}`
                : "Cuenta contable";
            return (
              <div key={line.id ?? `${line.account_id}-${index}`}>
                <span>
                  <strong>{accountLabel}</strong>
                  {line.memo ? <small>{line.memo}</small> : null}
                </span>
                <span>
                  {formatMoney(
                    line.debit,
                    draft.functional_currency ?? draft.currency,
                  )}
                  {line.transaction_amount &&
                  line.transaction_currency &&
                  line.transaction_currency !== draft.functional_currency &&
                  decimalValueIsPositive(line.debit) ? (
                    <small>
                      {formatMoney(
                        line.transaction_amount,
                        line.transaction_currency,
                      )}
                    </small>
                  ) : null}
                </span>
                <span>
                  {formatMoney(
                    line.credit,
                    draft.functional_currency ?? draft.currency,
                  )}
                  {line.transaction_amount &&
                  line.transaction_currency &&
                  line.transaction_currency !== draft.functional_currency &&
                  decimalValueIsPositive(line.credit) ? (
                    <small>
                      {formatMoney(
                        line.transaction_amount,
                        line.transaction_currency,
                      )}
                    </small>
                  ) : null}
                </span>
              </div>
            );
          })}
          {(draft.lines ?? []).length === 0 ? (
            <p className="journal-draft-drawer__empty">
              Este borrador todavía no tiene líneas.
            </p>
          ) : null}
        </div>
        <footer>
          <div>
            <span>
              Debe
              <strong>
                {formatMoney(
                  draft.total_debit,
                  draft.functional_currency ?? draft.currency,
                )}
              </strong>
            </span>
            <span>
              Haber
              <strong>
                {formatMoney(
                  draft.total_credit,
                  draft.functional_currency ?? draft.currency,
                )}
              </strong>
            </span>
            <span>
              Diferencia
              <strong>
                {formatMoney(
                  status.difference,
                  draft.functional_currency ?? draft.currency,
                )}
              </strong>
            </span>
            {originalTotals.map((totals) => (
              <Fragment key={totals.currency}>
                <span>
                  Debe original ({totals.currency})
                  <strong>
                    {formatMoney(totals.debit, totals.currency)}
                  </strong>
                </span>
                <span>
                  Haber original ({totals.currency})
                  <strong>
                    {formatMoney(totals.credit, totals.currency)}
                  </strong>
                </span>
              </Fragment>
            ))}
          </div>
        </footer>
      </aside>
    </div>
  );
}

function EntryDrawer({
  accounts,
  canManage,
  entry,
  onClose,
  onCopy,
  onOpenRelated,
  onReverse,
}: {
  accounts: Account[];
  canManage: boolean;
  entry: JournalEntryView;
  onClose: () => void;
  onCopy: () => void;
  onOpenRelated: (id: string) => void;
  onReverse: () => void;
}) {
  const drawer = useRef<HTMLElement>(null);
  useDialogFocus(drawer, onClose);
  const accountByID = new Map(accounts.map((account) => [account.id, account]));
  const originalTotals = journalOriginalTotals(entry);
  const manual =
    !entry.source_type ||
    entry.source_type === "manual" ||
    entry.source_type === "manual_draft" ||
    entry.kind === "manual";
  const adjustment =
    entry.posting_kind === "adjustment" || entry.kind === "adjustment";
  const reversal =
    entry.posting_kind === "reversal" || entry.kind === "reversal";
  const documentarySource = [
    "sale",
    "purchase",
    "receipt",
    "collection",
    "payment",
    "supplier_payment",
    "customer_credit_note",
    "customer_debit_note",
    "open_item",
    "open-item",
    "inventory",
    "fiscal",
    "fiscal_voucher",
    "fiscal_purchase",
  ].includes(entry.source_type ?? "");
  const canReverse =
    canManage &&
    !entry.reversed_by_entry_id &&
    !documentarySource &&
    (manual || adjustment || reversal);

  return (
    <div
      className="finance-drawer-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <aside
        aria-labelledby="entry-drawer-title"
        aria-modal="true"
        className="finance-drawer journal-entry-drawer"
        ref={drawer}
        role="dialog"
        tabIndex={-1}
      >
        <header>
          <div>
            <small>Asiento Nº {entry.entry_number}</small>
            <h2 id="entry-drawer-title">{entry.description}</h2>
            <span>
              {formatDate(entry.accounting_date)} · {entry.currency}
            </span>
          </div>
          <button
            aria-label="Cerrar detalle del asiento"
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </header>
        <div className="finance-drawer__facts">
          <span>
            Referencia<strong>{entry.reference || "Sin referencia"}</strong>
          </span>
          <span>
            Origen<strong>{journalSourceLabel(entry.source_type)}</strong>
          </span>
          <span>
            Tipo
            <strong>
              {entry.posting_kind
                ? journalPostingKindLabel(entry.posting_kind)
                : "Asiento general"}
            </strong>
          </span>
          <span>
            Creado<strong>{formatDateTime(entry.created_at)}</strong>
          </span>
          {entry.created_by ? (
            <span>
              Actor<strong>{entry.created_by}</strong>
            </span>
          ) : null}
          {entry.exchange_rate && entry.currency !== entry.functional_currency ? (
            <span>
              Cotización
              <strong>
                {entry.exchange_rate} · {entry.exchange_rate_source || "Sin fuente"}
              </strong>
            </span>
          ) : null}
          {entry.reversal_reason ? (
            <span>
              Motivo de reversa<strong>{entry.reversal_reason}</strong>
            </span>
          ) : null}
        </div>
        {entry.reverses_entry_id || entry.reversed_by_entry_id ? (
          <div className="journal-reversal-chain">
            {entry.reverses_entry_id ? (
              <button
                onClick={() => onOpenRelated(entry.reverses_entry_id as string)}
                type="button"
              >
                {entry.reverses_entry_number
                  ? `Reversa de #${entry.reverses_entry_number}`
                  : "Reversa de otro asiento"}{" "}
                · abrir original
              </button>
            ) : null}
            {entry.reversed_by_entry_id ? (
              <button
                onClick={() => onOpenRelated(entry.reversed_by_entry_id as string)}
                type="button"
              >
                {entry.reversed_by_entry_number
                  ? `Revertido por #${entry.reversed_by_entry_number}`
                  : "Este asiento fue revertido"}{" "}
                · abrir reversa
              </button>
            ) : null}
          </div>
        ) : null}
        <div className="finance-drawer__lines">
          <div>
            <span>Cuenta</span>
            <span>Debe</span>
            <span>Haber</span>
          </div>
          {(entry.lines ?? []).map((line) => {
            const account = accountByID.get(line.account_id);
            const accountLabel =
              line.account_code || line.account_name
                ? `${line.account_code ?? ""}${
                    line.account_code && line.account_name ? " · " : ""
                  }${line.account_name ?? ""}`
                : account
                  ? `${account.code} · ${account.name}`
                  : "Cuenta contable";
            return (
              <div key={line.id}>
                <span>
                  <strong>{accountLabel}</strong>
                  {line.memo ? <small>{line.memo}</small> : null}
                </span>
                <span>
                  {formatMoney(
                    line.debit,
                    entry.functional_currency ?? entry.currency,
                  )}
                  {line.transaction_amount &&
                  line.transaction_currency &&
                  line.transaction_currency !== entry.functional_currency &&
                  decimalValueIsPositive(line.debit) ? (
                    <small>
                      {formatMoney(
                        line.transaction_amount,
                        line.transaction_currency,
                      )}
                    </small>
                  ) : null}
                </span>
                <span>
                  {formatMoney(
                    line.credit,
                    entry.functional_currency ?? entry.currency,
                  )}
                  {line.transaction_amount &&
                  line.transaction_currency &&
                  line.transaction_currency !== entry.functional_currency &&
                  decimalValueIsPositive(line.credit) ? (
                    <small>
                      {formatMoney(
                        line.transaction_amount,
                        line.transaction_currency,
                      )}
                    </small>
                  ) : null}
                </span>
              </div>
            );
          })}
        </div>
        <footer>
          <div>
            <span>
              Debe
              <strong>
                {formatMoney(
                  entry.total_debit,
                  entry.functional_currency ?? entry.currency,
                )}
              </strong>
            </span>
            <span>
              Haber
              <strong>
                {formatMoney(
                  entry.total_credit,
                  entry.functional_currency ?? entry.currency,
                )}
              </strong>
            </span>
            {originalTotals.map((totals) => (
              <Fragment key={totals.currency}>
                <span>
                  Debe original ({totals.currency})
                  <strong>
                    {formatMoney(totals.debit, totals.currency)}
                  </strong>
                </span>
                <span>
                  Haber original ({totals.currency})
                  <strong>
                    {formatMoney(totals.credit, totals.currency)}
                  </strong>
                </span>
              </Fragment>
            ))}
          </div>
          {canManage ? (
            <div className="journal-entry-drawer__actions">
              {manual ? (
                <button onClick={onCopy} type="button">
                  Copiar como nuevo
                </button>
              ) : null}
              {canReverse ? (
                <button
                  className="directory-create-button"
                  onClick={onReverse}
                  type="button"
                >
                  Crear reversa
                </button>
              ) : null}
            </div>
          ) : null}
        </footer>
      </aside>
    </div>
  );
}

function JournalReversalDialog({
  entry,
  onClose,
  onCompleted,
}: {
  entry: JournalEntryView;
  onClose: () => void;
  onCompleted: (entry: JournalEntryView) => void;
}) {
  const api = useProductApi();
  const [date, setDate] = useState(() => {
    const today = calendarDate();
    return entry.accounting_date > today ? entry.accounting_date : today;
  });
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const reversalIdempotencyKey = useRef<LogicalOperationKey | undefined>(
    undefined,
  );

  async function reverse() {
    if (!date || !reason.trim()) {
      setError("Indicá la fecha contable y el motivo de la reversa.");
      return;
    }
    const requestBody = {
      accounting_date: date,
      reason: reason.trim(),
    };
    reversalIdempotencyKey.current = logicalOperationKey(
      reversalIdempotencyKey.current,
      "journal-reverse",
      `${entry.id}:${JSON.stringify(requestBody)}`,
    );
    setBusy(true);
    setError(undefined);
    try {
      const created = await api.request<JournalEntryView>(
        `/api/v1/accounting/journal-entries/${entry.id}/reverse`,
        {
          method: "POST",
          headers: {
            "Idempotency-Key": reversalIdempotencyKey.current.value,
          },
          body: JSON.stringify(requestBody),
        },
      );
      reversalIdempotencyKey.current = undefined;
      onCompleted(created);
    } catch (cause) {
      setError(
        journalErrorMessage(cause, "No pudimos crear la reversa."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <JournalModal
      eyebrow={`Asiento Nº ${entry.entry_number}`}
      onClose={() => {
        if (!busy) onClose();
      }}
      title="Crear reversa"
    >
      <div className="journal-reversal-fields">
        <label>
          Fecha de reversa
          <input
            min={entry.accounting_date}
            type="date"
            value={date}
            onChange={(event) => setDate(event.target.value)}
          />
        </label>
        <label>
          Motivo
          <textarea
            autoFocus
            maxLength={500}
            placeholder="Explicá por qué se revierte el asiento"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </label>
      </div>
      <div className="journal-reversal-preview">
        <header>
          <strong>Vista previa</strong>
          <span>{entry.lines?.length ?? 0} líneas invertidas</span>
        </header>
        {(entry.lines ?? []).map((line) => (
          <div key={line.id}>
            <span>{line.account_code || line.account_name || "Cuenta contable"}</span>
            <span>
              Debe{" "}
              {formatMoney(
                line.credit,
                entry.functional_currency ?? entry.currency,
              )}
              {line.transaction_amount &&
              line.transaction_currency &&
              decimalValueIsPositive(line.credit) ? (
                <small>
                  {" "}
                  · original{" "}
                  {formatMoney(
                    line.transaction_amount,
                    line.transaction_currency,
                  )}
                </small>
              ) : null}
            </span>
            <span>
              Haber{" "}
              {formatMoney(
                line.debit,
                entry.functional_currency ?? entry.currency,
              )}
              {line.transaction_amount &&
              line.transaction_currency &&
              decimalValueIsPositive(line.debit) ? (
                <small>
                  {" "}
                  · original{" "}
                  {formatMoney(
                    line.transaction_amount,
                    line.transaction_currency,
                  )}
                </small>
              ) : null}
            </span>
          </div>
        ))}
        <footer>
          <span>
            Debe{" "}
            {formatMoney(
              entry.total_credit,
              entry.functional_currency ?? entry.currency,
            )}
          </span>
          <span>
            Haber{" "}
            {formatMoney(
              entry.total_debit,
              entry.functional_currency ?? entry.currency,
            )}
          </span>
        </footer>
      </div>
      <p className="journal-modal__warning">
        El asiento original permanecerá intacto. Si su período está cerrado,
        elegí una fecha perteneciente a un período abierto.
      </p>
      {error ? (
        <span className="form-error" role="alert">
          {error}
        </span>
      ) : null}
      <div className="journal-modal__actions">
        <button disabled={busy} onClick={onClose} type="button">
          Cancelar
        </button>
        <button
          className="directory-create-button"
          disabled={busy || !date || !reason.trim()}
          onClick={() => void reverse()}
          type="button"
        >
          {busy ? "Creando reversa…" : "Confirmar reversa"}
        </button>
      </div>
    </JournalModal>
  );
}

function JournalModal({
  children,
  eyebrow,
  onClose,
  title,
}: {
  children: ReactNode;
  eyebrow: string;
  onClose: () => void;
  title: string;
}) {
  const modal = useRef<HTMLElement>(null);
  useDialogFocus(modal, onClose);
  return (
    <div
      className="journal-modal-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <section
        aria-labelledby={`journal-modal-${title.replace(/\s+/g, "-")}`}
        aria-modal="true"
        className="journal-modal"
        ref={modal}
        role="dialog"
        tabIndex={-1}
      >
        <header>
          <div>
            <small>{eyebrow}</small>
            <h2 id={`journal-modal-${title.replace(/\s+/g, "-")}`}>{title}</h2>
          </div>
          <button aria-label="Cerrar diálogo" onClick={onClose} type="button">
            ×
          </button>
        </header>
        <div className="journal-modal__body">{children}</div>
      </section>
    </div>
  );
}

function useDialogFocus(
  container: RefObject<HTMLElement | null>,
  onClose: () => void,
) {
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const element = container.current;
    element?.focus();
    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        onCloseRef.current();
        return;
      }
      if (event.key !== "Tab" || !element) return;
      const focusable = Array.from(
        element.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [href], [tabindex]:not([tabindex="-1"])',
        ),
      );
      if (focusable.length === 0) {
        event.preventDefault();
        element.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    }
    window.addEventListener("keydown", handleKey);
    return () => {
      window.removeEventListener("keydown", handleKey);
      previous?.focus();
    };
  }, [container]);
}

function OpenItemsPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [itemType, setItemType] = useState<OpenItemType>("receivable");
  const [items, setItems] = useState<OpenItem[]>([]);
  const [query, setQuery] = useState("");
  const [selectedItem, setSelectedItem] = useState<OpenItem>();
  const [cursor, setCursor] = useState<string>();
  const [cursorTrail, setCursorTrail] = useState<string[]>([]);
  const [page, setPage] = useState<PageInfo>({ total: 0 });
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const search = new URLSearchParams({
        item_type: itemType,
        open_only: "true",
        limit: "40",
      });
      if (cursor) search.set("cursor", cursor);
      const response = await api.request<OpenItemList>(
        `/api/v1/accounting/open-items?${search}`,
        { skipJSONContentType: true },
      );
      setItems(response.items);
      setPage(response.page);
    } catch (cause) {
      setError(message(cause, "No pudimos cargar las partidas abiertas."));
    } finally {
      setLoading(false);
    }
  }, [api, cursor, itemType]);

  useEffect(() => void load(), [load]);
  useEffect(() => {
    if (!canManage) setSelectedItem(undefined);
  }, [canManage]);

  function changeItemType(value: OpenItemType) {
    setItemType(value);
    setCursor(undefined);
    setCursorTrail([]);
    setSelectedItem(undefined);
  }

  const normalizedQuery = query.trim().toLocaleLowerCase("es");
  const visibleItems = items.filter((item) => {
    if (!normalizedQuery) return true;
    return [
      item.party_id,
      item.source_id,
      item.source_type,
      item.currency,
    ].some((value) => value.toLocaleLowerCase("es").includes(normalizedQuery));
  });
  const totals = openItemCurrencyTotals(items);

  return (
    <section className="directory-section open-items-section">
      <div className="finance-toolbar open-items-toolbar">
        <SectionSearch
          label="Buscar partidas abiertas"
          placeholder="Buscar partida, origen o moneda…"
          value={query}
          onChange={setQuery}
        />
        <div className="lifecycle-tabs" role="group" aria-label="Tipo de partida">
          <button
            className={itemType === "receivable" ? "is-active" : ""}
            onClick={() => changeItemType("receivable")}
            type="button"
          >
            A cobrar
          </button>
          <button
            className={itemType === "payable" ? "is-active" : ""}
            onClick={() => changeItemType("payable")}
            type="button"
          >
            A pagar
          </button>
        </div>
        <div className="open-items-summary" aria-label="Saldo abierto de la página">
          {totals.map(({ currency, value }) => (
            <span key={currency}>
              <small>{currency}</small>
              <strong>{formatMoney(value, currency)}</strong>
            </span>
          ))}
          {!loading && totals.length === 0 ? (
            <span>
              <small>Pendientes</small>
              <strong>0 partidas</strong>
            </span>
          ) : null}
        </div>
      </div>
      {!canManage ? <ReadOnlyNote /> : null}
      <InlineFeedback error={error} loading={loading} />
      <div className="directory-table-wrap">
        <table className="directory-table finance-table open-items-table">
          <thead>
            <tr>
              <th>Partida</th>
              <th>Emisión</th>
              <th>Vencimiento</th>
              <th>Importe original</th>
              <th>Saldo abierto</th>
              <th>Saldo contable</th>
              <th>Gestión</th>
            </tr>
          </thead>
          <tbody>
            {visibleItems.map((item) => {
              const due = openItemDueState(item);
              return (
                <tr key={item.id}>
                  <td className="directory-table__primary">
                    <strong>
                      {item.item_type === "receivable" ? "Cliente" : "Proveedor"}{" "}
                      · {shortReference(item.party_id)}
                    </strong>
                    <small className="finance-row-note" title={item.source_id}>
                      {openItemSourceLabel(item.source_type)} ·{" "}
                      {shortReference(item.source_id)}
                    </small>
                  </td>
                  <td>{formatDate(item.issued_at)}</td>
                  <td>
                    <span className={`open-item-due open-item-due--${due.tone}`}>
                      {due.label}
                    </span>
                  </td>
                  <td className="money-cell">
                    {formatMoney(item.original_amount, item.currency)}
                  </td>
                  <td className="money-cell">
                    <strong>{formatMoney(item.open_amount, item.currency)}</strong>
                  </td>
                  <td className="money-cell open-item-functional">
                    <strong>{formatDecimal(item.open_functional_amount)}</strong>
                    <small>Moneda funcional</small>
                  </td>
                  <td>
                    {canManage ? (
                      <button
                        className="open-item-settle-button"
                        onClick={() => setSelectedItem(item)}
                        type="button"
                      >
                        {item.item_type === "receivable"
                          ? "Registrar cobro"
                          : "Registrar pago"}
                      </button>
                    ) : null}
                  </td>
                </tr>
              );
            })}
            {!loading && visibleItems.length === 0 ? (
              <EmptyRow
                columns={7}
                text={
                  query.trim()
                    ? "No hay partidas que coincidan con la búsqueda."
                    : itemType === "receivable"
                      ? "No hay cobros pendientes."
                      : "No hay pagos pendientes."
                }
              />
            ) : null}
          </tbody>
        </table>
      </div>
      <CursorPagination
        currentPage={cursorTrail.length + 1}
        hasNext={Boolean(page.next_cursor)}
        hasPrevious={cursorTrail.length > 0}
        onNext={() => {
          if (!page.next_cursor) return;
          setCursorTrail((previous) => [...previous, cursor ?? ""]);
          setCursor(page.next_cursor ?? undefined);
        }}
        onPrevious={() => {
          const previous = cursorTrail.at(-1) ?? "";
          setCursorTrail((items) => items.slice(0, -1));
          setCursor(previous || undefined);
        }}
        total={page.total}
      />
      {selectedItem && canManage ? (
        <SettlementDrawer
          item={selectedItem}
          onClose={() => setSelectedItem(undefined)}
          onSettled={load}
        />
      ) : null}
    </section>
  );
}

type SettlementState =
  | { kind: "idle" }
  | { kind: "submitting" }
  | { kind: "error"; message: string }
  | { kind: "success" | "replay"; entry: Entry };

function SettlementDrawer({
  item,
  onClose,
  onSettled,
}: {
  item: OpenItem;
  onClose: () => void;
  onSettled: () => Promise<void>;
}) {
  const api = useProductApi();
  const drawer = useRef<HTMLElement>(null);
  const idempotencyKey = useRef(
    createIdempotencyKey(
      item.item_type === "receivable" ? "accounting-receipt" : "accounting-payment",
    ),
  );
  const [state, setState] = useState<SettlementState>({ kind: "idle" });
  const [submissionCount, setSubmissionCount] = useState(0);
  const [amount, setAmount] = useState(item.open_amount);
  const [accountingDate, setAccountingDate] = useState(calendarDate());
  const [paymentMethod, setPaymentMethod] =
    useState<PaymentMethod>("bank_transfer");
  const [exchangeRate, setExchangeRate] = useState(
    item.currency === "ARS" ? "1" : "",
  );
  const [exchangeRateDate, setExchangeRateDate] = useState(calendarDate());
  const [exchangeRateSource, setExchangeRateSource] = useState(
    item.currency === "ARS" ? "moneda funcional" : "",
  );

  useEffect(() => {
    drawer.current?.focus();
  }, []);
  useEffect(() => {
    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape" && state.kind !== "submitting") onClose();
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [onClose, state.kind]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const validationError = validateSettlement({
      accounting_date: accountingDate,
      amount,
      exchange_rate: exchangeRate,
      exchange_rate_date: exchangeRateDate,
      exchange_rate_source: exchangeRateSource,
      open_item_id: item.id,
      payment_method: paymentMethod,
    }, item);
    if (validationError) {
      setState({ kind: "error", message: validationError });
      return;
    }

    const isRetry = submissionCount > 0;
    setSubmissionCount((value) => value + 1);
    setState({ kind: "submitting" });
    const input: SettlementInput = {
      open_item_id: item.id,
      accounting_date: accountingDate,
      payment_method: paymentMethod,
      amount: amount.trim(),
      exchange_rate: exchangeRate.trim(),
      exchange_rate_date: exchangeRateDate,
      exchange_rate_source: exchangeRateSource.trim(),
    };
    try {
      const entry = await api.request<Entry>(
        item.item_type === "receivable"
          ? "/api/v1/accounting/receipts"
          : "/api/v1/accounting/supplier-payments",
        {
          method: "POST",
          headers: { "Idempotency-Key": idempotencyKey.current },
          body: JSON.stringify(input),
        },
      );
      setState({ kind: isRetry ? "replay" : "success", entry });
      await onSettled();
    } catch (cause) {
      setState({
        kind: "error",
        message: message(
          cause,
          item.item_type === "receivable"
            ? "No pudimos registrar el cobro."
            : "No pudimos registrar el pago.",
        ),
      });
    }
  }

  const settled = state.kind === "success" || state.kind === "replay";
  const title =
    item.item_type === "receivable" ? "Registrar cobro" : "Registrar pago";

  return (
    <div
      className="finance-drawer-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget && state.kind !== "submitting") {
          onClose();
        }
      }}
    >
      <aside
        aria-labelledby="settlement-drawer-title"
        aria-modal="true"
        className="finance-drawer settlement-drawer"
        ref={drawer}
        role="dialog"
        tabIndex={-1}
      >
        <header>
          <div>
            <small>
              {item.item_type === "receivable" ? "Cuenta por cobrar" : "Cuenta por pagar"} ·{" "}
              {item.currency}
            </small>
            <h2 id="settlement-drawer-title">{title}</h2>
            <span>
              Saldo {formatMoney(item.open_amount, item.currency)} · vence{" "}
              {item.due_date ? formatDate(item.due_date) : "sin fecha"}
            </span>
          </div>
          <button
            aria-label={`Cerrar ${title.toLocaleLowerCase("es")}`}
            disabled={state.kind === "submitting"}
            onClick={onClose}
            type="button"
          >
            ×
          </button>
        </header>
        <div className="finance-drawer__facts settlement-drawer__facts">
          <span>
            Contraparte
            <strong>
              {item.item_type === "receivable" ? "Cliente" : "Proveedor"} ·{" "}
              {shortReference(item.party_id)}
            </strong>
          </span>
          <span>
            Origen
            <strong>{openItemSourceLabel(item.source_type)}</strong>
          </span>
          <span>
            Importe original
            <strong>{formatMoney(item.original_amount, item.currency)}</strong>
          </span>
          <span>
            Valor contable abierto
            <strong>
              {formatDecimal(item.open_functional_amount)} · moneda funcional
            </strong>
          </span>
        </div>
        {settled ? (
          <div
            className={`settlement-result settlement-result--${state.kind}`}
            role="status"
          >
            <small>
              {state.kind === "replay" ? "Reintento idempotente" : "Movimiento confirmado"}
            </small>
            <strong>
              {item.item_type === "receivable" ? "Cobro" : "Pago"} contabilizado
            </strong>
            <span>
              Asiento Nº {state.entry.entry_number} ·{" "}
              {formatDate(state.entry.accounting_date)}
            </span>
            {state.kind === "replay" ? (
              <p>La misma solicitud fue recuperada sin generar un asiento duplicado.</p>
            ) : null}
            <button className="directory-create-button" onClick={onClose} type="button">
              Cerrar
            </button>
          </div>
        ) : (
          <form
            className="settlement-form"
            onSubmit={(event) => void submit(event)}
          >
            <label>
              Importe
              <span className="settlement-input-with-unit">
                <b>{item.currency}</b>
                <input
                  aria-label="Importe del movimiento"
                  autoFocus
                  inputMode="decimal"
                  onChange={(event) => setAmount(event.target.value)}
                  required
                  value={amount}
                />
              </span>
              <small>Máximo {formatMoney(item.open_amount, item.currency)}</small>
            </label>
            <label>
              Fecha contable
              <input
                aria-label="Fecha contable del movimiento"
                min={item.issued_at}
                onChange={(event) => {
                  const previous = accountingDate;
                  setAccountingDate(event.target.value);
                  if (exchangeRateDate === previous) {
                    setExchangeRateDate(event.target.value);
                  }
                }}
                required
                type="date"
                value={accountingDate}
              />
            </label>
            <label>
              Medio
              <select
                aria-label="Medio del movimiento"
                onChange={(event) =>
                  setPaymentMethod(event.target.value as PaymentMethod)
                }
                value={paymentMethod}
              >
                <option value="cash">Caja</option>
                <option value="bank_transfer">Transferencia bancaria</option>
                <option value="card">Tarjeta</option>
                <option value="wallet">Billetera</option>
                <option value="check">Cheque</option>
              </select>
            </label>
            <fieldset>
              <legend>Cotización aplicada</legend>
              <label>
                Cotización
                <input
                  aria-label="Cotización del movimiento"
                  inputMode="decimal"
                  onChange={(event) => setExchangeRate(event.target.value)}
                  readOnly={item.currency === "ARS"}
                  required
                  value={exchangeRate}
                />
              </label>
              <label>
                Fecha
                <input
                  aria-label="Fecha de la cotización"
                  max={accountingDate}
                  onChange={(event) => setExchangeRateDate(event.target.value)}
                  required
                  type="date"
                  value={exchangeRateDate}
                />
              </label>
              <label>
                Fuente
                <input
                  aria-label="Fuente de la cotización"
                  maxLength={120}
                  onChange={(event) => setExchangeRateSource(event.target.value)}
                  placeholder="BNA, ARCA o contrato"
                  readOnly={item.currency === "ARS"}
                  required
                  value={exchangeRateSource}
                />
              </label>
            </fieldset>
            {state.kind === "error" ? (
              <div className="inline-state inline-state--error" role="alert">
                {state.message}
              </div>
            ) : null}
            <div className="settlement-idempotency-note">
              <span aria-hidden="true">↻</span>
              <p>
                Si la conexión se corta, reintentá desde acá. La misma clave protege
                el movimiento contra duplicados.
              </p>
            </div>
            <footer>
              <button
                disabled={state.kind === "submitting"}
                onClick={onClose}
                type="button"
              >
                Cancelar
              </button>
              <button
                className="directory-create-button"
                disabled={state.kind === "submitting"}
                type="submit"
              >
                {state.kind === "submitting"
                  ? "Contabilizando…"
                  : state.kind === "error"
                    ? "Reintentar sin duplicar"
                    : item.item_type === "receivable"
                      ? "Contabilizar cobro"
                      : "Contabilizar pago"}
              </button>
            </footer>
          </form>
        )}
      </aside>
    </div>
  );
}

function LedgerPanel() {
  const api = useProductApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const queryFrom = searchParams.get("from");
  const queryTo = searchParams.get("to");
  const [accountID, setAccountID] = useState(
    searchParams.get("account_id") ?? "",
  );
  const [accounts, setAccounts] = useState<AccountingAccountView[]>([]);
  const [from, setFrom] = useState(
    () =>
      (queryFrom && isAccountingDate(queryFrom)
        ? queryFrom
        : `${new Date().getFullYear()}-01-01`),
  );
  const [to, setTo] = useState(
    () => (queryTo && isAccountingDate(queryTo) ? queryTo : calendarDate()),
  );
  const [query, setQuery] = useState("");
  const [origin, setOrigin] = useState("");
  const [cursor, setCursor] = useState<string>();
  const [cursorTrail, setCursorTrail] = useState<Array<string | undefined>>([]);
  const [ledger, setLedger] = useState<GeneralLedgerView>();
  const [accountsLoading, setAccountsLoading] = useState(true);
  const [ledgerLoading, setLedgerLoading] = useState(false);
  const [error, setError] = useState<string>();
  const [detailError, setDetailError] = useState<string>();
  const [entryLoadingID, setEntryLoadingID] = useState<string>();
  const [selectedEntry, setSelectedEntry] = useState<JournalEntryView>();
  const [exporting, setExporting] = useState<"csv" | "xlsx" | "pdf">();

  useEffect(() => {
    if (queryFrom && isAccountingDate(queryFrom)) {
      setFrom((current) => (current === queryFrom ? current : queryFrom));
    }
  }, [queryFrom]);

  useEffect(() => {
    if (queryTo && isAccountingDate(queryTo)) {
      setTo((current) => (current === queryTo ? current : queryTo));
    }
  }, [queryTo]);

  useEffect(() => {
    const controller = new AbortController();
    setAccountsLoading(true);
    api
      .request<AccountingAccountTree>(
        "/api/v1/accounting/accounts/tree?lifecycle_state=active&node_type=posting",
        {
          signal: controller.signal,
          skipJSONContentType: true,
        },
      )
      .then((tree) => {
        setAccounts(
          tree.items
            .filter(
              (account) =>
                account.node_type === "posting" &&
                account.lifecycle_state === "active" &&
                !account.context_only,
            )
            .sort(compareAccountsByCode),
        );
        setError(undefined);
      })
      .catch((cause) => {
        if (!controller.signal.aborted) {
          setError(
            ledgerErrorMessage(
              cause,
              "No pudimos cargar las cuentas imputables.",
            ),
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setAccountsLoading(false);
      });
    return () => controller.abort();
  }, [api]);

  useEffect(() => {
    const controller = new AbortController();
    if (!accountID) {
      setLedger(undefined);
      setLedgerLoading(false);
      return () => controller.abort();
    }

    const requestSearch = ledgerSearch({
      accountID,
      cursor,
      from,
      origin,
      query,
      to,
    });
    setLedgerLoading(true);
    api
      .request<GeneralLedgerView>(
        `/api/v1/accounting/general-ledger?${requestSearch}`,
        {
          signal: controller.signal,
          skipJSONContentType: true,
        },
      )
      .then((value) => {
        setLedger(value);
        setError(undefined);
      })
      .catch((cause) => {
        if (!controller.signal.aborted) {
          setLedger(undefined);
          setError(
            ledgerErrorMessage(cause, "No pudimos preparar el Libro Mayor."),
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLedgerLoading(false);
      });
    return () => controller.abort();
  }, [accountID, api, cursor, from, origin, query, to]);

  function resetCursor() {
    setCursor(undefined);
    setCursorTrail([]);
  }

  function selectAccount(account?: AccountingAccountView) {
    const nextID = account?.id ?? "";
    setAccountID(nextID);
    setLedger(undefined);
    resetCursor();
    const nextSearchParams = new URLSearchParams(searchParams);
    if (nextID) nextSearchParams.set("account_id", nextID);
    else nextSearchParams.delete("account_id");
    setSearchParams(nextSearchParams, { replace: true });
  }

  function updatePeriod(nextFrom: string, nextTo: string) {
    setFrom(nextFrom);
    setTo(nextTo);
    resetCursor();
    const nextSearchParams = new URLSearchParams(searchParams);
    nextSearchParams.set("from", nextFrom);
    nextSearchParams.set("to", nextTo);
    setSearchParams(nextSearchParams, { replace: true });
  }

  async function openEntry(entryID: string) {
    setEntryLoadingID(entryID);
    setDetailError(undefined);
    try {
      const entry = await api.request<JournalEntryView>(
        `/api/v1/accounting/journal-entries/${entryID}`,
        { skipJSONContentType: true },
      );
      setSelectedEntry(entry);
    } catch (cause) {
      setDetailError(
        ledgerErrorMessage(
          cause,
          "No pudimos abrir el detalle del asiento.",
        ),
      );
    } finally {
      setEntryLoadingID(undefined);
    }
  }

  async function exportLedger(format: "csv" | "xlsx" | "pdf") {
    if (!accountID) return;
    setExporting(format);
    setError(undefined);
    try {
      const requestSearch = ledgerSearch({
        accountID,
        from,
        origin,
        query,
        to,
      });
      requestSearch.set("format", format);
      const response = await api.requestResponse(
        `/api/v1/accounting/general-ledger/export?${requestSearch}`,
        { skipJSONContentType: true },
      );
      const blob = await response.blob();
      downloadBlob(
        blob,
        response.headers.get("content-disposition"),
        `mayor-${ledger?.account.code ?? "cuenta"}-${from}-${to}.${format}`,
      );
    } catch (cause) {
      setError(
        ledgerErrorMessage(cause, "No pudimos exportar el Libro Mayor."),
      );
    } finally {
      setExporting(undefined);
    }
  }

  const nextCursor = ledger?.page.next_cursor;
  const account = ledger?.account ?? accounts.find((item) => item.id === accountID);
  const loading = accountsLoading || ledgerLoading;

  return (
    <section className="directory-section ledger-panel">
      <div className="ledger-toolbar">
        <div className="ledger-account-picker">
          <span className="ledger-account-picker__label">Cuenta del Mayor</span>
          <AccountLookupCombobox
            accounts={accounts}
            emptyText="Elegí una cuenta imputable"
            label="Cuenta del Mayor"
            onSelect={selectAccount}
            selectedID={accountID}
          />
          {account ? (
            <span className="ledger-account-picker__selection">
              <strong>
                {account.code} · {account.name}
              </strong>
              <small>
                {accountTypeLabel(account.account_type)} · Naturaleza{" "}
                {account.normal_balance === "debit" ? "deudora" : "acreedora"}
              </small>
            </span>
          ) : null}
        </div>
        <div className="ledger-filters">
          <label>
            Desde
            <input
              aria-label="Mayor desde"
              max={to}
              onChange={(event) => {
                updatePeriod(event.target.value, to);
              }}
              type="date"
              value={from}
            />
          </label>
          <label>
            Hasta
            <input
              aria-label="Mayor hasta"
              min={from}
              onChange={(event) => {
                updatePeriod(from, event.target.value);
              }}
              type="date"
              value={to}
            />
          </label>
          <label>
            Origen
            <select
              aria-label="Origen del Mayor"
              onChange={(event) => {
                setOrigin(event.target.value);
                resetCursor();
              }}
              value={origin}
            >
              <option value="">Todos</option>
              <option value="manual">Manual</option>
              <option value="sale">Ventas</option>
              <option value="purchase">Compras</option>
              <option value="receipt">Cobros</option>
              <option value="payment">Pagos</option>
              <option value="inventory">Inventario</option>
              <option value="fiscal">Fiscal</option>
              <option value="adjustment">Ajustes</option>
              <option value="reversal">Reversas</option>
            </select>
          </label>
          <label className="ledger-query">
            Buscar movimientos
            <span>
              <span aria-hidden="true">⌕</span>
              <input
                aria-label="Buscar movimientos del Mayor"
                onChange={(event) => {
                  setQuery(event.target.value);
                  resetCursor();
                }}
                placeholder="Número, referencia, detalle…"
                type="search"
                value={query}
              />
            </span>
          </label>
        </div>
        <div className="ledger-export-actions" aria-label="Exportar Libro Mayor">
          {(["csv", "xlsx", "pdf"] as const).map((format) => (
            <button
              disabled={!accountID || Boolean(exporting) || ledgerLoading}
              key={format}
              onClick={() => void exportLedger(format)}
              type="button"
            >
              {exporting === format ? "Preparando…" : format.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      <InlineFeedback error={error ?? detailError} loading={loading} />

      {!accountID && !accountsLoading ? (
        <div className="ledger-empty">
          <span aria-hidden="true">≡</span>
          <strong>Elegí una cuenta imputable</strong>
          <p>
            El Mayor sigue una cuenta por vez para mostrar su saldo inicial,
            movimientos y saldo final sin mezclar naturalezas contables.
          </p>
        </div>
      ) : null}

      {ledger ? (
        <>
          <div className="ledger-balance-rail" aria-label="Resumen del Mayor">
            <LedgerBalanceCard
              currency={ledger.currency}
              kind="opening"
              label="Saldo inicial"
              side={ledgerBalanceSideLabel(ledger.opening_balance.side)}
              value={ledger.opening_balance.amount}
            />
            <LedgerBalanceCard
              currency={ledger.currency}
              kind="debit"
              label="Debe del período"
              side="D"
              value={ledger.total_debit}
            />
            <LedgerBalanceCard
              currency={ledger.currency}
              kind="credit"
              label="Haber del período"
              side="H"
              value={ledger.total_credit}
            />
            <LedgerBalanceCard
              currency={ledger.currency}
              kind="closing"
              label="Saldo final"
              side={ledgerBalanceSideLabel(ledger.closing_balance.side)}
              value={ledger.closing_balance.amount}
            />
          </div>

          <div className="directory-table-wrap ledger-table-wrap">
            <table className="directory-table finance-table ledger-table">
              <thead>
                <tr>
                  <th>Fecha</th>
                  <th>N°</th>
                  <th>Referencia / detalle</th>
                  <th>Origen</th>
                  <th>Debe</th>
                  <th>Haber</th>
                  <th>Saldo</th>
                </tr>
              </thead>
              <tbody>
                {ledger.items.map((item) => (
                  <tr
                    aria-label={`Abrir asiento número ${item.entry_number}`}
                    className="ledger-entry-row"
                    key={item.line_id}
                    onClick={() => void openEntry(item.entry_id)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        void openEntry(item.entry_id);
                      }
                    }}
                    tabIndex={0}
                  >
                    <td>{formatDate(item.accounting_date)}</td>
                    <td>
                      <span className="ledger-entry-number">
                        {entryLoadingID === item.entry_id
                          ? "Abriendo…"
                          : `#${item.entry_number}`}
                      </span>
                    </td>
                    <td>
                      <div className="ledger-entry-cell">
                        <strong>{item.reference || item.description}</strong>
                        {item.reference && item.description ? (
                          <span>{item.description}</span>
                        ) : null}
                        {item.memo ? <small>{item.memo}</small> : null}
                      </div>
                    </td>
                    <td>
                      <span className="ledger-origin">
                        {journalSourceLabel(item.origin)}
                      </span>
                    </td>
                    <td className="ledger-amount">
                      {decimalValueIsPositive(item.debit)
                        ? formatMoney(item.debit, ledger.currency)
                        : "—"}
                    </td>
                    <td className="ledger-amount">
                      {decimalValueIsPositive(item.credit)
                        ? formatMoney(item.credit, ledger.currency)
                        : "—"}
                    </td>
                    <td className="ledger-amount ledger-running-balance">
                      <LedgerSignedAmount
                        currency={ledger.currency}
                        side={ledgerBalanceSideLabel(item.balance.side)}
                        value={item.balance.amount}
                      />
                    </td>
                  </tr>
                ))}
                {!ledgerLoading && ledger.items.length === 0 ? (
                  <EmptyRow
                    columns={7}
                    text="No hay movimientos para estos filtros."
                  />
                ) : null}
              </tbody>
            </table>
          </div>

          <div className="ledger-pagination">
            <span>
              {ledger.page.total}{" "}
              {ledger.page.total === 1 ? "movimiento" : "movimientos"} ·
              página {cursorTrail.length + 1}
            </span>
            <div>
              <button
                disabled={cursorTrail.length === 0 || ledgerLoading}
                onClick={() => {
                  const previous = cursorTrail.at(-1);
                  setCursor(previous);
                  setCursorTrail((trail) => trail.slice(0, -1));
                }}
                type="button"
              >
                Anterior
              </button>
              <button
                disabled={!nextCursor || ledgerLoading}
                onClick={() => {
                  if (!nextCursor) return;
                  setCursorTrail((trail) => [...trail, cursor]);
                  setCursor(nextCursor);
                }}
                type="button"
              >
                Siguiente
              </button>
            </div>
          </div>
        </>
      ) : null}

      {selectedEntry ? (
        <EntryDrawer
          accounts={accounts}
          canManage={false}
          entry={selectedEntry}
          onClose={() => setSelectedEntry(undefined)}
          onCopy={() => undefined}
          onOpenRelated={(entryID) => void openEntry(entryID)}
          onReverse={() => undefined}
        />
      ) : null}
    </section>
  );
}

function LedgerBalanceCard({
  currency,
  kind,
  label,
  side,
  value,
}: {
  currency: string;
  kind: "opening" | "debit" | "credit" | "closing";
  label: string;
  side?: "D" | "H" | "—";
  value: string;
}) {
  return (
    <div className={`ledger-balance-card ledger-balance-card--${kind}`}>
      <span>{label}</span>
      {side ? (
        <strong>
          {formatMoney(value, currency)}
          <small>{side}</small>
        </strong>
      ) : (
        <LedgerSignedAmount currency={currency} value={value} />
      )}
    </div>
  );
}

function LedgerSignedAmount({
  currency,
  side: explicitSide,
  value,
}: {
  currency: string;
  side?: "D" | "H" | "—";
  value: string;
}) {
  const side =
    explicitSide ??
    (decimalValueIsNegative(value)
      ? "H"
      : decimalValuesEqual(value, "0")
        ? "—"
        : "D");
  const absolute = value.trim().replace(/^-/, "");
  return (
    <strong>
      {formatMoney(absolute, currency)}
      <small>{side}</small>
    </strong>
  );
}

type TrialBalanceAccountClass = "" | Account["account_type"];
type TrialBalanceScope = "nonzero" | "all";

function TrialBalancePanel() {
  const api = useProductApi();
  const navigate = useNavigate();
  const [from, setFrom] = useState(`${new Date().getFullYear()}-01-01`);
  const [to, setTo] = useState(calendarDate());
  const [query, setQuery] = useState("");
  const [accountClass, setAccountClass] = useState<TrialBalanceAccountClass>("");
  const [scope, setScope] = useState<TrialBalanceScope>("nonzero");
  const [cursor, setCursor] = useState<string>();
  const [cursorTrail, setCursorTrail] = useState<Array<string | undefined>>(
    [],
  );
  const [balance, setBalance] = useState<TrialBalanceView>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [exporting, setExporting] = useState<"csv" | "xlsx" | "pdf">();

  const resetCursor = useCallback(() => {
    setCursor(undefined);
    setCursorTrail([]);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const requestSearch = trialBalanceSearch({
      accountClass,
      cursor,
      from,
      includeZero: scope === "all",
      query,
      to,
    });
    setLoading(true);
    api
      .request<TrialBalanceView>(
        `/api/v1/accounting/trial-balance?${requestSearch}`,
        {
          signal: controller.signal,
          skipJSONContentType: true,
        },
      )
      .then((value) => {
        setBalance(value);
        setError(undefined);
      })
      .catch((cause) => {
        if (!controller.signal.aborted) {
          setBalance(undefined);
          setError(
            trialBalanceErrorMessage(
              cause,
              "No pudimos preparar las Sumas y saldos.",
            ),
          );
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [accountClass, api, cursor, from, query, scope, to]);

  function updatePeriod(nextFrom: string, nextTo: string) {
    setFrom(nextFrom);
    setTo(nextTo);
    resetCursor();
  }

  function updateQuery(value: string) {
    setQuery(value);
    resetCursor();
  }

  function updateAccountClass(value: TrialBalanceAccountClass) {
    setAccountClass(value);
    resetCursor();
  }

  function updateScope(value: TrialBalanceScope) {
    setScope(value);
    resetCursor();
  }

  function openLedger(item: TrialBalanceItem) {
    const nextSearch = new URLSearchParams({
      account_id: item.account_id,
      from,
      to,
    });
    navigate(`/accounting/ledger?${nextSearch}`);
  }

  async function exportTrialBalance(format: "csv" | "xlsx" | "pdf") {
    setExporting(format);
    setError(undefined);
    try {
      const requestSearch = trialBalanceSearch({
        accountClass,
        from,
        includeZero: scope === "all",
        query,
        to,
      });
      requestSearch.set("format", format);
      const response = await api.requestResponse(
        `/api/v1/accounting/trial-balance/export?${requestSearch}`,
        { skipJSONContentType: true },
      );
      const blob = await response.blob();
      downloadBlob(
        blob,
        response.headers.get("content-disposition"),
        `sumas-y-saldos-${from}-${to}.${format}`,
      );
    } catch (cause) {
      setError(
        trialBalanceErrorMessage(
          cause,
          "No pudimos exportar las Sumas y saldos.",
        ),
      );
    } finally {
      setExporting(undefined);
    }
  }

  const nextCursor = balance?.page.next_cursor;
  const totals = balance?.totals;
  const controls = balance?.controls;

  return (
    <section className="directory-section trial-balance-panel">
      <header className="trial-balance-heading">
        <div>
          <span>INFORME CONTABLE</span>
          <h2>Balance de sumas y saldos</h2>
          <p>Controlá apertura, movimientos y cierre de cada cuenta.</p>
        </div>
        {balance ? (
          <small>
            {balance.page.total} {balance.page.total === 1 ? "cuenta" : "cuentas"}
          </small>
        ) : null}
      </header>

      <div className="trial-balance-toolbar">
        <div className="trial-balance-toolbar__filters">
          <label>
            Desde
            <input
              aria-label="Sumas y saldos desde"
              max={to}
              onChange={(event) => updatePeriod(event.target.value, to)}
              type="date"
              value={from}
            />
          </label>
          <label>
            Hasta
            <input
              aria-label="Sumas y saldos hasta"
              min={from}
              onChange={(event) => updatePeriod(from, event.target.value)}
              type="date"
              value={to}
            />
          </label>
          <SectionSearch
            label="Buscar cuentas en Sumas y saldos"
            onChange={updateQuery}
            placeholder="Código, cuenta o rubro…"
            value={query}
          />
          <label>
            Clase
            <select
              aria-label="Clase de cuenta"
              onChange={(event) =>
                updateAccountClass(
                  event.target.value as TrialBalanceAccountClass,
                )
              }
              value={accountClass}
            >
              <option value="">Todas las clases</option>
              <option value="asset">Activo</option>
              <option value="liability">Pasivo</option>
              <option value="equity">Patrimonio</option>
              <option value="income">Ingresos</option>
              <option value="cost">Costos</option>
              <option value="expense">Gastos</option>
            </select>
          </label>
          <div
            aria-label="Cuentas incluidas"
            className="trial-balance-scope"
            role="group"
          >
            <button
              aria-pressed={scope === "nonzero"}
              className={scope === "nonzero" ? "is-active" : ""}
              onClick={() => updateScope("nonzero")}
              type="button"
            >
              Con saldo o movimientos
            </button>
            <button
              aria-pressed={scope === "all"}
              className={scope === "all" ? "is-active" : ""}
              onClick={() => updateScope("all")}
              type="button"
            >
              Todas
            </button>
          </div>
        </div>
        <div
          aria-label="Exportar Sumas y saldos"
          className="ledger-export-actions trial-balance-export-actions"
        >
          {(["csv", "xlsx", "pdf"] as const).map((format) => (
            <button
              disabled={Boolean(exporting) || loading}
              key={format}
              onClick={() => void exportTrialBalance(format)}
              type="button"
            >
              {exporting === format ? "Preparando…" : format.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      <InlineFeedback error={error} loading={loading} />

      {balance && totals && controls ? (
        <>
          <div
            aria-label="Control de sumas y saldos"
            className="trial-balance-control-rail"
          >
            <TrialBalanceControlCard
              currency={balance.currency}
              debit={totals.opening_debit}
              difference={controls.opening_difference}
              filtered={Boolean(query.trim() || accountClass)}
              label="Apertura"
              leftLabel="Deudor"
              credit={totals.opening_credit}
              rightLabel="Acreedor"
            />
            <TrialBalanceControlCard
              currency={balance.currency}
              debit={totals.movement_debit}
              difference={controls.movement_difference}
              filtered={Boolean(query.trim() || accountClass)}
              label="Movimientos"
              leftLabel="Debe"
              credit={totals.movement_credit}
              rightLabel="Haber"
            />
            <TrialBalanceControlCard
              currency={balance.currency}
              debit={totals.closing_debit}
              difference={controls.closing_difference}
              filtered={Boolean(query.trim() || accountClass)}
              label="Cierre"
              leftLabel="Deudor"
              credit={totals.closing_credit}
              rightLabel="Acreedor"
            />
          </div>

          <div className="directory-table-wrap trial-balance-table-wrap">
            <table className="directory-table finance-table trial-balance-table">
              <thead>
                <tr>
                  <th rowSpan={2}>Código</th>
                  <th rowSpan={2}>Cuenta</th>
                  <th colSpan={2} scope="colgroup">
                    Saldo inicial
                  </th>
                  <th colSpan={2} scope="colgroup">
                    Movimientos
                  </th>
                  <th colSpan={2} scope="colgroup">
                    Saldo final
                  </th>
                </tr>
                <tr>
                  <th scope="col">Deudor</th>
                  <th scope="col">Acreedor</th>
                  <th scope="col">Debe</th>
                  <th scope="col">Haber</th>
                  <th scope="col">Deudor</th>
                  <th scope="col">Acreedor</th>
                </tr>
              </thead>
              <tbody>
                {balance.items.map((item) => (
                  <tr
                    aria-label={`Abrir Mayor de ${item.code} ${item.name}`}
                    className="trial-balance-account-row"
                    key={item.account_id}
                    onClick={() => openLedger(item)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        openLedger(item);
                      }
                    }}
                    tabIndex={0}
                  >
                    <td className="trial-balance-code-cell">{item.code}</td>
                    <td className="trial-balance-account-cell">
                      <div>
                        <strong>{item.name}</strong>
                        {trialBalancePath(item.path, item.name) ? (
                          <small>{trialBalancePath(item.path, item.name)}</small>
                        ) : null}
                        <span>
                          <em>{trialBalanceClassLabel(item.account_class)}</em>
                          <em>{trialBalanceNormalBalanceLabel(item.normal_balance)}</em>
                          {item.lifecycle_state !== "active" ? (
                            <em className="is-archived">
                              {trialBalanceLifecycleLabel(item.lifecycle_state)}
                            </em>
                          ) : null}
                        </span>
                      </div>
                    </td>
                    <TrialBalanceBalanceCell
                      balance={item.opening_balance}
                      currency={balance.currency}
                      side="debit"
                    />
                    <TrialBalanceBalanceCell
                      balance={item.opening_balance}
                      currency={balance.currency}
                      side="credit"
                    />
                    <TrialBalanceAmountCell
                      currency={balance.currency}
                      value={item.debit}
                    />
                    <TrialBalanceAmountCell
                      currency={balance.currency}
                      value={item.credit}
                    />
                    <TrialBalanceBalanceCell
                      balance={item.closing_balance}
                      currency={balance.currency}
                      side="debit"
                    />
                    <TrialBalanceBalanceCell
                      balance={item.closing_balance}
                      currency={balance.currency}
                      side="credit"
                    />
                  </tr>
                ))}
                {!loading && balance.items.length === 0 ? (
                  <EmptyRow
                    columns={8}
                    text={
                      query.trim() || accountClass
                        ? "No hay cuentas que coincidan con los filtros."
                        : scope === "all"
                          ? "No hay cuentas imputables disponibles."
                          : "No hay saldos ni movimientos contabilizados en el período."
                    }
                  />
                ) : null}
              </tbody>
              <tfoot>
                <tr>
                  <th colSpan={2} scope="row">Totales</th>
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.opening_debit}
                  />
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.opening_credit}
                  />
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.movement_debit}
                  />
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.movement_credit}
                  />
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.closing_debit}
                  />
                  <TrialBalanceAmountCell
                    currency={balance.currency}
                    value={totals.closing_credit}
                  />
                </tr>
              </tfoot>
            </table>
          </div>

          <div className="ledger-pagination trial-balance-pagination">
            <span>
              {balance.page.total} {balance.page.total === 1 ? "cuenta" : "cuentas"}
              {" · página "}
              {cursorTrail.length + 1}
            </span>
            <div>
              <button
                disabled={cursorTrail.length === 0 || loading}
                onClick={() => {
                  const previous = cursorTrail.at(-1);
                  setCursor(previous);
                  setCursorTrail((trail) => trail.slice(0, -1));
                }}
                type="button"
              >
                Anterior
              </button>
              <button
                disabled={!nextCursor || loading}
                onClick={() => {
                  if (!nextCursor) return;
                  setCursorTrail((trail) => [...trail, cursor]);
                  setCursor(nextCursor);
                }}
                type="button"
              >
                Siguiente
              </button>
            </div>
          </div>
        </>
      ) : null}
    </section>
  );
}

function TrialBalanceControlCard({
  credit,
  currency,
  debit,
  difference,
  filtered,
  label,
  leftLabel,
  rightLabel,
}: {
  credit: string;
  currency: string;
  debit: string;
  difference: string;
  filtered: boolean;
  label: string;
  leftLabel: string;
  rightLabel: string;
}) {
  const balanced = decimalValuesEqual(difference, "0");
  return (
    <section className="trial-balance-control-card">
      <header>
        <strong>{label}</strong>
        <span
          className={
            filtered ? "is-filtered" : balanced ? "is-balanced" : "is-unbalanced"
          }
        >
          {filtered ? "Vista filtrada" : balanced ? "Equilibrado" : "Diferencia"}
        </span>
      </header>
      <dl>
        <div>
          <dt>{leftLabel}</dt>
          <dd>{formatMoney(debit, currency)}</dd>
        </div>
        <div>
          <dt>{rightLabel}</dt>
          <dd>{formatMoney(credit, currency)}</dd>
        </div>
      </dl>
      <footer>
        <span>Control</span>
        <strong>{formatMoney(difference, currency)}</strong>
      </footer>
    </section>
  );
}

function TrialBalanceBalanceCell({
  balance,
  currency,
  side,
}: {
  balance: TrialBalanceBalance;
  currency: string;
  side: "debit" | "credit";
}) {
  return (
    <td className="trial-balance-amount">
      {balance.side === side && decimalValueIsPositive(balance.amount)
        ? formatMoney(balance.amount, currency)
        : "—"}
    </td>
  );
}

function TrialBalanceAmountCell({
  currency,
  value,
}: {
  currency: string;
  value: string;
}) {
  return (
    <td className="trial-balance-amount">
      {decimalValueIsPositive(value) ? formatMoney(value, currency) : "—"}
    </td>
  );
}

function ReportsPanel() {
  const api = useProductApi();
  const [kind, setKind] = useState<
    "journal" | "balance-sheet" | "income-statement" | "aging" | "vat-position" | "financial-activity"
  >("journal");
  const [from, setFrom] = useState(`${new Date().getFullYear()}-01-01`);
  const [to, setTo] = useState(calendarDate());
  const [financialAccountID, setFinancialAccountID] = useState("");
  const [financialAccounts, setFinancialAccounts] = useState<FinancialAccount[]>([]);
  const [report, setReport] = useState<Report>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [exporting, setExporting] = useState<string>();
  const reportFinancialAccountID =
    kind === "financial-activity" ? financialAccountID : "";

  useEffect(() => {
    api
      .request<FinancialAccount[]>(
        "/api/v1/accounting/financial-accounts?lifecycle_state=active",
        { skipJSONContentType: true },
      )
      .then((currentFinancialAccounts) => {
        setFinancialAccounts(currentFinancialAccounts);
        setFinancialAccountID((current) =>
          current || currentFinancialAccounts[0]?.id || "",
        );
      })
      .catch((cause) => setError(message(cause, "No pudimos cargar las cuentas.")));
  }, [api]);

  useEffect(() => {
    const controller = new AbortController();
    if (kind === "financial-activity" && !reportFinancialAccountID) {
      setReport(undefined);
      setLoading(false);
      return () => controller.abort();
    }
    setLoading(true);
    const search = reportSearch(
      from,
      to,
      "",
      reportFinancialAccountID,
    );
    api.request<Report>(`/api/v1/accounting/reports/${kind}?${search}`, {
      signal: controller.signal,
      skipJSONContentType: true,
    })
      .then((value) => { setReport(value); setError(undefined); })
      .catch((cause) => {
        if (!controller.signal.aborted) {
          setError(message(cause, "No pudimos preparar el informe."));
        }
      })
      .finally(() => setLoading(false));
    return () => controller.abort();
  }, [api, from, kind, reportFinancialAccountID, to]);

  async function exportReport(format: "csv" | "xlsx" | "pdf") {
    setExporting(format);
    setError(undefined);
    try {
      const search = reportSearch(
        from,
        to,
        "",
        reportFinancialAccountID,
      );
      search.set("format", format);
      const response = await api.requestResponse(
        `/api/v1/accounting/reports/${kind}/export?${search}`,
        { skipJSONContentType: true },
      );
      const blob = await response.blob();
      downloadBlob(
        blob,
        response.headers.get("content-disposition"),
        `${kind}-${from}-${to}.${format}`,
      );
    } catch (cause) {
      setError(message(cause, "No pudimos exportar el informe."));
    } finally {
      setExporting(undefined);
    }
  }

  return (
    <section className="directory-section">
      <div className="finance-toolbar finance-toolbar--reports">
        <label className="finance-select">Informe<select value={kind} onChange={(event) => setKind(event.target.value as typeof kind)}><option value="journal">Libro Diario</option><option value="balance-sheet">Estado patrimonial</option><option value="income-statement">Estado de resultados</option><option value="aging">Cuentas corrientes</option><option value="vat-position">Posición IVA</option><option value="financial-activity">Movimiento de cajas y bancos</option></select></label>
        <label className="finance-select">Desde<input aria-label="Desde" max={to} type="date" value={from} onChange={(event) => setFrom(event.target.value)} /></label>
        <label className="finance-select">Hasta<input aria-label="Hasta" min={from} type="date" value={to} onChange={(event) => setTo(event.target.value)} /></label>
        {kind === "financial-activity" ? (
          <label className="finance-select finance-select--account">
            Cuenta financiera
            <select
              aria-label="Cuenta financiera del informe"
              value={financialAccountID}
              onChange={(event) => setFinancialAccountID(event.target.value)}
            >
              <option value="">Seleccionar cuenta</option>
              {financialAccounts.map((account) => (
                <option key={account.id} value={account.id}>
                  {account.name} · {financialAccountType(account.account_type)} ·{" "}
                  {account.currency}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        <div className="finance-export-actions" aria-label="Exportar informe">
          {(["csv", "xlsx", "pdf"] as const).map((format) => (
            <button
              disabled={
                Boolean(exporting) ||
                (kind === "financial-activity" && !financialAccountID)
              }
              key={format}
              onClick={() => void exportReport(format)}
              type="button"
            >
              {exporting === format ? "Preparando…" : format.toUpperCase()}
            </button>
          ))}
        </div>
        {report ? <div className="finance-totals"><span>Debe <strong>{formatMoney(report.total_debit, report.currency)}</strong></span><span>Haber <strong>{formatMoney(report.total_credit, report.currency)}</strong></span></div> : null}
      </div>
      <InlineFeedback error={error} loading={loading} />
      <div className="directory-table-wrap">
        <table className="directory-table finance-table">
          <thead><tr><th>Cuenta / rubro</th><th>Debe</th><th>Haber</th><th>Saldo</th></tr></thead>
          <tbody>
            {report?.rows.map((row) => <tr key={row.key}><td>{row.label}</td><td className="money-cell">{formatMoney(row.debit, report.currency)}</td><td className="money-cell">{formatMoney(row.credit, report.currency)}</td><td className="money-cell"><strong>{formatMoney(row.balance, report.currency)}</strong></td></tr>)}
            {!loading && !report?.rows.length ? <EmptyRow columns={4} text="No hay movimientos en el período." /> : null}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function ReconciliationPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const fileInput = useRef<HTMLInputElement>(null);
  const today = calendarDate();
  const monthStart = `${today.slice(0, 8)}01`;
  const [financialAccounts, setFinancialAccounts] = useState<FinancialAccount[]>([]);
  const [ledgerAccounts, setLedgerAccounts] = useState<Account[]>([]);
  const [chartAccounts, setChartAccounts] = useState<Account[]>([]);
  const [journalEntries, setJournalEntries] = useState<JournalEntryView[]>([]);
  const [items, setItems] = useState<Reconciliation[]>([]);
  const [selectedAccountID, setSelectedAccountID] = useState("");
  const [accountLifecycle, setAccountLifecycle] = useState<"active" | "archived">("active");
  const [accountQuery, setAccountQuery] = useState("");
  const [showAccountForm, setShowAccountForm] = useState(false);
  const [editingAccount, setEditingAccount] = useState<FinancialAccount>();
  const [statement, setStatement] = useState<StatementImport>();
  const [suggestions, setSuggestions] = useState<ReconciliationSuggestion[]>([]);
  const [matches, setMatches] = useState<ReconciliationMatchInput[]>([]);
  const [current, setCurrent] = useState<Reconciliation>();
  const [periodStart, setPeriodStart] = useState(monthStart);
  const [statementDate, setStatementDate] = useState(today);
  const [openingBalance, setOpeningBalance] = useState("0");
  const [statementBalance, setStatementBalance] = useState("0");
  const [manualMovementID, setManualMovementID] = useState("");
  const [manualJournalLineID, setManualJournalLineID] = useState("");
  const [manualJournalLineQuery, setManualJournalLineQuery] = useState("");
  const [manualAmount, setManualAmount] = useState("");
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState<string>();
  const [reconciliationCursor, setReconciliationCursor] = useState<string>();
  const [reconciliationCursorTrail, setReconciliationCursorTrail] = useState<string[]>([]);
  const [reconciliationPage, setReconciliationPage] = useState<PageInfo>({ total: 0 });

  const clearReconciliationWorkspace = useCallback(() => {
    setStatement(undefined);
    setSuggestions([]);
    setMatches([]);
    setCurrent(undefined);
    setManualMovementID("");
    setManualJournalLineID("");
    setManualJournalLineQuery("");
    setManualAmount("");
  }, []);

  const selectFinancialAccount = useCallback(
    (accountID: string) => {
      setSelectedAccountID(accountID);
      clearReconciliationWorkspace();
    },
    [clearReconciliationWorkspace],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const accountSearch = new URLSearchParams({
        lifecycle_state: accountLifecycle,
      });
      if (accountQuery.trim()) accountSearch.set("query", accountQuery.trim());
      const reconciliationSearch = new URLSearchParams({ limit: "30" });
      if (reconciliationCursor) {
        reconciliationSearch.set("cursor", reconciliationCursor);
      }
      const [configuredAccounts, reconciliations, chart] = await Promise.all([
        api.request<FinancialAccount[]>(
          `/api/v1/accounting/financial-accounts?${accountSearch}`,
        ),
        api.request<ReconciliationList>(
          `/api/v1/accounting/reconciliations?${reconciliationSearch}`,
        ),
        api.request<AccountList>(
          "/api/v1/accounting/accounts?lifecycle_state=active&limit=100",
        ),
      ]);
      const financialAccountItems = Array.isArray(configuredAccounts) ? configuredAccounts : [];
      const reconciliationItems = Array.isArray(reconciliations?.items)
        ? reconciliations.items
        : [];
      const chartItems = Array.isArray(chart?.items) ? chart.items : [];
      setFinancialAccounts(financialAccountItems);
      setItems(reconciliationItems);
      setReconciliationPage(reconciliations?.page ?? { total: 0 });
      setChartAccounts(chartItems);
      setLedgerAccounts(
        chartItems.filter(
          (account) => account.postable && account.account_type === "asset",
        ),
      );
      if (!financialAccountItems.some((account) => account.id === selectedAccountID)) {
        selectFinancialAccount(financialAccountItems[0]?.id ?? "");
      }
    } catch (cause) {
      setError(message(cause, "No pudimos cargar la conciliación."));
    } finally {
      setLoading(false);
    }
  }, [
    accountLifecycle,
    accountQuery,
    api,
    reconciliationCursor,
    selectedAccountID,
    selectFinancialAccount,
  ]);

  useEffect(() => void load(), [load]);
  useEffect(() => {
    const controller = new AbortController();
    const search = new URLSearchParams({
      from: periodStart,
      include_lines: "true",
      to: statementDate,
      limit: "100",
    });
    api
      .request<EntryList>(`/api/v1/accounting/journal-entries?${search}`, {
        signal: controller.signal,
        skipJSONContentType: true,
      })
      .then((response) =>
        setJournalEntries(Array.isArray(response?.items) ? response.items : []),
      )
      .catch((cause) => {
        if (!controller.signal.aborted) {
          setError(message(cause, "No pudimos cargar las líneas del mayor."));
        }
      });
    return () => controller.abort();
  }, [api, periodStart, statementDate]);
  useEffect(() => {
    if (!canManage) setShowAccountForm(false);
  }, [canManage]);

  const selectedAccount = financialAccounts.find(
    (account) => account.id === selectedAccountID,
  );
  const reconciliationLocked = current?.state === "completed";
  const journalLineOptions = journalLinesForReconciliation(
    journalEntries,
    chartAccounts,
    selectedAccount?.ledger_account_id,
  );

  async function saveFinancialAccount(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy("account");
    setError(undefined);
    try {
      const saved = await api.request<FinancialAccount>(
        editingAccount
          ? `/api/v1/accounting/financial-accounts/${editingAccount.id}`
          : "/api/v1/accounting/financial-accounts",
        {
          method: editingAccount ? "PUT" : "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey("financial-account"),
          },
          body: JSON.stringify({
            ledger_account_id:
              editingAccount?.ledger_account_id ??
              form.get("ledger_account_id"),
            account_type: form.get("account_type"),
            name: String(form.get("name") ?? "").trim(),
            currency: form.get("currency"),
            institution_name:
              String(form.get("institution_name") ?? "").trim() || undefined,
            external_reference:
              String(form.get("external_reference") ?? "").trim() || undefined,
            ...(editingAccount
              ? {
                  version: editingAccount.version,
                  archived: editingAccount.archived,
                }
              : {}),
          }),
        },
      );
      setShowAccountForm(false);
      setEditingAccount(undefined);
      selectFinancialAccount(saved.id);
      await load();
    } catch (cause) {
      setError(message(cause, "No pudimos guardar la cuenta financiera."));
    } finally {
      setBusy(undefined);
    }
  }

  async function toggleFinancialAccount(account: FinancialAccount) {
    setBusy("account-state");
    setError(undefined);
    try {
      await api.request<FinancialAccount>(
        `/api/v1/accounting/financial-accounts/${account.id}`,
        {
          method: "PUT",
          headers: {
            "Idempotency-Key": createIdempotencyKey("financial-account-state"),
          },
          body: JSON.stringify({
            ledger_account_id: account.ledger_account_id,
            account_type: account.account_type,
            name: account.name,
            currency: account.currency,
            institution_name: account.institution_name,
            external_reference: account.external_reference,
            version: account.version,
            archived: !account.archived,
          }),
        },
      );
      await load();
    } catch (cause) {
      setError(message(cause, "No pudimos actualizar la cuenta financiera."));
    } finally {
      setBusy(undefined);
    }
  }

  async function importStatement(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file || !selectedAccount || reconciliationLocked) return;
    const format = statementFormat(file.name);
    if (!format) {
      setError("Elegí un archivo CSV, XLSX u OFX.");
      return;
    }
    setBusy("import");
    setError(undefined);
    try {
      const contentBase64 = await readFileBase64(file);
      const imported = await api.request<StatementImport>(
        "/api/v1/accounting/statement-imports",
        {
          method: "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey("statement-import"),
          },
          body: JSON.stringify({
            financial_account_id: selectedAccount.id,
            file_name: file.name,
            format,
            currency: selectedAccount.currency,
            content_base64: contentBase64,
          }),
        },
      );
      setStatement(imported);
      setManualMovementID(imported.movements[0]?.id ?? "");
      const search = new URLSearchParams({
        financial_account_id: selectedAccount.id,
        from: periodStart,
        to: statementDate,
        max_days: "7",
      });
      const proposed = await api.request<ReconciliationSuggestion[]>(
        `/api/v1/accounting/statement-imports/${imported.id}/suggestions?${search}`,
        { skipJSONContentType: true },
      );
      setSuggestions(proposed);
    } catch (cause) {
      setError(message(cause, "No pudimos importar el extracto."));
    } finally {
      setBusy(undefined);
    }
  }

  function addSuggestion(suggestion: ReconciliationSuggestion) {
    if (reconciliationLocked) return;
    setMatches((previous) => {
      if (
        previous.some(
          (item) =>
            item.statement_movement_id === suggestion.statement_movement_id &&
            item.journal_line_id === suggestion.journal_line_id,
        )
      ) {
        return previous;
      }
      return [
        ...previous,
        {
          statement_movement_id: suggestion.statement_movement_id,
          journal_line_id: suggestion.journal_line_id,
          statement_amount: suggestion.amount,
          ledger_amount: suggestion.amount,
        },
      ];
    });
  }

  function addManualMatch() {
    if (
      reconciliationLocked ||
      !manualMovementID ||
      !manualJournalLineID.trim() ||
      !manualAmount
    ) return;
    setMatches((previous) => [
      ...previous,
      {
        statement_movement_id: manualMovementID,
        journal_line_id: manualJournalLineID.trim(),
        statement_amount: manualAmount,
        ledger_amount: manualAmount,
      },
    ]);
    setManualJournalLineID("");
    setManualJournalLineQuery("");
    setManualAmount("");
  }

  async function saveReconciliation() {
    if (!selectedAccount || reconciliationLocked) return;
    setBusy("save");
    setError(undefined);
    try {
      const saved = current
        ? await api.request<Reconciliation>(
            `/api/v1/accounting/reconciliations/${current.id}`,
            {
              method: "PUT",
              headers: {
                "Idempotency-Key": createIdempotencyKey("reconciliation-update"),
              },
              body: JSON.stringify({
                version: current.version,
                opening_balance: openingBalance,
                statement_balance: statementBalance,
                matches,
              }),
            },
          )
        : await api.request<Reconciliation>(
            "/api/v1/accounting/reconciliations",
            {
              method: "POST",
              headers: {
                "Idempotency-Key": createIdempotencyKey("reconciliation"),
              },
              body: JSON.stringify({
                account_id: selectedAccount.id,
                period_start: periodStart,
                statement_date: statementDate,
                opening_balance: openingBalance,
                statement_balance: statementBalance,
                currency: selectedAccount.currency,
                matches,
              }),
            },
          );
      setCurrent(saved);
      setMatches(saved.matches);
      await load();
    } catch (cause) {
      setError(message(cause, "No pudimos guardar la conciliación."));
    } finally {
      setBusy(undefined);
    }
  }

  async function openReconciliation(item: Reconciliation) {
    setBusy("open");
    setError(undefined);
    try {
      const detail = await api.request<Reconciliation>(
        `/api/v1/accounting/reconciliations/${item.id}`,
        { skipJSONContentType: true },
      );
      selectFinancialAccount(detail.account_id);
      setCurrent(detail);
      setPeriodStart(detail.period_start);
      setStatementDate(detail.statement_date);
      setOpeningBalance(detail.opening_balance);
      setStatementBalance(detail.statement_balance);
      setMatches(detail.matches);
      setManualJournalLineID("");
      setManualJournalLineQuery("");
    } catch (cause) {
      setError(message(cause, "No pudimos abrir la conciliación."));
    } finally {
      setBusy(undefined);
    }
  }

  async function transitionReconciliation(action: "close" | "reopen") {
    if (!current) return;
    const reason = window.prompt(
      action === "close" ? "Motivo del cierre" : "Motivo de la reapertura",
    );
    if (!reason?.trim()) return;
    setBusy(action);
    setError(undefined);
    try {
      const changed = await api.request<Reconciliation>(
        `/api/v1/accounting/reconciliations/${current.id}/${action}`,
        {
          method: "POST",
          headers: {
            "Idempotency-Key": createIdempotencyKey(`reconciliation-${action}`),
          },
          body: JSON.stringify({ version: current.version, reason: reason.trim() }),
        },
      );
      setCurrent(changed);
      await load();
    } catch (cause) {
      setError(
        message(
          cause,
          action === "close"
            ? "No pudimos cerrar la conciliación."
            : "No pudimos reabrir la conciliación.",
        ),
      );
    } finally {
      setBusy(undefined);
    }
  }

  return (
    <section className="directory-section reconciliation-section">
      <div className="finance-toolbar">
        <SectionSearch
          label="Buscar cuentas financieras"
          placeholder="Buscar banco, caja o billetera…"
          value={accountQuery}
          onChange={setAccountQuery}
        />
        <div className="lifecycle-tabs" role="group" aria-label="Estado de cuentas financieras">
          <button className={accountLifecycle === "active" ? "is-active" : ""} onClick={() => setAccountLifecycle("active")} type="button">Activas</button>
          <button className={accountLifecycle === "archived" ? "is-active" : ""} onClick={() => setAccountLifecycle("archived")} type="button">Archivadas</button>
        </div>
        {canManage ? (
          <button className="directory-create-button" onClick={() => {
            setEditingAccount(undefined);
            setShowAccountForm((value) => !value);
          }} type="button">
            <span>＋</span> Cuenta financiera
          </button>
        ) : null}
      </div>
      {!canManage ? <ReadOnlyNote /> : null}
      {canManage && showAccountForm ? (
        <form className="finance-form" key={editingAccount?.id ?? "new"} onSubmit={(event) => void saveFinancialAccount(event)}>
          <h2>{editingAccount ? "Editar cuenta financiera" : "Nueva cuenta financiera"}</h2>
          <label>
            Cuenta contable
            <select
              aria-label="Cuenta contable"
              defaultValue={editingAccount?.ledger_account_id ?? ""}
              disabled={Boolean(editingAccount)}
              name="ledger_account_id"
              required
            >
              <option value="">Seleccionar</option>
              {editingAccount &&
              !ledgerAccounts.some(
                (account) =>
                  account.id === editingAccount.ledger_account_id,
              ) ? (
                <option value={editingAccount.ledger_account_id}>
                  {editingAccount.ledger_account_code} ·{" "}
                  {editingAccount.ledger_account_name}
                </option>
              ) : null}
              {ledgerAccounts.map(accountOption)}
            </select>
            {editingAccount ? (
              <small className="finance-form__field-note">
                La cuenta contable no cambia; para usar otra, archivá ésta y
                creá una nueva.
              </small>
            ) : null}
          </label>
          <label>Tipo<select defaultValue={editingAccount?.account_type ?? "bank"} name="account_type"><option value="bank">Banco</option><option value="cash">Caja</option><option value="card">Tarjeta</option><option value="wallet">Billetera</option></select></label>
          <label>Nombre<input defaultValue={editingAccount?.name} name="name" required /></label>
          <label>Moneda<input defaultValue={editingAccount?.currency ?? "ARS"} maxLength={3} name="currency" required /></label>
          <label>Institución<input defaultValue={editingAccount?.institution_name} name="institution_name" /></label>
          <label>Referencia externa<input defaultValue={editingAccount?.external_reference} name="external_reference" /></label>
          <button className="directory-create-button" disabled={busy === "account"} type="submit">Guardar cuenta</button>
        </form>
      ) : null}
      <div className="financial-account-strip" aria-label="Cuentas financieras">
        {financialAccounts.map((account) => (
          <button
            className={selectedAccountID === account.id ? "is-active" : ""}
            key={account.id}
            onClick={() => selectFinancialAccount(account.id)}
            type="button"
          >
            <span>{financialAccountType(account.account_type)}</span>
            <strong>{account.name}</strong>
            <small>{account.ledger_account_code} · {account.currency}</small>
          </button>
        ))}
        {!loading && financialAccounts.length === 0 ? (
          <span className="financial-account-strip__empty">
            No hay cuentas financieras en este estado.
          </span>
        ) : null}
        {canManage && selectedAccount ? (
          <div className="financial-account-strip__actions">
            <button
              className="financial-account-strip__state"
              onClick={() => {
                setEditingAccount(selectedAccount);
                setShowAccountForm(true);
              }}
              type="button"
            >
              Editar
            </button>
            <button
              className="financial-account-strip__state"
              disabled={busy === "account-state"}
              onClick={() => void toggleFinancialAccount(selectedAccount)}
              type="button"
            >
              {selectedAccount.archived ? "Restaurar cuenta" : "Archivar cuenta"}
            </button>
          </div>
        ) : null}
      </div>
      <InlineFeedback error={error} loading={loading} />
      {reconciliationLocked ? (
        <div className="reconciliation-lock" role="status">
          <strong>Conciliación cerrada</strong>
          <span>Los saldos y vinculaciones quedan bloqueados. Reabrila con permiso y motivo para volver a editar.</span>
        </div>
      ) : null}

      <div className="reconciliation-workbench">
        <article className="reconciliation-pane reconciliation-pane--statement">
          <header>
            <div><small>Extracto</small><strong>Movimientos importados</strong></div>
            {canManage && selectedAccount ? (
              <>
                <input
                  accept=".csv,.xlsx,.ofx"
                  aria-label="Archivo de extracto"
                  className="visually-hidden"
                  onChange={(event) => void importStatement(event)}
                  ref={fileInput}
                  type="file"
                />
                <button disabled={busy === "import" || reconciliationLocked} onClick={() => fileInput.current?.click()} type="button">
                  {busy === "import" ? "Importando…" : "Importar CSV · XLSX · OFX"}
                </button>
              </>
            ) : null}
          </header>
          {statement ? (
            <>
              <p className="reconciliation-file">{statement.file_name}<span>{statement.movements.length} movimientos · SHA {statement.sha256.slice(0, 10)}</span></p>
              <div className="reconciliation-list">
                {statement.movements.map((movement) => (
                  <div key={movement.id}>
                    <span>{formatDate(movement.booked_at)}<small>{movement.reference || "Sin referencia"}</small></span>
                    <span>{movement.description}<small>{movement.fingerprint.slice(0, 10)}</small></span>
                    <strong>{formatMoney(movement.amount, movement.currency)}</strong>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="reconciliation-empty">Importá un extracto para comparar sus movimientos.</div>
          )}
        </article>

        <article className="reconciliation-pane reconciliation-pane--ledger">
          <header>
            <div><small>Mayor</small><strong>Sugerencias explicables</strong></div>
            <span>{matches.length} vinculaciones</span>
          </header>
          {suggestions.length > 0 ? (
            <div className="reconciliation-list">
              {suggestions.map((suggestion) => (
                <div key={`${suggestion.statement_movement_id}-${suggestion.journal_line_id}`}>
                  <span>
                    Coincidencia {Math.round(suggestion.score)} %
                    <small>{journalLineOptionLabel(journalLineOptions, suggestion.journal_line_id)} · {suggestion.reasons.join(" · ")}</small>
                  </span>
                  <strong>{formatMoney(suggestion.amount, selectedAccount?.currency ?? "ARS")}</strong>
                  {canManage ? <button disabled={reconciliationLocked} onClick={() => addSuggestion(suggestion)} type="button">Vincular</button> : null}
                </div>
              ))}
            </div>
          ) : (
            <div className="reconciliation-empty">Las sugerencias del mayor aparecerán acá.</div>
          )}
          {canManage && statement && !reconciliationLocked ? (
            <div className="reconciliation-manual">
              <label>Movimiento<select aria-label="Movimiento del extracto" value={manualMovementID} onChange={(event) => setManualMovementID(event.target.value)}>{statement.movements.map((movement) => <option key={movement.id} value={movement.id}>{formatDate(movement.booked_at)} · {movement.description}</option>)}</select></label>
              <SearchableJournalLineSelect
                onClear={() => setManualJournalLineID("")}
                onQueryChange={setManualJournalLineQuery}
                onSelect={(option) => {
                  setManualJournalLineID(option.id);
                  setManualJournalLineQuery(option.label);
                  if (!manualAmount) setManualAmount(option.amount);
                }}
                options={journalLineOptions}
                query={manualJournalLineQuery}
                selectedID={manualJournalLineID}
              />
              <label>Importe<input aria-label="Importe a vincular" inputMode="decimal" placeholder="0.00" value={manualAmount} onChange={(event) => setManualAmount(event.target.value)} /></label>
              <button onClick={addManualMatch} type="button">Agregar parcial</button>
            </div>
          ) : null}
        </article>
      </div>

      <div className="reconciliation-control">
        <label>Desde<input aria-label="Inicio de conciliación" disabled={reconciliationLocked} type="date" value={periodStart} onChange={(event) => setPeriodStart(event.target.value)} /></label>
        <label>Fecha de extracto<input aria-label="Fecha de extracto" disabled={reconciliationLocked} type="date" value={statementDate} onChange={(event) => setStatementDate(event.target.value)} /></label>
        <label>Saldo inicial<input aria-label="Saldo inicial" disabled={reconciliationLocked} inputMode="decimal" value={openingBalance} onChange={(event) => setOpeningBalance(event.target.value)} /></label>
        <label>Saldo de extracto<input aria-label="Saldo de extracto" disabled={reconciliationLocked} inputMode="decimal" value={statementBalance} onChange={(event) => setStatementBalance(event.target.value)} /></label>
        <div className="reconciliation-control__actions">
          {canManage && !reconciliationLocked ? <button className="directory-create-button" disabled={!selectedAccount || busy === "save"} onClick={() => void saveReconciliation()} type="button">{current ? "Guardar cambios" : "Crear conciliación"}</button> : null}
          {canManage && current?.state !== "completed" ? <button disabled={busy === "close"} onClick={() => void transitionReconciliation("close")} type="button">Cerrar</button> : null}
          {canManage && current?.state === "completed" ? <button disabled={busy === "reopen"} onClick={() => void transitionReconciliation("reopen")} type="button">Reabrir</button> : null}
        </div>
      </div>
      {matches.length > 0 ? (
        <div className="reconciliation-match-rail">
          {matches.map((match, index) => (
            <span key={`${match.statement_movement_id}-${match.journal_line_id}-${index}`}>
              {statementMovementLabel(statement, match.statement_movement_id)}
              <i aria-hidden="true">↔</i>
              {journalLineOptionLabel(journalLineOptions, match.journal_line_id)}
              <strong>{formatMoney(match.statement_amount, selectedAccount?.currency ?? "ARS")}</strong>
              {canManage && !reconciliationLocked ? <button aria-label={`Quitar vinculación ${index + 1}`} onClick={() => setMatches((previous) => previous.filter((_, itemIndex) => itemIndex !== index))} type="button">×</button> : null}
            </span>
          ))}
        </div>
      ) : null}

      <div className="directory-table-wrap">
        <table className="directory-table finance-table">
          <thead><tr><th>Fecha</th><th>Cuenta</th><th>Extracto</th><th>Mayor</th><th>Diferencia</th><th>Estado</th></tr></thead>
          <tbody>
            {items.map((item) => (
              <tr
                aria-label={`Abrir conciliación de ${financialAccountName(financialAccounts, item.account_id)}`}
                key={item.id}
                onClick={() => void openReconciliation(item)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    void openReconciliation(item);
                  }
                }}
                tabIndex={0}
              >
                <td>{formatDate(item.statement_date)}</td>
                <td>{financialAccountName(financialAccounts, item.account_id)}</td>
                <td className="money-cell">{formatMoney(item.statement_balance, item.currency)}</td>
                <td className="money-cell">{formatMoney(item.ledger_balance, item.currency)}</td>
                <td className="money-cell">{formatMoney(item.difference, item.currency)}</td>
                <td><span className={`status-pill status-pill--${item.state}`}>{reconciliationState(item.state)}</span></td>
              </tr>
            ))}
            {!loading && items.length === 0 ? <EmptyRow columns={6} text="Todavía no hay conciliaciones." /> : null}
          </tbody>
        </table>
      </div>
      <CursorPagination
        currentPage={reconciliationCursorTrail.length + 1}
        hasNext={Boolean(reconciliationPage.next_cursor)}
        hasPrevious={reconciliationCursorTrail.length > 0}
        onNext={() => {
          if (!reconciliationPage.next_cursor) return;
          setReconciliationCursorTrail((previous) => [
            ...previous,
            reconciliationCursor ?? "",
          ]);
          setReconciliationCursor(
            reconciliationPage.next_cursor ?? undefined,
          );
        }}
        onPrevious={() => {
          const previous = reconciliationCursorTrail.at(-1) ?? "";
          setReconciliationCursorTrail((items) => items.slice(0, -1));
          setReconciliationCursor(previous || undefined);
        }}
        total={reconciliationPage.total}
      />
    </section>
  );
}

function PeriodsPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [periods, setPeriods] = useState<Period[]>([]);
  const [error, setError] = useState<string>();
  const [showCreate, setShowCreate] = useState(false);
  const [busy, setBusy] = useState<string>();
  const [annualDraft, setAnnualDraft] = useState<Draft>();
  const load = useCallback(() => api.request<Period[]>("/api/v1/accounting/periods").then(setPeriods).catch((cause) => setError(message(cause, "No pudimos cargar los períodos."))), [api]);
  useEffect(() => void load(), [load]);
  useEffect(() => {
    if (!canManage) setShowCreate(false);
  }, [canManage]);

  async function createPeriod(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy("create");
    setError(undefined);
    try {
      await api.request<Period>("/api/v1/accounting/periods", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("period-create") },
        body: JSON.stringify({
          start_date: form.get("start_date"),
          end_date: form.get("end_date"),
        }),
      });
      setShowCreate(false);
      await load();
    } catch (cause) {
      setError(message(cause, "No pudimos crear el período."));
    } finally {
      setBusy(undefined);
    }
  }

  async function transition(period: Period, desired_state: Period["state"]) {
    const reason = window.prompt("Motivo del cambio de estado");
    if (!reason?.trim()) return;
    setBusy(period.id);
    try {
      await api.request<Period>(`/api/v1/accounting/periods/${period.id}/transition`, { method: "POST", headers: { "Idempotency-Key": createIdempotencyKey("period") }, body: JSON.stringify({ desired_state, version: period.version, reason: reason.trim() }) });
      await load();
    } catch (cause) { setError(message(cause, "No pudimos cambiar el período.")); }
    finally { setBusy(undefined); }
  }

  async function createAnnualDraft(period: Period) {
    setBusy(`annual-${period.id}`);
    setError(undefined);
    try {
      setAnnualDraft(
        await api.request<Draft>(
          `/api/v1/accounting/periods/${period.id}/annual-close-draft`,
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey("annual-close"),
            },
            body: JSON.stringify({
              version: period.version,
            }),
          },
        ),
      );
    } catch (cause) {
      setError(message(cause, "No pudimos preparar el cierre anual."));
    } finally {
      setBusy(undefined);
    }
  }

  return (
    <section className="directory-section">
      <div className="finance-callout finance-callout--close">
        <div><small>Control contable</small><strong>Cerrar es verificar</strong></div>
        <p>El checklist reúne fiscales pendientes, mappings, cotizaciones y bancos sin conciliar. Cada reapertura exige un motivo y queda auditada.</p>
        {canManage ? <button className="directory-create-button" onClick={() => setShowCreate((value) => !value)} type="button">Nuevo período</button> : null}
      </div>
      {!canManage ? <ReadOnlyNote /> : null}
      {canManage && showCreate ? (
        <form className="finance-form" onSubmit={(event) => void createPeriod(event)}>
          <label>Inicio<input name="start_date" required type="date" /></label>
          <label>Fin<input name="end_date" required type="date" /></label>
          <button className="directory-create-button" disabled={busy === "create"} type="submit">Crear período</button>
        </form>
      ) : null}
      <InlineFeedback error={error} />
      {annualDraft ? (
        <div className="finance-work-result">
          <span>Cierre anual preparado</span>
          <strong>{annualDraft.description}</strong>
          <small>Borrador {annualDraft.id.slice(0, 8)} · Debe {formatMoney(annualDraft.total_debit, annualDraft.currency)} / Haber {formatMoney(annualDraft.total_credit, annualDraft.currency)}</small>
        </div>
      ) : null}
      <div className="period-grid">
        {periods.map((period) => (
          <article className="period-card" key={period.id}>
            <header>
              <span>{formatDate(period.start_date)} — {formatDate(period.end_date)}</span>
              <span className={`status-pill status-pill--${period.state}`}>{periodState(period.state)}</span>
            </header>
            <ul>
              {(period.checklist ?? []).map((check) => (
                <li className={check.clear ? "is-clear" : ""} key={check.code}>
                  <span>{check.clear ? "✓" : "!"}</span>
                  {checklistLabel(check.code)}
                  {check.count ? <strong>{check.count}</strong> : null}
                </li>
              ))}
              {!period.checklist?.length ? <li className="is-clear"><span>✓</span>Sin observaciones</li> : null}
            </ul>
            {canManage ? (
              <footer>
                <button disabled={busy === `annual-${period.id}`} onClick={() => void createAnnualDraft(period)} type="button">Borrador anual</button>
                {period.state === "open" ? (
                  <button disabled={busy === period.id} onClick={() => void transition(period, "soft_closed")} type="button">Cierre preliminar</button>
                ) : period.state === "soft_closed" ? (
                  <>
                    <button disabled={busy === period.id} onClick={() => void transition(period, "open")} type="button">Reabrir</button>
                    <button className="is-primary" disabled={busy === period.id} onClick={() => void transition(period, "locked")} type="button">Bloquear</button>
                  </>
                ) : (
                  <button disabled={busy === period.id} onClick={() => void transition(period, "soft_closed")} type="button">Reabrir con auditoría</button>
                )}
              </footer>
            ) : null}
          </article>
        ))}
      </div>
      {periods.length === 0 ? <div className="directory-empty"><strong>No hay períodos configurados</strong><span>Creá el primer período para habilitar los controles de cierre.</span></div> : null}
    </section>
  );
}

type EditableExchangeRate = Omit<ClosingExchangeRateInput, "source_checksum">;

function InflationPanel({ canManage }: { canManage: boolean }) {
  const api = useProductApi();
  const [result, setResult] = useState<InflationAdjustment>();
  const [revaluation, setRevaluation] = useState<CurrencyRevaluation>();
  const [periods, setPeriods] = useState<Period[]>([]);
  const [selectedPeriodID, setSelectedPeriodID] = useState("");
  const [closingDate, setClosingDate] = useState(calendarDate());
  const [rates, setRates] = useState<EditableExchangeRate[]>([
    {
      currency: "USD",
      rate: "",
      date: calendarDate(),
      source: "BNA",
      source_reference: "",
    },
  ]);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  const [busy, setBusy] = useState<string>();

  useEffect(() => {
    api
      .request<Period[]>("/api/v1/accounting/periods")
      .then((value) => {
        setPeriods(value);
        setSelectedPeriodID((current) => current || value[0]?.id || "");
      })
      .catch((cause) =>
        setError(message(cause, "No pudimos cargar los períodos.")),
      );
  }, [api]);

  async function importIndex(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setBusy("index");
    setError(undefined);
    setNotice(undefined);
    try {
      await api.request<void>("/api/v1/accounting/inflation-indices", {
        method: "POST",
        headers: { "Idempotency-Key": createIdempotencyKey("inflation-index") },
        body: JSON.stringify([
          {
            period: form.get("period"),
            value: form.get("value"),
            source: form.get("source"),
            source_document: form.get("source_document"),
          },
        ]),
      });
      setNotice("Índice importado con su fuente y checksum.");
      event.currentTarget.reset();
    } catch (cause) {
      setError(message(cause, "No pudimos importar el índice."));
    } finally {
      setBusy(undefined);
    }
  }

  async function previewAdjustment() {
    if (!selectedPeriodID) return;
    setBusy("adjustment");
    setError(undefined);
    try {
      setResult(
        await api.request<InflationAdjustment>(
          "/api/v1/accounting/inflation-adjustments",
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey("inflation"),
            },
            body: JSON.stringify({ period_id: selectedPeriodID }),
          },
        ),
      );
    } catch (cause) {
      setError(message(cause, "No pudimos calcular el ajuste."));
    } finally {
      setBusy(undefined);
    }
  }

  function updateRate(
    index: number,
    field: keyof EditableExchangeRate,
    value: string,
  ) {
    setRates((previous) =>
      previous.map((rate, itemIndex) =>
        itemIndex === index ? { ...rate, [field]: value } : rate,
      ),
    );
  }

  async function createRevaluation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy("revaluation");
    setError(undefined);
    try {
      const verifiedRates = await Promise.all(
        rates.map(async (rate) => ({
          ...rate,
          currency: rate.currency.toUpperCase(),
          source_checksum: await sha256Text(
            [
              rate.currency.toUpperCase(),
              rate.rate,
              rate.date,
              rate.source,
              rate.source_reference ?? "",
            ].join("|"),
          ),
        })),
      );
      setRevaluation(
        await api.request<CurrencyRevaluation>(
          "/api/v1/accounting/currency-revaluations",
          {
            method: "POST",
            headers: {
              "Idempotency-Key": createIdempotencyKey("currency-revaluation"),
            },
            body: JSON.stringify({
              closing_date: closingDate,
              rates: verifiedRates,
            }),
          },
        ),
      );
    } catch (cause) {
      setError(message(cause, "No pudimos preparar la revaluación."));
    } finally {
      setBusy(undefined);
    }
  }

  return (
    <section className="directory-section">
      <div className="finance-callout finance-callout--inflation">
        <div><small>Moneda homogénea</small><strong>Ajustes asistidos</strong></div>
        <p>Índices y cotizaciones conservan una fuente verificable. Cada cálculo termina en un borrador para revisión; nunca se contabiliza en silencio.</p>
      </div>
      {!canManage ? <ReadOnlyNote /> : null}
      {canManage ? (
        <div className="adjustment-grid">
          <article>
            <header><small>Índices FACPCE</small><strong>Importar índice mensual</strong></header>
            <form className="finance-form finance-form--stacked" onSubmit={(event) => void importIndex(event)}>
              <label>Período<input name="period" placeholder="2026-06" pattern="[0-9]{4}-[0-9]{2}" required /></label>
              <label>Índice<input min="0" name="value" required step="0.000001" type="number" /></label>
              <label>Fuente<input defaultValue="FACPCE" name="source" required /></label>
              <label>Documento fuente<input name="source_document" placeholder="URL o referencia normalizada" required /></label>
              <button className="directory-create-button" disabled={busy === "index"} type="submit">Importar índice</button>
            </form>
          </article>
          <article>
            <header><small>RT 6 / RT 54</small><strong>Previsualizar ajuste</strong></header>
            <div className="finance-form finance-form--stacked">
              <label>Período<select aria-label="Período del ajuste" value={selectedPeriodID} onChange={(event) => setSelectedPeriodID(event.target.value)}><option value="">Seleccionar</option>{periods.map((period) => <option key={period.id} value={period.id}>{formatDate(period.start_date)} — {formatDate(period.end_date)}</option>)}</select></label>
              <button className="directory-create-button" disabled={!selectedPeriodID || busy === "adjustment"} onClick={() => void previewAdjustment()} type="button">Generar papel de trabajo</button>
            </div>
          </article>
        </div>
      ) : null}
      <InlineFeedback error={error} />
      {notice ? <div className="inline-state inline-state--success">{notice}</div> : null}
      {result ? (
        <div className="finance-work-result inflation-result">
          <span>RECPAM estimado</span>
          <strong>{formatMoney(result.recpam, "ARS")}</strong>
          <small>{result.lines.length} partidas · Borrador {result.draft.id.slice(0, 8)} listo para revisar.</small>
        </div>
      ) : null}

      {canManage ? (
        <form className="revaluation-card" onSubmit={(event) => void createRevaluation(event)}>
          <header>
            <div><small>Multimoneda</small><strong>Revaluación de saldos</strong></div>
            <label>Fecha de cierre<input aria-label="Fecha de revaluación" type="date" value={closingDate} onChange={(event) => setClosingDate(event.target.value)} /></label>
          </header>
          <div className="revaluation-rates">
            {rates.map((rate, index) => (
              <div key={index}>
                <label>Moneda<input aria-label={`Moneda ${index + 1}`} maxLength={3} required value={rate.currency} onChange={(event) => updateRate(index, "currency", event.target.value)} /></label>
                <label>Cotización<input aria-label={`Cotización ${index + 1}`} min="0" required step="0.000001" type="number" value={rate.rate} onChange={(event) => updateRate(index, "rate", event.target.value)} /></label>
                <label>Fecha<input aria-label={`Fecha de cotización ${index + 1}`} required type="date" value={rate.date} onChange={(event) => updateRate(index, "date", event.target.value)} /></label>
                <label>Fuente<input aria-label={`Fuente ${index + 1}`} required value={rate.source} onChange={(event) => updateRate(index, "source", event.target.value)} /></label>
                <label className="finance-form__wide">Referencia<input aria-label={`Referencia ${index + 1}`} placeholder="URL, boletín o comprobante" required value={rate.source_reference ?? ""} onChange={(event) => updateRate(index, "source_reference", event.target.value)} /></label>
                {rates.length > 1 ? <button aria-label={`Quitar cotización ${index + 1}`} onClick={() => setRates((previous) => previous.filter((_, itemIndex) => itemIndex !== index))} type="button">Quitar</button> : null}
              </div>
            ))}
          </div>
          <footer>
            <button onClick={() => setRates((previous) => [...previous, { currency: "EUR", rate: "", date: closingDate, source: "BNA", source_reference: "" }])} type="button">＋ Otra cotización</button>
            <button className="directory-create-button" disabled={busy === "revaluation"} type="submit">Generar revaluación</button>
          </footer>
        </form>
      ) : null}
      {revaluation ? (
        <div className="finance-work-result revaluation-result">
          <span>Resultado neto de cambio</span>
          <strong>{formatMoney(revaluation.net_result, revaluation.functional_currency)}</strong>
          <small>{revaluation.lines.length} saldos revaluados · Borrador {revaluation.draft.id.slice(0, 8)} listo para revisar.</small>
        </div>
      ) : null}
    </section>
  );
}

type JournalLineOption = {
  id: string;
  label: string;
  amount: string;
};

function SearchableJournalLineSelect({
  onClear,
  onQueryChange,
  onSelect,
  options,
  query,
  selectedID,
}: {
  onClear: () => void;
  onQueryChange: (value: string) => void;
  onSelect: (option: JournalLineOption) => void;
  options: JournalLineOption[];
  query: string;
  selectedID: string;
}) {
  const [open, setOpen] = useState(false);
  const normalized = query.trim().toLocaleLowerCase("es");
  const visibleOptions = options
    .filter((option) =>
      !normalized ||
      option.label.toLocaleLowerCase("es").includes(normalized),
    )
    .slice(0, 8);

  return (
    <div className="journal-line-combobox">
      <label htmlFor="reconciliation-journal-line-search">Línea del asiento</label>
      <input
        aria-autocomplete="list"
        aria-controls="reconciliation-journal-lines"
        aria-expanded={open}
        aria-label="Buscar línea del asiento"
        autoComplete="off"
        id="reconciliation-journal-line-search"
        onBlur={() => setOpen(false)}
        onChange={(event) => {
          onQueryChange(event.target.value);
          if (selectedID) onClear();
          setOpen(true);
        }}
        onFocus={() => setOpen(true)}
        placeholder="Buscar asiento, cuenta o detalle…"
        role="combobox"
        value={query}
      />
      {open ? (
        <div className="journal-line-combobox__options" id="reconciliation-journal-lines" role="listbox">
          {visibleOptions.map((option) => (
            <button
              aria-selected={selectedID === option.id}
              key={option.id}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => {
                onSelect(option);
                setOpen(false);
              }}
              role="option"
              type="button"
            >
              {option.label}
            </button>
          ))}
          {visibleOptions.length === 0 ? <span>No hay líneas que coincidan.</span> : null}
        </div>
      ) : null}
    </div>
  );
}

function CursorPagination({
  currentPage,
  hasNext,
  hasPrevious,
  onNext,
  onPrevious,
  total,
}: {
  currentPage: number;
  hasNext: boolean;
  hasPrevious: boolean;
  onNext: () => void;
  onPrevious: () => void;
  total: number;
}) {
  return (
    <nav aria-label="Paginación" className="cursor-pagination">
      <span>{total} registros · página {currentPage}</span>
      <div>
        <button disabled={!hasPrevious} onClick={onPrevious} type="button">Anterior</button>
        <button disabled={!hasNext} onClick={onNext} type="button">Siguiente</button>
      </div>
    </nav>
  );
}

function accountTreeRows(
  accounts: AccountingAccountView[],
  collapsed: Set<string>,
) {
  const accountByID = new Map(accounts.map((account) => [account.id, account]));
  const children = new Map<string, AccountingAccountView[]>();
  const roots: AccountingAccountView[] = [];
  for (const account of accounts) {
    const parent = account.parent_id
      ? accountByID.get(account.parent_id)
      : undefined;
    if (parent) {
      const siblings = children.get(parent.id) ?? [];
      siblings.push(account);
      children.set(parent.id, siblings);
    } else {
      roots.push(account);
    }
  }
  roots.sort(compareAccountsByCode);
  children.forEach((siblings) => siblings.sort(compareAccountsByCode));

  const result: Array<{
    account: AccountingAccountView;
    depth: number;
    hasChildren: boolean;
  }> = [];
  const visited = new Set<string>();
  function visit(account: AccountingAccountView, depth: number) {
    if (visited.has(account.id)) return;
    visited.add(account.id);
    const descendants = children.get(account.id) ?? [];
    result.push({ account, depth, hasChildren: descendants.length > 0 });
    if (!collapsed.has(account.id)) {
      descendants.forEach((child) => visit(child, depth + 1));
    }
  }
  roots.forEach((account) => visit(account, 0));
  return result;
}

function compareAccountsByCode(
  left: Pick<Account, "code">,
  right: Pick<Account, "code">,
) {
  return left.code.localeCompare(right.code, "es", {
    numeric: true,
    sensitivity: "base",
  });
}

function normalizeAccountView(
  value: Account | AccountingAccountView,
): AccountingAccountView {
  const account = value as Partial<AccountingAccountView> & Account;
  const nodeType = account.node_type ?? (account.postable ? "posting" : "group");
  const protectedAccount = account.system_managed ?? false;
  const active = account.lifecycle_state === "active";
  const defaultCanEdit = active && !protectedAccount;
  const capabilities = account.capabilities;
  return {
    ...account,
    node_type: nodeType,
    depth: account.depth ?? 0,
    path: Array.isArray(account.path) ? account.path : [],
    has_children: account.has_children ?? false,
    used: account.used ?? false,
    mapped: account.mapped ?? false,
    system_managed: protectedAccount,
    context_only: account.context_only ?? false,
    capabilities: {
      can_edit_name: capabilities?.can_edit_name ?? defaultCanEdit,
      can_edit_structure:
        capabilities?.can_edit_structure ??
        (defaultCanEdit && !Boolean(account.used)),
      can_archive: capabilities?.can_archive ?? active,
      can_trash:
        capabilities?.can_trash ??
        (active && !Boolean(account.used) && !Boolean(account.mapped)),
      can_restore:
        capabilities?.can_restore ?? account.lifecycle_state !== "active",
      can_duplicate:
        capabilities?.can_duplicate ?? nodeType === "posting",
      edit_blockers: capabilities?.edit_blockers ?? [],
      archive_blockers: capabilities?.archive_blockers ?? [],
      trash_blockers: capabilities?.trash_blockers ?? [],
      restore_blockers: capabilities?.restore_blockers ?? [],
    },
  };
}

function countAccountLifecycles(accounts: AccountingAccountView[]) {
  return accounts.reduce(
    (totals, account) => {
      if (account.lifecycle_state === "active") totals.active += 1;
      else if (account.lifecycle_state === "archived") totals.archived += 1;
      else totals.trashed += 1;
      return totals;
    },
    { active: 0, archived: 0, trashed: 0 },
  );
}

function normalBalanceForType(
  accountType: Account["account_type"],
): Account["normal_balance"] {
  return accountType === "asset" ||
    accountType === "cost" ||
    accountType === "expense"
    ? "debit"
    : "credit";
}

function accountFormInitialState(
  account: AccountingAccountView | undefined,
  mode: AccountEditorMode,
  nodeType: AccountNodeType,
): AccountFormState {
  const accountType = account?.account_type ?? "asset";
  return {
    code: mode === "duplicate" ? "" : account?.code ?? "",
    name: account?.name ?? "",
    accountType,
    normalBalance:
      account?.normal_balance ?? normalBalanceForType(accountType),
    monetaryClassification:
      nodeType === "group"
        ? "not_applicable"
        : account?.monetary_classification === "not_applicable"
          ? "monetary"
          : account?.monetary_classification ?? "monetary",
    parentID: account?.parent_id ?? "",
  };
}

function accountFromFormState(
  account: AccountingAccountView,
  value: AccountFormState,
  nodeType: AccountNodeType,
): AccountingAccountView {
  return {
    ...account,
    code: value.code,
    name: value.name,
    node_type: nodeType,
    postable: nodeType === "posting",
    account_type: value.accountType,
    normal_balance:
      nodeType === "group"
        ? normalBalanceForType(value.accountType)
        : value.normalBalance,
    monetary_classification:
      nodeType === "group"
        ? "not_applicable"
        : value.monetaryClassification,
    parent_id: value.parentID || null,
  };
}

function monetaryClassificationLabel(
  value: Account["monetary_classification"],
) {
  return {
    monetary: "Monetaria",
    non_monetary: "No monetaria",
    not_applicable: "No aplicable",
  }[value];
}

function accountUsageLabel(account: AccountingAccountView) {
  if (account.used) return "En uso";
  if (account.mapped) return "Vinculada";
  if (account.has_children) return "Con cuentas hijas";
  return "Sin uso";
}

function accountMatchesDefinition(
  account: AccountingAccountView,
  definition: AccountingMappingDefinition,
) {
  return (
    account.node_type === "posting" &&
    account.lifecycle_state === "active" &&
    (definition.compatible_account_types.length === 0 ||
      definition.compatible_account_types.includes(account.account_type)) &&
    (definition.compatible_normal_balances.length === 0 ||
      definition.compatible_normal_balances.includes(account.normal_balance)) &&
    (definition.compatible_monetary_classifications.length === 0 ||
      definition.compatible_monetary_classifications.includes(
        account.monetary_classification,
      ))
  );
}

function accountCapabilityBlockers(account: AccountingAccountView) {
  return [
    ...account.capabilities.edit_blockers,
    ...account.capabilities.archive_blockers,
    ...account.capabilities.trash_blockers,
    ...account.capabilities.restore_blockers,
  ].filter((blocker, index, blockers) => blockers.indexOf(blocker) === index);
}

function accountBlockerLabel(code: string) {
  return (
    {
      ACCOUNTING_ACCOUNT_STRUCTURE_LOCKED:
        "La estructura quedó bloqueada porque la cuenta ya tiene uso.",
      ACCOUNTING_ACCOUNT_PARENT_INVALID:
        "El rubro superior no pertenece a la misma clase contable.",
      ACCOUNTING_ACCOUNT_HIERARCHY_CYCLE:
        "La relación generaría un ciclo en el plan de cuentas.",
      ACCOUNTING_MAPPING_INCOMPATIBLE:
        "La cuenta no es compatible con uno de sus mappings.",
      ACCOUNTING_ACCOUNT_MAPPED:
        "Remapeá el posteo automático antes de cambiar su estado.",
      ACCOUNTING_ACCOUNT_FINANCIAL_DEPENDENCY:
        "La cuenta está vinculada a una cuenta financiera activa.",
      ACCOUNTING_ACCOUNT_HAS_ACTIVE_CHILDREN:
        "Primero cambiá el estado de sus cuentas hijas activas.",
      ACCOUNTING_ACCOUNT_PARENT_INACTIVE:
        "Primero restaurá todos sus rubros superiores.",
      ACCOUNTING_ACCOUNT_PROTECTED:
        "Esta cuenta está protegida por la plantilla del sistema.",
    }[code] ?? code
  );
}

function accountErrorMessage(cause: unknown, fallback: string) {
  const normalized = normalizeHttpError(cause, { fallbackMessage: fallback });
  const messages: Record<string, string> = {
    ACCOUNTING_ACCOUNT_STRUCTURE_LOCKED:
      "La cuenta ya tiene uso. Sólo podés cambiar su nombre.",
    ACCOUNTING_ACCOUNT_PARENT_INVALID:
      "Elegí un rubro superior activo de la misma clase contable.",
    ACCOUNTING_ACCOUNT_HIERARCHY_CYCLE:
      "Ese rubro superior generaría un ciclo en el plan de cuentas.",
    ACCOUNTING_MAPPING_INCOMPATIBLE:
      "La cuenta elegida no es compatible con ese rol de posteo automático.",
    ACCOUNTING_ACCOUNT_MAPPED:
      "Remapeá el posteo automático antes de archivar esta cuenta.",
    ACCOUNTING_ACCOUNT_FINANCIAL_DEPENDENCY:
      "La cuenta está vinculada a una caja, banco o medio de cobro activo.",
    ACCOUNTING_ACCOUNT_HAS_ACTIVE_CHILDREN:
      "Primero archivá o enviá a papelera sus cuentas hijas activas.",
    ACCOUNTING_ACCOUNT_PARENT_INACTIVE:
      "Primero restaurá los rubros superiores de esta cuenta.",
    ACCOUNTING_ACCOUNT_PROTECTED:
      "Esta cuenta pertenece a la plantilla del sistema y es de sólo lectura.",
    VERSION_CONFLICT:
      "La cuenta cambió en otra sesión. Recargá la última versión o guardala como nueva.",
    RESOURCE_NOT_FOUND: "La cuenta ya no está disponible.",
  };
  if (normalized.code && messages[normalized.code]) {
    return messages[normalized.code];
  }
  if (normalized.kind === "network") {
    return "No pudimos conectarnos con Pymes. Revisá la conexión y reintentá.";
  }
  if (normalized.kind === "authorization") {
    return "No tenés permiso para realizar esta operación.";
  }
  if (normalized.kind === "not_found") return messages.RESOURCE_NOT_FOUND;
  if (normalized.kind === "server") {
    return "Pymes no pudo completar la operación. Reintentá en unos instantes.";
  }
  return fallback;
}

function journalLinesForReconciliation(
  entries: JournalEntryView[],
  accounts: Account[],
  ledgerAccountID?: string,
): JournalLineOption[] {
  const accountByID = new Map(accounts.map((account) => [account.id, account]));
  return entries.flatMap((entry) =>
    (entry.lines ?? [])
      .filter((line) => !ledgerAccountID || line.account_id === ledgerAccountID)
      .map((line) => {
        const account = accountByID.get(line.account_id);
        const side = !decimalValuesEqual(line.debit, "0") ? "Debe" : "Haber";
        const amount = side === "Debe" ? line.debit : line.credit;
        return {
          id: line.id,
          amount,
          label: `Asiento ${entry.entry_number} · ${formatDate(entry.accounting_date)} · ${
            account ? `${account.code} ${account.name}` : "Cuenta contable"
          } · ${side} ${formatMoney(amount, entry.currency)}${
            line.memo ? ` · ${line.memo}` : ""
          }`,
        };
      }),
  );
}

function journalLineOptionLabel(options: JournalLineOption[], id: string) {
  return options.find((option) => option.id === id)?.label ?? "Línea contable";
}

function statementMovementLabel(statement: StatementImport | undefined, id: string) {
  const movement = statement?.movements.find((item) => item.id === id);
  return movement
    ? `${formatDate(movement.booked_at)} · ${movement.description}`
    : "Movimiento del extracto";
}

function financialAccountName(accounts: FinancialAccount[], id: string) {
  return accounts.find((account) => account.id === id)?.name ?? "Cuenta financiera";
}

function blankJournalLine(): EditableJournalLine {
  return {
    localID: `journal-line-${Date.now()}-${nextJournalLineID++}`,
    accountID: "",
    debit: "",
    credit: "",
    memo: "",
  };
}

let nextJournalLineID = 1;

function editableJournalLine(line: JournalLineView): EditableJournalLine {
  const transactionAmount = line.transaction_amount;
  return {
    localID: line.id,
    accountID: line.account_id,
    accountCode: line.account_code,
    accountName: line.account_name,
    debit:
      transactionAmount && decimalValueIsPositive(line.debit)
        ? transactionAmount
        : line.debit,
    credit:
      transactionAmount && decimalValueIsPositive(line.credit)
        ? transactionAmount
        : line.credit,
    memo: line.memo ?? "",
  };
}

function journalLineIsBlank(line: EditableJournalLine) {
  return (
    !line.accountID &&
    !line.debit.trim() &&
    !line.credit.trim() &&
    !line.memo.trim()
  );
}

function journalDraftStructureIssue(lines: EditableJournalLine[]) {
  for (const line of lines) {
    if (journalLineIsBlank(line)) continue;
    const debit = parseExactDecimal(line.debit || "0");
    const credit = parseExactDecimal(line.credit || "0");
    if (!debit || !credit) {
      return "Usá importes numéricos con punto decimal, por ejemplo 1250.50.";
    }
    if (debit.coefficient < 0n || credit.coefficient < 0n) {
      return "Los importes de Debe y Haber no pueden ser negativos.";
    }
    const hasDebit = debit.coefficient > 0n;
    const hasCredit = credit.coefficient > 0n;
    if (!line.accountID) {
      return "Cada línea con contenido debe tener una cuenta.";
    }
    if (hasDebit === hasCredit) {
      return hasDebit
        ? "Una línea no puede tener importes en Debe y Haber al mismo tiempo."
        : "Cada línea guardada debe tener un importe positivo en Debe o Haber.";
    }
  }
  return undefined;
}

function journalFormPostingStatus({
  accounts,
  description,
  difference,
  lines,
  totalDebit,
}: {
  accounts: Account[];
  description: string;
  difference: string;
  lines: EditableJournalLine[];
  totalCredit: string;
  totalDebit: string;
}): JournalPostingStatusView {
  const enteredLines = lines.filter((line) => !journalLineIsBlank(line));
  const issues: JournalPostingIssue[] = [];
  const structureIssue = journalDraftStructureIssue(lines);
  if (structureIssue) {
    issues.push(
      enteredLines.some((line) => !line.accountID)
        ? "line_account_required"
        : "line_side_invalid",
    );
  }
  const lineAccounts = enteredLines
    .filter((line) => line.accountID)
    .map((line) => accounts.find((account) => account.id === line.accountID));
  if (
    lineAccounts.some(
      (account) => !account || account.lifecycle_state !== "active",
    )
  ) {
    issues.push("account_archived");
  }
  if (
    lineAccounts.some(
      (account) =>
        account?.lifecycle_state === "active" && !account.postable,
    )
  ) {
    issues.push("account_not_postable");
  }
  if (!description.trim()) issues.push("description_required");
  if (enteredLines.length < 2) issues.push("minimum_lines");
  if (!decimalValueIsPositive(totalDebit)) issues.push("zero_total");
  if (
    parseExactDecimal(difference) &&
    !decimalValuesEqual(difference, "0")
  ) {
    issues.push("unbalanced");
  }

  const blocked = issues.some((issue) =>
    [
      "line_account_required",
      "line_side_invalid",
      "account_archived",
      "account_not_postable",
      "period_closed",
    ].includes(issue),
  );
  const incomplete = issues.some((issue) =>
    ["description_required", "minimum_lines", "zero_total"].includes(issue),
  );
  const state: JournalPostingState = incomplete
    ? "incomplete"
    : blocked
      ? "blocked"
      : issues.includes("unbalanced")
        ? "unbalanced"
        : "ready";
  return { state, difference, issues };
}

function draftPostingStatus(draft: JournalDraftView): JournalPostingStatusView {
  if (draft.posting_status) return draft.posting_status;
  const difference = subtractDecimalStrings(
    draft.total_debit,
    draft.total_credit,
  );
  const issues: JournalPostingIssue[] = [];
  if (!draft.description.trim()) issues.push("description_required");
  if ((draft.line_count ?? draft.lines?.length ?? 0) < 2) {
    issues.push("minimum_lines");
  }
  if (!decimalValueIsPositive(draft.total_debit)) issues.push("zero_total");
  if (!decimalValuesEqual(difference, "0")) issues.push("unbalanced");
  return {
    difference,
    issues,
    state: issues.some((issue) =>
      ["description_required", "minimum_lines", "zero_total"].includes(issue),
    )
      ? "incomplete"
      : issues.includes("unbalanced")
        ? "unbalanced"
        : issues.length > 0
          ? "blocked"
          : "ready",
  };
}

function postingIssueCopy(issue?: JournalPostingIssue) {
  const copy: Record<JournalPostingIssue, string> = {
    description_required: "Agregá un detalle antes de contabilizar.",
    minimum_lines:
      "Para contabilizar, el asiento debe tener al menos dos líneas.",
    line_account_required:
      "Seleccioná una cuenta en cada línea que tenga contenido.",
    line_side_invalid:
      "Cada línea debe tener un importe positivo sólo en Debe o sólo en Haber.",
    unbalanced: "Para contabilizar, Debe y Haber deben coincidir.",
    zero_total: "Para contabilizar, el total debe ser mayor que cero.",
    period_closed:
      "La fecha pertenece a un período cerrado. Elegí un período abierto.",
    account_archived:
      "Una de las cuentas fue archivada. Reemplazala antes de contabilizar.",
    account_not_postable:
      "Una de las cuentas es un rubro y no admite imputaciones.",
  };
  return issue ? copy[issue] : undefined;
}

function subtractDecimalStrings(left: string, right: string) {
  const parsedLeft = parseExactDecimal(left);
  const parsedRight = parseExactDecimal(right);
  if (!parsedLeft || !parsedRight) return "valor inválido";
  const scale = Math.max(parsedLeft.scale, parsedRight.scale);
  const coefficient =
    parsedLeft.coefficient * 10n ** BigInt(scale - parsedLeft.scale) -
    parsedRight.coefficient * 10n ** BigInt(scale - parsedRight.scale);
  return exactDecimalString({
    coefficient: coefficient < 0n ? -coefficient : coefficient,
    scale,
  });
}

function multiplyDecimalStrings(left: string, right: string) {
  const parsedLeft = parseExactDecimal(left);
  const parsedRight = parseExactDecimal(right);
  if (!parsedLeft || !parsedRight) return "valor inválido";
  return exactDecimalString({
    coefficient: parsedLeft.coefficient * parsedRight.coefficient,
    scale: parsedLeft.scale + parsedRight.scale,
  });
}

function hydrateJournalDraft(
  draft: JournalDraftView,
  setters: {
    setCurrency: (value: string) => void;
    setDate: (value: string) => void;
    setDescription: (value: string) => void;
    setExchangeRate: (value: string) => void;
    setExchangeRateDate: (value: string) => void;
    setExchangeRateSource: (value: string) => void;
    setLines: (value: EditableJournalLine[]) => void;
    setReference: (value: string) => void;
  },
) {
  setters.setDate(draft.accounting_date);
  setters.setReference(draft.reference ?? "");
  setters.setDescription(draft.description);
  setters.setCurrency(draft.currency);
  setters.setExchangeRate(draft.exchange_rate ?? "1");
  setters.setExchangeRateDate(
    draft.exchange_rate_date ?? draft.accounting_date,
  );
  setters.setExchangeRateSource(draft.exchange_rate_source ?? "");
  const lines = draft.lines?.map(editableJournalLine) ?? [];
  setters.setLines(
    lines.length >= 2
      ? lines
      : [...lines, ...Array.from({ length: 2 - lines.length }, blankJournalLine)],
  );
}

function journalSourceLabel(source?: string) {
  const labels: Record<string, string> = {
    manual: "Manual",
    manual_draft: "Manual",
    sale: "Venta",
    purchase: "Compra",
    receipt: "Cobro",
    collection: "Cobro",
    supplier_payment: "Pago",
    payment: "Pago",
    customer_credit_note: "Devolución",
    customer_debit_note: "Nota de débito",
    currency_revaluation: "Revaluación",
    annual_closing: "Cierre anual",
    journal_entry: "Asiento",
    inventory: "Inventario",
    fiscal: "Fiscal",
    fiscal_voucher: "Comprobante fiscal",
    fiscal_purchase: "Compra fiscal",
  };
  return source ? labels[source] ?? source : "Manual";
}

function ledgerBalanceSideLabel(
  side: GeneralLedgerBalanceView["side"],
): "D" | "H" | "—" {
  return side === "debit" ? "D" : side === "credit" ? "H" : "—";
}

function journalPostingKindLabel(kind: string) {
  const labels: Record<string, string> = {
    standard: "Asiento general",
    adjustment: "Ajuste",
    closing: "Cierre",
    opening: "Apertura",
    reversal: "Reversa",
    primary: "Asiento principal",
  };
  return labels[kind] ?? kind;
}

function journalOriginalTotals(
  entry: Pick<JournalEntryView, "currency" | "functional_currency" | "lines">,
) {
  const valuesByCurrency = new Map<
    string,
    { debit: string[]; credit: string[] }
  >();
  for (const line of entry.lines ?? []) {
    if (!line.transaction_amount || !line.transaction_currency) continue;
    const currency = line.transaction_currency.toUpperCase();
    const values = valuesByCurrency.get(currency) ?? {
      debit: [],
      credit: [],
    };
    if (decimalValueIsPositive(line.debit)) {
      values.debit.push(line.transaction_amount);
    } else if (decimalValueIsPositive(line.credit)) {
      values.credit.push(line.transaction_amount);
    }
    valuesByCurrency.set(currency, values);
  }
  const functionalCurrency = entry.functional_currency?.toUpperCase();
  if (
    valuesByCurrency.size === 0 ||
    (valuesByCurrency.size === 1 &&
      functionalCurrency &&
      valuesByCurrency.has(functionalCurrency))
  ) {
    return [];
  }
  return [...valuesByCurrency.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([currency, values]) => ({
      currency,
      debit: sumDecimalStrings(
        values.debit.length > 0 ? values.debit : ["0"],
      ),
      credit: sumDecimalStrings(
        values.credit.length > 0 ? values.credit : ["0"],
      ),
    }));
}

function logicalOperationKey(
  current: LogicalOperationKey | undefined,
  prefix: string,
  signature: string,
) {
  return current?.signature === signature
    ? current
    : {
        signature,
        value: createIdempotencyKey(prefix),
      };
}

function journalErrorMessage(cause: unknown, fallback: string) {
  const normalized = normalizeHttpError(cause, { fallbackMessage: fallback });
  const messages: Record<string, string> = {
    ACCOUNTING_UNBALANCED:
      "Para contabilizar, Debe y Haber deben coincidir y ser mayores que cero.",
    ACCOUNTING_PERIOD_CLOSED:
      "La fecha pertenece a un período cerrado. Elegí un período abierto.",
    ACCOUNTING_ACCOUNT_ARCHIVED:
      "Una cuenta del asiento fue archivada. Reemplazala y volvé a intentar.",
    ACCOUNTING_ACCOUNT_NOT_POSTABLE:
      "Una cuenta seleccionada es un rubro y no admite imputaciones.",
    ACCOUNTING_ENTRY_ALREADY_REVERSED:
      "Este asiento ya tiene una reversa registrada.",
    ACCOUNTING_REVERSAL_NOT_ALLOWED:
      "Este asiento proviene de una operación y no puede revertirse desde el Diario. Corregí la operación de origen.",
    ACCOUNTING_RECONCILIATION_CLOSED:
      "La conciliación está cerrada. Reabrí la conciliación antes de modificar sus vínculos.",
    VERSION_CONFLICT:
      "El borrador cambió en otra sesión. Recargá la última versión o guardalo como nuevo.",
    RESOURCE_NOT_FOUND: "El asiento o borrador ya no está disponible.",
    IDEMPOTENCY_KEY_CONFLICT:
      "La operación ya fue enviada con otros datos. Volvé a intentarlo.",
    IDEMPOTENCY_IN_PROGRESS:
      "La operación todavía se está procesando. Esperá un momento y actualizá.",
  };
  if (normalized.code && messages[normalized.code]) {
    return messages[normalized.code];
  }
  if (normalized.kind === "network") {
    return "No pudimos conectarnos con Pymes. Revisá la conexión y reintentá.";
  }
  if (normalized.kind === "authorization") {
    return "No tenés permiso para realizar esta operación.";
  }
  if (normalized.kind === "not_found") {
    return "El asiento o borrador ya no está disponible.";
  }
  if (normalized.kind === "server") {
    return "Pymes no pudo completar la operación. Reintentá en unos instantes.";
  }
  return fallback;
}

function ledgerErrorMessage(cause: unknown, fallback: string) {
  const normalized = normalizeHttpError(cause, { fallbackMessage: fallback });
  const messages: Record<string, string> = {
    ACCOUNT_NOT_POSTABLE:
      "Elegí una cuenta imputable. Los rubros no tienen movimientos propios.",
    ACCOUNTING_ACCOUNT_NOT_POSTABLE:
      "Elegí una cuenta imputable. Los rubros no tienen movimientos propios.",
    ACCOUNTING_ACCOUNT_ARCHIVED:
      "La cuenta está archivada. Podés consultar su historia desde el Plan de cuentas.",
    ACCOUNTING_PERIOD_INVALID:
      "El rango de fechas no es válido. Revisá las fechas desde y hasta.",
    RESOURCE_NOT_FOUND:
      "La cuenta o el asiento ya no está disponible en esta organización.",
  };
  if (normalized.code && messages[normalized.code]) {
    return messages[normalized.code];
  }
  if (normalized.kind === "network") {
    return "No pudimos conectarnos con Pymes. Revisá la conexión y reintentá.";
  }
  if (normalized.kind === "authorization") {
    return "No tenés permiso para consultar este Libro Mayor.";
  }
  if (normalized.kind === "not_found") {
    return "La cuenta o el asiento ya no está disponible en esta organización.";
  }
  if (normalized.kind === "server") {
    return "Pymes no pudo preparar el Libro Mayor. Reintentá en unos instantes.";
  }
  return fallback;
}

type ExactDecimal = { coefficient: bigint; scale: number };

function parseExactDecimal(value: string): ExactDecimal | undefined {
  const match = value.trim().match(/^([+-]?)(\d+)(?:\.(\d+))?$/);
  if (!match) return undefined;
  const fraction = match[3] ?? "";
  const sign = match[1] === "-" ? -1n : 1n;
  return {
    coefficient: sign * BigInt(`${match[2]}${fraction}`),
    scale: fraction.length,
  };
}

function sumDecimalStrings(values: string[]) {
  const parsed = values.map(parseExactDecimal);
  if (parsed.some((value) => value === undefined)) return "valor inválido";
  const exact = parsed as ExactDecimal[];
  const scale = exact.reduce((maximum, value) => Math.max(maximum, value.scale), 0);
  const coefficient = exact.reduce(
    (sum, value) =>
      sum + value.coefficient * 10n ** BigInt(scale - value.scale),
    0n,
  );
  return exactDecimalString({ coefficient, scale });
}

function exactDecimalString(value: ExactDecimal) {
  const negative = value.coefficient < 0;
  const digits = (negative ? -value.coefficient : value.coefficient)
    .toString()
    .padStart(value.scale + 1, "0");
  if (value.scale === 0) return `${negative ? "-" : ""}${digits}`;
  const integer = digits.slice(0, -value.scale);
  const fraction = digits.slice(-value.scale).replace(/0+$/, "");
  return `${negative ? "-" : ""}${integer}${fraction ? `.${fraction}` : ""}`;
}

function decimalValuesEqual(left: string, right: string) {
  const parsedLeft = parseExactDecimal(left);
  const parsedRight = parseExactDecimal(right);
  if (!parsedLeft || !parsedRight) return false;
  const scale = Math.max(parsedLeft.scale, parsedRight.scale);
  return (
    parsedLeft.coefficient * 10n ** BigInt(scale - parsedLeft.scale) ===
    parsedRight.coefficient * 10n ** BigInt(scale - parsedRight.scale)
  );
}

function compareDecimalStrings(left: string, right: string) {
  const parsedLeft = parseExactDecimal(left);
  const parsedRight = parseExactDecimal(right);
  if (!parsedLeft || !parsedRight) return undefined;
  const scale = Math.max(parsedLeft.scale, parsedRight.scale);
  const normalizedLeft =
    parsedLeft.coefficient * 10n ** BigInt(scale - parsedLeft.scale);
  const normalizedRight =
    parsedRight.coefficient * 10n ** BigInt(scale - parsedRight.scale);
  return normalizedLeft === normalizedRight
    ? 0
    : normalizedLeft < normalizedRight
      ? -1
      : 1;
}

function decimalValueIsPositive(value: string) {
  const parsed = parseExactDecimal(value);
  return Boolean(parsed && parsed.coefficient > 0n);
}

function decimalValueIsNegative(value: string) {
  const parsed = parseExactDecimal(value);
  return Boolean(parsed && parsed.coefficient < 0n);
}

function validateSettlement(input: SettlementInput, item: OpenItem) {
  if (!decimalValueIsPositive(input.amount)) {
    return "Ingresá un importe mayor que cero.";
  }
  if ((compareDecimalStrings(input.amount, item.open_amount) ?? 1) > 0) {
    return `El importe no puede superar ${formatMoney(item.open_amount, item.currency)}.`;
  }
  if (!decimalValueIsPositive(input.exchange_rate)) {
    return "Ingresá una cotización mayor que cero.";
  }
  if (
    item.currency === "ARS" &&
    !decimalValuesEqual(input.exchange_rate, "1")
  ) {
    return "La cotización de una partida en ARS debe ser 1.";
  }
  if (!input.accounting_date || input.accounting_date < item.issued_at) {
    return "La fecha contable no puede ser anterior a la emisión.";
  }
  if (
    !input.exchange_rate_date ||
    input.exchange_rate_date > input.accounting_date
  ) {
    return "La fecha de cotización no puede ser posterior a la fecha contable.";
  }
  if (!input.exchange_rate_source.trim()) {
    return "Indicá la fuente de la cotización.";
  }
  return undefined;
}

function openItemCurrencyTotals(items: OpenItem[]) {
  const grouped = new Map<string, string[]>();
  for (const item of items) {
    const values = grouped.get(item.currency) ?? [];
    values.push(item.open_amount);
    grouped.set(item.currency, values);
  }
  return [...grouped.entries()]
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([currency, values]) => ({
      currency,
      value: sumDecimalStrings(values),
    }));
}

function openItemDueState(item: OpenItem) {
  if (!item.due_date) {
    return { label: "Sin vencimiento", tone: "neutral" } as const;
  }
  const today = calendarDate();
  if (item.due_date < today) {
    return {
      label: `Vencida · ${formatDate(item.due_date)}`,
      tone: "overdue",
    } as const;
  }
  if (item.due_date === today) {
    return { label: "Vence hoy", tone: "today" } as const;
  }
  return {
    label: formatDate(item.due_date),
    tone: "future",
  } as const;
}

function shortReference(value: string) {
  return value.length > 10 ? value.slice(0, 8).toUpperCase() : value;
}

function openItemSourceLabel(value: string) {
  return (
    {
      sale: "Venta",
      fiscal_voucher: "Comprobante fiscal",
      purchase: "Compra",
      fiscal_purchase: "Comprobante de proveedor",
      manual: "Asiento manual",
    }[value] ?? value.replaceAll("_", " ")
  );
}

function InlineFeedback({ error, loading }: { error?: string; loading?: boolean }) {
  if (error) return <div className="inline-state inline-state--error" role="alert">{error}</div>;
  if (loading) return <div className="inline-state">Cargando…</div>;
  return null;
}

function ReadOnlyNote() {
  return (
    <div className="finance-readonly-note">
      Estás viendo Contabilidad en modo lectura. Un responsable contable puede
      realizar cambios.
    </div>
  );
}

function EmptyRow({ columns, text }: { columns: number; text: string }) {
  return <tr><td className="directory-empty" colSpan={columns}><strong>{text}</strong><span>La información aparecerá acá cuando esté disponible.</span></td></tr>;
}

function accountOption(account: Account) {
  return <option key={account.id} value={account.id}>{account.code} · {account.name}</option>;
}

function accountTypeLabel(value: Account["account_type"]) {
  return ({ asset: "Activo", liability: "Pasivo", equity: "Patrimonio", income: "Ingresos", cost: "Costos", expense: "Gastos" } as const)[value];
}

function accountGroupLabel(code: string) {
  return ({ "1": "Activo", "2": "Pasivo", "3": "Patrimonio", "4": "Ingresos", "5": "Costos", "6": "Gastos" } as const)[code] ?? code;
}

function mappingRoleLabel(role: string) {
  return (
    {
      cash: "Caja",
      bank: "Banco",
      receivable: "Deudores por ventas",
      payable: "Proveedores",
      revenue: "Ventas",
      inventory: "Bienes de cambio",
      cogs: "Costo de mercadería vendida",
      card_clearing: "Tarjetas a cobrar",
      wallet_clearing: "Billeteras a cobrar",
      checks_clearing: "Valores a depositar",
      purchase_expense: "Compras y servicios",
      credit_note_payable: "Notas de crédito a aplicar",
      fx_gain: "Diferencia de cambio positiva",
      fx_loss: "Diferencia de cambio negativa",
      rounding_difference: "Diferencia de redondeo",
      current_result: "Resultado del ejercicio",
      recpam: "RECPAM",
    }[role] ?? role.replaceAll("_", " ")
  );
}

function periodState(value: Period["state"]) {
  return value === "open" ? "Abierto" : value === "soft_closed" ? "Cierre preliminar" : "Bloqueado";
}

function reconciliationState(value: Reconciliation["state"]) {
  return value === "draft"
    ? "En preparación"
    : value === "completed"
      ? "Cerrada"
      : "Reabierta";
}

function financialAccountType(value: FinancialAccount["account_type"]) {
  return {
    bank: "Banco",
    cash: "Caja",
    card: "Tarjeta",
    wallet: "Billetera",
  }[value];
}

function checklistLabel(code: string) {
  return (
    {
      unposted_documents: "Documentos sin asiento",
      fiscal_pending: "Comprobantes fiscales pendientes",
      posting_errors: "Errores de contabilización",
      missing_mappings: "Mappings contables incompletos",
      missing_exchange_rates: "Cotizaciones faltantes",
      unreconciled_accounts: "Cuentas sin conciliar",
    }[code] ?? code.replaceAll("_", " ")
  );
}

function reportSearch(
  from: string,
  to: string,
  accountID: string,
  financialAccountID = "",
) {
  const search = new URLSearchParams({ from, to });
  if (accountID) search.set("account_id", accountID);
  if (financialAccountID) {
    search.set("financial_account_id", financialAccountID);
  }
  return search;
}

function ledgerSearch({
  accountID,
  cursor,
  from,
  origin,
  query,
  to,
}: {
  accountID: string;
  cursor?: string;
  from: string;
  origin: string;
  query: string;
  to: string;
}) {
  const search = new URLSearchParams({
    account_id: accountID,
    from,
    limit: "50",
    to,
  });
  if (cursor) search.set("cursor", cursor);
  if (origin) search.set("origin", origin);
  if (query.trim()) search.set("query", query.trim());
  return search;
}

function trialBalanceSearch({
  accountClass,
  cursor,
  from,
  includeZero,
  query,
  to,
}: {
  accountClass: TrialBalanceAccountClass;
  cursor?: string;
  from: string;
  includeZero: boolean;
  query: string;
  to: string;
}) {
  const search = new URLSearchParams({
    from,
    include_zero: String(includeZero),
    limit: "50",
    to,
  });
  if (accountClass) search.set("account_class", accountClass);
  if (cursor) search.set("cursor", cursor);
  if (query.trim()) search.set("query", query.trim());
  return search;
}

function isAccountingDate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(Date.UTC(year, month - 1, day));
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  );
}

function trialBalancePath(path: string[], name: string) {
  const hierarchy = path.filter(
    (segment, index) =>
      !(index === path.length - 1 && segment.trim() === name.trim()),
  );
  return hierarchy.join(" / ");
}

function trialBalanceClassLabel(value: TrialBalanceItem["account_class"]) {
  return accountTypeLabel(value);
}

function trialBalanceNormalBalanceLabel(
  value: TrialBalanceItem["normal_balance"],
) {
  return value === "debit" ? "Naturaleza deudora" : "Naturaleza acreedora";
}

function trialBalanceLifecycleLabel(
  value: TrialBalanceItem["lifecycle_state"],
) {
  return value === "archived"
    ? "Archivada"
    : value === "trashed"
      ? "En papelera"
      : "Activa";
}

function trialBalanceErrorMessage(cause: unknown, fallback: string) {
  const normalized = normalizeHttpError(cause, { fallbackMessage: fallback });
  const messages: Record<string, string> = {
    ACCOUNTING_INVALID_PERIOD:
      "El rango de fechas no es válido. Revisá Desde y Hasta.",
    RESOURCE_NOT_FOUND:
      "La información contable ya no está disponible en esta organización.",
  };
  if (normalized.code && messages[normalized.code]) {
    return messages[normalized.code];
  }
  if (normalized.kind === "network") {
    return "No pudimos conectarnos con Pymes. Revisá la conexión y reintentá.";
  }
  if (normalized.kind === "authorization") {
    return "No tenés permiso para consultar las Sumas y saldos.";
  }
  if (normalized.kind === "server") {
    return "Pymes no pudo preparar las Sumas y saldos. Reintentá en unos instantes.";
  }
  return fallback;
}

function downloadBlob(
  blob: Blob,
  contentDisposition: string | null,
  fallbackName: string,
) {
  const match = contentDisposition?.match(/filename\*?=(?:UTF-8''|")?([^";]+)/i);
  const fileName = match?.[1] ? decodeURIComponent(match[1].trim()) : fallbackName;
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  URL.revokeObjectURL(url);
}

function statementFormat(fileName: string) {
  const extension = fileName.split(".").pop()?.toLowerCase();
  return extension === "csv" || extension === "xlsx" || extension === "ofx"
    ? extension
    : undefined;
}

function readFileBase64(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error("No pudimos leer el archivo."));
    reader.onload = () => {
      if (typeof reader.result !== "string") {
        reject(new Error("El archivo no tiene un formato válido."));
        return;
      }
      resolve(reader.result.split(",", 2)[1] ?? "");
    };
    reader.readAsDataURL(file);
  });
}

async function sha256Text(value: string) {
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

function formatMoney(value: string, currency: string) {
  const match = value.trim().match(/^(-?)(\d+)(?:\.(\d+))?$/);
  if (!match) return `${currency} ${value}`;
  const integer = (match[2] ?? "0").replace(/^0+(?=\d)/, "");
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const fraction = (match[3] ?? "").padEnd(2, "0");
  return `${currency} ${match[1] ?? ""}${grouped},${fraction}`;
}

function formatDecimal(value: string) {
  return formatMoney(value, "").trimStart();
}

function formatDate(value: string) {
  const date = new Date(`${value.slice(0, 10)}T00:00:00`);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat("es-AR").format(date);
}

function formatDateTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat("es-AR", {
        dateStyle: "short",
        timeStyle: "short",
      }).format(date);
}

function message(cause: unknown, fallback: string) {
  return cause instanceof Error ? cause.message : fallback;
}
