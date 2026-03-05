package dto

type CreateCheckoutRequest struct {
	PlanCode   string `json:"plan_code" binding:"required"`
	SuccessURL string `json:"success_url" binding:"required"`
	CancelURL  string `json:"cancel_url" binding:"required"`
}

type CreatePortalRequest struct {
	ReturnURL string `json:"return_url" binding:"required"`
}
