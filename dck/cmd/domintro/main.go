package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	domintro "go-dom-intro/dck"
)

func main() {
	ebiten.SetWindowSize(768, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Remake of the \"Dom intro\" in Golang + Ebiten")
	ebiten.SetScreenClearedEveryFrame(false)
	game := domintro.NewGame()
	defer game.Cleanup()
	if err := ebiten.RunGame(newDrawOnUpdateGame(game)); err != nil {
		log.Fatal(err)
	}
}
