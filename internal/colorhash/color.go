package colorhash

import (
	"hash"
	"hash/fnv"
	"image/color"
	"math"
	"sync"

	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

// Hasher describes a string hasher that outputs a color.
type Hasher interface {
	Hash(name string) color.RGBA
}

var (
	// FNVHasher is the alternative hasher for color hashing.
	FNVHasher = func() hash.Hash32 { return fnv.New32a() }
	// DJB2Hasher is the string hasher used for color hashing.
	DJB2Hasher = newDJB32
)

var (
	// LightColorHasher generates a pastel color name for use with a dark
	// background.
	LightColorHasher Hasher = HSVHasher{
		FNVHasher,
		[2]float64{0.3, 0.4},
		[2]float64{0.9, 1.0},
	}
	// DarkColorHasher generates a darker, stronger color name for use with a
	// light background.
	DarkColorHasher Hasher = HSVHasher{
		FNVHasher,
		[2]float64{0.9, 1.0},
		[2]float64{0.6, 0.7},
	}
)

// RGBHex converts the given color to a HTML hex color string. The alpha value
// is ignored.
func RGBHex(c color.RGBA) string {
	return textutil.RGBHex(c)
}

// HSVHasher describes a color hasher that accepts saturation and value
// parameters in the HSV color space.
type HSVHasher struct {
	H func() hash.Hash32 // hashing function
	S [2]float64         // saturation
	V [2]float64         // value
}

const (
	nHue = 31 // hue count; 31 so I get to keep my pink color
	nSat = 10
	nVal = 10
)

// Hash hashes the given name using the parameters inside HSVHasher.
func (h HSVHasher) Hash(name string) color.RGBA {
	hasher := h.H()
	hasher.Write([]byte(name))

	hash := hasher.Sum32()

	// Calculate the range within [0, 360] in integer.
	hue := float64((hash % (nHue + 1)) * (360 / nHue))

	// Calculate the range within [0, 1] using modulo and division, then scale
	// it up/down to the intended range using multiplication and addition.
	sat := h.S[0] + (float64(hash%(nSat+1)) / nSat * (h.S[1] - h.S[0]))
	val := h.V[0] + (float64(hash%(nVal+1)) / nVal * (h.V[1] - h.V[0]))

	return hsvrgb(hue, sat, val)
}

// hsvrgb is taken from lucasb-eyer/go-colorful, licensed under the MIT license.
func hsvrgb(h, s, v float64) color.RGBA {
	Hp := h / 60.0
	C := v * s
	X := C * (1.0 - math.Abs(math.Mod(Hp, 2.0)-1.0))

	m := v - C
	r, g, b := 0.0, 0.0, 0.0

	switch {
	case 0.0 <= Hp && Hp < 1.0:
		r = C
		g = X
	case 1.0 <= Hp && Hp < 2.0:
		r = X
		g = C
	case 2.0 <= Hp && Hp < 3.0:
		g = C
		b = X
	case 3.0 <= Hp && Hp < 4.0:
		g = X
		b = C
	case 4.0 <= Hp && Hp < 5.0:
		r = X
		b = C
	case 5.0 <= Hp && Hp < 6.0:
		r = C
		b = X
	}

	return color.RGBA{
		R: uint8((m + r) * 0xFF),
		G: uint8((m + g) * 0xFF),
		B: uint8((m + b) * 0xFF),
		A: 0xFF,
	}
}

var (
	defaultHasher = LightColorHasher
	hasherMutex   sync.RWMutex
)

// DefaultHasher returns the default color hasher.
func DefaultHasher() Hasher {
	hasherMutex.RLock()
	defer hasherMutex.RUnlock()

	return defaultHasher
}

// SetDefaultHasher sets the default hasher that the package uses.
func SetDefaultHasher(hasher Hasher) {
	hasherMutex.Lock()
	defer hasherMutex.Unlock()

	defaultHasher = hasher
}
