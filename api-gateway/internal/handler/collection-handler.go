package handler

import (
	"context"
	"log"
	"shared/pkg/model"
	pb "shared/proto/buffer"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

// CollectionHandler with batching support
type CollectionHandler struct {
	client  pb.CollectionServiceClient
	batcher ReqBatcherInterface[pb.CollectionServiceClient, pb.Response]
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
	baseBatcher *ReqBatcher[pb.CollectionServiceClient, pb.Response]
}

// NewGrpcBatcher creates a new gRPC batcher
func NewGrpcBatcher(client pb.CollectionServiceClient, batchWindow time.Duration) *CollectionReqBatcher {
	return &CollectionReqBatcher{
		baseBatcher: NewReqBatcher[pb.CollectionServiceClient, pb.Response](
			client,
			batchWindow,
		),
	}
}

func (b *CollectionReqBatcher) GetBatch(ctx context.Context, params QueryParams) (*pb.Response, error) {
	req := &BatchRequest[pb.Response]{
		ctx:    ctx,
		params: params,
		resp:   make(chan *pb.Response, 1),
		err:    make(chan error, 1),
	}

	b.baseBatcher.mu.Lock()
	b.baseBatcher.pending = append(b.baseBatcher.pending, req)
	if b.baseBatcher.timer == nil {
		b.baseBatcher.timer = time.AfterFunc(b.baseBatcher.batchWindow, b.flush)
	}
	b.baseBatcher.mu.Unlock()

	select {
	case r := <-req.resp:
		return r, nil
	case e := <-req.err:
		return nil, e
	}
}

func (b *CollectionReqBatcher) flush() {
	b.baseBatcher.mu.Lock()
	pending := b.baseBatcher.pending
	b.baseBatcher.pending = nil
	b.baseBatcher.timer = nil
	b.baseBatcher.mu.Unlock()

	var params QueryParams
	if len(pending) > 0 {
		params = pending[0].params
	}
	filter, sort := BuildFilterAndSort(params)
	request := pb.GetCollectionRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   int32(params.Skip),
		Limit:  int32(params.Limit),
	}

	// Make a single backend call for all pending requests
	resp, err := b.baseBatcher.client.GetCollection(context.Background(), &request)
	for _, req := range pending {
		if err != nil {
			req.err <- err
		} else {
			req.resp <- resp
		}
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
	params := ParseQueryParams(c)
	filter, sort := BuildFilterAndSort(params)
	request := pb.GetCollectionRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   int32(params.Skip),
		Limit:  int32(params.Limit),
	}

	response, err := h.client.GetCollection(c, &request)
	if err != nil {
		message := ExtractErrorMessage(err)
		c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
		return
	}

	collections := model.FromPbCollections(response.Collection)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{collections}))
}

func (h *CollectionHandler) GetCollectionBatch(c *gin.Context) {
	params := ParseQueryParams(c)

	if h.batcher != nil {
		// Use batcher for multiple requests
		response, err := h.batcher.GetBatch(c.Request.Context(), params)
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
