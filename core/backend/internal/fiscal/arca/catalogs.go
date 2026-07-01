// Package arca es el adapter de los web services de ARCA (ex-AFIP) para
// facturación electrónica: WSAA (autenticación) y WSFEv1 (CAE). Se conservan los
// nombres literales de los web services y los dominios afip.gov.ar porque son los
// reales y vigentes (ARCA no los renombró). Reimplementación limpia en Go,
// extrapolada de arca-facturacion (MIT) y pyafipws (referencia).
package arca

// Tipos de comprobante (CbteTipo) — WSFEv1.
const (
	CbteFacturaA     = 1
	CbteNotaDebitoA  = 2
	CbteNotaCreditoA = 3
	CbteFacturaB     = 6
	CbteNotaDebitoB  = 7
	CbteNotaCreditoB = 8
	CbteFacturaC     = 11
	CbteNotaDebitoC  = 12
	CbteNotaCreditoC = 13
	CbteFacturaE     = 19
)

// Tipos de documento del receptor (DocTipo).
const (
	DocCUIT            = 80
	DocCUIL            = 86
	DocDNI             = 96
	DocConsumidorFinal = 99
)

// Conceptos (Concepto).
const (
	ConceptoProductos = 1
	ConceptoServicios = 2
	ConceptoAmbos     = 3
)

// Condición IVA del receptor (CondicionIVAReceptorId) — obligatorio desde 2026.
const (
	CondIVAResponsableInscripto = 1
	CondIVAExento               = 4
	CondIVAConsumidorFinal      = 5
	CondIVAMonotributo          = 6
)

// IDs de alícuota IVA (AlicIva.Id) en WSFEv1.
const (
	IvaID0   = 3 // 0%
	IvaID105 = 4 // 10.5%
	IvaID21  = 5 // 21%
	IvaID27  = 6 // 27%
	IvaID5   = 8 // 5%
	IvaID25  = 9 // 2.5%
)

// ivaRateToID mapea una alícuota (porcentaje) al ID de ARCA. Clave en centésimas
// de punto para comparar exacto sin problemas de float (2100 = 21.00%).
var ivaRateToID = map[int]int{
	0:    IvaID0,
	1050: IvaID105,
	2100: IvaID21,
	2700: IvaID27,
	500:  IvaID5,
	250:  IvaID25,
}

// IvaIDForRate devuelve el ID de alícuota ARCA para un porcentaje dado (ej. 21.0
// -> 5). ok=false si la alícuota no está soportada por ARCA.
func IvaIDForRate(rate float64) (id int, ok bool) {
	key := int(rate*100 + 0.5)
	id, ok = ivaRateToID[key]
	return id, ok
}

// Códigos de moneda ARCA.
const (
	MonedaPesos = "PES"
	MonedaDolar = "DOL"
)

// Endpoints por ambiente. Se conservan los dominios afip.gov.ar (reales).
type Endpoints struct {
	WSAA string // LoginCms
	WSFE string // WSFEv1
}

func EndpointsFor(production bool) Endpoints {
	if production {
		return Endpoints{
			WSAA: "https://wsaa.afip.gov.ar/ws/services/LoginCms",
			WSFE: "https://servicios1.afip.gov.ar/wsfev1/service.asmx",
		}
	}
	return Endpoints{
		WSAA: "https://wsaahomo.afip.gov.ar/ws/services/LoginCms",
		WSFE: "https://wswhomo.afip.gov.ar/wsfev1/service.asmx",
	}
}
