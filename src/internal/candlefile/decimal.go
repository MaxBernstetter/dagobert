// Package candlefile reads and writes the candle Parquet files of the lake.
//
// Current schema (schema_version 2): open_time and close_time are INT64
// (delta encoded), open/high/low/close are DECIMAL(18,8) on INT64 and volume
// is DECIMAL(28,8) on FIXED_LEN_BYTE_ARRAY(12), all Snappy compressed, no
// dictionary. The legacy schema (schema_version 1) stores the five values as
// decimal strings; readers accept both, writers write only the current one.
package candlefile

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// Fixed-scale layout of the value columns.
const (
	Scale           = 8  // Binance's maximum number of decimals
	PricePrecision  = 18 // open, high, low, close: INT64
	VolumePrecision = 28 // volume: FIXED_LEN_BYTE_ARRAY(VolumeBytes)
	VolumeBytes     = 12
)

// ValueError reports a value that the fixed-scale layout cannot hold exactly.
type ValueError struct {
	Column string
	Value  string
	Reason string
}

func (e *ValueError) Error() string {
	return fmt.Sprintf("candle %s value %q: %s", e.Column, e.Value, e.Reason)
}

// scaledDigits turns a plain decimal string into its unscaled integer digits
// at Scale: "100.25" -> sign "", "10025000000". It rejects more than Scale
// significant decimals (trailing zeros beyond Scale are exact and accepted),
// exponents, empty input and more than precision digits.
func scaledDigits(column, s string, precision int) (neg bool, digits string, err error) {
	bad := func(reason string) (bool, string, error) {
		return false, "", &ValueError{Column: column, Value: s, Reason: reason}
	}
	t := strings.TrimSpace(s)
	if t != "" && (t[0] == '-' || t[0] == '+') {
		neg = t[0] == '-'
		t = t[1:]
	}
	intPart, frac, _ := strings.Cut(t, ".")
	if intPart == "" && frac == "" {
		return bad("not a decimal number")
	}
	for _, part := range []string{intPart, frac} {
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return bad("not a plain decimal number")
			}
		}
	}
	if len(frac) > Scale {
		if strings.TrimRight(frac[Scale:], "0") != "" {
			return bad(fmt.Sprintf("more than %d decimals", Scale))
		}
		frac = frac[:Scale]
	}
	frac += strings.Repeat("0", Scale-len(frac))
	digits = strings.TrimLeft(intPart+frac, "0")
	if len(digits) > precision {
		return bad(fmt.Sprintf("does not fit DECIMAL(%d,%d)", precision, Scale))
	}
	if digits == "" {
		digits, neg = "0", false
	}
	return neg, digits, nil
}

// EncodePrice converts a decimal string to its DECIMAL(18,8) unscaled value.
func EncodePrice(column, s string) (int64, error) {
	neg, digits, err := scaledDigits(column, s, PricePrecision)
	if err != nil {
		return 0, err
	}
	v, err := strconv.ParseInt(digits, 10, 64) // 18 digits always fit
	if err != nil {
		return 0, &ValueError{Column: column, Value: s, Reason: err.Error()}
	}
	if neg {
		v = -v
	}
	return v, nil
}

// EncodeVolume converts a decimal string to the DECIMAL(28,8) unscaled value
// as a 12 byte big-endian two's complement integer.
func EncodeVolume(column, s string) ([VolumeBytes]byte, error) {
	var out [VolumeBytes]byte
	neg, digits, err := scaledDigits(column, s, VolumePrecision)
	if err != nil {
		return out, err
	}
	v, _ := new(big.Int).SetString(digits, 10)
	if neg {
		v.Neg(v)
		v.Add(v, new(big.Int).Lsh(big.NewInt(1), 8*VolumeBytes))
	}
	v.FillBytes(out[:])
	return out, nil
}

// bigFromVolume decodes a 12 byte two's complement integer.
func bigFromVolume(b []byte) *big.Int {
	v := new(big.Int).SetBytes(b)
	if len(b) > 0 && b[0]&0x80 != 0 {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(8*len(b))))
	}
	return v
}

// formatScaled renders unscaled/10^scale with exactly scale decimals.
func formatScaled(unscaled string, scale int) string {
	neg := strings.HasPrefix(unscaled, "-")
	unscaled = strings.TrimPrefix(unscaled, "-")
	if scale > 0 {
		if len(unscaled) <= scale {
			unscaled = strings.Repeat("0", scale-len(unscaled)+1) + unscaled
		}
		unscaled = unscaled[:len(unscaled)-scale] + "." + unscaled[len(unscaled)-scale:]
	}
	if neg {
		return "-" + unscaled
	}
	return unscaled
}

// NormalizeDecimal canonicalizes a decimal string for numeric equality:
// no sign on zero, no leading zeros, no trailing fraction zeros.
func NormalizeDecimal(s string) string {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	i, f, _ := strings.Cut(s, ".")
	i = strings.TrimLeft(i, "0")
	if i == "" {
		i = "0"
	}
	f = strings.TrimRight(f, "0")
	out := i
	if f != "" {
		out += "." + f
	}
	if neg && out != "0" {
		out = "-" + out
	}
	return out
}

const exactFloat = 1 << 53

// floatFromUnscaled converts unscaled/10^scale to the float64 nearest the
// decimal value, the same result strconv.ParseFloat gives for its string.
func floatFromUnscaled(v int64, scale int) float64 {
	if v > -exactFloat && v < exactFloat && scale >= 0 && scale < len(pow10) {
		return float64(v) / pow10[scale]
	}
	f, _ := strconv.ParseFloat(formatScaled(strconv.FormatInt(v, 10), scale), 64)
	return f
}

var pow10 = [...]float64{1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22}
