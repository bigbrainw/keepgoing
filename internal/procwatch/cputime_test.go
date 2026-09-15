package procwatch

import "testing"

func TestParseCPUTime(t *testing.T) {
	cases := map[string]float64{
		"0:01.23":    1.23,
		"1:52.33":    112.33,
		"10:05.00":   605,
		"1:23:45":    5025,
		"0:00.00":    0,
		"91:19.98":   5479.98,
		"1-02:15:30": 2*3600 + 15*60 + 30,
	}
	for in, want := range cases {
		got := ParseCPUTime(in)
		if abs(got-want) > 0.01 {
			t.Errorf("ParseCPUTime(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSubtreeCPU(t *testing.T) {
	cpu := map[int]float64{1: 10, 2: 5, 3: 3, 4: 2}
	children := map[int][]int{
		1: {2, 3},
		2: {4},
	}
	got := SubtreeCPU(1, children, cpu)
	want := 20.0
	if got != want {
		t.Errorf("SubtreeCPU(1) = %v, want %v", got, want)
	}
	if SubtreeCPU(4, children, cpu) != 2 {
		t.Errorf("leaf should be 2")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
