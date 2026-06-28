package permission_test

import (
	"testing"

	"element-skin/backend/internal/permission"
)

func TestBitSetBooleanOperationsExactly(t *testing.T) {
	left := permission.NewBitSet(130)
	left.Set(0)
	left.Set(65)
	left.Set(129)
	right := permission.NewBitSet(130)
	right.Set(65)
	right.Set(128)

	and := left.And(right)
	if and.Has(0) || !and.Has(65) || and.Has(129) || and.Has(128) {
		t.Fatalf("AND mismatch: %#v", []uint64(and))
	}

	or := left.Or(right)
	if !or.Has(0) || !or.Has(65) || !or.Has(128) || !or.Has(129) {
		t.Fatalf("OR mismatch: %#v", []uint64(or))
	}

	andNot := left.AndNot(right)
	if !andNot.Has(0) || andNot.Has(65) || andNot.Has(128) || !andNot.Has(129) {
		t.Fatalf("AND NOT mismatch: %#v", []uint64(andNot))
	}
}
