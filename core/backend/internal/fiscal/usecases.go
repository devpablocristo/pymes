package fiscal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/devpablocristo/platform/errors/go/domainerr"
	"github.com/devpablocristo/pymes/core/backend/internal/fiscal/arca"
	fiscaldomain "github.com/devpablocristo/pymes/core/backend/internal/fiscal/usecases/domain"
)

// wsfeService es el nombre del servicio ARCA para el TA (literal, no se renombra).
const wsfeService = "wsfe"

// SettingsRecord es la config fiscal completa (incluye material sensible). El
// repository la persiste; el usecase la enmascara para exponerla.
type SettingsRecord struct {
	OrgID              uuid.UUID
	CUIT               string
	Environment        string
	TaxCondition       string
	CertPEM            string
	KeyEncrypted       string
	DefaultPointOfSale int
	Enabled            bool
	UpdatedAt          time.Time
}

type RepositoryPort interface {
	GetSettings(ctx context.Context, orgID uuid.UUID) (SettingsRecord, error)
	SaveSettings(ctx context.Context, rec SettingsRecord) error
	GetTicket(ctx context.Context, orgID uuid.UUID, service string) (fiscaldomain.AuthTicket, error)
	SaveTicket(ctx context.Context, orgID uuid.UUID, service string, ta fiscaldomain.AuthTicket) error
	SaveVoucher(ctx context.Context, v fiscaldomain.FiscalVoucher) error
	GetVoucher(ctx context.Context, orgID, id uuid.UUID) (fiscaldomain.FiscalVoucher, error)
	GetAuthorizedVoucherBySale(ctx context.Context, orgID, saleID uuid.UUID) (fiscaldomain.FiscalVoucher, error)
	GetAuthorizedVoucherByReturn(ctx context.Context, orgID, returnID uuid.UUID) (fiscaldomain.FiscalVoucher, error)
	ListVouchers(ctx context.Context, orgID uuid.UUID, limit int) ([]fiscaldomain.FiscalVoucher, error)
}

// ReturnFiscalData es el snapshot de una devolución para emitir su nota de crédito.
type ReturnFiscalData struct {
	SaleID   uuid.UUID
	Subtotal float64
	TaxTotal float64
	Total    float64
	Items    []SaleFiscalItem
}

// ReturnReader lee los datos fiscales de una devolución (adapter sobre returns).
type ReturnReader interface {
	GetReturnFiscalData(ctx context.Context, orgID, returnID uuid.UUID) (ReturnFiscalData, error)
}

// SaleFiscalItem es lo que el fiscal necesita de cada línea de venta para el IVA.
type SaleFiscalItem struct {
	TaxRate  float64
	Subtotal float64
}

// SaleFiscalData es el snapshot de la venta para emitir su comprobante.
type SaleFiscalData struct {
	Currency        string
	Subtotal        float64
	TaxTotal        float64
	Total           float64
	Items           []SaleFiscalItem
	CustomerTaxID   string
	CustomerCondIVA string
}

// SalesReader lee los datos fiscales de una venta (adapter sobre sales+party).
type SalesReader interface {
	GetSaleFiscalData(ctx context.Context, orgID, saleID uuid.UUID) (SaleFiscalData, error)
}

// CryptoPort cifra/descifra la clave privada del certificado (paymentgateway.Crypto).
type CryptoPort interface {
	Encrypt(plain string) (string, error)
	Decrypt(cipher string) (string, error)
}

// ArcaClientPort autentica contra el WSAA (impl real en internal/fiscal/arca).
type ArcaClientPort interface {
	Login(ctx context.Context, creds arca.Credentials, service string) (arca.TA, error)
	LastAuthorized(ctx context.Context, prod bool, ta arca.TA, cuit int64, ptoVta, cbteTipo int) (int64, error)
	RequestCAE(ctx context.Context, prod bool, ta arca.TA, cuit int64, req arca.CAERequest) (arca.CAEResult, error)
}

type Usecases struct {
	repo         RepositoryPort
	crypto       CryptoPort
	arca         ArcaClientPort
	salesReader  SalesReader
	returnReader ReturnReader

	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

// Option configura dependencias opcionales sin romper la firma.
type Option func(*Usecases)

// WithSalesReader cablea el lector de datos fiscales de la venta (necesario para emitir).
func WithSalesReader(sr SalesReader) Option { return func(u *Usecases) { u.salesReader = sr } }

// WithReturnReader cablea el lector de devoluciones (necesario para notas de crédito).
func WithReturnReader(rr ReturnReader) Option { return func(u *Usecases) { u.returnReader = rr } }

func NewUsecases(repo RepositoryPort, crypto CryptoPort, arcaClient ArcaClientPort, opts ...Option) *Usecases {
	u := &Usecases{repo: repo, crypto: crypto, arca: arcaClient, locks: map[string]*sync.Mutex{}}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// lockFor devuelve el mutex que serializa emisiones por (org, pto vta, tipo),
// para no romper la correlatividad de numeración. (Single-instance; multi-instance
// usaría un advisory lock de Postgres — ver plan.)
func (u *Usecases) lockFor(key string) *sync.Mutex {
	u.locksMu.Lock()
	defer u.locksMu.Unlock()
	m, ok := u.locks[key]
	if !ok {
		m = &sync.Mutex{}
		u.locks[key] = m
	}
	return m
}

// SaveSettingsInput es la config editable. CertPEM/KeyPEM son opcionales: si se
// envían, se reemplaza el certificado (la clave se guarda cifrada).
type SaveSettingsInput struct {
	CUIT               string
	Environment        string
	TaxCondition       string
	DefaultPointOfSale int
	Enabled            bool
	CertPEM            string
	KeyPEM             string
}

func (u *Usecases) GetSettings(ctx context.Context, orgID uuid.UUID) (fiscaldomain.FiscalSettings, error) {
	rec, err := u.repo.GetSettings(ctx, orgID)
	if errors.Is(err, ErrNotFound) {
		// Sin config todavía: devolvemos defaults vacíos (no es error).
		return fiscaldomain.FiscalSettings{OrgID: orgID, Environment: "homologation", DefaultPointOfSale: 1}, nil
	}
	if err != nil {
		return fiscaldomain.FiscalSettings{}, err
	}
	return maskSettings(rec), nil
}

func (u *Usecases) SaveSettings(ctx context.Context, orgID uuid.UUID, in SaveSettingsInput) (fiscaldomain.FiscalSettings, error) {
	if orgID == uuid.Nil {
		return fiscaldomain.FiscalSettings{}, domainerr.Validation("org_id is required")
	}
	env := strings.TrimSpace(strings.ToLower(in.Environment))
	if env == "" {
		env = "homologation"
	}
	if env != "homologation" && env != "production" {
		return fiscaldomain.FiscalSettings{}, domainerr.Validation("environment must be homologation or production")
	}
	cuit := strings.TrimSpace(in.CUIT)
	if cuit != "" && !isNumeric(cuit) {
		return fiscaldomain.FiscalSettings{}, domainerr.Validation("cuit must be numeric (no dashes)")
	}
	pos := in.DefaultPointOfSale
	if pos <= 0 {
		pos = 1
	}

	// Cargar lo existente para no pisar el certificado si no se reenvía.
	rec, err := u.repo.GetSettings(ctx, orgID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalSettings{}, err
	}
	rec.OrgID = orgID
	rec.CUIT = cuit
	rec.Environment = env
	rec.TaxCondition = strings.TrimSpace(in.TaxCondition)
	rec.DefaultPointOfSale = pos
	rec.Enabled = in.Enabled

	cert := strings.TrimSpace(in.CertPEM)
	key := strings.TrimSpace(in.KeyPEM)
	switch {
	case cert != "" && key != "":
		enc, err := u.crypto.Encrypt(key)
		if err != nil {
			return fiscaldomain.FiscalSettings{}, err
		}
		rec.CertPEM = cert
		rec.KeyEncrypted = enc
	case cert != "" || key != "":
		return fiscaldomain.FiscalSettings{}, domainerr.Validation("cert_pem and key_pem must be provided together")
	}

	if err := u.repo.SaveSettings(ctx, rec); err != nil {
		return fiscaldomain.FiscalSettings{}, err
	}
	return maskSettings(rec), nil
}

// Authenticate obtiene un TA válido del WSAA, reusando el cacheado si sigue
// vigente. Es el flujo que valida la configuración del certificado end-to-end.
func (u *Usecases) Authenticate(ctx context.Context, orgID uuid.UUID) (fiscaldomain.AuthTicket, error) {
	if ta, err := u.repo.GetTicket(ctx, orgID, wsfeService); err == nil && ta.Valid(time.Now()) {
		return ta, nil
	} else if err != nil && !errors.Is(err, ErrNotFound) {
		return fiscaldomain.AuthTicket{}, err
	}

	rec, err := u.repo.GetSettings(ctx, orgID)
	if errors.Is(err, ErrNotFound) {
		return fiscaldomain.AuthTicket{}, domainerr.Validation("fiscal settings not configured")
	}
	if err != nil {
		return fiscaldomain.AuthTicket{}, err
	}
	if rec.CertPEM == "" || rec.KeyEncrypted == "" {
		return fiscaldomain.AuthTicket{}, domainerr.Validation("certificate not configured")
	}
	keyPEM, err := u.crypto.Decrypt(rec.KeyEncrypted)
	if err != nil {
		return fiscaldomain.AuthTicket{}, err
	}
	cuit, _ := strconv.ParseInt(rec.CUIT, 10, 64)
	ta, err := u.arca.Login(ctx, arca.Credentials{
		CUIT: cuit, CertPEM: rec.CertPEM, KeyPEM: keyPEM, Production: rec.Environment == "production",
	}, wsfeService)
	if err != nil {
		return fiscaldomain.AuthTicket{}, domainerr.Unavailable("wsaa authentication failed: " + err.Error())
	}
	out := fiscaldomain.AuthTicket{Token: ta.Token, Sign: ta.Sign, ExpiresAt: ta.ExpiresAt}
	if err := u.repo.SaveTicket(ctx, orgID, wsfeService, out); err != nil {
		return fiscaldomain.AuthTicket{}, err
	}
	return out, nil
}

// EmitInput son los parámetros de emisión. VoucherType/PointOfSale/Concepto en 0
// se resuelven automáticamente (según condición IVA, settings, y productos).
// Para servicios (Concepto 2/3) se usan las fechas de servicio; si faltan, se
// asume la fecha de emisión. ExchangeRate solo aplica a moneda extranjera.
type EmitInput struct {
	SaleID      uuid.UUID
	VoucherType int
	PointOfSale int
	Concepto    int
	// Fechas de servicio (YYYY-MM-DD), obligatorias en ARCA para Concepto 2/3.
	ServiceFrom string
	ServiceTo   string
	PaymentDue  string
	// ExchangeRate es la cotización de la moneda extranjera (MonCotiz). En pesos es 1.
	ExchangeRate float64
	Actor        string
}

// EmitVoucher emite el comprobante fiscal de una venta: autentica, reserva número
// correlativo (serializado), solicita CAE y persiste el resultado con QR.
func (u *Usecases) EmitVoucher(ctx context.Context, orgID uuid.UUID, in EmitInput) (fiscaldomain.FiscalVoucher, error) {
	if u.salesReader == nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Internal("sales reader not configured")
	}
	if orgID == uuid.Nil || in.SaleID == uuid.Nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("org_id and sale_id are required")
	}
	// Idempotencia: una venta tiene a lo sumo un comprobante autorizado.
	if existing, err := u.repo.GetAuthorizedVoucherBySale(ctx, orgID, in.SaleID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, err
	}

	rec, err := u.repo.GetSettings(ctx, orgID)
	if errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("fiscal settings not configured")
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	if !rec.Enabled {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("fiscal emission not enabled for this org")
	}

	sale, err := u.salesReader.GetSaleFiscalData(ctx, orgID, in.SaleID)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	docTipo, docNro, recCond := resolveReceptor(sale.CustomerTaxID, sale.CustomerCondIVA)
	vtype := in.VoucherType
	if vtype == 0 {
		vtype = computeVoucherType(rec.TaxCondition, recCond)
	}
	pos := in.PointOfSale
	if pos <= 0 {
		pos = rec.DefaultPointOfSale
	}
	concepto := in.Concepto
	if concepto == 0 {
		concepto = arca.ConceptoProductos
	}
	neto, iva, total, lines, err := buildImports(vtype, sale)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation(err.Error())
	}

	// Moneda: mapear el código de la venta al MonId de ARCA. En pesos la cotización
	// es siempre 1; en moneda extranjera se usa la cotización provista.
	monID := currencyToArca(sale.Currency)
	cotiz := in.ExchangeRate
	if monID == arca.MonedaPesos || cotiz <= 0 {
		cotiz = 1
	}

	ta, err := u.Authenticate(ctx, orgID)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	cuit, _ := strconv.ParseInt(rec.CUIT, 10, 64)
	prod := rec.Environment == "production"

	// Serializar por (org, pto vta, tipo) para no romper la correlatividad.
	lk := u.lockFor(fmt.Sprintf("%s:%d:%d", orgID, pos, vtype))
	lk.Lock()
	defer lk.Unlock()

	last, err := u.arca.LastAuthorized(ctx, prod, taToArca(ta), cuit, pos, vtype)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Unavailable("wsfe last authorized: " + err.Error())
	}
	nro := last + 1
	now := time.Now()
	today := now.Format("20060102")

	// Fechas de servicio: obligatorias para Concepto 2/3; default a la emisión.
	var fchServDesde, fchServHasta, fchVtoPago string
	if concepto == arca.ConceptoServicios || concepto == arca.ConceptoAmbos {
		fchServDesde = toArcaDate(in.ServiceFrom, today)
		fchServHasta = toArcaDate(in.ServiceTo, fchServDesde)
		fchVtoPago = toArcaDate(in.PaymentDue, today)
	}

	voucher := fiscaldomain.FiscalVoucher{
		ID: uuid.New(), OrgID: orgID, SaleID: &in.SaleID, VoucherType: vtype, PointOfSale: pos,
		CbteNro: nro, Concepto: concepto, DocTipo: docTipo, DocNro: docNro, CondicionIVAReceptor: recCond,
		Currency: monID, ExchangeRate: cotiz, ImpNeto: neto, ImpIVA: iva, ImpTotal: total, IvaBreakdown: lines,
		Status: "pending", CreatedBy: in.Actor, CreatedAt: now,
	}
	if err := u.repo.SaveVoucher(ctx, voucher); err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}

	res, err := u.arca.RequestCAE(ctx, prod, taToArca(ta), cuit, arca.CAERequest{
		PtoVta: pos, CbteTipo: vtype, Concepto: concepto, DocTipo: docTipo, DocNro: docNro,
		CbteNro: nro, CbteFch: today, ImpTotal: total, ImpNeto: neto, ImpIVA: iva,
		MonID: monID, MonCotiz: cotiz, CondicionIVAReceptorID: recCond, Iva: ivaLinesToArca(lines),
		FchServDesde: fchServDesde, FchServHasta: fchServHasta, FchVtoPago: fchVtoPago,
	})
	if err != nil {
		voucher.Status = "error"
		voucher.AfipResult = err.Error()
		_ = u.repo.SaveVoucher(ctx, voucher)
		return fiscaldomain.FiscalVoucher{}, domainerr.Unavailable("wsfe request cae: " + err.Error())
	}

	voucher.AfipResult = res.Resultado
	voucher.Observations = toNotes(res.Observations)
	voucher.Errors = toNotes(res.Errors)
	if res.Resultado == "A" {
		voucher.Status = "authorized"
		voucher.CAE = res.CAE
		voucher.CAEVto = res.CAEFchVto
		if res.CbteNro > 0 {
			voucher.CbteNro = res.CbteNro
		}
		emittedAt := now
		voucher.EmittedAt = &emittedAt
		caeNum, _ := strconv.ParseInt(res.CAE, 10, 64)
		docNum, _ := strconv.ParseInt(docNro, 10, 64)
		if qr, qerr := arca.BuildQRURL(arca.QRInput{
			Fecha: now.Format("2006-01-02"), CUIT: cuit, PtoVta: pos, TipoCmp: vtype, NroCmp: voucher.CbteNro,
			Importe: total, Moneda: monID, Ctz: cotiz, TipoDocRec: docTipo, NroDocRec: docNum, CodAut: caeNum,
		}); qerr == nil {
			voucher.QRURL = qr
		}
	} else {
		voucher.Status = "rejected"
	}
	if err := u.repo.SaveVoucher(ctx, voucher); err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	return voucher, nil
}

// EmitCreditNoteInput emite una NC contra la factura autorizada de la venta de
// una devolución. VoucherType se deriva del comprobante original.
type EmitCreditNoteInput struct {
	ReturnID    uuid.UUID
	PointOfSale int
	Actor       string
}

// EmitCreditNote emite la nota de crédito de una devolución, referenciando la
// factura original (CbtesAsoc).
func (u *Usecases) EmitCreditNote(ctx context.Context, orgID uuid.UUID, in EmitCreditNoteInput) (fiscaldomain.FiscalVoucher, error) {
	if u.returnReader == nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Internal("return reader not configured")
	}
	if orgID == uuid.Nil || in.ReturnID == uuid.Nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("org_id and return_id are required")
	}
	if existing, err := u.repo.GetAuthorizedVoucherByReturn(ctx, orgID, in.ReturnID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, err
	}
	rec, err := u.repo.GetSettings(ctx, orgID)
	if errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("fiscal settings not configured")
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	if !rec.Enabled {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("fiscal emission not enabled for this org")
	}
	ret, err := u.returnReader.GetReturnFiscalData(ctx, orgID, in.ReturnID)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	inv, err := u.repo.GetAuthorizedVoucherBySale(ctx, orgID, ret.SaleID)
	if errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("la venta no tiene factura autorizada; emitila antes de la nota de crédito")
	}
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	ncType, ok := ncTypeFor(inv.VoucherType)
	if !ok {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation("el tipo de factura no admite nota de crédito automática")
	}
	neto, iva, total, lines, err := buildImports(ncType, SaleFiscalData{Subtotal: ret.Subtotal, TaxTotal: ret.TaxTotal, Total: ret.Total, Items: ret.Items})
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Validation(err.Error())
	}

	ta, err := u.Authenticate(ctx, orgID)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	cuit, _ := strconv.ParseInt(rec.CUIT, 10, 64)
	prod := rec.Environment == "production"
	pos := in.PointOfSale
	if pos <= 0 {
		pos = inv.PointOfSale
	}

	lk := u.lockFor(fmt.Sprintf("%s:%d:%d", orgID, pos, ncType))
	lk.Lock()
	defer lk.Unlock()

	last, err := u.arca.LastAuthorized(ctx, prod, taToArca(ta), cuit, pos, ncType)
	if err != nil {
		return fiscaldomain.FiscalVoucher{}, domainerr.Unavailable("wsfe last authorized: " + err.Error())
	}
	nro := last + 1
	now := time.Now()
	saleID := ret.SaleID
	invID := inv.ID
	// La NC hereda la moneda y cotización del comprobante original.
	monID := inv.Currency
	if monID == "" {
		monID = arca.MonedaPesos
	}
	cotiz := inv.ExchangeRate
	if monID == arca.MonedaPesos || cotiz <= 0 {
		cotiz = 1
	}
	voucher := fiscaldomain.FiscalVoucher{
		ID: uuid.New(), OrgID: orgID, SaleID: &saleID, ReturnID: &in.ReturnID, AssociatedVoucherID: &invID,
		VoucherType: ncType, PointOfSale: pos, CbteNro: nro, Concepto: arca.ConceptoProductos,
		DocTipo: inv.DocTipo, DocNro: inv.DocNro, CondicionIVAReceptor: inv.CondicionIVAReceptor,
		Currency: monID, ExchangeRate: cotiz, ImpNeto: neto, ImpIVA: iva, ImpTotal: total, IvaBreakdown: lines,
		Status: "pending", CreatedBy: in.Actor, CreatedAt: now,
	}
	if err := u.repo.SaveVoucher(ctx, voucher); err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	res, err := u.arca.RequestCAE(ctx, prod, taToArca(ta), cuit, arca.CAERequest{
		PtoVta: pos, CbteTipo: ncType, Concepto: arca.ConceptoProductos, DocTipo: inv.DocTipo, DocNro: inv.DocNro,
		CbteNro: nro, CbteFch: now.Format("20060102"), ImpTotal: total, ImpNeto: neto, ImpIVA: iva,
		MonID: monID, MonCotiz: cotiz, CondicionIVAReceptorID: inv.CondicionIVAReceptor, Iva: ivaLinesToArca(lines),
		CbtesAsoc: []arca.CbteAsoc{{Tipo: inv.VoucherType, PtoVta: inv.PointOfSale, Nro: inv.CbteNro}},
	})
	if err != nil {
		voucher.Status = "error"
		voucher.AfipResult = err.Error()
		_ = u.repo.SaveVoucher(ctx, voucher)
		return fiscaldomain.FiscalVoucher{}, domainerr.Unavailable("wsfe request cae: " + err.Error())
	}
	voucher.AfipResult = res.Resultado
	voucher.Observations = toNotes(res.Observations)
	voucher.Errors = toNotes(res.Errors)
	if res.Resultado == "A" {
		voucher.Status = "authorized"
		voucher.CAE = res.CAE
		voucher.CAEVto = res.CAEFchVto
		if res.CbteNro > 0 {
			voucher.CbteNro = res.CbteNro
		}
		emittedAt := now
		voucher.EmittedAt = &emittedAt
		caeNum, _ := strconv.ParseInt(res.CAE, 10, 64)
		docNum, _ := strconv.ParseInt(inv.DocNro, 10, 64)
		if qr, qerr := arca.BuildQRURL(arca.QRInput{
			Fecha: now.Format("2006-01-02"), CUIT: cuit, PtoVta: pos, TipoCmp: ncType, NroCmp: voucher.CbteNro,
			Importe: total, Moneda: monID, Ctz: cotiz, TipoDocRec: inv.DocTipo, NroDocRec: docNum, CodAut: caeNum,
		}); qerr == nil {
			voucher.QRURL = qr
		}
	} else {
		voucher.Status = "rejected"
	}
	if err := u.repo.SaveVoucher(ctx, voucher); err != nil {
		return fiscaldomain.FiscalVoucher{}, err
	}
	return voucher, nil
}

func (u *Usecases) GetVoucher(ctx context.Context, orgID, id uuid.UUID) (fiscaldomain.FiscalVoucher, error) {
	out, err := u.repo.GetVoucher(ctx, orgID, id)
	if errors.Is(err, ErrNotFound) {
		return fiscaldomain.FiscalVoucher{}, domainerr.NotFoundf("fiscal_voucher", id.String())
	}
	return out, err
}

func (u *Usecases) ListVouchers(ctx context.Context, orgID uuid.UUID, limit int) ([]fiscaldomain.FiscalVoucher, error) {
	return u.repo.ListVouchers(ctx, orgID, limit)
}

func taToArca(ta fiscaldomain.AuthTicket) arca.TA {
	return arca.TA{Token: ta.Token, Sign: ta.Sign, ExpiresAt: ta.ExpiresAt}
}

func toNotes(ns []arca.Note) []fiscaldomain.Note {
	if len(ns) == 0 {
		return nil
	}
	out := make([]fiscaldomain.Note, 0, len(ns))
	for _, n := range ns {
		out = append(out, fiscaldomain.Note{Code: n.Code, Msg: n.Msg})
	}
	return out
}

func maskSettings(rec SettingsRecord) fiscaldomain.FiscalSettings {
	return fiscaldomain.FiscalSettings{
		OrgID:              rec.OrgID,
		CUIT:               rec.CUIT,
		Environment:        rec.Environment,
		TaxCondition:       rec.TaxCondition,
		DefaultPointOfSale: rec.DefaultPointOfSale,
		HasCertificate:     rec.CertPEM != "" && rec.KeyEncrypted != "",
		Enabled:            rec.Enabled,
		UpdatedAt:          rec.UpdatedAt,
	}
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
