package matching

import "testing"

func mkOrder(id int64, outcome, side string, price, qty int64) Order {
	o := Order{
		ID:        id,
		Outcome:   outcome,
		Side:      side,
		Price:     price,
		Quantity:  qty,
		Remaining: qty,
	}
	Normalize(&o)
	return o
}

func TestFullMatchSingle(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 550, 60))

	fills := b.Match(mkOrder(2, "YES", "buy", 600, 60))

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price != 550 || fills[0].Quantity != 60 {
		t.Fatalf("expected 60@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left")
	}
}

func TestPartialFillIncoming(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 550, 40))

	fills := b.Match(mkOrder(2, "YES", "buy", 600, 100))

	if len(fills) != 1 || fills[0].Quantity != 40 {
		t.Fatalf("expected 1 fill of 40, got %+v", fills)
	}
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left")
	}
	bid, ok := b.BestBid()
	if !ok || bid != 600 {
		t.Fatalf("expected leftover bid at 600, got %d ok=%v", bid, ok)
	}
	if b.bids[600][0].Remaining != 60 {
		t.Fatalf("expected resting bid remaining 60, got %d", b.bids[600][0].Remaining)
	}
}

func TestPartialFillResting(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 550, 100))

	fills := b.Match(mkOrder(2, "YES", "buy", 600, 60))

	if len(fills) != 1 || fills[0].Quantity != 60 {
		t.Fatalf("expected 1 fill of 60, got %+v", fills)
	}
	ask, ok := b.BestAsk()
	if !ok || ask != 550 {
		t.Fatalf("expected ask still at 550, got %d ok=%v", ask, ok)
	}
	if b.asks[550][0].Remaining != 40 {
		t.Fatalf("expected resting ask remaining 40, got %d", b.asks[550][0].Remaining)
	}
}

func TestMultiLevelSweep(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 550, 40))
	b.Add(mkOrder(2, "YES", "sell", 560, 40))

	fills := b.Match(mkOrder(3, "YES", "buy", 600, 100))

	if len(fills) != 2 {
		t.Fatalf("expected 2 fills, got %d: %+v", len(fills), fills)
	}
	if fills[0].Price != 550 || fills[0].Quantity != 40 {
		t.Fatalf("fill[0] expected 40@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	if fills[1].Price != 560 || fills[1].Quantity != 40 {
		t.Fatalf("fill[1] expected 40@560, got %d@%d", fills[1].Quantity, fills[1].Price)
	}
	if _, ok := b.BestAsk(); ok {
		t.Fatalf("expected no asks left after sweep")
	}
	if bid, ok := b.BestBid(); !ok || bid != 600 || b.bids[600][0].Remaining != 20 {
		t.Fatalf("expected leftover bid 20@600, got %d ok=%v", bid, ok)
	}
}

func TestNoCross(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 560, 50))

	fills := b.Match(mkOrder(2, "YES", "buy", 540, 50))

	if len(fills) != 0 {
		t.Fatalf("expected 0 fills, got %d", len(fills))
	}
	if ask, ok := b.BestAsk(); !ok || ask != 560 || b.asks[560][0].Remaining != 50 {
		t.Fatalf("expected ask untouched at 560 qty 50")
	}
	if bid, ok := b.BestBid(); !ok || bid != 540 {
		t.Fatalf("expected incoming to rest as bid at 540, got %d", bid)
	}
}

func TestSellSideMirror(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "buy", 550, 60))

	fills := b.Match(mkOrder(2, "YES", "sell", 500, 60))

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Price != 550 || fills[0].Quantity != 60 {
		t.Fatalf("expected 60@550, got %d@%d", fills[0].Quantity, fills[0].Price)
	}
	if _, ok := b.BestBid(); ok {
		t.Fatalf("expected no bids left")
	}
}

func TestMintMatch(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "NO", "buy", 400, 10))

	fills := b.Match(mkOrder(2, "YES", "buy", 600, 10))

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Type != "mint" {
		t.Errorf("expected fill type 'mint', got %q", fills[0].Type)
	}
	if fills[0].Quantity != 10 {
		t.Errorf("expected quantity 10, got %d", fills[0].Quantity)
	}
	if fills[0].Price != 600 {
		t.Errorf("expected norm price 600, got %d", fills[0].Price)
	}
}

func TestTransferMatchStillWorks(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "YES", "sell", 550, 10))

	fills := b.Match(mkOrder(2, "YES", "buy", 600, 10))

	if len(fills) != 1 {
		t.Fatalf("expected 1 fill, got %d", len(fills))
	}
	if fills[0].Type != "transfer" {
		t.Errorf("expected fill type 'transfer', got %q", fills[0].Type)
	}
}

func TestMintNoCross(t *testing.T) {
	b := NewBook()
	b.Add(mkOrder(1, "NO", "buy", 400, 10))

	fills := b.Match(mkOrder(2, "YES", "buy", 500, 10))

	if len(fills) != 0 {
		t.Fatalf("expected 0 fills, got %d", len(fills))
	}
}
