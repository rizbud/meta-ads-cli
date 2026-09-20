package money

import "testing"

func TestFormat(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{1051167, "IDR", "IDR 1,051,167"},
		{1500, "USD", "USD 15.00"},
		{150000, "USD", "USD 1,500.00"},
		{2000000, "idr", "IDR 2,000,000"},
		{17715, "JPY", "JPY 17,715"},
		{1000, "KRW", "KRW 1,000"},
		{17715, "", "177.15"},
		{0, "USD", "USD 0.00"},
		{0, "IDR", "IDR 0"},
		{-1500, "USD", "USD -15.00"},
		{-1051167, "IDR", "IDR -1,051,167"},
	}
	for _, c := range cases {
		if got := Format(c.minor, c.currency); got != c.want {
			t.Errorf("Format(%d, %q) = %q, want %q", c.minor, c.currency, got, c.want)
		}
	}
}
