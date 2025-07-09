package aptosstream

import "testing"

func TestWithTransactionsCountZero(t *testing.T) {
	var c Client
	opt := WithTransactionsCount(0)
	opt(&c)
	if c.transactionsCount != nil {
		t.Errorf("expected transactionsCount to be nil, got %v", c.transactionsCount)
	}
}

func TestWithTransactionsCountNonZero(t *testing.T) {
	var c Client
	count := uint64(5)
	opt := WithTransactionsCount(count)
	opt(&c)
	if c.transactionsCount == nil || *c.transactionsCount != count {
		t.Errorf("expected transactionsCount to be %v, got %v", count, c.transactionsCount)
	}
}
