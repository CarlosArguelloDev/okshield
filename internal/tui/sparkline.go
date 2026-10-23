package tui

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// HistoryCapacity bounds the per-container metrics ring buffer (120 samples).
const HistoryCapacity = 120

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

// Braille column bitmasks from 0 (empty) to 4 (full column) for U+2800..U+28FF
// used by compact single-row sparklines.
var (
	brailleLeft  = [5]rune{0x00, 0x40, 0x44, 0x46, 0x47}
	brailleRight = [5]rune{0x00, 0x80, 0xA0, 0xB0, 0xB8}

	// Single-dot bitmasks indexed by subRow (0 = top .. 3 = bottom) for
	// gotop-style multi-row Braille line charts.
	brailleDotLeft  = [4]rune{0x01, 0x02, 0x04, 0x40}
	brailleDotRight = [4]rune{0x08, 0x10, 0x20, 0x80}
)

// SmoothSeries applies an Exponential Moving Average (EMA, alpha=0.45) followed
// by a 5-point weighted moving average kernel [1, 2, 4, 2, 1]/10 to transform
// abrupt step changes into smooth, organic wave transitions.
func SmoothSeries(values []float64) []float64 {
	n := len(values)
	if n == 0 {
		return nil
	}
	out := make([]float64, n)
	if n == 1 {
		out[0] = values[0]
		return out
	}

	// Pass 1: forward EMA to soften sudden spikes.
	const alpha = 0.45
	ema := make([]float64, n)
	ema[0] = values[0]
	for i := 1; i < n; i++ {
		ema[i] = alpha*values[i] + (1.0-alpha)*ema[i-1]
	}

	// Pass 2: 5-point triangular moving average [1, 2, 4, 2, 1] / 10.
	weights := [5]float64{1, 2, 4, 2, 1}
	for i := 0; i < n; i++ {
		var sum, wSum float64
		for k := -2; k <= 2; k++ {
			idx := clampInt(i+k, 0, n-1)
			w := weights[k+2]
			sum += ema[idx] * w
			wSum += w
		}
		out[i] = sum / wSum
	}
	return out
}

// interpolateWave resamples a smoothed series across `targetPoints` horizontal
// dot columns using cosine interpolation between sample points so transitions
// between ticks render as fluid waves instead of staircases.
func interpolateWave(values []float64, targetPoints int) []float64 {
	if targetPoints <= 0 {
		return nil
	}
	smoothed := SmoothSeries(values)
	n := len(smoothed)
	out := make([]float64, targetPoints)
	if n == 0 {
		return out
	}
	if n == 1 {
		for i := range out {
			out[i] = smoothed[0]
		}
		return out
	}

	// Each raw sample spans `step` horizontal dots (2 dots per tick), keeping
	// the timeline right-aligned at "ahora" while smoothly interpolating
	// intermediate points.
	const dotsPerSample = 2.0
	totalSpan := float64(n-1) * dotsPerSample
	startOffset := float64(targetPoints-1) - totalSpan

	for x := 0; x < targetPoints; x++ {
		rel := float64(x) - startOffset
		if rel <= 0 {
			out[x] = smoothed[0]
			continue
		}
		pos := rel / dotsPerSample
		i0 := int(math.Floor(pos))
		if i0 >= n-1 {
			out[x] = smoothed[n-1]
			continue
		}
		frac := pos - float64(i0)
		// Cosine interpolation for organic S-curve slope between samples.
		mu := (1.0 - math.Cos(frac*math.Pi)) * 0.5
		out[x] = smoothed[i0]*(1.0-mu) + smoothed[i0+1]*mu
	}
	return out
}

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

// BrailleChart renders a large multi-row gotop-style Unicode Braille line chart
// of `width` character columns (2*width horizontal dot samples) by `height`
// character rows (4*height vertical dot levels). Before plotting, it applies
// SmoothSeries + cosine wave interpolation so transitions appear as smooth
// curves rather than square steps.
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

	grid := make([][]rune, height)
	for r := range grid {
		row := make([]rune, width)
		for c := range row {
			row[c] = 0x2800
		}
		grid[r] = row
	}

	if len(values) == 0 {
		out := make([]string, height)
		for r := range grid {
			out[r] = string(grid[r])
		}
		return out
	}

	wave := interpolateWave(values, totalDotsX)

	yTopFor := func(v float64) int {
		if v <= 0 || maxVal <= 0 {
			return totalDotsY - 1
		}
		ratio := v / maxVal
		if ratio > 1 {
			ratio = 1
		}
		yBottom := int(math.Round(ratio * float64(totalDotsY-1)))
		return clampInt((totalDotsY-1)-yBottom, 0, totalDotsY-1)
	}

	setDot := func(x, yTop int) {
		charCol := x / 2
		charRow := yTop / 4
		subRow := yTop % 4
		if charRow < 0 || charRow >= height || charCol < 0 || charCol >= width {
			return
		}
		if x%2 == 0 {
			grid[charRow][charCol] |= brailleDotLeft[subRow]
		} else {
			grid[charRow][charCol] |= brailleDotRight[subRow]
		}
	}

	prevY := yTopFor(wave[0])
	for x := 0; x < totalDotsX; x++ {
		curY := yTopFor(wave[x])
		setDot(x, curY)
		if curY < prevY-1 {
			for y := curY + 1; y < prevY; y++ {
				setDot(x, y)
			}
		} else if curY > prevY+1 {
			for y := prevY + 1; y < curY; y++ {
				setDot(x, y)
			}
		}
		prevY = curY
	}

	out := make([]string, height)
	for r := range grid {
		out[r] = string(grid[r])
	}
	return out
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
	s.net[idx] = s.rxRateBps + s.txRateBps
	s.disk[idx] = s.readRate + s.writeRate
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
		return fmt.Sprintf("%7s/s", humanBytes(uint64(max(0, int64(val)))))
	default:
		return fmt.Sprintf("%8.0f%%", val)
	}
}
