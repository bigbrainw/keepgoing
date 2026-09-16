package thermolog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGapReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 9, 15, 14, 0, 0, 0, time.Local)
	rows := []string{
		base.Format(time.RFC3339) + ",false,nominal,60.0,false,0,claude,0,false",
		base.Add(30 * time.Second).Format(time.RFC3339) + ",false,nominal,61.0,false,0,claude,0,false",
		base.Add(3 * time.Minute).Format(time.RFC3339) + ",false,nominal,62.0,false,0,claude,0,false",
		base.Add(3*time.Minute+30*time.Second).Format(time.RFC3339) + ",false,nominal,63.0,false,0,claude,0,false",
	}
	content := header + strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := GapReport()
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || g.Count != 1 {
		t.Fatalf("expected 1 gap, got %+v", g)
	}
	if g.Longest != 2*time.Minute+30*time.Second {
		t.Fatalf("longest gap = %s", g.Longest)
	}
	line := FormatGapLine(g)
	if !strings.Contains(line, "gaps: 1") || !strings.Contains(line, "14:00") {
		t.Fatalf("unexpected line: %q", line)
	}
}

func TestGapReportNone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Format(time.RFC3339)
	content := header + ts + ",false,nominal,60.0,false,0,claude,0,false\n"
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := GapReport()
	if err != nil {
		t.Fatal(err)
	}
	if g != nil {
		t.Fatalf("expected no gaps, got %+v", g)
	}
}
