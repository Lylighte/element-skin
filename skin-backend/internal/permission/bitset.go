package permission

type BitSet []uint64

func NewBitSet(bitCount int) BitSet {
	if bitCount <= 0 {
		return nil
	}
	return make(BitSet, (bitCount+63)/64)
}

func (b BitSet) Clone() BitSet {
	if len(b) == 0 {
		return nil
	}
	out := make(BitSet, len(b))
	copy(out, b)
	return out
}

func (b BitSet) Has(bitIndex int) bool {
	if bitIndex < 0 {
		return false
	}
	word := bitIndex / 64
	if word >= len(b) {
		return false
	}
	return b[word]&(uint64(1)<<uint(bitIndex%64)) != 0
}

func (b BitSet) Set(bitIndex int) {
	if bitIndex < 0 {
		return
	}
	word := bitIndex / 64
	if word >= len(b) {
		return
	}
	b[word] |= uint64(1) << uint(bitIndex%64)
}

func (b BitSet) Clear(bitIndex int) {
	if bitIndex < 0 {
		return
	}
	word := bitIndex / 64
	if word >= len(b) {
		return
	}
	b[word] &^= uint64(1) << uint(bitIndex%64)
}

func (b BitSet) And(other BitSet) BitSet {
	out := b.Clone()
	for i := range out {
		if i < len(other) {
			out[i] &= other[i]
		} else {
			out[i] = 0
		}
	}
	return out
}

func (b BitSet) Or(other BitSet) BitSet {
	n := len(b)
	if len(other) > n {
		n = len(other)
	}
	out := make(BitSet, n)
	copy(out, b)
	for i := range other {
		out[i] |= other[i]
	}
	return out
}

func (b BitSet) AndNot(other BitSet) BitSet {
	out := b.Clone()
	for i := range out {
		if i < len(other) {
			out[i] &^= other[i]
		}
	}
	return out
}

func (b BitSet) Empty() bool {
	for _, word := range b {
		if word != 0 {
			return false
		}
	}
	return true
}
