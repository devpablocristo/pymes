package fiscal

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/arca"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
)

type fakeRepo struct {
	settings           *SettingsRecord
	ticket             *fiscaldomain.AuthTicket
	savedTicket        *fiscaldomain.AuthTicket
	vouchers           map[uuid.UUID]fiscaldomain.FiscalVoucher
	authorizedBySale   map[uuid.UUID]fiscaldomain.FiscalVoucher
	authorizedByReturn map[uuid.UUID]fiscaldomain.FiscalVoucher
}

func (f *fakeRepo) GetAuthorizedVoucherByReturn(_ context.Context, _, returnID uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	if v, ok := f.authorizedByReturn[returnID]; ok {
		return v, nil
	}
	return fiscaldomain.FiscalVoucher{}, ErrNotFound
}

type fakeReturns struct {
	data ReturnFiscalData
	err  error
}

func (f fakeReturns) GetReturnFiscalData(_ context.Context, _, _ uuid.UUID) (ReturnFiscalData, error) {
	return f.data, f.err
}

func (f *fakeRepo) SaveVoucher(_ context.Context, v fiscaldomain.FiscalVoucher) error {
	if f.vouchers == nil {
		f.vouchers = map[uuid.UUID]fiscaldomain.FiscalVoucher{}
	}
	f.vouchers[v.ID] = v
	return nil
}
func (f *fakeRepo) GetVoucher(_ context.Context, _, id uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	if v, ok := f.vouchers[id]; ok {
		return v, nil
	}
	return fiscaldomain.FiscalVoucher{}, ErrNotFound
}
func (f *fakeRepo) GetAuthorizedVoucherBySale(_ context.Context, _, saleID uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	if v, ok := f.authorizedBySale[saleID]; ok {
		return v, nil
	}
	return fiscaldomain.FiscalVoucher{}, ErrNotFound
}
func (f *fakeRepo) ListVouchers(_ context.Context, _ uuid.UUID, _ int) ([]fiscaldomain.FiscalVoucher, error) {
	return nil, nil
}

func (f *fakeRepo) GetSettings(_ context.Context, _ uuid.UUID) (SettingsRecord, error) {
	if f.settings == nil {
		return SettingsRecord{}, ErrNotFound
	}
	return *f.settings, nil
}
func (f *fakeRepo) SaveSettings(_ context.Context, rec SettingsRecord) error {
	cp := rec
	f.settings = &cp
	return nil
}
func (f *fakeRepo) GetTicket(_ context.Context, _ uuid.UUID, _ string) (fiscaldomain.AuthTicket, error) {
	if f.ticket == nil {
		return fiscaldomain.AuthTicket{}, ErrNotFound
	}
	return *f.ticket, nil
}
func (f *fakeRepo) SaveTicket(_ context.Context, _ uuid.UUID, _ string, ta fiscaldomain.AuthTicket) error {
	f.savedTicket = &ta
	return nil
}

type fakeCrypto struct{}

func (fakeCrypto) Encrypt(p string) (string, error) { return "enc:" + p, nil }
func (fakeCrypto) Decrypt(c string) (string, error) { return strings.TrimPrefix(c, "enc:"), nil }

type fakeArca struct {
	calls     int
	lastCred  arca.Credentials
	lastAuth  int64
	caeResult arca.CAEResult
	caeErr    error
	caeReq    arca.CAERequest
}

func (f *fakeArca) Login(_ context.Context, creds arca.Credentials, _ string) (arca.TA, error) {
	f.calls++
	f.lastCred = creds
	return arca.TA{Token: "TOK", Sign: "SGN", ExpiresAt: time.Now().Add(12 * time.Hour)}, nil
}
func (f *fakeArca) LastAuthorized(_ context.Context, _ bool, _ arca.TA, _ int64, _, _ int) (int64, error) {
	return f.lastAuth, nil
}
func (f *fakeArca) RequestCAE(_ context.Context, _ bool, _ arca.TA, _ int64, req arca.CAERequest) (arca.CAEResult, error) {
	f.caeReq = req
	if f.caeErr != nil {
		return arca.CAEResult{}, f.caeErr
	}
	return f.caeResult, nil
}

type fakeSales struct {
	data SaleFiscalData
	err  error
}

func (f fakeSales) GetSaleFiscalData(_ context.Context, _, _ uuid.UUID) (SaleFiscalData, error) {
	return f.data, f.err
}

func TestSaveSettingsEncryptsAndMasks(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	uc := NewUsecases(repo, fakeCrypto{}, &fakeArca{})
	org := uuid.New()

	out, err := uc.SaveSettings(context.Background(), org, SaveSettingsInput{
		CUIT: "20111111112", Environment: "homologation", TaxCondition: "RI",
		DefaultPointOfSale: 3, Enabled: true, CertPEM: "CERT", KeyPEM: "SECRETKEY",
	})
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if !out.HasCertificate || out.CUIT != "20111111112" || out.DefaultPointOfSale != 3 {
		t.Fatalf("masked settings wrong: %+v", out)
	}
	// La clave se guardó cifrada, nunca en claro.
	if repo.settings.KeyEncrypted != "enc:SECRETKEY" {
		t.Fatalf("key not encrypted: %q", repo.settings.KeyEncrypted)
	}
}

func TestSaveSettingsValidation(t *testing.T) {
	t.Parallel()
	uc := NewUsecases(&fakeRepo{}, fakeCrypto{}, &fakeArca{})
	org := uuid.New()
	if _, err := uc.SaveSettings(context.Background(), org, SaveSettingsInput{Environment: "prod-typo"}); err == nil {
		t.Fatalf("expected invalid environment error")
	}
	if _, err := uc.SaveSettings(context.Background(), org, SaveSettingsInput{CertPEM: "CERT"}); err == nil {
		t.Fatalf("expected cert-without-key error")
	}
	if _, err := uc.SaveSettings(context.Background(), org, SaveSettingsInput{CUIT: "20-1"}); err == nil {
		t.Fatalf("expected non-numeric cuit error")
	}
}

func TestAuthenticateUsesCache(t *testing.T) {
	t.Parallel()
	valid := fiscaldomain.AuthTicket{Token: "cached", Sign: "s", ExpiresAt: time.Now().Add(2 * time.Hour)}
	arcaCli := &fakeArca{}
	uc := NewUsecases(&fakeRepo{ticket: &valid}, fakeCrypto{}, arcaCli)

	ta, err := uc.Authenticate(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if ta.Token != "cached" || arcaCli.calls != 0 {
		t.Fatalf("should use cache without calling WSAA: token=%s calls=%d", ta.Token, arcaCli.calls)
	}
}

func TestAuthenticateFetchesWhenNoTicket(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{settings: &SettingsRecord{
		CUIT: "20111111112", Environment: "production", CertPEM: "CERT", KeyEncrypted: "enc:KEY",
	}}
	arcaCli := &fakeArca{}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli)

	ta, err := uc.Authenticate(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if ta.Token != "TOK" || arcaCli.calls != 1 {
		t.Fatalf("expected WSAA call: token=%s calls=%d", ta.Token, arcaCli.calls)
	}
	// La clave se descifró y el ambiente production se propagó.
	if arcaCli.lastCred.KeyPEM != "KEY" || !arcaCli.lastCred.Production {
		t.Fatalf("credentials wrong: %+v", arcaCli.lastCred)
	}
	if repo.savedTicket == nil || repo.savedTicket.Token != "TOK" {
		t.Fatalf("ticket not cached")
	}
}

func emitFixture() (*fakeRepo, *fakeArca) {
	repo := &fakeRepo{
		settings: &SettingsRecord{
			CUIT: "20111111112", Environment: "homologation", TaxCondition: "responsable_inscripto",
			CertPEM: "C", KeyEncrypted: "enc:K", DefaultPointOfSale: 3, Enabled: true,
		},
		ticket: &fiscaldomain.AuthTicket{Token: "t", Sign: "s", ExpiresAt: time.Now().Add(2 * time.Hour)},
	}
	arcaCli := &fakeArca{lastAuth: 10, caeResult: arca.CAEResult{Resultado: "A", CAE: "75000000000001", CAEFchVto: "20260801", CbteNro: 11}}
	return repo, arcaCli
}

func TestEmitVoucherAuthorized(t *testing.T) {
	t.Parallel()
	repo, arcaCli := emitFixture()
	sales := fakeSales{data: SaleFiscalData{Currency: "ARS", Subtotal: 100, TaxTotal: 21, Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithSalesReader(sales))

	out, err := uc.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: uuid.New()})
	if err != nil {
		t.Fatalf("EmitVoucher: %v", err)
	}
	if out.Status != "authorized" || out.CAE != "75000000000001" || out.CbteNro != 11 {
		t.Fatalf("bad voucher: %+v", out)
	}
	if out.VoucherType != arca.CbteFacturaB { // RI emisor + CF receptor => B
		t.Fatalf("expected Factura B, got %d", out.VoucherType)
	}
	if out.ImpTotal != 121 || out.ImpIVA != 21 || out.QRURL == "" {
		t.Fatalf("bad importes/QR: total=%.2f iva=%.2f qr=%q", out.ImpTotal, out.ImpIVA, out.QRURL)
	}
	// El request a ARCA llevó el número reservado (last+1) y la condición IVA receptor.
	if arcaCli.caeReq.CbteNro != 11 || arcaCli.caeReq.CondicionIVAReceptorID != arca.CondIVAConsumidorFinal {
		t.Fatalf("bad CAE request: %+v", arcaCli.caeReq)
	}
}

func TestEmitVoucherServices(t *testing.T) {
	t.Parallel()
	repo, arcaCli := emitFixture()
	sales := fakeSales{data: SaleFiscalData{Currency: "ARS", Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithSalesReader(sales))

	// Concepto servicios con fechas explícitas → se propagan al CAE.
	_, err := uc.EmitVoucher(context.Background(), uuid.New(), EmitInput{
		SaleID: uuid.New(), Concepto: arca.ConceptoServicios,
		ServiceFrom: "2026-06-01", ServiceTo: "2026-06-30", PaymentDue: "2026-07-10",
	})
	if err != nil {
		t.Fatalf("EmitVoucher servicios: %v", err)
	}
	if arcaCli.caeReq.Concepto != arca.ConceptoServicios {
		t.Fatalf("expected concepto servicios, got %d", arcaCli.caeReq.Concepto)
	}
	if arcaCli.caeReq.FchServDesde != "20260601" || arcaCli.caeReq.FchServHasta != "20260630" || arcaCli.caeReq.FchVtoPago != "20260710" {
		t.Fatalf("bad service dates: %+v", arcaCli.caeReq)
	}

	// Sin fechas → default a la fecha de emisión (hoy).
	repo2, arca2 := emitFixture()
	uc2 := NewUsecases(repo2, fakeCrypto{}, arca2, WithSalesReader(sales))
	if _, err := uc2.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: uuid.New(), Concepto: arca.ConceptoServicios}); err != nil {
		t.Fatalf("EmitVoucher servicios default: %v", err)
	}
	today := time.Now().Format("20060102")
	if arca2.caeReq.FchServDesde != today || arca2.caeReq.FchServHasta != today || arca2.caeReq.FchVtoPago != today {
		t.Fatalf("expected default service dates = today (%s): %+v", today, arca2.caeReq)
	}
}

func TestEmitVoucherForeignCurrency(t *testing.T) {
	t.Parallel()
	// Moneda extranjera: MonId=DOL, MonCotiz = cotización provista, persistida.
	repo, arcaCli := emitFixture()
	sales := fakeSales{data: SaleFiscalData{Currency: "USD", Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithSalesReader(sales))
	out, err := uc.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: uuid.New(), ExchangeRate: 1400.5})
	if err != nil {
		t.Fatalf("EmitVoucher USD: %v", err)
	}
	if arcaCli.caeReq.MonID != arca.MonedaDolar || arcaCli.caeReq.MonCotiz != 1400.5 {
		t.Fatalf("bad currency in CAE req: %+v", arcaCli.caeReq)
	}
	if out.Currency != arca.MonedaDolar || out.ExchangeRate != 1400.5 {
		t.Fatalf("currency not persisted on voucher: %+v", out)
	}

	// En pesos la cotización siempre es 1, aunque llegue otra.
	repo2, arca2 := emitFixture()
	salesARS := fakeSales{data: SaleFiscalData{Currency: "ARS", Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc2 := NewUsecases(repo2, fakeCrypto{}, arca2, WithSalesReader(salesARS))
	out2, err := uc2.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: uuid.New(), ExchangeRate: 1400})
	if err != nil {
		t.Fatalf("EmitVoucher ARS: %v", err)
	}
	if arca2.caeReq.MonID != arca.MonedaPesos || arca2.caeReq.MonCotiz != 1 || out2.ExchangeRate != 1 {
		t.Fatalf("pesos should force cotiz=1: req=%+v out.rate=%.2f", arca2.caeReq, out2.ExchangeRate)
	}
}

func TestEmitVoucherRejected(t *testing.T) {
	t.Parallel()
	repo, arcaCli := emitFixture()
	arcaCli.caeResult = arca.CAEResult{Resultado: "R", Errors: []arca.Note{{Code: 10016, Msg: "comprobante inválido"}}}
	sales := fakeSales{data: SaleFiscalData{Currency: "ARS", Subtotal: 100, TaxTotal: 21, Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithSalesReader(sales))

	out, err := uc.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: uuid.New()})
	if err != nil {
		t.Fatalf("rejected no debería ser error de transporte: %v", err)
	}
	if out.Status != "rejected" || len(out.Errors) != 1 {
		t.Fatalf("expected rejected with errors: %+v", out)
	}
}

func TestEmitVoucherIdempotent(t *testing.T) {
	t.Parallel()
	saleID := uuid.New()
	prior := fiscaldomain.FiscalVoucher{ID: uuid.New(), CbteNro: 7, Status: "authorized", CAE: "ALREADY"}
	repo, arcaCli := emitFixture()
	repo.authorizedBySale = map[uuid.UUID]fiscaldomain.FiscalVoucher{saleID: prior}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithSalesReader(fakeSales{}))

	out, err := uc.EmitVoucher(context.Background(), uuid.New(), EmitInput{SaleID: saleID})
	if err != nil {
		t.Fatalf("EmitVoucher: %v", err)
	}
	if out.CAE != "ALREADY" || out.CbteNro != 7 {
		t.Fatalf("expected existing authorized voucher, got %+v", out)
	}
}

func TestEmitCreditNote(t *testing.T) {
	t.Parallel()
	saleID := uuid.New()
	invID := uuid.New()
	repo, arcaCli := emitFixture()
	repo.authorizedBySale = map[uuid.UUID]fiscaldomain.FiscalVoucher{
		saleID: {ID: invID, VoucherType: arca.CbteFacturaB, PointOfSale: 3, CbteNro: 11, DocTipo: arca.DocConsumidorFinal, DocNro: "0", CondicionIVAReceptor: arca.CondIVAConsumidorFinal, Status: "authorized"},
	}
	arcaCli.lastAuth = 5
	arcaCli.caeResult = arca.CAEResult{Resultado: "A", CAE: "77000000000002", CAEFchVto: "20260801", CbteNro: 6}
	rr := fakeReturns{data: ReturnFiscalData{SaleID: saleID, Subtotal: 100, TaxTotal: 21, Total: 121, Items: []SaleFiscalItem{{TaxRate: 21, Subtotal: 100}}}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithReturnReader(rr))

	out, err := uc.EmitCreditNote(context.Background(), uuid.New(), EmitCreditNoteInput{ReturnID: uuid.New()})
	if err != nil {
		t.Fatalf("EmitCreditNote: %v", err)
	}
	if out.Status != "authorized" || out.VoucherType != arca.CbteNotaCreditoB {
		t.Fatalf("expected authorized NC B, got status=%s type=%d", out.Status, out.VoucherType)
	}
	if out.AssociatedVoucherID == nil || *out.AssociatedVoucherID != invID || out.ReturnID == nil {
		t.Fatalf("NC should link to original invoice + return: %+v", out)
	}
	// El request llevó el comprobante asociado (la factura original).
	if len(arcaCli.caeReq.CbtesAsoc) != 1 || arcaCli.caeReq.CbtesAsoc[0].Tipo != arca.CbteFacturaB || arcaCli.caeReq.CbtesAsoc[0].Nro != 11 {
		t.Fatalf("bad CbtesAsoc: %+v", arcaCli.caeReq.CbtesAsoc)
	}
}

func TestEmitCreditNoteWithoutInvoice(t *testing.T) {
	t.Parallel()
	repo, arcaCli := emitFixture() // sin authorizedBySale → no hay factura original
	rr := fakeReturns{data: ReturnFiscalData{SaleID: uuid.New(), Subtotal: 100, TaxTotal: 21, Total: 121}}
	uc := NewUsecases(repo, fakeCrypto{}, arcaCli, WithReturnReader(rr))
	if _, err := uc.EmitCreditNote(context.Background(), uuid.New(), EmitCreditNoteInput{ReturnID: uuid.New()}); err == nil {
		t.Fatalf("expected error when sale has no authorized invoice")
	}
}

func TestAuthenticateErrors(t *testing.T) {
	t.Parallel()
	// Sin settings.
	uc := NewUsecases(&fakeRepo{}, fakeCrypto{}, &fakeArca{})
	if _, err := uc.Authenticate(context.Background(), uuid.New()); err == nil {
		t.Fatalf("expected not-configured error")
	}
	// Settings sin certificado.
	uc2 := NewUsecases(&fakeRepo{settings: &SettingsRecord{CUIT: "20", Environment: "homologation"}}, fakeCrypto{}, &fakeArca{})
	if _, err := uc2.Authenticate(context.Background(), uuid.New()); err == nil {
		t.Fatalf("expected no-certificate error")
	}
}
