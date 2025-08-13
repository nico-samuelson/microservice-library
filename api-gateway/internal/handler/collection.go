package handler

import (
	"log"
	"shared/pkg/model"
	pb "shared/proto/buffer"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type CollectionHandler struct {
	client pb.CollectionServiceClient
}

func NewCollectionHandler(conn *grpc.ClientConn) *CollectionHandler {
	return &CollectionHandler{
		client: pb.NewCollectionServiceClient(conn),
	}
}

func (h *CollectionHandler) GetCollection(c *gin.Context) {
	request := pb.GetCollectionRequest{}
	response, err := h.client.GetCollection(c, &request)

	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{response.Collection}))
}

func (h *CollectionHandler) GetCollectionById(c *gin.Context) {
	id, ok := c.Params.Get("id")

	if !ok {
		log.Println("Id not specified in request params")
		c.JSON(500, BuildHttpResponse(false, 500, "ID Not Specified", []interface{}{}))
		return
	}
	request := pb.FindCollectionRequest{Id: id}
	response, err := h.client.FindCollectionById(c, &request)

	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{response.Collection}))
}

func (h *CollectionHandler) CreateCollection(c *gin.Context) {
	var collection pb.Collection
	if err := c.BindJSON(&collection); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	request := pb.AddCollectionRequest{Collection: &collection}
	response, err := h.client.AddCollection(c, &request)

	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{response.Collection}))
}

func (h *CollectionHandler) UpdateCollection(c *gin.Context) {
	id, ok := c.Params.Get("id")
	if !ok {
		log.Println("Id not specified in request params")
		c.JSON(500, BuildHttpResponse(false, 500, "ID Not Specified", []interface{}{}))
		return
	}
	var collection map[string]interface{}
	if err := c.BindJSON(&collection); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	structPayload, err := structpb.NewStruct(collection)
	if err != nil {
		log.Printf("Error creating struct: %s", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	request := pb.UpdateCollectionRequest{
		Payload: structPayload,
		Id:      id,
	}
	response, err := h.client.UpdateCollection(c, &request)
	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{response.Collection}))
}

func (h *CollectionHandler) DeleteCollection(c *gin.Context) {
	id, ok := c.Params.Get("id")
	if !ok {
		log.Println("Id not specified in request params")
		c.JSON(500, model.HttpResponse{
			Success: false,
			Code:    500,
			Data:    []interface{}{},
			Message: "ID not specified",
		})
		return
	}
	request := pb.DeleteCollectionRequest{Id: id}
	response, err := h.client.DeleteCollection(c, &request)

	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{response.Collection}))
}
