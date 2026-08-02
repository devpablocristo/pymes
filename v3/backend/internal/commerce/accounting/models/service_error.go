package accountingapi

// ServiceErrorPayload is the stable error body returned by the private
// Accounting service.
type ServiceErrorPayload struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

type ProvisioningPayload struct {
	OrganizationID string `json:"organization_id"`
	DisplayName    string `json:"display_name"`
}
