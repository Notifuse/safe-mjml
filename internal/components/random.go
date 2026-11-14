package components

import (
	"math/rand"
	"strings"
)

// newElementID returns the 16-hex-digit ID of an element that needs one, such
// as a carousel or a navbar checkbox: from RenderOpts.IDGenerator when set,
// otherwise a random one, as MJML does. A generated value that is not 16
// lowercase hex digits is replaced by a random ID: it goes into CSS selectors
// and attributes unescaped.
func (bc *BaseComponent) newElementID() string {
	if bc.RenderOpts != nil && bc.RenderOpts.IDGenerator != nil {
		if id := bc.RenderOpts.IDGenerator(); isElementID(id) {
			return id
		}
	}
	return genRandomHexString(16)
}

// isElementID reports whether id is 16 lowercase hex digits, the form of the
// IDs MJML generates.
func isElementID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if c := id[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// genRandomHexString generates a random hexadecimal string of the specified length.
// This replicates MJML's genRandomHexString function which uses Math.random()
// to generate random hex digits for unique element IDs.
func genRandomHexString(length int) string {
	var sb strings.Builder
	sb.Grow(length)

	for range length {
		sb.WriteByte("0123456789abcdef"[rand.Intn(16)])
	}

	return sb.String()
}
