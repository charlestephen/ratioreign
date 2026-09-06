package announce

import "testing"

func TestClampInt(t *testing.T) {
	cases := []struct {
		v, max int64
		want   int
	}{
		{v: 300, max: maxReasonableInterval, want: 300},
		{v: -1, max: maxReasonableInterval, want: 0},
		{v: 1 << 40, max: maxReasonableInterval, want: maxReasonableInterval},
		{v: 0, max: maxReasonablePeerCount, want: 0},
	}
	for _, c := range cases {
		if got := clampInt(c.v, c.max); got != c.want {
			t.Errorf("clampInt(%d, %d) = %d, want %d", c.v, c.max, got, c.want)
		}
	}
}

func TestParseResponseClampsOutOfRangeValues(t *testing.T) {
	// interval = 2^40 (way beyond any real tracker value, and beyond what
	// fits in a 32-bit int) should come back clamped, not wrapped/negative.
	body := []byte("d8:completei5e10:incompletei9e8:intervali1099511627776ee")
	resp, err := parseResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Interval != maxReasonableInterval {
		t.Errorf("Interval = %d, want %d (clamped)", resp.Interval, maxReasonableInterval)
	}
	if resp.Complete != 5 || resp.Incomplete != 9 {
		t.Errorf("Complete/Incomplete = %d/%d, want 5/9", resp.Complete, resp.Incomplete)
	}
}
