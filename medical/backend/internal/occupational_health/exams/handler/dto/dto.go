package dto

type ExamResponse struct {
	ID              string   `json:"id"`
	OrgID        string   `json:"org_id"`
	PatientName     string   `json:"patient_name"`
	PatientDocument string   `json:"patient_document"`
	EmployerName    string   `json:"employer_name"`
	ClientName      string   `json:"client_name"`
	PaymentMethod   string   `json:"payment_method"`
	ExamType        string   `json:"exam_type"`
	Status          string   `json:"status"`
	ScheduledAt     *string  `json:"scheduled_at,omitempty"`
	CompletedAt     *string  `json:"completed_at,omitempty"`
	Result          string   `json:"result"`
	Notes           string   `json:"notes"`
	IsFavorite      bool     `json:"is_favorite"`
	Tags            []string `json:"tags"`
	ImageURLs       []string `json:"image_urls"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type ListExamsResponse struct {
	Items      []ExamResponse `json:"items"`
	Total      int64          `json:"total"`
	HasMore    bool           `json:"has_more"`
	NextCursor *string        `json:"next_cursor"`
}

type CreateExamRequest struct {
	PatientName     string   `json:"patient_name"`
	PatientDocument string   `json:"patient_document"`
	EmployerName    string   `json:"employer_name"`
	ClientName      string   `json:"client_name"`
	PaymentMethod   string   `json:"payment_method"`
	ExamType        string   `json:"exam_type"`
	Status          string   `json:"status"`
	ScheduledAt     *string  `json:"scheduled_at"`
	Result          string   `json:"result"`
	Notes           string   `json:"notes"`
	IsFavorite      bool     `json:"is_favorite"`
	Tags            []string `json:"tags"`
	ImageURLs       []string `json:"image_urls"`
}

type UpdateExamRequest struct {
	PatientName     *string   `json:"patient_name"`
	PatientDocument *string   `json:"patient_document"`
	EmployerName    *string   `json:"employer_name"`
	ClientName      *string   `json:"client_name"`
	PaymentMethod   *string   `json:"payment_method"`
	ExamType        *string   `json:"exam_type"`
	Status          *string   `json:"status"`
	ScheduledAt     *string   `json:"scheduled_at"`
	CompletedAt     *string   `json:"completed_at"`
	Result          *string   `json:"result"`
	Notes           *string   `json:"notes"`
	IsFavorite      *bool     `json:"is_favorite"`
	Tags            *[]string `json:"tags"`
	ImageURLs       *[]string `json:"image_urls"`
}
