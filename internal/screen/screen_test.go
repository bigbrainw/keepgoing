package screen

import "testing"

func TestShouldSleep(t *testing.T) {
	const threshold = 120.0
	cases := []struct {
		name    string
		agents  bool
		idle    float64
		thresh  float64
		fired   bool
		want    bool
	}{
		{"below threshold", true, 60, threshold, false, false},
		{"at threshold", true, 120, threshold, false, true},
		{"above threshold", true, 300, threshold, false, true},
		{"already fired", true, 300, threshold, true, false},
		{"no agents", false, 300, threshold, false, false},
		{"disabled", true, 300, 0, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldSleep(tc.agents, tc.idle, tc.thresh, tc.fired)
			if got != tc.want {
				t.Fatalf("ShouldSleep(%v, %v, %v, %v) = %v, want %v",
					tc.agents, tc.idle, tc.thresh, tc.fired, got, tc.want)
			}
		})
	}
}
