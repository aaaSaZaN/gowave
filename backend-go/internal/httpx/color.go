package httpx

import (
	"fmt"
	"math/big"
	"strings"
)

// ColorFromID ports colorFromId from @kastov/uuid-color 0.1.0: a SplitMix64
// finalizer spreads sequential ids, and the low 24 bits are the RGB color.
func ColorFromID(id uint64) string {
	z := id
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	return fmt.Sprintf("#%06x", z&0xffffff)
}

// ColorFromUUID ports colorFromUuid: the UUID's low 24 bits are the color.
func ColorFromUUID(uuid string) string {
	value, ok := new(big.Int).SetString(strings.ReplaceAll(uuid, "-", ""), 16)
	if !ok {
		return "#000000"
	}
	return fmt.Sprintf("#%06x", new(big.Int).And(value, big.NewInt(0xffffff)).Int64())
}
