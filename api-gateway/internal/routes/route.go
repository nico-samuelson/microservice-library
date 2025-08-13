package routes

import (
	"apigateway/internal/handler"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

func SetupRoutes(connections map[string]*grpc.ClientConn) *gin.Engine {
	collectionHandler := handler.NewCollectionHandler(connections["collection"])
	bookHandler := handler.NewBookHandler(connections["book"])
	borrowHandler := handler.NewBorrowHandler(connections["borrow"])

	router := gin.Default()

	// router.POST("/login", handler.GetBooks)
	// router.POST("/register", handler.GetBooks)

	// Define routes for collections
	router.GET("/collections", collectionHandler.GetCollection)
	router.GET("/collections/:id", collectionHandler.GetCollectionById)
	router.POST("/collections", collectionHandler.CreateCollection)
	router.PUT("/collections/:id", collectionHandler.UpdateCollection)
	router.DELETE("/collections/:id", collectionHandler.DeleteCollection)

	router.GET("/books", bookHandler.GetBook)
	router.GET("/books/:id", bookHandler.GetBookById)
	router.POST("/books", bookHandler.CreateBook)
	router.PUT("/books/:id", bookHandler.UpdateBook)
	router.DELETE("/books/:id", bookHandler.DeleteBook)

	router.POST("/borrow", borrowHandler.BorrowBook)
	router.POST("/return", borrowHandler.ReturnBook)
	// router.GET("/borrow-history/user/:id", handler.GetBookById)
	// router.GET("/borrow-history/book/:id", handler.GetBookById)

	// Additional routes can be added here as needed

	return router
}
