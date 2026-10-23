package tui

import (
	"strings"
	"time"
)

// HistoryCapacity bounds the per-container metrics ring buffer (60 samples =
// 60-90s of history at 1-1.5s sampling interval, <1KB per container).
const HistoryCapacity = 60

// Braille column bitmasks from 0 (empty) to 4 (full column) for U+2800..U+28FF.
// Left column uses dots 7, 3, 2, 1; right column uses dots 8, 6, 5, 4.
var (
	brailleLeft  = [5]rune{0x00, 0x40, 0x44, 0x46, 0x47}
	brailleRight = [5]rune{0x00, 0x80, 0xA0, 0xB0, 0xB8}
)

// BrailleSparkline renders a slice of float64 samples into a single-line
// Unicode Braille graph (U+2800..U+28FF) of `width` characters (packing 2*width
// time samples horizontally and 4 vertical levels per character).
// If maxVal <= 0, the maximum value in `values` is used (with a floor of 1.0).
func BrailleSparkline(values []float64, width int, maxVal float64) string {
	if width <= 0 {
		return ""
	}
	totalSamples := width * 2
	if maxVal <= 0 {
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
		if maxVal <= 0 {
			maxVal = 100.0
		}
	}

	var b strings.Builder
	b.Grow(width * 3) // each Braille rune is 3 UTF-8 bytes

	startIdx := len(values) - totalSamples
	for pair := 0; pair < width; pair++ {
		iLeft := startIdx + pair*2
		iRight := iLeft + 1

		var vLeft, vRight float64
		if iLeft >= 0 && iLeft < len(values) {
			vLeft = values[iLeft]
		}
		if iRight >= 0 && iRight < len(values) {
			vRight = values[iRight]
		}

		lvlLeft := brailleLevel(vLeft, maxVal)
		lvlRight := brailleLevel(vRight, maxVal)
		b.WriteRune(0x2800 | brailleLeft[lvlLeft] | brailleRight[lvlRight])
	}
	return b.String()
}

func brailleLevel(v, maxVal float64) int {
	if v <= 0 || maxVal <= 0 {
		return 0
	}
	ratio := v / maxVal
	if ratio >= 0.875 {
		return 4
	}
	if ratio >= 0.625 {
		return 3
	}
	if ratio >= 0.325 {
		return 2
	}
	if ratio > 0.02 {
		return 1
	}
	return 0
}

// MetricSeries is a fixed-capacity ring buffer of recent CPU and RAM samples
// plus network/disk I/O counters for a container.
type MetricSeries struct {
	cpu        [HistoryCapacity]float64
	mem        [HistoryCapacity]float64
	head       int
	count      int
	lastRx     uint64
	lastTx     uint64
	rxRateBps  float64
	txRateBps  float64
	lastSample time.Time
}

// Push appends a new CPU% and Mem% sample and updates network throughput rates.
func (s *MetricSeries) Push(cpuPct, memPct float64, rxBytes, txBytes uint64, now time.Time) {
	idx := (s.head + s.count) % HistoryCapacity
	if s.count == HistoryCapacity {
		s.head = (s.head + 1) % HistoryCapacity
	} else {
		s.count++
	}
	s.cpu[idx] = cpuPct
	s.mem[idx] = memPct

	if !s.lastSample.IsZero() && now.After(s.lastSample) {
		dt := now.Sub(s.lastSample).Seconds()
		if dt > 0 {
			if rxBytes >= s.lastRx {
				s.rxRateBps = float64(rxBytes-s.lastRx) / dt
			}
			if txBytes >= s.lastTx {
				s.txRateBps = float64(txBytes-s.lastTx) / dt
			}
		}
	}
	s.lastRx = rxBytes
	s.lastTx = txBytes
	s.lastSample = now
}

// CPUSamples returns the chronological CPU% history into dst (or a new slice).
func (s *MetricSeries) CPUSamples(dst []float64) []float64 {
	dst = dst[:0]
	for i := 0; i < s.count; i++ {
		dst = append(dst, s.cpu[(s.head+i)%HistoryCapacity])
	}
	return dst
}

// MemSamples returns the chronological Mem% history into dst (or a new slice).
func (s *MetricSeries) MemSamples(dst []float64) []float64 {
	dst = dst[:0]
	for i := 0; i < s.count; i++ {
		dst = append(dst, s.mem[(s.head+i)%HistoryCapacity])
	}
	return dst
}

// NetRates returns the latest RX and TX throughput in bytes/second.
func (s *MetricSeries) NetRates() (rxBps, txBps float64) {
	return s.rxRateBps, s.txRateBps
}
