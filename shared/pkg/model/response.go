package model

type HttpResponse struct {
	Success bool          `json:"success"`
	Code    int           `json:"code"`
	Data    []interface{} `json:"data"`
	Message string        `json:"message"`
}

type GrpcResponse struct {
	Success bool
	Data    []interface{}
	Message string
}
