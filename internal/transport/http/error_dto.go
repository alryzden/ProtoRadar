package httptransport

type errorResponse struct {
	Error apiErrorDTO `json:"error"`
}

type apiErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}
