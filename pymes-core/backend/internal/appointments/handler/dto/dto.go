package dto

type CreateAppointmentRequest struct {
	CustomerID    *string        `json:"customer_id,omitempty"`
	CustomerName  string         `json:"customer_name" binding:"required"`
	CustomerPhone string         `json:"customer_phone"`
	Title         string         `json:"title" binding:"required"`
	Description   string         `json:"description"`
	Status        string         `json:"status"`
	StartAt       string         `json:"start_at" binding:"required"`
	EndAt         string         `json:"end_at,omitempty"`
	Duration      int            `json:"duration,omitempty"`
	Location      string         `json:"location"`
	AssignedTo    string         `json:"assigned_to"`
	Color         string         `json:"color"`
	Notes         string         `json:"notes"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type UpdateAppointmentRequest struct {
	CustomerID    *string        `json:"customer_id,omitempty"`
	CustomerName  *string        `json:"customer_name,omitempty"`
	CustomerPhone *string        `json:"customer_phone,omitempty"`
	Title         *string        `json:"title,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Status        *string        `json:"status,omitempty"`
	StartAt       *string        `json:"start_at,omitempty"`
	EndAt         *string        `json:"end_at,omitempty"`
	Duration      *int           `json:"duration,omitempty"`
	Location      *string        `json:"location,omitempty"`
	AssignedTo    *string        `json:"assigned_to,omitempty"`
	Color         *string        `json:"color,omitempty"`
	Notes         *string        `json:"notes,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}
