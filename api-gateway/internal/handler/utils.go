package handler

import (
	"shared/pkg/model"

	"google.golang.org/grpc/status"
)

func BuildHttpResponse(success bool, code int, message string, data []interface{}) model.HttpResponse {
	return model.HttpResponse{
		Success: success,
		Code:    code,
		Message: message,
		Data:    data,
	}
}

func ExtractErrorMessage(err error) string {
	st, ok := status.FromError(err)

	if !ok {
		return "Internal Server Error"
	}

	return st.Message()
}
