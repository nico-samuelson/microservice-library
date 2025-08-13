package handler

import (
	"log"
	"shared/pkg/model"
	pb "shared/proto/buffer"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type BookHandler struct {
	client pb.BookServiceClient
}

func NewBookHandler(conn *grpc.ClientConn) *BookHandler {
	return &BookHandler{
		client: pb.NewBookServiceClient(conn),
	}
}

func (h *BookHandler) GetBook(c *gin.Context) {
	request := pb.GetBookRequest{}
	response, err := h.client.GetBook(c, &request)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	books := model.FromPbBooks(response.Book)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{books}))
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
