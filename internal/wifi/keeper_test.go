//go:build darwin

package wifi

import (
	"testing"
	"time"
)

type fakeClock struct {
	t time.Time
}

func (c *fakeClock) Now() time.Time { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func TestNextJoinThreshold(t *testing.T) {
	if nextJoinThreshold(0) != 120*time.Second {
		t.Fatalf("first join at 120s, got %v", nextJoinThreshold(0))
	}
	if nextJoinThreshold(1) != 240*time.Second {
		t.Fatalf("second join at 240s, got %v", nextJoinThreshold(1))
	}
	if nextJoinThreshold(2) != 480*time.Second {
		t.Fatalf("third join at 480s, got %v", nextJoinThreshold(2))
	}
	if nextJoinThreshold(3) != 720*time.Second {
		t.Fatalf("fourth join at 720s, got %v", nextJoinThreshold(3))
	}
}

func TestShouldTryJoin(t *testing.T) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	k := &Keeper{ssid: "Test", now: clk.Now}
	offline := clk.t

	// Before 120s: no join
	clk.Advance(119 * time.Second)
	if k.shouldTryJoin(clk.Now().Sub(offline)) {
		t.Fatal("should not join before 120s")
	}

	// At 120s: first join
	clk.Advance(1 * time.Second)
	if !k.shouldTryJoin(clk.Now().Sub(offline)) {
		t.Fatal("should join at 120s")
	}
	k.joinAttempts++
	k.lastJoinTry = clk.Now()

	// Before 240s: no second join
	clk.Advance(119 * time.Second)
	if k.shouldTryJoin(clk.Now().Sub(offline)) {
		t.Fatal("should not join again before 240s")
	}

	// At 240s: second join
	clk.Advance(1 * time.Second)
	if !k.shouldTryJoin(clk.Now().Sub(offline)) {
		t.Fatal("should join at 240s")
	}
}

func TestJoinOutputFailed(t *testing.T) {
	cases := []struct {
		out string
		fail bool
	}{
		{"Could not find network Oh yeah", true},
		{"Failed to join Wi-Fi network", true},
		{"Error: -3905", true},
		{"", false},
		{"done", false},
	}
	for _, c := range cases {
		if joinOutputFailed(c.out) != c.fail {
			t.Fatalf("joinOutputFailed(%q) = %v, want %v", c.out, !c.fail, c.fail)
		}
	}
}

func TestFirstLine(t *testing.T) {
	if firstLine("  hello\nworld") != "hello" {
		t.Fatal(firstLine("  hello\nworld"))
	}
}
