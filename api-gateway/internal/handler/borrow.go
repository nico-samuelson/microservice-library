package handler

import (
	"shared/pkg/model"
	pb "shared/proto/buffer"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type BorrowHandler struct {
	client pb.BorrowServiceClient
}

func NewBorrowHandler(conn *grpc.ClientConn) *BorrowHandler {
	return &BorrowHandler{
		client: pb.NewBorrowServiceClient(conn),
	}
}

func (h *BorrowHandler) BorrowBook(c *gin.Context) {
	var borrowRequest pb.BorrowRequest
	if err := c.BindJSON(&borrowRequest); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	response, err := h.client.BorrowBook(c, &borrowRequest)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	borrows := model.FromPbBorrows(response.Borrow)
	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{borrows}))
}

func (h *BorrowHandler) ReturnBook(c *gin.Context) {
	var returnRequest pb.ReturnRequest
	if err := c.BindJSON(&returnRequest); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	response, err := h.client.ReturnBook(c, &returnRequest)
	if err != nil {
		c.JSON(500, BuildHttpResponse(false, 500, ExtractErrorMessage(err), []interface{}{}))
		return
	}

	c.JSON(200, BuildHttpResponse(true, 200, response.Message, []interface{}{model.FromPbBorrows(response.Borrow)}))
}
