package domintro

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/olivierh59500/democonstructionkit/sound"
)

// TestMusicPCMCompatibility preserves the audible level and PCM of the original adapter.
func TestMusicPCMCompatibility(t *testing.T) {
	player, err := sound.Open("music.ym", ymData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		t.Fatal(err)
	}
	defer player.Close()
	data := make([]byte, sampleRate*4)
	for start := 0; start < len(data); {
		end := min(start+4096, len(data))
		n, err := player.Read(data[start:end])
		if err != nil {
			t.Fatal(err)
		}
		if n != end-start {
			t.Fatalf("read %d/%d", n, end-start)
		}
		start = end
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != "fa3f25948e01ba28379df554f15b5ad3467dbdf635e043d665f2e46fddaab03f" {
		t.Fatalf("soundtrack PCM changed: %s", got)
	}
}
