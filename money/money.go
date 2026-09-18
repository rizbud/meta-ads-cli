// Package money formats Meta monetary amounts, which the API expresses in the
// currency's minor units, for display alongside an ISO 4217 currency code.
package money

import (
	"fmt"
	"strings"
)

var zeroDecimalCurrencies = map[string]struct{}{
	"BIF": {}, "CLP": {}, "DJF": {}, "GNF": {}, "ISK": {}, "JPY": {},
	"KMF": {}, "KRW": {}, "PYG": {}, "RWF": {}, "UGX": {}, "VND": {},
	"VUV": {}, "XAF": {}, "XOF": {}, "XPF": {},
}

// Format renders a minor-unit amount in the given currency code. Zero-decimal
// currencies are rendered without decimals. When currency is empty the amount
// is rendered without a currency prefix.
func Format(minor int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if _, zero := zeroDecimalCurrencies[currency]; zero {
		return fmt.Sprintf("%s %d", currency, minor)
	}
	major := float64(minor) / 100
	if currency == "" {
		return fmt.Sprintf("%.2f", major)
	}
	return fmt.Sprintf("%s %.2f", currency, major)
}
