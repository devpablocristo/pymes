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
	for legacyID, canonicalID := range legacyCapabilityAliases() {
		if cap, ok := byID[strings.ToLower(canonicalID)]; ok {
			byID[strings.ToLower(legacyID)] = cap
		}
	}
	return &Registry{byID: byID}
}

func (r *Registry) List() []Capability {
	out := make([]Capability, 0, len(r.byID))
	seen := make(map[string]struct{}, len(r.byID))
	for _, cap := range r.byID {
		key := strings.ToLower(strings.TrimSpace(cap.ID))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
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
		readCapability("pymes.customers.search", "customers", "read", "Buscar clientes y contactos comerciales.", readChannels),
		readCapability("pymes.services.search", "services", "read", "Buscar servicios publicados por el tenant.", readChannels),
		readCapability("pymes.inventory.search", "inventory", "read", "Buscar productos, servicios e items de inventario.", readChannels),
		readCapability("pymes.cashflow.summary", "cashflow", "read", "Consultar resumen de caja por periodo.", readChannels),
		readCapability("pymes.accounts.summary", "accounts", "read", "Consultar resumen de cuentas por cobrar y pagar.", readChannels),
		readCapability("pymes.get_work_orders", "work_orders", "read", "Consultar ordenes de trabajo demoradas para automations IA.", readChannels),
		readCapability("pymes.get_appointments", "appointments", "read", "Consultar turnos no confirmados para automations IA.", readChannels),
		readCapability("pymes.get_low_stock", "inventory", "read", "Consultar items con stock bajo para automations IA.", readChannels),
		readCapability("pymes.get_customers", "customers", "read", "Consultar clientes inactivos para automations IA.", readChannels),
		readCapability("pymes.get_revenue_comparison", "dashboard", "read", "Consultar comparacion de facturacion mensual para automations IA.", readChannels),
		writeCapability("pymes.quotes.create", "quotes", "create", "Crear presupuesto comercial.", RiskMedium, "pymes.quotes.create", writeChannels),
		writeCapability("pymes.sales.create", "sales", "create", "Crear una venta y sus efectos operativos asociados.", RiskHigh, "pymes.sales.create", writeChannels),
		writeCapability("pymes.payments.link", "payments", "create", "Generar link de pago para una venta.", RiskHigh, "pymes.payments.link", writeChannels),
		writeCapability("pymes.procurement_requests.create", "procurement_requests", "create", "Crear solicitud de compra interna.", RiskMedium, "pymes.procurement_requests.create", writeChannels),
		writeCapability("pymes.scheduling.book", "scheduling", "create", "Reservar turno o agenda operativa.", RiskMedium, "pymes.scheduling.book", writeChannels),
		writeCapability("pymes.send_whatsapp_text", "whatsapp", "notify", "Enviar mensaje WhatsApp de texto desde automation IA.", RiskMedium, "notification.send", writeChannels),
		writeCapability("pymes.send_whatsapp_template", "whatsapp", "notify", "Enviar template WhatsApp desde automation IA.", RiskMedium, "notification.send", writeChannels),
	}
}

func legacyCapabilityAliases() map[string]string {
	return map[string]string{
		"customers.search":           "pymes.customers.search",
		"products.search":            "pymes.inventory.search",
		"services.search":            "pymes.services.search",
		"inventory.read_stock":       "pymes.inventory.search",
		"quotes.create":              "pymes.quotes.create",
		"sales.create":               "pymes.sales.create",
		"payments.generate_link":     "pymes.payments.link",
		"procurement.create_request": "pymes.procurement_requests.create",
		"scheduling.book":            "pymes.scheduling.book",
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
		OwnerModule:            "core",
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
		OwnerModule:            "core",
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
