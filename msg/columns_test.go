package msg

import "testing"

func TestColumnNumberInt(t *testing.T) {
	c := ICAT_COLUMN_DATA_NAME

	if c.Int() != 403 {
		t.Errorf("Int() = %d, want 403", c.Int())
	}

	if c.AggregationLevel() != 1 {
		t.Errorf("AggregationLevel() = %d, want 1", c.AggregationLevel())
	}
}
