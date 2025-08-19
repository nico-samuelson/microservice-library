package handler

import (
	"log"
	"shared/pkg/model"
	pb "shared/proto/buffer"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type QueryParams struct {
	Filter bson.M
	Sort   *bson.D
	Skip   int
	Limit  int
}

// Extracts and validates query parameters from the request
func ParseQueryParams(c *gin.Context) QueryParams {
	params := QueryParams{
		Filter: bson.M{},
		Skip:   0,
		Limit:  10,
	}

	// Parse pagination
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			params.Skip = (page - 1) * params.Limit
		}
	}

	if skipStr := c.Query("skip"); skipStr != "" {
		if skip, err := strconv.Atoi(skipStr); err == nil && skip >= 0 {
			params.Skip = skip
		}
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit <= 100 {
			params.Limit = limit
		}
	}

	// Parse filters - expecting format: ?filter[field]=value&filter[status]=active
	for key, values := range c.Request.URL.Query() {
		if strings.HasPrefix(key, "filter[") && strings.HasSuffix(key, "]") {
			fieldName := strings.TrimSuffix(strings.TrimPrefix(key, "filter["), "]")
			if len(values) > 0 && values[0] != "" {
				params.Filter[fieldName] = values[0]
			}
		}
	}

	// Parse sorting - expecting format: ?sort=field1,-field2 (- for desc)
	if sortStr := c.Query("sort"); sortStr != "" {
		sortFields := strings.Split(sortStr, ",")
		sortDoc := bson.D{}

		for _, field := range sortFields {
			field = strings.TrimSpace(field)
			if field != "" {
				if strings.HasPrefix(field, "-") {
					sortDoc = append(sortDoc, bson.E{Key: strings.TrimPrefix(field, "-"), Value: -1})
				} else {
					sortDoc = append(sortDoc, bson.E{Key: field, Value: 1})
				}
			}
		}

		if len(sortDoc) > 0 {
			params.Sort = &sortDoc
		}
	}

	return params
}

func BuildFilterAndSort(params QueryParams) (*structpb.Struct, []*pb.Sort) {
	filter, err := structpb.NewStruct(params.Filter)
	if err != nil {
		log.Printf("Error parsing filter params: %v", err)
		return nil, nil
	}

	var sorts []*pb.Sort
	for _, sort := range *params.Sort {
		direction, ok := sort.Value.(int)
		if !ok {
			log.Printf("Can't convert element to int: %v", direction)
			return filter, nil
		}

		sorts = append(sorts, &pb.Sort{
			Key:       sort.Key,
			Direction: int32(direction),
		})
	}

	return filter, sorts
}

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
