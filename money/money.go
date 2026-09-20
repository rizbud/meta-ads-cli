// Package money formats Meta monetary amounts, which the API expresses in the
// currency's minor units, for display alongside an ISO 4217 currency code.
package money

import (
	"fmt"
	"strings"
)

var zeroDecimalCurrencies = map[string]struct{}{
	"BIF": {}, "CLP": {}, "DJF": {}, "GNF": {}, "IDR": {}, "ISK": {}, "JPY": {},
	"KMF": {}, "KRW": {}, "PYG": {}, "RWF": {}, "UGX": {}, "VND": {},
	"VUV": {}, "XAF": {}, "XOF": {}, "XPF": {},
}

// formatThousands inserts commas as thousands separators for the integer part.
func formatThousands(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return sign + s
	}
	var b strings.Builder
	b.WriteString(sign)
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteByte(',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		if i > rem {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// Format renders a minor-unit amount in the given currency code. Zero-decimal
// currencies are rendered without decimals and with thousands separators. When
// currency is empty the amount is rendered without a currency prefix.
func Format(minor int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if _, zero := zeroDecimalCurrencies[currency]; zero {
		formatted := formatThousands(minor)
		if currency == "" {
			return formatted
		}
		return fmt.Sprintf("%s %s", currency, formatted)
	}

	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	whole := minor / 100
	fraction := minor % 100
	formattedWhole := formatThousands(whole)
	if currency == "" {
		return fmt.Sprintf("%s%s.%02d", sign, formattedWhole, fraction)
	}
	return fmt.Sprintf("%s %s%s.%02d", currency, sign, formattedWhole, fraction)
}
