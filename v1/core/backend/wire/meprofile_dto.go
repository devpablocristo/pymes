package wire

// PatchMeProfileRequest body for PATCH /users/me/profile (extensión Pymes; JWT únicamente).
// Contrato alineado con lo deseable en core/saas (User extendido); el módulo core aún no lo expone en el tipo User.
type PatchMeProfileRequest struct {
	Name       *string `json:"name"`
	GivenName  *string `json:"given_name"`
	FamilyName *string `json:"family_name"`
	Phone      *string `json:"phone"`
}
