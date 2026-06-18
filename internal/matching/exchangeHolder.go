package matching

import "sync"

type Exchange struct {
	mu    sync.Mutex
	books map[int64]*Book
}

func NewExchange() *Exchange {
	return &Exchange{
		books: make(map[int64]*Book),
	}
}

func (e *Exchange) BookFor(marketID int64) *Book {
	e.mu.Lock()
	defer e.mu.Unlock()

	book, exists := e.books[marketID]
	if !exists {
		book = NewBook()
		e.books[marketID] = book
	}
	return book
}
