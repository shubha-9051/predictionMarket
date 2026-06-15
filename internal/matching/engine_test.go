package matching

import "testing"

// helper: build an order with Remaining set to Quantity (so we can't forget it)
func ord(id int64, side string, price, qty int64) Order {
	return Order{ID: id, Side: side, Price: price, Quantity: qty, Remaining: qty}
}

// Case 1: incoming buy fully matches a single resting ask.
func TestFullMatchSingle(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "sell", 550, 60)) // resting ask: 60 @ 550

	fills := b.Match(ord(2, "buy", 600, 60)) // incoming buy: 60 @ 600

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price != 550 || fills[0].Quantity != 60 {
		t.Fatalf("expected fill 60@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	// the ask should be gone from the book
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left, but BestAsk found one")
	}
}

// Case 2: incoming buy is larger than the resting ask — partial fill of INCOMING.
// Leftover should rest as a bid.
func TestPartialFillIncoming(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "sell", 550, 40)) // ask: 40 @ 550

	fills := b.Match(ord(2, "buy", 600, 100)) // buy: 100 @ 600

	if len(fills) != 1 || fills[0].Quantity != 40 {
		t.Fatalf("expected 1 fill of 40, got %+v", fills)
	}
	// ask gone
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left")
	}
	// incoming's leftover 60 should rest as a bid at its OWN price, 600
	bid, ok := b.BestBid()
	if !ok || bid != 600 {
		t.Fatalf("expected leftover bid at 600, got bid=%d ok=%v", bid, ok)
	}
	if b.bids[600][0].Remaining != 60 {
		t.Fatalf("expected resting bid remaining 60, got %d", b.bids[600][0].Remaining)
	}
}

// Case 3: incoming buy is smaller than the resting ask — partial fill of RESTING.
// The ask stays on the book with reduced quantity.
func TestPartialFillResting(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "sell", 550, 100)) // ask: 100 @ 550

	fills := b.Match(ord(2, "buy", 600, 60)) // buy: 60 @ 600

	if len(fills) != 1 || fills[0].Quantity != 60 {
		t.Fatalf("expected 1 fill of 60, got %+v", fills)
	}
	// ask should STILL be there, with 40 remaining
	ask, ok := b.BestAsk()
	if !ok || ask != 550 {
		t.Fatalf("expected ask still at 550, got ask=%d ok=%v", ask, ok)
	}
	if b.asks[550][0].Remaining != 40 {
		t.Fatalf("expected resting ask remaining 40, got %d", b.asks[550][0].Remaining)
	}
}

// Case 4: incoming buy sweeps TWO price levels. The hard one.
func TestMultiLevelSweep(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "sell", 550, 40)) // best ask
	b.Add(ord(2, "sell", 560, 40)) // next level

	fills := b.Match(ord(3, "buy", 600, 100)) // buy 100, crosses both

	if len(fills) != 2 {
		t.Fatalf("expected 2 fills, got %d: %+v", len(fills), fills)
	}
	// first fill should be the cheaper level (price priority), then the next
	if fills[0].Price != 550 || fills[0].Quantity != 40 {
		t.Fatalf("fill[0] expected 40@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	if fills[1].Price != 560 || fills[1].Quantity != 40 {
		t.Fatalf("fill[1] expected 40@560, got %d@%d", fills[1].Quantity, fills[1].Price)
	}
	// both asks consumed
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left after sweep")
	}
	// leftover 20 rests as a bid at 600
	if bid, ok := b.BestBid(); !ok || bid != 600 || b.bids[600][0].Remaining != 20 {
		t.Fatalf("expected leftover bid 20@600, got bid=%d ok=%v", bid, ok)
	}
}

// Case 5: no cross — incoming buy too cheap. Nothing trades; it rests.
func TestNoCross(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "sell", 560, 50)) // ask at 560

	fills := b.Match(ord(2, "buy", 540, 50)) // buy at 540 — below the ask

	if len(fills) != 0 {
		t.Fatalf("expected 0 fills, got %d", len(fills))
	}
	// ask untouched
	if ask, ok := b.BestAsk(); !ok || ask != 560 || b.asks[560][0].Remaining != 50 {
		t.Fatalf("expected ask untouched at 560 qty 50")
	}
	// incoming rests as a bid at 540
	if bid, ok := b.BestBid(); !ok || bid != 540 {
		t.Fatalf("expected incoming to rest as bid at 540, got %d", bid)
	}
}

// Case 6: sell-side mirror — proves the sell branch of matchOnce works.
func TestSellSideMirror(t *testing.T) {
	b := NewBook()
	b.Add(ord(1, "buy", 550, 60)) // resting bid: 60 @ 550

	fills := b.Match(ord(2, "sell", 500, 60)) // incoming sell: 60 @ 500 (crosses, 500 <= 550)

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	// trade at the RESTING bid's price, 550
	if fills[0].Price != 550 || fills[0].Quantity != 60 {
		t.Fatalf("expected 60@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	// bid consumed
	if _, ok := b.BestBid(); ok {
		t.Fatalf("expected no bids left")
	}
}
