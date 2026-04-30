package incusosimage

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	minimumSizeLength       = 2
	kibibyte          int64 = 1024
	mebibyte                = kibibyte * 1024
	gibibyte                = mebibyte * 1024
	tebibyte                = gibibyte * 1024
)

func parseSize(value string) (int64, error) {
	normalized := strings.TrimSpace(strings.ToUpper(value))
	if len(normalized) < minimumSizeLength {
		return 0, fmt.Errorf("invalid size %q", value)
	}

	suffix := normalized[len(normalized)-1:]
	multiplier, ok := map[string]int64{
		"K": kibibyte,
		"M": mebibyte,
		"G": gibibyte,
		"T": tebibyte,
	}[suffix]
	if !ok {
		return 0, fmt.Errorf("unsupported size suffix %q", suffix)
	}

	amount, err := strconv.ParseInt(normalized[:len(normalized)-1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse size amount: %w", err)
	}
	if amount <= 0 {
		return 0, errors.New("size amount must be positive")
	}

	return amount * multiplier, nil
}
