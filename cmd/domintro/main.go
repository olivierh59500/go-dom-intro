package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	domintro "go-dom-intro"
)

func main() {
	ebiten.SetWindowSize(768, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Remake of the \"Dom intro\" in Golang + Ebiten")
	ebiten.SetScreenClearedEveryFrame(false)
	if err := ebiten.RunGame(newDrawOnUpdateGame(domintro.NewGame())); err != nil {
		log.Fatal(err)
	}
}
