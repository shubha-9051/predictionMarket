package matching

import "fmt"

type Order struct {
	ID         int64
	Side       string
	Price      int64
	Quantity   int64
	Remaining  int64
	SequenceNo int64
}

type Book struct {
	bids map[int64][]Order
	asks map[int64][]Order
}

type Fill struct {
	IncomingOrderID int64
	RestingOrderID  int64
	Price           int64
	Quantity        int64
}

func NewBook() *Book {
	return &Book{
		bids: make(map[int64][]Order),
		asks: make(map[int64][]Order),
	}
}

func (b *Book) Add(order Order) error {
	switch order.Side {
	case "buy":
		b.bids[order.Price] = append(b.bids[order.Price], order)
	case "sell":
		b.asks[order.Price] = append(b.asks[order.Price], order)
	default:
		return fmt.Errorf("unknown side: %q", order.Side)
	}
	return nil
}

func (b *Book) BestBid() (int64, bool) {
	best := int64(0)
	found := false
	for price := range b.bids {
		if len(b.bids[price]) == 0 {
			continue
		}
		if !found || price > best {
			best = price
			found = true
		}
	}
	return best, found
}

func (b *Book) BestAsk() (int64, bool) {
	best := int64(0)
	found := false
	for price := range b.asks {
		if len(b.asks[price]) == 0 {
			continue
		}
		if !found || price < best {
			best = price
			found = true
		}
	}
	return best, found
}

// matchOnce does ONE match: incoming order vs the single best opposing order.
// It branches on the incoming side, because a buy matches asks and a sell matches bids.
func (b *Book) matchOnce(incoming *Order) (Fill, bool) {
	var restingPrice int64
	var ok bool
	var queue []Order
	var oppositeSide map[int64][]Order

	if incoming.Side == "buy" {
		// buy matches against ASKS; cross when incoming.Price >= best ask
		restingPrice, ok = b.BestAsk()
		if !ok || incoming.Price < restingPrice {
			return Fill{}, false
		}
		oppositeSide = b.asks
	} else {
		// sell matches against BIDS; cross when incoming.Price <= best bid
		restingPrice, ok = b.BestBid()
		if !ok || incoming.Price > restingPrice {
			return Fill{}, false
		}
		oppositeSide = b.bids
	}

	queue = oppositeSide[restingPrice]
	resting := &queue[0] // front of FIFO queue = oldest = time priority

	tradeQty := min(incoming.Remaining, resting.Remaining)

	fill := Fill{
		IncomingOrderID: incoming.ID,
		RestingOrderID:  resting.ID,
		Price:           restingPrice, // trade at the RESTING order's price
		Quantity:        tradeQty,
	}

	incoming.Remaining -= tradeQty
	resting.Remaining -= tradeQty

	if resting.Remaining == 0 {
		oppositeSide[restingPrice] = queue[1:] // FIFO pop
	} else {
		queue[0] = *resting
		oppositeSide[restingPrice] = queue
	}

	return fill, true
}

func (b *Book) Match(incoming Order) []Fill {
	var fills []Fill
	for incoming.Remaining > 0 {
		fill, ok := b.matchOnce(&incoming)
		if !ok {
			break
		}
		fills = append(fills, fill)
	}
	if incoming.Remaining > 0 {
		b.Add(incoming) // rest the unfilled remainder
	}
	return fills
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
