package tui

import (
	"testing"
	"time"
	"unicode/utf8"
)

func TestBrailleSparkline(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		width  int
		maxVal float64
		want   string
	}{
		{
			name:   "empty_zeroes",
			values: nil,
			width:  4,
			maxVal: 100,
			want:   "⠀⠀⠀⠀",
		},
		{
			name:   "full_scale",
			values: []float64{100, 100, 100, 100},
			width:  2,
			maxVal: 100,
			want:   "⣿⣿",
		},
		{
			name:   "ramp_up",
			values: []float64{0, 20, 50, 75, 80, 100},
			width:  3,
			maxVal: 100,
			want:   "⢀⣴⣾",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BrailleSparkline(tt.values, tt.width, tt.maxVal)
			if utf8.RuneCountInString(got) != tt.width {
				t.Fatalf("rune count = %d, want %d", utf8.RuneCountInString(got), tt.width)
			}
			if got != tt.want {
				t.Errorf("BrailleSparkline() = %q (%x), want %q (%x)", got, []rune(got), tt.want, []rune(tt.want))
			}
			for _, r := range got {
				if r < 0x2800 || r > 0x28FF {
					t.Errorf("rune %U outside Braille block U+2800..U+28FF", r)
				}
			}
		})
	}
}

func TestMetricSeriesRingBuffer(t *testing.T) {
	var ms MetricSeries
	t0 := time.Now()
	for i := 0; i < HistoryCapacity+10; i++ {
		ms.PushFull(float64(i), float64(i*2), uint64(i*1024), uint64(i*512), uint64(i*2048), uint64(i*1024), t0.Add(time.Duration(i)*time.Second))
	}
	cpu := ms.CPUSamples(make([]float64, 0, HistoryCapacity))
	if len(cpu) != HistoryCapacity {
		t.Fatalf("len(cpu) = %d, want %d", len(cpu), HistoryCapacity)
	}
	if cpu[0] != 10 || cpu[len(cpu)-1] != float64(HistoryCapacity+9) {
		t.Errorf("unexpected ring wrap: first=%v last=%v", cpu[0], cpu[len(cpu)-1])
	}
	rx, tx := ms.NetRates()
	if rx != 1024 || tx != 512 {
		t.Errorf("rates = (%v, %v), want (1024, 512)", rx, tx)
	}
	rows := BrailleChart([]float64{50, 50, 50, 50}, 12, 6, 100)
	if len(rows) != 6 {
		t.Fatalf("BrailleChart rows = %d, want 6", len(rows))
	}
	for i, r := range rows {
		if utf8.RuneCountInString(r) != 12 {
			t.Errorf("row %d rune count = %d, want 12", i, utf8.RuneCountInString(r))
		}
	}
	// Bottom row (row 5) should be empty (all U+2800) for a flat 50% line.
	for _, ch := range rows[5] {
		if ch != 0x2800 {
			t.Errorf("bottom row should be blank U+2800 for 50%% line, got %U", ch)
		}
	}

	// Verify interpolateCanvas preserves exact raw endpoints while linearly
	// interpolating intermediate Braille dot columns.
	ramp := interpolateCanvas([]float64{20, 80}, 4)
	if len(ramp) != 4 || ramp[1] != 20 || ramp[2] != 50 || ramp[3] != 80 {
		t.Errorf("interpolateCanvas failed to preserve raw endpoints with linear midpoint: %v", ramp)
	}
}

func BenchmarkBrailleSparkline(b *testing.B) {
	samples := make([]float64, 60)
	for i := range samples {
		samples[i] = float64((i * 7) % 100)
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = BrailleSparkline(samples, 24, 100)
	}
}
