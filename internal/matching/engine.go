package matching

import "fmt"

type Order struct {
	ID         int64
	UserID     int64
	Outcome    string
	Side       string
	Price      int64
	Quantity   int64
	Remaining  int64
	SequenceNo int64

	NormSide  string
	NormPrice int64
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
	Type            string
}

const PairTotal = 1000

func Normalize(o *Order) {
	switch {
	case o.Outcome == "YES" && o.Side == "buy":
		o.NormSide = "buy"
		o.NormPrice = o.Price

	case o.Outcome == "YES" && o.Side == "sell":
		o.NormSide = "sell"
		o.NormPrice = o.Price

	case o.Outcome == "NO" && o.Side == "buy":
		o.NormSide = "sell"
		o.NormPrice = PairTotal - o.Price

	case o.Outcome == "NO" && o.Side == "sell":
		o.NormSide = "buy"
		o.NormPrice = PairTotal - o.Price
	}
}

func IsMint(incoming, resting *Order) bool {
	if incoming.Side == "buy" && resting.Side == "buy" &&
		incoming.Outcome != resting.Outcome {
		return true
	}
	return false
}

func NewBook() *Book {
	return &Book{
		bids: make(map[int64][]Order),
		asks: make(map[int64][]Order),
	}
}

func (b *Book) Add(order Order) error {
	switch order.NormSide {
	case "buy":
		b.bids[order.NormPrice] = append(b.bids[order.NormPrice], order)
	case "sell":
		b.asks[order.NormPrice] = append(b.asks[order.NormPrice], order)
	default:
		return fmt.Errorf("unknown norm side: %q", order.NormSide)
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

func (b *Book) matchOnce(incoming *Order) (Fill, bool) {
	var restingPrice int64
	var ok bool
	var queue []Order
	var oppositeSide map[int64][]Order

	if incoming.NormSide == "buy" {
		restingPrice, ok = b.BestAsk()
		if !ok || incoming.NormPrice < restingPrice {
			return Fill{}, false
		}
		oppositeSide = b.asks
	} else {
		restingPrice, ok = b.BestBid()
		if !ok || incoming.NormPrice > restingPrice {
			return Fill{}, false
		}
		oppositeSide = b.bids
	}

	queue = oppositeSide[restingPrice]
	resting := &queue[0]

	tradeQty := min(incoming.Remaining, resting.Remaining)

	fillType := "transfer"
	if IsMint(incoming, resting) {
		fillType = "mint"
	}
	fill := Fill{
		IncomingOrderID: incoming.ID,
		RestingOrderID:  resting.ID,
		Price:           restingPrice,
		Quantity:        tradeQty,
		Type:            fillType,
	}

	incoming.Remaining -= tradeQty
	resting.Remaining -= tradeQty

	if resting.Remaining == 0 {
		oppositeSide[restingPrice] = queue[1:]
	} else {
		queue[0] = *resting
		oppositeSide[restingPrice] = queue
	}

	return fill, true
}

func (b *Book) Match(incoming Order) []Fill {
	Normalize(&incoming)
	var fills []Fill
	for incoming.Remaining > 0 {
		fill, ok := b.matchOnce(&incoming)
		if !ok {
			break
		}
		fills = append(fills, fill)
	}
	if incoming.Remaining > 0 {
		b.Add(incoming)
	}
	return fills
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
