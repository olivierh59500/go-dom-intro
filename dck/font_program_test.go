package domintro

import (
	"strconv"
	"strings"
	"testing"
)

func TestSharedFontProgramPreservesEveryMessageSlot(t *testing.T) {
	g := &Game{}
	g.fullText = g.getFullText()
	g.preAnalyzeFontChanges()
	var fonts []int
	var masks [4]strings.Builder
	active := 0
	for i := 0; i < len(g.fullText); {
		if i+4 < len(g.fullText) && g.fullText[i:i+3] == "^Cs" && g.fullText[i+4] == ';' && g.fullText[i+3] >= '0' && g.fullText[i+3] <= '3' {
			active = int(g.fullText[i+3] - '0')
			i += 5
			continue
		}
		fonts = append(fonts, active)
		for bank := range masks {
			b := byte(' ')
			if bank == active {
				b = g.fullText[i]
			}
			masks[bank].WriteByte(b)
		}
		i++
	}
	if len(fonts) != g.fontProgram.Len() {
		t.Fatalf("glyph count %d != %d", g.fontProgram.Len(), len(fonts))
	}
	for bank := range masks {
		if got := g.fontProgram.MaskedText(strconv.Itoa(bank), ' '); got != masks[bank].String() {
			t.Fatalf("bank %d changed", bank)
		}
	}
	for i, want := range fonts {
		if got := g.fontProgram.FontAt(i); got != strconv.Itoa(want) {
			t.Fatalf("font at %d = %s, want %d", i, got, want)
		}
	}
}
