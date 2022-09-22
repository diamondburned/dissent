package colorhash

import "hash"

// The DJB2 hashing implementation is taken from https://github.com/dim13/djb2,
// which is licensed under the ISC license.

const djb2Magic djb2 = 5381

type djb2 uint32

// newDJB32 creates a new DJB2 hasher.
func newDJB32() hash.Hash32 {
	d := djb2Magic
	return &d
}

func (d *djb2) BlockSize() int { return 1 }

func (d *djb2) Reset() { *d = djb2Magic }

func (d *djb2) Size() int { return 4 }

func (d *djb2) Sum(b []byte) []byte {
	// Similar to fnv32a.
	v := uint32(*d)
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func (d *djb2) Sum32() uint32 {
	return uint32(*d)
}

func (d *djb2) Write(p []byte) (int, error) {
	for _, v := range p {
		*d = ((*d << 5) + *d) + djb2(v)
	}
	return len(p), nil
}
