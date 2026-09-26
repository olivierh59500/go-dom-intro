package textdata

import (
	"math"
	"strconv"
	"testing"

	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/scrolltext"
)

func TestFourSizeClockMatchesAuthoredTextAndLegacyCueTiming(t *testing.T) {
	program, err := scrolltext.NewFontProgram(Message(), scrolltext.DomSizes, "0")
	if err != nil {
		t.Fatal(err)
	}
	fonts := make([]int, program.Len())
	for i := range fonts {
		fonts[i], err = strconv.Atoi(program.FontAt(i))
		if err != nil || fonts[i] < 0 || fonts[i] > 3 {
			t.Fatalf("invalid font cue at glyph %d: %v", i, err)
		}
	}
	clock, err := motion.NewScaledTextClock(motion.ScaledTextClockConfig{
		FontAt: fonts, Scales: []float64{1, 2, 4, 8}, BaseSpeeds: []float64{8, 4, 2, 1},
		ViewportWidth: 640, TileWidth: 40, StartOffset: 640,
		SpeedMultiplier: 1, Lookahead: 1, WrapInclusive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	oldOffset, oldBank, multiplier := 640.0, 0, 1.0
	var bankSeen [4]bool
	wraps := 0
	for tick := 1; tick <= 100000; tick++ {
		switch tick {
		case 500:
			multiplier = .5
		case 1000:
			multiplier = 4
		case 10000:
			multiplier = 1
		case 30000:
			multiplier = 2
		}
		if err := clock.SetSpeedMultiplier(multiplier); err != nil {
			t.Fatal(err)
		}
		oldOffset -= []float64{8, 4, 2, 1}[oldBank] * multiplier
		if oldOffset <= -float64(len(fonts)*40) {
			oldOffset += float64(len(fonts)*40 + 640)
			wraps++
		}
		scales := [...]float64{1, 2, 4, 8}
		var offsets [4]float64
		for bank, scale := range scales {
			offsets[bank] = oldOffset*scale + (1-scale)*640
		}
		cell := 40 * scales[oldBank]
		left := int(math.Floor(-offsets[oldBank] / cell))
		if left < 0 {
			left = 0
		}
		glyph := min(left+int(math.Ceil(640/cell))+1, len(fonts)-1)
		oldBank = fonts[glyph]
		bankSeen[oldBank] = true
		clock.Step()
		if clock.ActiveBank() != oldBank || clock.LookaheadIndex() != glyph {
			t.Fatalf("tick %d: active bank or lookahead changed", tick)
		}
		for bank, want := range offsets {
			if got := clock.Offset(bank); math.Abs(got-want) > 1e-9 {
				t.Fatalf("tick %d bank %d: %.12f != %.12f", tick, bank, got, want)
			}
		}
	}
	if wraps < 2 || !bankSeen[0] || !bankSeen[1] || !bankSeen[2] || !bankSeen[3] {
		t.Fatalf("choreography coverage: wraps=%d banks=%v", wraps, bankSeen)
	}
}
