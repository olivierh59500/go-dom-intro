package domintro

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"

	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/sound"
)

func TestAnimatedStarsPreserveSeededFrameAndRespawnSequence(t *testing.T) {
	const seed = 1989
	oldRandom, newRandom := rand.New(rand.NewSource(seed)), rand.New(rand.NewSource(seed))
	config, err := presets.DOMAnimatedStars(nil, presets.DefaultDOMStarOptions(newRandom.Float64))
	if err != nil {
		t.Fatal(err)
	}
	field, err := motion.NewFrameField(config.Motion)
	if err != nil {
		t.Fatal(err)
	}
	var old [8][4]float64
	for i := range old {
		old[i] = [4]float64{math.Round(oldRandom.Float64()*9) * 64,
			math.Round(oldRandom.Float64() * 354), math.Round(oldRandom.Float64()*4) + 4,
			math.Round(oldRandom.Float64() * 10)}
	}
	check := func(tick int) {
		t.Helper()
		for i, item := range field.Samples() {
			want := old[i]
			if item.X != want[0] || item.Y != want[1] || item.Rate != 1/want[2] || math.Abs(item.Phase-want[3]) > 1e-12 {
				t.Fatalf("tick %d sprite %d: %+v, want %v", tick, i, item, want)
			}
		}
	}
	check(0)
	for tick := 1; tick <= 1000; tick++ {
		for i := range old {
			old[i][3] += 1 / old[i][2]
			if old[i][3] >= 9 {
				old[i] = [4]float64{math.Round(oldRandom.Float64()*9) * 64,
					math.Round(oldRandom.Float64() * 354), math.Round(oldRandom.Float64()*4) + 4, 0}
			}
		}
		if err := field.Step(); err != nil {
			t.Fatal(err)
		}
		check(tick)
	}
}

func TestTextTilesStripsControlCodesAndPreservesSpacing(t *testing.T) {
	got := textTiles(" A^Cs2;B ")
	want := []int{-1, tileIndex('A'), tileIndex('B'), -1}
	if len(got) != len(want) {
		t.Fatalf("tiles length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tiles[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMusicStreamReadProducesStereoWithoutAllocating(t *testing.T) {
	player, err := sound.Open("music.ym", ymData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := player.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	buffer := make([]byte, 4096*4)
	read := func() {
		n, err := player.Read(buffer)
		if err != nil {
			t.Fatalf("Read: %v", err)
		}
		if n != len(buffer) {
			t.Fatalf("Read bytes = %d, want %d", n, len(buffer))
		}
	}

	read()
	for i := 0; i < len(buffer); i += 4 {
		left := binary.LittleEndian.Uint16(buffer[i : i+2])
		right := binary.LittleEndian.Uint16(buffer[i+2 : i+4])
		if left != right {
			t.Fatalf("frame %d is not mono duplicated to stereo: %d != %d", i/4, left, right)
		}
	}

	if allocations := testing.AllocsPerRun(20, read); allocations != 0 {
		t.Fatalf("Read allocations = %v, want 0", allocations)
	}
}
