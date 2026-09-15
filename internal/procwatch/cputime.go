package procwatch

import (
	"strconv"
	"strings"
)

// ParseCPUTime converts macOS ps cputime ([{yy}-d]hh:mm:ss or mm:ss.cc) to seconds.
func ParseCPUTime(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		s = s[i+1:]
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		mm, err1 := strconv.Atoi(parts[0])
		sec, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 != nil || err2 != nil {
			return 0
		}
		return float64(mm)*60 + sec
	case 3:
		hh, err1 := strconv.Atoi(parts[0])
		mm, err2 := strconv.Atoi(parts[1])
		sec, err3 := strconv.ParseFloat(parts[2], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return 0
		}
		return float64(hh)*3600 + float64(mm)*60 + sec
	default:
		return 0
	}
}

// SubtreeCPU returns total CPU seconds for pid and all descendants.
func SubtreeCPU(pid int, children map[int][]int, cpu map[int]float64) float64 {
	sum := cpu[pid]
	for _, c := range children[pid] {
		sum += SubtreeCPU(c, children, cpu)
	}
	return sum
}
