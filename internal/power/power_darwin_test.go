package power

import "testing"

func TestParseBatteryOutput(t *testing.T) {
	sample := "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=123)	24%; discharging; present: true\n"
	pct, ok := ParseBatteryOutput(sample)
	if !ok || pct != 24 {
		t.Fatalf("got %d ok=%v want 24", pct, ok)
	}
}

func TestParseBatteryOutputMissing(t *testing.T) {
	if _, ok := ParseBatteryOutput("no percent here"); ok {
		t.Fatal("expected not ok")
	}
}
