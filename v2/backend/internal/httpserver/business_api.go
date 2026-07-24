package httpserver

import (
	"net/http"

	"github.com/devpablocristo/pymes/v2/backend/internal/api"
	"github.com/google/uuid"
)

// The business handlers live on IAMAPI so the existing authenticated product
// boundary remains the only OpenAPI server. Each method is implemented in the
// accounting and fiscal adapter files; this file keeps their shared failure
// behavior in one place.
func businessUnavailable(w http.ResponseWriter) {
	writeAPIError(
		w,
		http.StatusServiceUnavailable,
		"BUSINESS_UNAVAILABLE",
		"The business module is unavailable",
	)
}

func (h *IAMAPI) ListAccountingMappings(w http.ResponseWriter, r *http.Request) {
	h.listAccountingMappings(w, r)
}

func (h *IAMAPI) UpdateAccountingMappings(w http.ResponseWriter, r *http.Request, params api.UpdateAccountingMappingsParams) {
	h.updateAccountingMappings(w, r, params)
}

func (h *IAMAPI) ListAccountingAccounts(w http.ResponseWriter, r *http.Request, params api.ListAccountingAccountsParams) {
	h.listAccountingAccounts(w, r, params)
}

func (h *IAMAPI) CreateAccountingAccount(w http.ResponseWriter, r *http.Request, params api.CreateAccountingAccountParams) {
	h.createAccountingAccount(w, r, params)
}

func (h *IAMAPI) GetAccountingAccount(w http.ResponseWriter, r *http.Request, accountID api.AccountID) {
	h.getAccountingAccount(w, r, accountID)
}

func (h *IAMAPI) UpdateAccountingAccount(w http.ResponseWriter, r *http.Request, accountID api.AccountID, params api.UpdateAccountingAccountParams) {
	h.updateAccountingAccount(w, r, accountID, params)
}

func (h *IAMAPI) TransitionAccountingAccount(
	w http.ResponseWriter,
	r *http.Request,
	accountID api.AccountID,
	action api.TransitionAccountingAccountParamsLifecycleAction,
	params api.TransitionAccountingAccountParams,
) {
	h.transitionAccountingAccount(w, r, accountID, action, params)
}

func (h *IAMAPI) PreviewInflationAdjustment(w http.ResponseWriter, r *http.Request, params api.PreviewInflationAdjustmentParams) {
	h.previewInflationAdjustment(w, r, params)
}

func (h *IAMAPI) ListJournalDrafts(w http.ResponseWriter, r *http.Request, params api.ListJournalDraftsParams) {
	h.listJournalDrafts(w, r, params)
}

func (h *IAMAPI) CreateJournalDraft(w http.ResponseWriter, r *http.Request, params api.CreateJournalDraftParams) {
	h.createJournalDraft(w, r, params)
}

func (h *IAMAPI) UpdateJournalDraft(w http.ResponseWriter, r *http.Request, draftID api.DraftID, params api.UpdateJournalDraftParams) {
	h.updateJournalDraft(w, r, draftID, params)
}

func (h *IAMAPI) PostJournalDraft(w http.ResponseWriter, r *http.Request, draftID api.DraftID, params api.PostJournalDraftParams) {
	h.postJournalDraft(w, r, draftID, params)
}

func (h *IAMAPI) ListJournalEntries(w http.ResponseWriter, r *http.Request, params api.ListJournalEntriesParams) {
	h.listJournalEntries(w, r, params)
}

func (h *IAMAPI) GetJournalEntry(w http.ResponseWriter, r *http.Request, entryID api.EntryID) {
	h.getJournalEntry(w, r, entryID)
}

func (h *IAMAPI) ReverseJournalEntry(w http.ResponseWriter, r *http.Request, entryID api.EntryID, params api.ReverseJournalEntryParams) {
	h.reverseJournalEntry(w, r, entryID, params)
}

func (h *IAMAPI) ListAccountingOpenItems(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListAccountingOpenItemsParams,
) {
	h.listAccountingOpenItems(w, r, params)
}

func (h *IAMAPI) CreateAccountingReceipt(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateAccountingReceiptParams,
) {
	h.createAccountingSettlement(w, r, params.IdempotencyKey, false)
}

func (h *IAMAPI) CreateAccountingSupplierPayment(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateAccountingSupplierPaymentParams,
) {
	h.createAccountingSettlement(w, r, params.IdempotencyKey, true)
}

func (h *IAMAPI) ListAccountingPeriods(w http.ResponseWriter, r *http.Request) {
	h.listAccountingPeriods(w, r)
}

func (h *IAMAPI) CreateAccountingPeriod(w http.ResponseWriter, r *http.Request, params api.CreateAccountingPeriodParams) {
	h.createAccountingPeriod(w, r, params)
}

func (h *IAMAPI) TransitionAccountingPeriod(w http.ResponseWriter, r *http.Request, periodID api.PeriodID, params api.TransitionAccountingPeriodParams) {
	h.transitionAccountingPeriod(w, r, periodID, params)
}

func (h *IAMAPI) CreateAnnualClosingDraft(
	w http.ResponseWriter,
	r *http.Request,
	periodID api.PeriodID,
	params api.CreateAnnualClosingDraftParams,
) {
	h.createAnnualClosingDraft(w, r, periodID, params)
}

func (h *IAMAPI) ListFinancialAccounts(w http.ResponseWriter, r *http.Request, params api.ListFinancialAccountsParams) {
	h.listFinancialAccounts(w, r, params)
}

func (h *IAMAPI) CreateFinancialAccount(w http.ResponseWriter, r *http.Request, params api.CreateFinancialAccountParams) {
	h.createFinancialAccount(w, r, params)
}

func (h *IAMAPI) UpdateFinancialAccount(
	w http.ResponseWriter,
	r *http.Request,
	financialAccountID api.FinancialAccountID,
	params api.UpdateFinancialAccountParams,
) {
	h.updateFinancialAccount(w, r, financialAccountID, params)
}

func (h *IAMAPI) ImportAccountingStatement(w http.ResponseWriter, r *http.Request, params api.ImportAccountingStatementParams) {
	h.importAccountingStatement(w, r, params)
}

func (h *IAMAPI) SuggestAccountingMatches(
	w http.ResponseWriter,
	r *http.Request,
	statementImportID api.StatementImportID,
	params api.SuggestAccountingMatchesParams,
) {
	h.suggestAccountingMatches(w, r, statementImportID, params)
}

func (h *IAMAPI) ListAccountingReconciliations(w http.ResponseWriter, r *http.Request, params api.ListAccountingReconciliationsParams) {
	h.listAccountingReconciliations(w, r, params)
}

func (h *IAMAPI) CreateAccountingReconciliation(w http.ResponseWriter, r *http.Request, params api.CreateAccountingReconciliationParams) {
	h.createAccountingReconciliation(w, r, params)
}

func (h *IAMAPI) GetAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
) {
	h.getAccountingReconciliation(w, r, reconciliationID)
}

func (h *IAMAPI) UpdateAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
	params api.UpdateAccountingReconciliationParams,
) {
	h.updateAccountingReconciliation(w, r, reconciliationID, params)
}

func (h *IAMAPI) TransitionAccountingReconciliation(
	w http.ResponseWriter,
	r *http.Request,
	reconciliationID api.ReconciliationID,
	action api.TransitionAccountingReconciliationParamsReconciliationAction,
	params api.TransitionAccountingReconciliationParams,
) {
	h.transitionAccountingReconciliation(w, r, reconciliationID, action, params)
}

func (h *IAMAPI) GetAccountingReport(
	w http.ResponseWriter,
	r *http.Request,
	report api.GetAccountingReportParamsReport,
	params api.GetAccountingReportParams,
) {
	h.getAccountingReport(w, r, report, params)
}

func (h *IAMAPI) ExportAccountingReport(
	w http.ResponseWriter,
	r *http.Request,
	report api.ExportAccountingReportParamsReport,
	params api.ExportAccountingReportParams,
) {
	h.exportAccountingReport(w, r, report, params)
}

func (h *IAMAPI) ImportInflationIndices(w http.ResponseWriter, r *http.Request, params api.ImportInflationIndicesParams) {
	h.importInflationIndices(w, r, params)
}

func (h *IAMAPI) CreateCurrencyRevaluation(w http.ResponseWriter, r *http.Request, params api.CreateCurrencyRevaluationParams) {
	h.createCurrencyRevaluation(w, r, params)
}

func (h *IAMAPI) RotateFiscalCertificate(w http.ResponseWriter, r *http.Request, params api.RotateFiscalCertificateParams) {
	h.rotateFiscalCertificate(w, r, params)
}

func (h *IAMAPI) CreateFiscalCreditNote(w http.ResponseWriter, r *http.Request, params api.CreateFiscalCreditNoteParams) {
	h.createFiscalCreditNote(w, r, params)
}

func (h *IAMAPI) CreateFiscalDebitNote(w http.ResponseWriter, r *http.Request, params api.CreateFiscalDebitNoteParams) {
	h.createFiscalDebitNote(w, r, params)
}

func (h *IAMAPI) GetIVASimple(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.GetIVASimpleParams,
) {
	h.getIVASimple(w, r, period, params)
}

func (h *IAMAPI) GetIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.GetIVASimpleWorkflowParams,
) {
	h.getIVASimpleWorkflow(w, r, period, params)
}

func (h *IAMAPI) PrepareIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.PrepareIVASimpleWorkflowParams,
) {
	h.prepareIVASimpleWorkflow(w, r, period, params)
}

func (h *IAMAPI) CloseIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.CloseIVASimpleWorkflowParams,
) {
	h.closeIVASimpleWorkflow(w, r, period, params)
}

func (h *IAMAPI) ExportIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.ExportIVASimpleWorkflowParams,
) {
	h.exportIVASimpleWorkflow(w, r, period, params)
}

func (h *IAMAPI) ReopenIVASimpleWorkflow(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	params api.ReopenIVASimpleWorkflowParams,
) {
	h.reopenIVASimpleWorkflow(w, r, period, params)
}

func (h *IAMAPI) DownloadIVASimpleExport(
	w http.ResponseWriter,
	r *http.Request,
	period string,
	exportID uuid.UUID,
	params api.DownloadIVASimpleExportParams,
) {
	h.downloadIVASimpleExport(w, r, period, exportID, params)
}

func (h *IAMAPI) ListFiscalPointsOfSale(w http.ResponseWriter, r *http.Request) {
	h.listFiscalPointsOfSale(w, r)
}

func (h *IAMAPI) CreateFiscalPointOfSale(w http.ResponseWriter, r *http.Request, params api.CreateFiscalPointOfSaleParams) {
	h.createFiscalPointOfSale(w, r, params)
}

func (h *IAMAPI) GetFiscalSettings(
	w http.ResponseWriter,
	r *http.Request,
	params api.GetFiscalSettingsParams,
) {
	h.getFiscalSettings(w, r, params)
}

func (h *IAMAPI) GetLatestFiscalHomologation(w http.ResponseWriter, r *http.Request) {
	h.getLatestFiscalHomologation(w, r)
}

func (h *IAMAPI) EnableFiscalProduction(
	w http.ResponseWriter,
	r *http.Request,
	params api.EnableFiscalProductionParams,
) {
	h.enableFiscalProduction(w, r, params)
}

func (h *IAMAPI) ListFiscalPurchaseVouchers(
	w http.ResponseWriter,
	r *http.Request,
	params api.ListFiscalPurchaseVouchersParams,
) {
	h.listFiscalPurchaseVouchers(w, r, params)
}

func (h *IAMAPI) CreateFiscalPurchaseVoucher(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateFiscalPurchaseVoucherParams,
) {
	h.createFiscalPurchaseVoucher(w, r, params)
}

func (h *IAMAPI) UpdateFiscalSettings(w http.ResponseWriter, r *http.Request, params api.UpdateFiscalSettingsParams) {
	h.updateFiscalSettings(w, r, params)
}

func (h *IAMAPI) ListFiscalVouchers(w http.ResponseWriter, r *http.Request, params api.ListFiscalVouchersParams) {
	h.listFiscalVouchers(w, r, params)
}

func (h *IAMAPI) CreateFiscalVoucher(w http.ResponseWriter, r *http.Request, params api.CreateFiscalVoucherParams) {
	h.createFiscalVoucher(w, r, params)
}

func (h *IAMAPI) GetFiscalVoucher(w http.ResponseWriter, r *http.Request, voucherID api.VoucherID) {
	h.getFiscalVoucher(w, r, voucherID)
}

func (h *IAMAPI) GetFiscalVoucherPDF(w http.ResponseWriter, r *http.Request, voucherID api.VoucherID) {
	h.getFiscalVoucherPDF(w, r, voucherID)
}
