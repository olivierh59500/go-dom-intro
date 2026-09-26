package domintro

import (
	"strconv"
	"strings"
	"testing"

	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/scrolltext"
	"go-dom-intro/dck/internal/textdata"
)

func TestSharedFontProgramPreservesEveryMessageSlot(t *testing.T) {
	text := textdata.Message()
	config, err := presets.DOMSizeBank(text, nil)
	if err != nil {
		t.Fatal(err)
	}
	program, err := scrolltext.NewFontProgram(config.Text, config.Controls, config.InitialFont)
	if err != nil {
		t.Fatal(err)
	}
	var fonts []int
	var masks [4]strings.Builder
	active := 0
	for i := 0; i < len(text); {
		if i+4 < len(text) && text[i:i+3] == "^Cs" && text[i+4] == ';' && text[i+3] >= '0' && text[i+3] <= '3' {
			active = int(text[i+3] - '0')
			i += 5
			continue
		}
		fonts = append(fonts, active)
		for bank := range masks {
			b := byte(' ')
			if bank == active {
				b = text[i]
			}
			masks[bank].WriteByte(b)
		}
		i++
	}
	if len(fonts) != program.Len() {
		t.Fatalf("glyph count %d != %d", program.Len(), len(fonts))
	}
	for bank := range masks {
		if got := program.MaskedText(strconv.Itoa(bank), ' '); got != masks[bank].String() {
			t.Fatalf("bank %d changed", bank)
		}
	}
	for i, want := range fonts {
		if got := program.FontAt(i); got != strconv.Itoa(want) {
			t.Fatalf("font at %d = %s, want %d", i, got, want)
		}
	}
}
