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

type BookHandler struct {
	client  pb.BookServiceClient
	batcher ReqBatcherInterface[pb.BookServiceClient, pb.BookResponse]
}

func NewBookHandler(conn *grpc.ClientConn) *BookHandler {
	return &BookHandler{
		client: pb.NewBookServiceClient(conn),
	}
}

func NewBookHandlerWithBatching(conn *grpc.ClientConn, batchWindow time.Duration) *BookHandler {
	client := pb.NewBookServiceClient(conn)
	return &BookHandler{
		client:  client,
		batcher: NewBookReqBatcher(client, batchWindow),
	}
}

// GrpcBatcher handles batching for gRPC calls
type BookReqBatcher struct {
	baseBatcher *ReqBatcher[pb.BookServiceClient, pb.BookResponse]
}

func NewBookReqBatcher(client pb.BookServiceClient, batchWindow time.Duration) *BookReqBatcher {
	return &BookReqBatcher{
		baseBatcher: NewReqBatcher[pb.BookServiceClient, pb.BookResponse](client, batchWindow),
	}
}

// BatchingMiddleware returns middleware function for this handler
func (h *BookHandler) BatchingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.batcher != nil {
			c.Set("book_batcher", h.batcher)
		}
		c.Next()
	}
}

func (h *BookHandler) GetBook(c *gin.Context) {
	params := ParseQueryParams(c)
	filter, sort := BuildFilterAndSort(params)
	request := pb.GetBookRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   int32(params.Skip),
		Limit:  int32(params.Limit),
	}

	response, err := h.client.GetBook(c, &request)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
}

func (h *BookHandler) GetBookBatch(c *gin.Context) {
	params := ParseQueryParams(c)

	if h.batcher != nil {
		// Use batcher for multiple requests
		response, err := h.batcher.GetBatch(c.Request.Context(), params)
		if err != nil {
			message := ExtractErrorMessage(err)
			c.JSON(500, BuildHttpResponse(false, 500, message, []interface{}{}))
			return
		}
		c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{model.FromPbBooks(response.Book)}))
	} else {
		h.GetBook(c)
	}
}

func (b *BookReqBatcher) GetBatch(ctx context.Context, params QueryParams) (*pb.BookResponse, error) {
	req := &BatchRequest[pb.BookResponse]{
		ctx:  ctx,
		resp: make(chan *pb.BookResponse, 1),
		err:  make(chan error, 1),
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

func (b *BookReqBatcher) flush() {
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
	request := pb.GetBookRequest{
		Filter: filter,
		Sort:   sort,
		Skip:   int32(params.Skip),
		Limit:  int32(params.Limit),
	}

	// Make a single backend call for all pending requests
	resp, err := b.baseBatcher.client.GetBook(context.Background(), &request)
	for _, req := range pending {
		log.Printf("Flushing batch with %d requests", len(pending))
		if err != nil {
			req.err <- err
		} else {
			req.resp <- resp
		}
	}
}

func (h *BookHandler) GetBookById(c *gin.Context) {
	id, ok := c.Params.Get("id")

	if !ok {
		log.Println("Id not specified in request params")
		c.JSON(500, BuildHttpResponse(false, 500, "ID Not Specified", []interface{}{}))
		return
	}
	request := pb.FindBookRequest{Id: id}
	response, err := h.client.FindBookById(c, &request)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
}

func (h *BookHandler) CreateBook(c *gin.Context) {
	var book model.Book
	if err := c.BindJSON(&book); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	pbBook := model.ToPbBook(&book)
	pbBook.CollectionId = book.CollectionId.Hex()
	request := pb.AddBookRequest{Book: pbBook}
	response, err := h.client.AddBook(c, &request)

	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
}

func (h *BookHandler) UpdateBook(c *gin.Context) {
	id, ok := c.Params.Get("id")
	if !ok {
		log.Println("Id not specified in request params")
		c.JSON(500, BuildHttpResponse(false, 500, "ID Not Specified", []interface{}{}))
		return
	}

	var book map[string]interface{}
	if err := c.BindJSON(&book); err != nil {
		log.Printf("Error binding json: %s", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	structPayload, err := structpb.NewStruct(book)
	if err != nil {
		log.Printf("Error creating struct: %s", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// pbBook := model.ToPbBook(&book)
	request := pb.UpdateBookRequest{
		Payload: structPayload,
		Id:      id,
	}
	response, err := h.client.UpdateBook(c, &request)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
}

func (h *BookHandler) DeleteBook(c *gin.Context) {
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
	request := pb.DeleteBookRequest{Id: id}
	response, err := h.client.DeleteBook(c, &request)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
}
