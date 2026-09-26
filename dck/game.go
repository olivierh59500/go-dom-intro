// Package domintro implements the Dom intro remake.
package domintro

import (
	"bytes"
	kit "github.com/olivierh59500/democonstructionkit"
	"github.com/olivierh59500/democonstructionkit/presets"
	"github.com/olivierh59500/democonstructionkit/sprites"
	originalassets "go-dom-intro"
	"image"
	"image/color"

	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/effects"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/sound"
	"go-dom-intro/dck/internal/textdata"

	imagedraw "image/draw"

	_ "image/png"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	audio "github.com/olivierh59500/democonstructionkit/sound/output"
	"golang.org/x/image/colornames"
)

var assets = originalassets.
	DCKAssetAssets()

var ymData = originalassets.DCKAssetYmData()

const (
	screenWidth  = 768
	screenHeight = 540
	sampleRate   = 48000
)

type Game struct {
	starsImage *ebiten.Image
	logoImage  *ebiten.Image
	scrollRast *ebiten.Image
	backRast   *ebiten.Image
	starAtlas  *sprites.Atlas
	stars      *sprites.AnimatedField
	backSlices []*ebiten.Image
	baseFont   *ebiten.Image

	scroll   *scrolling.Scrolling
	sizeBank *scrolling.SizeBank
	textMask *effects.Mask

	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool
	musicStarted bool

	stop             int
	backgroundMotion *motion.WrapBank
	motionErr        error
	spinc            float64
}

func NewGame() *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	g := &Game{stop: 1, spinc: 1}
	g.backgroundMotion, g.motionErr = motion.NewWrapBank(motion.WrapBankConfig{
		Start: []float64{0}, Velocity: []float64{2},
		Upper: &motion.WrapLimit{Boundary: 654, Restart: 0, Inclusive: true},
	})
	if g.motionErr != nil {
		return g
	}
	g.loadAssets()
	g.cacheSubImages()
	if g.motionErr != nil {
		return g
	}

	sizeConfig, err := presets.DOMSizeBank(textdata.Message(), g.baseFont)
	if err != nil {
		g.motionErr = err
		return g
	}
	g.scroll, g.motionErr = scrolling.New(scrolling.Config{SizeBank: &sizeConfig})
	if g.motionErr != nil {
		return g
	}
	g.sizeBank = g.scroll.SizeBankController()
	raster, err := composite.NewRasterOverlay(presets.DOMRasterCopies(g.scrollRast))
	if err != nil {
		g.motionErr = err
		return g
	}
	g.textMask, g.motionErr = effects.NewMaskWith(presets.DOMScrollMask(raster, g.scroll))
	if g.motionErr != nil {
		return g
	}

	starConfig, err := presets.DOMAnimatedStars(g.starAtlas.Tiles, presets.DefaultDOMStarOptions(rng.Float64))
	if err != nil {
		g.motionErr = err
		return g
	}
	g.stars, g.motionErr = sprites.NewAnimatedField(starConfig)
	if g.motionErr != nil {
		return g
	}

	return g
}

func (g *Game) loadAssets() {
	g.starsImage = g.loadImage("rep_stars.png")
	g.logoImage = g.loadImage("rep_ik+_logo.png")
	g.scrollRast = g.loadRepeatedImage("rep_ik+_rast1.png", 640)
	g.backRast = g.loadRepeatedImage("rep_ik+_rast2.png", screenWidth)

	g.baseFont = g.loadImage("rep_ik+_font0.png")
}

func (g *Game) cacheSubImages() {
	const (
		starWidth       = 64
		starHeight      = 46
		backSliceHeight = 36
	)

	g.starAtlas, g.motionErr = sprites.NewAtlas(sprites.AtlasConfig{
		Image: g.starsImage, TileW: starWidth, TileH: starHeight,
	})
	if g.motionErr != nil {
		return
	}

	maxBackSliceY := g.backRast.Bounds().Dy() - backSliceHeight
	g.backSlices = make([]*ebiten.Image, maxBackSliceY/2+1)
	for index := range g.backSlices {
		y := index * 2
		rect := image.Rect(0, y, screenWidth, y+backSliceHeight)
		g.backSlices[index] = g.backRast.SubImage(rect).(*ebiten.Image)
	}

}

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)

	music, err := sound.Open("music.ym", ymData, sound.Options{SampleRate: sampleRate, Loop: true, PCMFormat: sound.PCM16, Gain: 0.5})
	if err != nil {
		log.Printf("Failed to open music: %v", err)
		return
	}
	g.musicStream = music

	player, err := g.audioContext.NewPlayer(music)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		if closeErr := g.musicStream.Close(); closeErr != nil {
			log.Printf("Failed to close music stream: %v", closeErr)
		}
		g.musicStream = nil
		return
	}
	g.audioPlayer = player
}

func (g *Game) startMusic() {
	if g.musicStarted || g.audioPlayer == nil {
		return
	}
	g.audioPlayer.Play()
	g.musicStarted = true
}

func (g *Game) loadImage(name string) *ebiten.Image {
	return ebiten.NewImageFromImage(g.loadDecodedImage(name))
}

func (g *Game) loadRepeatedImage(name string, width int) *ebiten.Image {
	source := g.loadDecodedImage(name)
	bounds := source.Bounds()
	if width <= bounds.Dx() || bounds.Dx() <= 0 {
		return ebiten.NewImageFromImage(source)
	}

	repeated := image.NewRGBA(image.Rect(0, 0, width, bounds.Dy()))
	for x := 0; x < width; x += bounds.Dx() {
		tileWidth := min(bounds.Dx(), width-x)
		destination := image.Rect(x, 0, x+tileWidth, bounds.Dy())
		imagedraw.Draw(repeated, destination, source, bounds.Min, imagedraw.Src)
	}
	return ebiten.NewImageFromImage(repeated)
}

func (g *Game) loadDecodedImage(name string) image.Image {
	b, err := assets.ReadFile("assets/" + name)
	if err != nil {
		log.Printf("Failed to read asset %s: %v", name, err)
		return solidImage(100, 100, colornames.Red)
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		log.Printf("Failed to decode asset %s: %v", name, err)
		return solidImage(100, 100, colornames.Red)
	}

	// Log image dimensions
	bounds := img.Bounds()
	//	log.Printf("Loaded asset %s: %dx%d", name, bounds.Dx(), bounds.Dy())

	// Check if image is too large for atlas (Ebiten limit is around 16384 pixels in any dimension)
	maxSize := 4096
	if bounds.Dx() > maxSize || bounds.Dy() > maxSize {
		log.Printf("WARNING: Image %s is too large (%dx%d), cropping to manageable size", name, bounds.Dx(), bounds.Dy())

		// For font images, crop to a usable portion (top part contains the characters)
		if strings.Contains(name, "font") {
			fontWidth := bounds.Dx()
			fontHeight := min(bounds.Dy(), maxSize) // Take first 4096 pixels of height

			croppedImg := image.NewRGBA(image.Rect(0, 0, fontWidth, fontHeight))
			imagedraw.Draw(croppedImg, croppedImg.Bounds(), img, bounds.Min, imagedraw.Src)

			log.Printf("Cropped font %s to %dx%d", name, fontWidth, fontHeight)
			return croppedImg
		}

		return solidImage(min(bounds.Dx(), maxSize), min(bounds.Dy(), maxSize), colornames.Gray)
	}

	return img
}

func solidImage(width, height int, fill color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	imagedraw.Draw(img, img.Bounds(), image.NewUniform(fill), image.Point{}, imagedraw.Src)
	return img
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (g *Game) Update() error {
	if g.motionErr != nil {
		return g.motionErr
	}
	// On Android, NewGame is called while the native library is loading and
	// before the Activity has installed Ebitengine's context. Open audio only
	// once the game loop is running.
	if !g.audioReady {
		g.audioReady = true
		g.initAudio()
		g.startMusic()
	}

	if ebiten.IsKeyPressed(ebiten.KeyF1) {
		if g.spinc < 4 && g.spinc >= 1 {
			g.spinc++
		} else if g.spinc == 0.5 {
			g.spinc = 1
		} else if g.spinc == 0.25 {
			g.spinc = 0.5
		}
		if err := g.sizeBank.SetSpeedMultiplier(g.spinc); err != nil {
			return err
		}
	}
	if ebiten.IsKeyPressed(ebiten.KeyF2) {
		if g.spinc > 1 {
			g.spinc--
		} else if g.spinc == 0.5 {
			g.spinc = 0.25
		} else if g.spinc == 1 {
			g.spinc = 0.5
		}
		if err := g.sizeBank.SetSpeedMultiplier(g.spinc); err != nil {
			return err
		}
	}

	g.backgroundMotion.Step()

	if g.stars != nil {
		if err := g.stars.Update(kit.Frame{}); err != nil {
			return err
		}
	}

	if g.textMask != nil {
		if err := g.textMask.Update(kit.Frame{}); err != nil {
			return err
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.motionErr != nil {
		screen.Fill(colornames.Black)
		return
	}
	if g.stop > 0 {
		screen.Fill(colornames.Black)

		if g.backRast != nil {
			for j := 0; j < 11; j++ {
				sy := int(g.backgroundMotion.At(0)) + j*4
				index := sy / 2
				if index < len(g.backSlices) {
					drawImageAt(screen, g.backSlices[index], 0, 60+2+j*36)
				}
			}
		}

		if g.textMask != nil {
			g.textMask.Draw(screen)
		}

		if g.logoImage != nil {
			opLogo := &ebiten.DrawImageOptions{}
			opLogo.GeoM.Translate(64, 60+36)
			screen.DrawImage(g.logoImage, opLogo)
		}

		if g.stars != nil {
			g.stars.Draw(screen)
		}
	}
}

func drawImageAt(dest, src *ebiten.Image, x, y int) {
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	composite.Instance{Image: src, Options: op}.Draw(dest)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

// Cleanup releases DCK surfaces and audio streams when the screen closes.
func (g *Game) Cleanup() {
	if g.textMask != nil {
		_ = g.textMask.Close()
		g.textMask = nil
	} else if g.scroll != nil {
		_ = g.scroll.Close()
	}
	g.scroll = nil
	g.sizeBank = nil
	if g.audioPlayer != nil {
		_ = g.audioPlayer.Close()
		g.audioPlayer = nil
	}
	if g.musicStream != nil {
		_ = g.musicStream.Close()
		g.musicStream = nil
	}
}
