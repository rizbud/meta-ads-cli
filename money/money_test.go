package money

import "testing"

func TestFormat(t *testing.T) {
	cases := []struct {
		minor    int64
		currency string
		want     string
	}{
		{1051167, "IDR", "IDR 10511.67"},
		{1500, "USD", "USD 15.00"},
		{2000000, "idr", "IDR 20000.00"},
		{17715, "JPY", "JPY 17715"},
		{1000, "KRW", "KRW 1000"},
		{17715, "", "177.15"},
		{0, "USD", "USD 0.00"},
	}
	for _, c := range cases {
		if got := Format(c.minor, c.currency); got != c.want {
			t.Errorf("Format(%d, %q) = %q, want %q", c.minor, c.currency, got, c.want)
		}
	}
}
