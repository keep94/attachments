package attachments

import (
	"fmt"
	"math"
	"strconv"
)

var suffixes = map[int]string{0: "B", 3: "KB", 6: "MB", 9: "GB"}

func formatSize(size int64) string {
	if size < 0 {
		return "--"
	}
	if size == 0 {
		return "0 B"
	}
	mantissa, exponent := toMantissaAndExponent(size)
	mantissa, exponent = roundToThreeSignificantDigits(mantissa, exponent)

	// Make exponent be 0, 3, 6, or 9. precision is number of decimal
	// places of mantissa.
	precision := 2
	if precision > exponent {
		precision = exponent
	}
	for exponent > 9 || exponent%3 > 0 {
		mantissa *= 10.0
		exponent--
		if precision > 0 {
			precision--
		}
	}

	return fmt.Sprintf(
		"%s %s",
		strconv.FormatFloat(mantissa, 'f', precision, 64),
		suffixes[exponent],
	)
}

func toMantissaAndExponent(size int64) (mantissa float64, exponent int) {
	if size <= 0 {
		panic("size must be greater than 0")
	}
	exponent = -1
	mantissa = 0.0
	for size > 0 {
		digit := size % 10
		size /= 10
		mantissa /= 10.0
		mantissa += float64(digit)
		exponent++
	}
	return
}

func roundToThreeSignificantDigits(
	mantissa float64, exponent int) (float64, int) {

	// Round mantissa to 2 decimal places
	mantissa = math.Floor(mantissa*100.0+0.5) / 100.0

	// Take care of special case when we round mantissa up past 10.
	if mantissa >= 10.0 {
		mantissa /= 10.0
		exponent++
	}
	return mantissa, exponent
}
