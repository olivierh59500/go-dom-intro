// Package mobile exposes the game to ebitenmobile.
package mobile

import (
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	domintro "go-dom-intro"
)

func init() {
	enginemobile.SetGame(domintro.NewGame())
}

// Dummy forces gomobile to include this package in the Android binding.
func Dummy() {}
