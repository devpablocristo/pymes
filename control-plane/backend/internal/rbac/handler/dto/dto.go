package dto

type PermissionInput struct {
	Resource string `json:"resource" binding:"required"`
	Action   string `json:"action" binding:"required"`
}

type CreateRoleRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Permissions []PermissionInput `json:"permissions" binding:"required"`
}

type UpdateRoleRequest struct {
	Description *string           `json:"description"`
	Permissions []PermissionInput `json:"permissions"`
}
