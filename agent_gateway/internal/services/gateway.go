package services

type GatewayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *GatewayError) Error() string {
	return e.Message
}
