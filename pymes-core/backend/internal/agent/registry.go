package agent

import (
	"sort"
	"strings"
)

type Registry struct {
	byID map[string]Capability
}

func NewRegistry() *Registry {
	caps := coreCapabilities()
	byID := make(map[string]Capability, len(caps))
	for _, cap := range caps {
		byID[strings.ToLower(strings.TrimSpace(cap.ID))] = cap
	}
	return &Registry{byID: byID}
}

func (r *Registry) List() []Capability {
	out := make([]Capability, 0, len(r.byID))
	for _, cap := range r.byID {
		out = append(out, cap)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (r *Registry) Get(id string) (Capability, bool) {
	cap, ok := r.byID[strings.ToLower(strings.TrimSpace(id))]
	return cap, ok
}

func coreCapabilities() []Capability {
	readChannels := []Channel{ChannelHumanUI, ChannelInternalAgent, ChannelExternalAgent, ChannelMCP}
	writeChannels := []Channel{ChannelHumanUI, ChannelInternalAgent, ChannelExternalAgent, ChannelMCP}
	return []Capability{
		readCapability("customers.search", "customers", "read", "Buscar clientes y contactos comerciales.", readChannels),
		readCapability("products.search", "products", "read", "Buscar productos del catalogo.", readChannels),
		readCapability("services.search", "services", "read", "Buscar servicios publicados por la organizacion.", readChannels),
		readCapability("inventory.read_stock", "inventory", "read", "Consultar stock y stock bajo.", readChannels),
		writeCapability("quotes.create", "quotes", "create", "Crear presupuesto comercial.", RiskMedium, "quote.create", writeChannels),
		writeCapability("sales.create", "sales", "create", "Crear una venta y sus efectos operativos asociados.", RiskHigh, "sale.create", writeChannels),
		writeCapability("payments.generate_link", "payments", "create", "Generar link de pago para una venta.", RiskHigh, "payment_link.generate", writeChannels),
		writeCapability("procurement.create_request", "procurement_requests", "create", "Crear solicitud de compra interna.", RiskMedium, "procurement.request", writeChannels),
		writeCapability("procurement.submit_request", "procurement_requests", "submit", "Enviar solicitud de compra a aprobacion.", RiskHigh, "procurement.submit", writeChannels),
		writeCapability("scheduling.book", "scheduling", "create", "Reservar turno o agenda operativa.", RiskMedium, "scheduling.book", writeChannels),
		writeCapability("cashflow.create_movement", "cashflow", "create", "Registrar movimiento de caja.", RiskHigh, "cashflow.movement", writeChannels),
	}
}

func readCapability(id, resource, action, description string, channels []Channel) Capability {
	return Capability{
		ID:                     id,
		Resource:               resource,
		Action:                 action,
		Description:            description,
		InputSchema:            objectSchema(map[string]any{"search": map[string]any{"type": "string"}}),
		OutputSchema:           objectSchema(map[string]any{"items": map[string]any{"type": "array"}}),
		RiskLevel:              RiskRead,
		RequiresConfirmation:   false,
		RequiresReview:         false,
		RequiresIdempotencyKey: false,
		AllowedChannels:        channels,
		RBACResource:           resource,
		RBACAction:             action,
		AuditAction:            resource + ".agent_read",
		OwnerModule:            "pymes-core",
		NexusActionType:        id,
		ExecutorStatus:         "contract_only",
	}
}

func writeCapability(id, resource, action, description string, risk RiskLevel, nexusAction string, channels []Channel) Capability {
	return Capability{
		ID:                     id,
		Resource:               resource,
		Action:                 action,
		Description:            description,
		InputSchema:            objectSchema(map[string]any{"payload": map[string]any{"type": "object"}}),
		OutputSchema:           objectSchema(map[string]any{"status": map[string]any{"type": "string"}}),
		RiskLevel:              risk,
		RequiresConfirmation:   true,
		RequiresReview:         true,
		RequiresIdempotencyKey: true,
		AllowedChannels:        channels,
		RBACResource:           resource,
		RBACAction:             action,
		AuditAction:            id,
		OwnerModule:            "pymes-core",
		NexusActionType:        nexusAction,
		ExecutorStatus:         "contract_only",
	}
}

func objectSchema(properties map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           properties,
	}
}

func channelAllowed(cap Capability, channel Channel) bool {
	for _, allowed := range cap.AllowedChannels {
		if allowed == channel {
			return true
		}
	}
	return false
}
