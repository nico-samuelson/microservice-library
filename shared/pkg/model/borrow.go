package model

import (
	"log"
	pb "shared/proto/buffer"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Borrow struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	BookId       primitive.ObjectID `bson:"book_id" json:"book_id" validate:"required"`
	UserId       primitive.ObjectID `bson:"user_id" json:"user_id" validate:"required"`
	CollectionId primitive.ObjectID `bson:"collection_id" json:"collection_id" validate:"required"`
	BorrowDate   time.Time          `bson:"borrow_date" json:"borrow_date" validate:"required"`
	DueDate      *time.Time         `bson:"due_date,omitempty" json:"due_date,omitempty" validate:"required,gtfield=BorrowDate"`
	ReturnDate   *time.Time         `bson:"return_date,omitempty" json:"return_date,omitempty" validate:"omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at" validate:"required"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at" validate:"required"`
}

type BorrowUpdateRequest struct {
	BookId       *primitive.ObjectID `json:"book_id,omitempty" validate:"omitempty,min=1,max=200"`
	UserId       *primitive.ObjectID `json:"user_id,omitempty" validate:"omitempty,min=1,max=200"`
	CollectionId *primitive.ObjectID `json:"collection_id,omitempty" validate:"omitempty,min=1,max=200"`
	BorrowDate   *time.Time          `json:"borrow_date,omitempty" validate:"omitempty"`
	DueDate      *time.Time          `json:"due_date,omitempty" validate:"omitempty,gtfield=BorrowDate"`
	ReturnDate   *time.Time          `json:"return_date,omitempty" validate:"omitempty"`
}

func ToPbBorrow(c *Borrow) *pb.Borrow {
	if c == nil {
		return nil
	}

	var returnDate string
	if c.ReturnDate != nil {
		returnDate = c.ReturnDate.Format(time.RFC3339)
	}

	return &pb.Borrow{
		Id:           c.Id.Hex(),
		BookId:       c.BookId.Hex(),
		UserId:       c.UserId.Hex(),
		CollectionId: c.CollectionId.Hex(),
		BorrowDate:   c.BorrowDate.Format(time.RFC3339),
		DueDate:      c.DueDate.Format(time.RFC3339),
		ReturnDate:   returnDate,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    c.UpdatedAt.Format(time.RFC3339),
	}
}

func FromPbBorrow(p *pb.Borrow) *Borrow {
	if p == nil {
		return nil
	}

	// log.Println(p)

	objId, err := primitive.ObjectIDFromHex(p.Id)
	if err != nil {
		log.Printf("Failed to convert ID from hex: %v", err)
		return nil
	}

	bookId, _ := primitive.ObjectIDFromHex(p.BookId)
	userId, _ := primitive.ObjectIDFromHex(p.UserId)
	collectionId, _ := primitive.ObjectIDFromHex(p.CollectionId)

	borrowDate, err := time.Parse(time.RFC3339, p.BorrowDate)
	if err != nil {
		log.Printf("Failed to parse borrow date: %v", err)
		return nil
	}

	dueDate, err := time.Parse(time.RFC3339, p.DueDate)
	if err != nil {
		log.Printf("Failed to parse due date: %v", err)
		return nil
	}

	var returnDate time.Time
	if p.ReturnDate != "" {
		returnDate, _ = time.Parse(time.RFC3339, p.ReturnDate)
	}

	createdAt, err := time.Parse(time.RFC3339, p.CreatedAt)
	if err != nil {
		log.Printf("Failed to parse created at date: %v", err)
		return nil
	}

	updatedAt, err := time.Parse(time.RFC3339, p.UpdatedAt)
	if err != nil {
		log.Printf("Failed to parse updated at date: %v", err)
		return nil
	}

	return &Borrow{
		Id:           objId,
		BookId:       bookId,
		UserId:       userId,
		CollectionId: collectionId,
		BorrowDate:   borrowDate,
		DueDate:      &dueDate,
		ReturnDate:   &returnDate,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func FromPbBorrows(pBorrows []*pb.Borrow) []*Borrow {
	var borrows []*Borrow
	for _, p := range pBorrows {
		if borrow := FromPbBorrow(p); borrow != nil {
			borrows = append(borrows, borrow)
		}
	}
	return borrows
}

func ToPbBorrows(cBorrows []*Borrow) []*pb.Borrow {
	var pBorrows []*pb.Borrow
	for _, c := range cBorrows {
		if p := ToPbBorrow(c); p != nil {
			pBorrows = append(pBorrows, p)
		}
	}
	return pBorrows
}
