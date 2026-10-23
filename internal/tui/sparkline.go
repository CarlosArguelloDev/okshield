package tui

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// HistoryCapacity bounds the per-container metrics ring buffer (60 samples =
// 60-90s of history at 1-1.5s sampling interval, <2KB per container).
const HistoryCapacity = 60

// MetricKind identifies which resource metric is plotted in the large Braille chart.
type MetricKind int

const (
	MetricCPU MetricKind = iota
	MetricMem
	MetricNet
	MetricDisk
	metricKindCount
)

var metricNames = [...]string{"CPU %", "Memoria %", "Red RX/TX", "Disco I/O"}

func (k MetricKind) String() string { return metricNames[k] }

// Braille column bitmasks from 0 (empty) to 4 (full column) for U+2800..U+28FF.
// Left column uses dots 7, 3, 2, 1; right column uses dots 8, 6, 5, 4.
var (
	brailleLeft  = [5]rune{0x00, 0x40, 0x44, 0x46, 0x47}
	brailleRight = [5]rune{0x00, 0x80, 0xA0, 0xB0, 0xB8}
)

// BrailleSparkline renders a slice of float64 samples into a single-line
// Unicode Braille graph (U+2800..U+28FF) of `width` characters (packing 2*width
// time samples horizontally and 4 vertical levels per character).
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
	b.Grow(width * 3)

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

// BrailleChart renders a large multi-row Unicode Braille area chart of `width`
// character columns (2*width horizontal dot samples) by `height` character rows
// (4*height vertical dot levels). Each returned string is one row of Braille
// characters (without axes, so the caller can style axes and chart separately).
func BrailleChart(values []float64, width, height int, maxVal float64) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if maxVal <= 0 {
		for _, v := range values {
			if v > maxVal {
				maxVal = v
			}
		}
		if maxVal <= 0 {
			maxVal = 1.0
		}
	}

	totalDotsX := width * 2
	totalDotsY := height * 4
	startIdx := len(values) - totalDotsX

	rows := make([]string, height)
	var b strings.Builder
	for r := 0; r < height; r++ {
		b.Reset()
		b.Grow(width * 3)
		invRow := height - 1 - r
		baseDots := invRow * 4

		for pair := 0; pair < width; pair++ {
			iL := startIdx + pair*2
			iR := iL + 1
			var vL, vR float64
			if iL >= 0 && iL < len(values) {
				vL = values[iL]
			}
			if iR >= 0 && iR < len(values) {
				vR = values[iR]
			}
			dotsL := dotCount(vL, maxVal, totalDotsY)
			dotsR := dotCount(vR, maxVal, totalDotsY)
			lvlL := clampInt(dotsL-baseDots, 0, 4)
			lvlR := clampInt(dotsR-baseDots, 0, 4)
			b.WriteRune(0x2800 | brailleLeft[lvlL] | brailleRight[lvlR])
		}
		rows[r] = b.String()
	}
	return rows
}

func dotCount(v, maxVal float64, totalDotsY int) int {
	if v <= 0 || maxVal <= 0 {
		return 0
	}
	ratio := v / maxVal
	if ratio > 1 {
		ratio = 1
	}
	dots := int(math.Round(ratio * float64(totalDotsY)))
	if dots == 0 && v > 0 {
		return 1
	}
	return dots
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

// MetricSeries is a fixed-capacity ring buffer of recent CPU, RAM, Network and
// Disk I/O samples for a container.
type MetricSeries struct {
	cpu        [HistoryCapacity]float64
	mem        [HistoryCapacity]float64
	net        [HistoryCapacity]float64
	disk       [HistoryCapacity]float64
	head       int
	count      int
	lastRx     uint64
	lastTx     uint64
	lastRead   uint64
	lastWrite  uint64
	rxRateBps  float64
	txRateBps  float64
	readRate   float64
	writeRate  float64
	lastSample time.Time
}

// Push appends a new CPU%, Mem%, Network and Block I/O sample.
func (s *MetricSeries) Push(cpuPct, memPct float64, rxBytes, txBytes uint64, now time.Time) {
	s.PushFull(cpuPct, memPct, rxBytes, txBytes, s.lastRead, s.lastWrite, now)
}

// PushFull appends CPU%, Mem%, Network RX/TX and Block Read/Write counters.
func (s *MetricSeries) PushFull(cpuPct, memPct float64, rxBytes, txBytes, blkRead, blkWrite uint64, now time.Time) {
	if !s.lastSample.IsZero() && now.After(s.lastSample) {
		dt := now.Sub(s.lastSample).Seconds()
		if dt > 0 {
			if rxBytes >= s.lastRx {
				s.rxRateBps = float64(rxBytes-s.lastRx) / dt
			}
			if txBytes >= s.lastTx {
				s.txRateBps = float64(txBytes-s.lastTx) / dt
			}
			if blkRead >= s.lastRead {
				s.readRate = float64(blkRead-s.lastRead) / dt
			}
			if blkWrite >= s.lastWrite {
				s.writeRate = float64(blkWrite-s.lastWrite) / dt
			}
		}
	}
	s.lastRx = rxBytes
	s.lastTx = txBytes
	s.lastRead = blkRead
	s.lastWrite = blkWrite
	s.lastSample = now

	idx := (s.head + s.count) % HistoryCapacity
	if s.count == HistoryCapacity {
		s.head = (s.head + 1) % HistoryCapacity
	} else {
		s.count++
	}
	s.cpu[idx] = cpuPct
	s.mem[idx] = memPct
	netVal := s.rxRateBps + s.txRateBps
	if netVal == 0 && (rxBytes+txBytes) > 0 {
		netVal = float64(rxBytes + txBytes)
	}
	diskVal := s.readRate + s.writeRate
	if diskVal == 0 && (blkRead+blkWrite) > 0 {
		diskVal = float64(blkRead + blkWrite)
	}
	s.net[idx] = netVal
	s.disk[idx] = diskVal
}

// Samples returns the chronological history for the given MetricKind.
func (s *MetricSeries) Samples(kind MetricKind, dst []float64) []float64 {
	dst = dst[:0]
	for i := 0; i < s.count; i++ {
		idx := (s.head + i) % HistoryCapacity
		switch kind {
		case MetricMem:
			dst = append(dst, s.mem[idx])
		case MetricNet:
			dst = append(dst, s.net[idx])
		case MetricDisk:
			dst = append(dst, s.disk[idx])
		default:
			dst = append(dst, s.cpu[idx])
		}
	}
	return dst
}

// CPUSamples returns the chronological CPU% history into dst.
func (s *MetricSeries) CPUSamples(dst []float64) []float64 {
	return s.Samples(MetricCPU, dst)
}

// MemSamples returns the chronological Mem% history into dst.
func (s *MetricSeries) MemSamples(dst []float64) []float64 {
	return s.Samples(MetricMem, dst)
}

// NetRates returns the latest RX and TX throughput in bytes/second.
func (s *MetricSeries) NetRates() (rxBps, txBps float64) {
	return s.rxRateBps, s.txRateBps
}

// FormatMetricAxis formats a Y-axis value according to the MetricKind.
func FormatMetricAxis(kind MetricKind, val float64) string {
	switch kind {
	case MetricNet, MetricDisk:
		return fmt.Sprintf("%7s", humanBytes(uint64(max(0, int64(val)))))
	default:
		return fmt.Sprintf("%6.0f%%", val)
	}
}
