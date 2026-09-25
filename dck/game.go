// Package domintro implements the Dom intro remake.
package domintro

import (
	"bytes"
	"github.com/olivierh59500/democonstructionkit/presets"
	originalassets "go-dom-intro"
	"image"
	"image/color"

	"github.com/olivierh59500/democonstructionkit/composite"
	"github.com/olivierh59500/democonstructionkit/motion"
	"github.com/olivierh59500/democonstructionkit/scrolling"
	"github.com/olivierh59500/democonstructionkit/scrolltext"
	"github.com/olivierh59500/democonstructionkit/sound"
	"strconv"

	imagedraw "image/draw"

	_ "image/png"
	"log"
	"math"
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
	starFrames []*ebiten.Image
	backSlices []*ebiten.Image
	mergeTop   *ebiten.Image
	font0      *ebiten.Image
	font1      *ebiten.Image
	font2      *ebiten.Image
	font3      *ebiten.Image

	scrollCanvas1 *ebiten.Image
	scrollCanvas2 *ebiten.Image
	scrollCanvas3 *ebiten.Image
	scrollCanvas4 *ebiten.Image
	offScroll     *ebiten.Image
	mergeCanvas   *ebiten.Image

	scrollText1 *ScrollText
	scrollText2 *ScrollText
	scrollText3 *ScrollText
	scrollText4 *ScrollText

	audioContext *audio.Context
	audioPlayer  *audio.Player
	musicStream  *sound.Stream
	audioReady   bool
	musicStarted bool

	rng              *rand.Rand
	stop             int
	backgroundMotion *motion.WrapBank
	rasterMotion     *motion.WrapBank
	motionErr        error
	actSize          int
	spinc            float64
	infStars         [8][4]float64

	// Scroll text state
	fullText    string
	fontProgram *scrolltext.FontProgram
}

type ScrollText struct {
	renderer *scrolling.Scrolling
	canvas   *ebiten.Image
	glyphs   []*ebiten.Image
	tiles    []int
	speed    float64
	offset   float64
	tileW    int
	scaleX   float64
	scaleY   float64
}

func NewGame() *Game {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	g := &Game{
		scrollCanvas1: ebiten.NewImage(640, 32),
		scrollCanvas2: ebiten.NewImage(640, 64),
		scrollCanvas3: ebiten.NewImage(640, 128),
		scrollCanvas4: ebiten.NewImage(640, 384),
		offScroll:     ebiten.NewImage(640, 400),
		mergeCanvas:   ebiten.NewImage(640, 400),

		rng:     rng,
		stop:    1,
		actSize: 0,
		spinc:   1,
	}
	g.backgroundMotion, g.motionErr = motion.NewWrapBank(motion.WrapBankConfig{
		Start: []float64{0}, Velocity: []float64{2},
		Upper: &motion.WrapLimit{Boundary: 654, Restart: 0, Inclusive: true},
	})
	if g.motionErr != nil {
		return g
	}
	g.rasterMotion, g.motionErr = motion.NewWrapBank(motion.WrapBankConfig{
		Start: []float64{200}, Velocity: []float64{-1},
		Lower: &motion.WrapLimit{Boundary: 0, Restart: 200, Inclusive: true},
	})
	if g.motionErr != nil {
		return g
	}

	g.loadAssets()
	g.cacheSubImages()

	// Initialize scroll text state
	g.fullText = g.getFullText()
	g.preAnalyzeFontChanges()

	// Create scroll texts - all use the same source text and base font tiles
	smallText := g.fontProgram.MaskedText("0", ' ')
	normalText := g.fontProgram.MaskedText("1", ' ')
	mediumText := g.fontProgram.MaskedText("2", ' ')
	bigText := g.fontProgram.MaskedText("3", ' ')

	g.scrollText1 = g.newScrollText(g.scrollCanvas1, g.font0, 40, 32, 1.0, 1.0, smallText)
	g.scrollText2 = g.newScrollText(g.scrollCanvas2, g.font1, 40, 32, 2.0, 2.0, normalText)
	g.scrollText3 = g.newScrollText(g.scrollCanvas3, g.font2, 40, 32, 4.0, 4.0, mediumText)
	g.scrollText4 = g.newScrollText(g.scrollCanvas4, g.font3, 40, 32, 8.0, 12.0, bigText)

	g.setSpeed()

	for i := 0; i < 8; i++ {
		g.infStars[i][0] = math.Round(g.rng.Float64()*9) * 64
		g.infStars[i][1] = math.Round(g.rng.Float64() * 354)
		g.infStars[i][2] = math.Round(g.rng.Float64()*4) + 4
		g.infStars[i][3] = math.Round(g.rng.Float64() * 10)
	}

	return g
}

func (g *Game) loadAssets() {
	g.starsImage = g.loadImage("rep_stars.png")
	g.logoImage = g.loadImage("rep_ik+_logo.png")
	g.scrollRast = g.loadRepeatedImage("rep_ik+_rast1.png", 640)
	g.backRast = g.loadRepeatedImage("rep_ik+_rast2.png", screenWidth)

	// Use only font0 and scale it for other sizes (font3 is non-uniform: 8x width, 12x height).
	baseFontImage := g.loadImage("rep_ik+_font0.png")
	g.font0 = baseFontImage // 1x scale
	g.font1 = baseFontImage // Will be scaled 2x during rendering
	g.font2 = baseFontImage // Will be scaled 4x during rendering
	g.font3 = baseFontImage // Will be scaled 8x/12x during rendering
}

func (g *Game) cacheSubImages() {
	const (
		starWidth       = 64
		starHeight      = 46
		backSliceHeight = 36
	)

	starCount := g.starsImage.Bounds().Dx() / starWidth
	g.starFrames = make([]*ebiten.Image, starCount)
	for tile := range g.starFrames {
		rect := image.Rect(tile*starWidth, 0, (tile+1)*starWidth, starHeight)
		g.starFrames[tile] = g.starsImage.SubImage(rect).(*ebiten.Image)
	}

	maxBackSliceY := g.backRast.Bounds().Dy() - backSliceHeight
	g.backSlices = make([]*ebiten.Image, maxBackSliceY/2+1)
	for index := range g.backSlices {
		y := index * 2
		rect := image.Rect(0, y, screenWidth, y+backSliceHeight)
		g.backSlices[index] = g.backRast.SubImage(rect).(*ebiten.Image)
	}

	g.mergeTop = g.mergeCanvas.SubImage(image.Rect(0, 0, g.mergeCanvas.Bounds().Dx(), 2)).(*ebiten.Image)
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

func (g *Game) newScrollText(canvas *ebiten.Image, font *ebiten.Image, tileW, tileH int, scaleX, scaleY float64, text string) *ScrollText {
	glyphCount := font.Bounds().Dy() / tileH
	glyphs, err := scrolling.GridImages(font, image.Pt(tileW, tileH), 1, glyphCount)
	if err != nil {
		panic(err)
	}

	return &ScrollText{
		canvas: canvas,
		glyphs: glyphs,
		tiles:  textTiles(text),
		tileW:  tileW,
		scaleX: scaleX,
		scaleY: scaleY,
		offset: float64(canvas.Bounds().Dx()),
	}
}

func textTiles(text string) []int {
	tokens, err := scrolltext.Parse(text, scrolltext.DomSizes)
	if err != nil {
		panic(err)
	}
	tiles := make([]int, 0, len(text))
	for _, token := range tokens {
		if token.Kind != scrolltext.Text {
			continue
		}
		for _, r := range token.Text {
			if r == ' ' {
				tiles = append(tiles, -1)
			} else {
				tiles = append(tiles, tileIndex(r))
			}
		}
	}
	return tiles
}

var tileIndex = func() func(rune) int {
	lookup, err := presets.TileLookup("go-dom-intro", true)
	if err != nil {
		panic(err)
	}
	return func(r rune) int { index, _ := lookup(r); return index }
}()

func (g *Game) getFullText() string {
	spc0 := "                 "
	spc1 := "         "
	spc2 := "     "
	spc3 := "   "

	text := "          THE UNION IS PROUD TO PRESENT YOU :" + spc0 + "^Cs2;INTERNATIONAL KARATE PLUS" + spc2 + "^Cs0;CRACKED  BY" + spc0 + "^Cs3;DOM AND CORWIN" + spc3 + "^Cs1;FROM THE" + spc1 + "^Cs3;REPLICANTS AND DMA" + spc3
	text += "^Cs1; PRESS F1-F5 AND SEE (IF YOU CAN !!!!) AND LIST..........    A SPECIAL HI TO WILD-XEROX OR RANK-COPPER MY MASTER!!!!!ARF.... HEEEUUUU JUST A LITTLE QUESTION : WHO HAVE" + spc1
	text += "^Cs2;BARBARIAN 2 ????????" + spc2 + "^Cs1;RRRRHHHHHAAAAAAAAAAA!!!!!! ANYBODY ????? I NEED BLOOD RRRHHAAAA!!!!! NEED HEAD !!!!! OOOUUUIIIINNNN I WEEP .. I CRY...... I RAVE , I'M DELIRIOUS I'M CAUGHT IN THE ACT-HANDED!!!!!!!" + spc1
	text += "^Cs0;OK KO I STOP, I RESET, I BREAK, I DRINK,I FLY, I CR...-CR... HIHIHI FINALLY I SAY :" + spc0 + "^Cs3;SHEAT" + spc3 + "    ^Cs2;HEY HAVE-YOU CANAL PLUS??????????    WHAT ???????    I SAY CANAL PLUS    BORDEL !! (IN FRENCH)"
	text += " YOU DON'T HAVE !!!! BUY THIS AND YOU WILL SEE MY MASTER : I NAME : RANK-COOPER ARF ARF HE TURN ONE'S BACK ON THE CAMERA    OOOUUFF!!!HIHI GGGGGGGGGOOOOOOOOODDDDDDDDD" + spc2 + "^Cs1;IT'S ALL FOR DAY......" + spc1
	text += "^Cs0;REMEMBER YOU BARBARIAN 2 AND CANAL PLUS AND MY MASTER OF COURSE........ HI TO : ALL MEMBERS OF DMA(ESPECIALLY LOCKBUSTER FOR ORIGINAL), DELTA FORCE, TEX, BLADE RUNNERS, CHON-CHON, ALDO, ST-CONNEXION, THE HOBBIT BROTHERS, "
	text += "ABC 85, THE BARBARIANS......." + spc0
	text += "^Cs0;              "

	return text
}

func (g *Game) setSpeed() {
	switch g.actSize {
	case 0:
		g.scrollText1.speed = 8 * g.spinc
		g.scrollText2.speed = 16 * g.spinc
		g.scrollText3.speed = 32 * g.spinc
		g.scrollText4.speed = 64 * g.spinc
	case 1:
		g.scrollText1.speed = 4 * g.spinc
		g.scrollText2.speed = 8 * g.spinc
		g.scrollText3.speed = 16 * g.spinc
		g.scrollText4.speed = 32 * g.spinc
	case 2:
		g.scrollText1.speed = 2 * g.spinc
		g.scrollText2.speed = 4 * g.spinc
		g.scrollText3.speed = 8 * g.spinc
		g.scrollText4.speed = 16 * g.spinc
	case 3:
		g.scrollText1.speed = 1 * g.spinc
		g.scrollText2.speed = 2 * g.spinc
		g.scrollText3.speed = 4 * g.spinc
		g.scrollText4.speed = 8 * g.spinc
	}
}

func (st *ScrollText) advance() {
	st.offset -= st.speed
	if len(st.tiles) == 0 {
		return
	}

	scaledTileW := float64(st.tileW) * st.scaleX
	totalWidth := float64(len(st.tiles)) * scaledTileW
	if totalWidth > 0 && st.offset <= -totalWidth {
		st.offset += totalWidth + float64(st.canvas.Bounds().Dx())
	}
}

func (g *Game) activeScrollText() *ScrollText {
	switch g.actSize {
	case 1:
		return g.scrollText2
	case 2:
		return g.scrollText3
	case 3:
		return g.scrollText4
	default:
		return g.scrollText1
	}
}

func (st *ScrollText) drawAtOffset(offset float64) {
	st.canvas.Clear()
	if st.renderer == nil {
		images := make([]*ebiten.Image, len(st.tiles))
		for i, tile := range st.tiles {
			if tile >= 0 && tile < len(st.glyphs) {
				images[i] = st.glyphs[tile]
			}
		}
		var err error
		st.renderer, err = scrolling.FromImages(images, float64(st.tileW))
		if err != nil {
			panic(err)
		}
	}
	state := scrolling.IdentityState()
	state.X = offset
	state.ScaleX = st.scaleX
	state.ScaleY = st.scaleY
	if offset < 0 {
		state.First = int(math.Floor(-offset / (float64(st.tileW) * st.scaleX)))
	}
	state.Map = func(s scrolling.Sample, op *ebiten.DrawImageOptions) bool {
		return s.X < float64(st.canvas.Bounds().Dx())
	}
	st.renderer.DrawAt(st.canvas, state)
}

func (g *Game) preAnalyzeFontChanges() {
	var err error
	g.fontProgram, err = scrolltext.NewFontProgram(g.fullText, scrolltext.DomSizes, "0")
	if err != nil {
		panic(err)
	}
}

func (g *Game) updateActSizeFromScroll() {
	if g.fontProgram.Len() == 0 {
		return
	}

	st := g.scrollText1
	switch g.actSize {
	case 1:
		st = g.scrollText2
	case 2:
		st = g.scrollText3
	case 3:
		st = g.scrollText4
	}
	if st == nil {
		return
	}
	tileW := float64(st.tileW) * st.scaleX
	if tileW <= 0 {
		return
	}
	leftGlyph := int(math.Floor(-st.offset / tileW))
	if leftGlyph < 0 {
		leftGlyph = 0
	}
	visibleGlyphs := int(math.Ceil(float64(st.canvas.Bounds().Dx())/tileW)) + 1
	glyphPos := leftGlyph + visibleGlyphs
	if glyphPos >= g.fontProgram.Len() {
		glyphPos = g.fontProgram.Len() - 1
	}

	size, err := strconv.Atoi(g.fontProgram.FontAt(glyphPos))
	if err != nil {
		panic(err)
	}
	if size != g.actSize {
		g.actSize = size
		g.setSpeed()
	}
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
		g.setSpeed()
	}
	if ebiten.IsKeyPressed(ebiten.KeyF2) {
		if g.spinc > 1 {
			g.spinc--
		} else if g.spinc == 0.5 {
			g.spinc = 0.25
		} else if g.spinc == 1 {
			g.spinc = 0.5
		}
		g.setSpeed()
	}

	g.backgroundMotion.Step()
	g.rasterMotion.Step()

	for i := 0; i < 8; i++ {
		g.infStars[i][3] += 1 / g.infStars[i][2]
		if g.infStars[i][3] >= 9 {
			g.infStars[i][0] = math.Round(g.rng.Float64()*9) * 64
			g.infStars[i][1] = math.Round(g.rng.Float64() * 354)
			g.infStars[i][2] = math.Round(g.rng.Float64()*4) + 4
			g.infStars[i][3] = 0
		}
	}

	if g.scrollText1 != nil {
		g.scrollText1.advance()
		baseOffset := g.scrollText1.offset
		baseWidth := float64(g.scrollText1.canvas.Bounds().Dx())
		if g.scrollText2 != nil {
			g.scrollText2.offset = baseOffset*g.scrollText2.scaleX + (1-g.scrollText2.scaleX)*baseWidth
		}
		if g.scrollText3 != nil {
			g.scrollText3.offset = baseOffset*g.scrollText3.scaleX + (1-g.scrollText3.scaleX)*baseWidth
		}
		if g.scrollText4 != nil {
			g.scrollText4.offset = baseOffset*g.scrollText4.scaleX + (1-g.scrollText4.scaleX)*baseWidth
		}
	}
	g.updateActSizeFromScroll()
	activeScroll := g.activeScrollText()
	activeScroll.drawAtOffset(activeScroll.offset)

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.motionErr != nil {
		screen.Fill(colornames.Black)
		return
	}
	if g.stop > 0 {
		if g.offScroll != nil {
			g.offScroll.Clear()
		}

		switch g.actSize {
		case 0:
			if g.scrollCanvas1 != nil {
				drawRepeatedVertically(g.offScroll, g.scrollCanvas1, 2, 36, 11)
			}
		case 1:
			if g.scrollCanvas2 != nil {
				drawRepeatedVertically(g.offScroll, g.scrollCanvas2, 2, 66, 6)
			}
		case 2:
			if g.scrollCanvas3 != nil {
				drawRepeatedVertically(g.offScroll, g.scrollCanvas3, 0, 134, 3)
			}
		case 3:
			if g.scrollCanvas4 != nil {
				drawImageAt(g.offScroll, g.scrollCanvas4, 0, 4)
			}
		}

		if g.mergeCanvas != nil {
			g.mergeCanvas.Clear()
		}

		if g.scrollRast != nil {
			drawImageAt(g.mergeCanvas, g.scrollRast, 0, int(g.rasterMotion.At(0))-200)
			drawImageAt(g.mergeCanvas, g.scrollRast, 0, int(g.rasterMotion.At(0)))
			drawImageAt(g.mergeCanvas, g.scrollRast, 0, int(g.rasterMotion.At(0))+200)
		}

		if g.mergeCanvas != nil && g.offScroll != nil {
			op := &ebiten.DrawImageOptions{}
			op.Blend = ebiten.BlendDestinationIn
			op.GeoM.Translate(0, 2)
			g.mergeCanvas.DrawImage(g.offScroll, op)
			g.mergeTop.Clear()
		}

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

		if g.mergeCanvas != nil {
			op2 := &ebiten.DrawImageOptions{}
			op2.GeoM.Translate(64, 60)
			screen.DrawImage(g.mergeCanvas, op2)
		}

		if g.logoImage != nil {
			opLogo := &ebiten.DrawImageOptions{}
			opLogo.GeoM.Translate(64, 60+36)
			screen.DrawImage(g.logoImage, opLogo)
		}

		if g.starsImage != nil {
			for i := 0; i < 8; i++ {
				tile := int(math.Round(g.infStars[i][3]))
				if tile >= 0 && tile < len(g.starFrames) {
					drawImageAt(screen, g.starFrames[tile], 64+int(g.infStars[i][0]), 60+int(g.infStars[i][1]))
				}
			}
		}
	}
}

func drawImageAt(dest, src *ebiten.Image, x, y int) {
	op := ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	composite.Instance{Image: src, Options: op}.Draw(dest)
}

func drawRepeatedVertically(dest, src *ebiten.Image, y, step, count int) {
	if src == nil || dest == nil {
		return
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(0, float64(y))
	for range count {
		dest.DrawImage(src, &op)
		op.GeoM.Translate(0, float64(step))
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}
