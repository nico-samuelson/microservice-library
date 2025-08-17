package handler

import (
	"context"
	"log"
	"shared/pkg/model"
	pb "shared/proto/buffer"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// CollectionHandler with batching support
type CollectionHandler struct {
	client  pb.CollectionServiceClient
	batcher *CollectionReqBatcher
}

func NewCollectionHandler(conn *grpc.ClientConn) *CollectionHandler {
	return &CollectionHandler{
		client: pb.NewCollectionServiceClient(conn),
	}
}

func NewCollectionHandlerWithBatching(conn *grpc.ClientConn, batchWindow time.Duration) *CollectionHandler {
	client := pb.NewCollectionServiceClient(conn)
	return &CollectionHandler{
		client:  client,
		batcher: NewGrpcBatcher(client, batchWindow),
	}
}

// GrpcBatcher handles batching for gRPC calls
type CollectionReqBatcher struct {
	client      pb.CollectionServiceClient
	batchWindow time.Duration
	mu          sync.Mutex
	pending     []*CollectionBatchRequest
	timer       *time.Timer
}

type CollectionBatchRequest struct {
	ctx  context.Context
	resp chan *pb.Response
	err  chan error
}

// NewGrpcBatcher creates a new gRPC batcher
func NewGrpcBatcher(client pb.CollectionServiceClient, batchWindow time.Duration) *CollectionReqBatcher {
	return &CollectionReqBatcher{
		client:      client,
		batchWindow: batchWindow,
		pending:     []*CollectionBatchRequest{},
	}
}

// BatchingMiddleware returns middleware function for this handler
func (h *CollectionHandler) BatchingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.batcher != nil {
			c.Set("collection_batcher", h.batcher)
		}
		c.Next()
	}
}

// GetCollection gets all collections with pagination and caching
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

func (h *CollectionHandler) GetCollectionBatch(c *gin.Context) {
	if h.batcher != nil {
		// Use batcher for multiple requests
		response, err := h.batcher.GetCollectionsBatch(c.Request.Context())
		if err != nil {
			message := ExtractErrorMessage(err)
			c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
			return
		}
		c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{model.FromPbCollections(response.Collection)}))
	} else {
		h.GetCollection(c)
	}
}

func (b *CollectionReqBatcher) GetCollectionsBatch(ctx context.Context) (*pb.Response, error) {
	req := &CollectionBatchRequest{
		ctx:  ctx,
		resp: make(chan *pb.Response, 1),
		err:  make(chan error, 1),
	}

	b.mu.Lock()
	b.pending = append(b.pending, req)
	if b.timer == nil {
		b.timer = time.AfterFunc(b.batchWindow, b.flush)
	}
	b.mu.Unlock()

	select {
	case r := <-req.resp:
		return r, nil
	case e := <-req.err:
		return nil, e
	}
}

func (b *CollectionReqBatcher) flush() {
	b.mu.Lock()
	pending := b.pending
	b.pending = nil
	b.timer = nil
	b.mu.Unlock()

	// Make a single backend call for all pending requests
	resp, err := b.client.GetCollection(context.Background(), &pb.GetCollectionRequest{})
	for _, req := range pending {
		if err != nil {
			req.err <- err
		} else {
			req.resp <- resp
		}
	}
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
