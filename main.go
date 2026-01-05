package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/olivierh59500/ym-player/pkg/stsound"
	"golang.org/x/image/colornames"
)

//go:embed assets/*
var assets embed.FS

//go:embed assets/eliminator.ym
var ymData []byte

const (
	screenWidth  = 768
	screenHeight = 540
	sampleRate   = 44100
)

type Game struct {
	starsImage     *ebiten.Image
	logoImage      *ebiten.Image
	scrollRast     *ebiten.Image
	backRast       *ebiten.Image
	font0          *ebiten.Image
	font1          *ebiten.Image
	font2          *ebiten.Image
	font3          *ebiten.Image

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

	audioContext     *audio.Context
	audioPlayer      *audio.Player
	ymPlayer         *YMPlayer
	musicStarted     bool

	rng       *rand.Rand
	stop      int
	vbl       int
	posY      float64
	posY2     float64
	actSize   int
	spinc     float64
	infStars  [8][4]float64

	// Scroll text state
	fullText    string
	fontChanges []FontChange
	totalGlyphs int
}

type FontChange struct {
	position int
	newSize  int
}

// YMPlayer wraps a YM stream for Ebiten audio.
type YMPlayer struct {
	player       *stsound.StSound
	sampleRate   int
	buffer       []int16
	mutex        sync.Mutex
	position     int64
	totalSamples int64
	loop         bool
	volume       float64
}

func NewYMPlayer(data []byte, sampleRate int, loop bool) (*YMPlayer, error) {
	player := stsound.CreateWithRate(sampleRate)

	if err := player.LoadMemory(data); err != nil {
		player.Destroy()
		return nil, fmt.Errorf("failed to load YM data: %w", err)
	}

	player.SetLoopMode(loop)

	info := player.GetInfo()
	totalSamples := int64(info.MusicTimeInMs) * int64(sampleRate) / 1000

	return &YMPlayer{
		player:       player,
		sampleRate:   sampleRate,
		buffer:       make([]int16, 4096),
		totalSamples: totalSamples,
		loop:         loop,
		volume:       0.5,
	}, nil
}

func (y *YMPlayer) Read(p []byte) (n int, err error) {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	samplesNeeded := len(p) / 4
	outBuffer := make([]int16, samplesNeeded*2)

	processed := 0
	for processed < samplesNeeded {
		chunkSize := samplesNeeded - processed
		if chunkSize > len(y.buffer) {
			chunkSize = len(y.buffer)
		}

		if !y.player.Compute(y.buffer[:chunkSize], chunkSize) {
			if !y.loop {
				for i := processed * 2; i < len(outBuffer); i++ {
					outBuffer[i] = 0
				}
				err = io.EOF
				break
			}
		}

		for i := 0; i < chunkSize; i++ {
			sample := int16(float64(y.buffer[i]) * y.volume)
			outBuffer[(processed+i)*2] = sample
			outBuffer[(processed+i)*2+1] = sample
		}

		processed += chunkSize
		y.position += int64(chunkSize)
	}

	buf := make([]byte, 0, len(outBuffer)*2)
	for _, sample := range outBuffer {
		buf = append(buf, byte(sample), byte(sample>>8))
	}

	copy(p, buf)
	n = len(buf)
	if n > len(p) {
		n = len(p)
	}

	return n, err
}

func (y *YMPlayer) Close() error {
	y.mutex.Lock()
	defer y.mutex.Unlock()

	if y.player != nil {
		y.player.Destroy()
		y.player = nil
	}
	return nil
}

type ScrollText struct {
	canvas       *ebiten.Image
	font         *ebiten.Image
	text         string
	speed        float64
	offset       float64
	tileW        int
	tileH        int
	scaleX       float64
	scaleY       float64
	visibleChars int
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

		rng:           rng,
		stop:          1,
		posY2:         200,
		actSize:       0,
		spinc:         1,
	}

	g.loadAssets()

	// Initialize scroll text state
	g.fullText = g.getFullText()
	g.preAnalyzeFontChanges()

	// Create scroll texts - all use the same source text and base font tiles
	smallText := g.rebuildText(0, true)
	normalText := g.rebuildText(1, false)
	mediumText := g.rebuildText(2, false)
	bigText := g.rebuildText(3, false)

	g.scrollText1 = g.newScrollText(g.scrollCanvas1, g.font0, 40, 32, 1.0, 1.0, smallText)
	g.scrollText2 = g.newScrollText(g.scrollCanvas2, g.font1, 40, 32, 2.0, 2.0, normalText)
	g.scrollText3 = g.newScrollText(g.scrollCanvas3, g.font2, 40, 32, 4.0, 4.0, mediumText)
	g.scrollText4 = g.newScrollText(g.scrollCanvas4, g.font3, 40, 32, 8.0, 12.0, bigText)

	g.setSpeed()

	for i := 0; i < 8; i++ {
		g.infStars[i][0] = math.Round(g.rng.Float64() * 9) * 64
		g.infStars[i][1] = math.Round(g.rng.Float64() * 354)
		g.infStars[i][2] = math.Round(g.rng.Float64()*4) + 4
		g.infStars[i][3] = math.Round(g.rng.Float64() * 10)
	}

	g.initAudio()
	g.startMusic()

	return g
}

func (g *Game) loadAssets() {
	g.starsImage = g.loadImage("rep_stars.png")
	g.logoImage = g.loadImage("rep_ik+_logo.png")
	g.scrollRast = g.loadImage("rep_ik+_rast1.png")
	g.backRast = g.loadImage("rep_ik+_rast2.png")
	
	// Use only font0 and scale it for other sizes (font3 is non-uniform: 8x width, 12x height).
	baseFontImage := g.loadImage("rep_ik+_font0.png")
	g.font0 = baseFontImage // 1x scale
	g.font1 = baseFontImage // Will be scaled 2x during rendering
	g.font2 = baseFontImage // Will be scaled 4x during rendering
	g.font3 = baseFontImage // Will be scaled 8x/12x during rendering
}

func (g *Game) initAudio() {
	g.audioContext = audio.NewContext(sampleRate)

	ym, err := NewYMPlayer(ymData, sampleRate, true)
	if err != nil {
		log.Printf("Failed to create YM player: %v", err)
		return
	}
	g.ymPlayer = ym

	player, err := g.audioContext.NewPlayer(ym)
	if err != nil {
		log.Printf("Failed to create audio player: %v", err)
		g.ymPlayer.Close()
		g.ymPlayer = nil
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
	f, err := assets.Open("assets/" + name)
	if err != nil {
		log.Printf("Failed to open asset %s: %v", name, err)
		// Return a blank image as fallback
		img := ebiten.NewImage(100, 100)
		img.Fill(colornames.Red)
		return img
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		log.Printf("Failed to read asset %s: %v", name, err)
		img := ebiten.NewImage(100, 100)
		img.Fill(colornames.Red)
		return img
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		log.Printf("Failed to decode asset %s: %v", name, err)
		img := ebiten.NewImage(100, 100)
		img.Fill(colornames.Red)
		return img
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
			
			// Create new image with cropped content
			croppedImg := ebiten.NewImage(fontWidth, fontHeight)
			sourceImg := ebiten.NewImageFromImage(img)
			
			// Draw the top portion of the original image
			op := &ebiten.DrawImageOptions{}
			srcRect := image.Rect(0, 0, fontWidth, fontHeight)
			croppedImg.DrawImage(sourceImg.SubImage(srcRect).(*ebiten.Image), op)
			
			log.Printf("Cropped font %s to %dx%d", name, fontWidth, fontHeight)
			return croppedImg
		}
		
		// Create a smaller fallback image for other assets
		fallbackImg := ebiten.NewImage(min(bounds.Dx(), maxSize), min(bounds.Dy(), maxSize))
		fallbackImg.Fill(colornames.Gray)
		return fallbackImg
	}
	
	return ebiten.NewImageFromImage(img)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (g *Game) newScrollText(canvas *ebiten.Image, font *ebiten.Image, tileW, tileH int, scaleX, scaleY float64, text string) *ScrollText {
	st := &ScrollText{
		canvas:       canvas,
		font:         font,
		text:         text,
		tileW:        tileW,
		tileH:        tileH,
		scaleX:       scaleX,
		scaleY:       scaleY,
		offset:       float64(canvas.Bounds().Dx()),
		visibleChars: countVisibleChars(text),
	}
	return st
}

func parseControlCode(text string, i int) (int, bool) {
	if i+4 >= len(text) || text[i] != '^' {
		return 0, false
	}
	switch text[i : i+5] {
	case "^Cs0;":
		return 0, true
	case "^Cs1;":
		return 1, true
	case "^Cs2;":
		return 2, true
	case "^Cs3;":
		return 3, true
	default:
		return 0, false
	}
}

func countVisibleChars(text string) int {
	count := 0
	for i := 0; i < len(text); {
		if _, ok := parseControlCode(text, i); ok {
			i += 5
			continue
		}
		count++
		i++
	}
	return count
}

func tileIndex(char rune) int {
	if char < ' ' || char > 'Z' {
		return 0
	}
	return int(char - ' ')
}

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

func (g *Game) rebuildText(size int, defaultActive bool) string {
	var sb strings.Builder
	active := defaultActive
	text := g.fullText

	for i := 0; i < len(text); {
		if codeSize, ok := parseControlCode(text, i); ok {
			active = codeSize == size
			sb.WriteString(text[i : i+5])
			i += 5
			continue
		}
		if active {
			sb.WriteByte(text[i])
		} else {
			sb.WriteByte(' ')
		}
		i++
	}

	return sb.String()
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

func (st *ScrollText) draw() {
	st.offset -= st.speed
	if st.visibleChars == 0 {
		return
	}

	scaledTileW := float64(st.tileW) * st.scaleX
	totalWidth := float64(st.visibleChars) * scaledTileW
	if totalWidth > 0 && st.offset <= -totalWidth {
		st.offset += totalWidth + float64(st.canvas.Bounds().Dx())
	}

	st.drawAtOffset(st.offset)
}

func (st *ScrollText) drawAt(offset float64) {
	st.offset = offset
	if st.visibleChars == 0 {
		return
	}
	st.drawAtOffset(st.offset)
}

func (st *ScrollText) drawAtOffset(offset float64) {
	st.canvas.Clear() // Clear to transparent

	scaledTileW := float64(st.tileW) * st.scaleX
	x := offset
	i := 0
	for x < float64(st.canvas.Bounds().Dx()) {
		if i >= len(st.text) {
			break
		}
		if _, ok := parseControlCode(st.text, i); ok {
			i += 5
			continue
		}

		char := rune(st.text[i])
		tileId := tileIndex(char)
		subRect := image.Rect(0, tileId*st.tileH, st.tileW, (tileId+1)*st.tileH)
		if subRect.Max.X <= st.font.Bounds().Dx() && subRect.Max.Y <= st.font.Bounds().Dy() {
			sub := st.font.SubImage(subRect).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			// Apply scaling
			op.GeoM.Scale(st.scaleX, st.scaleY)
			op.GeoM.Translate(x, 0)
			op.Filter = ebiten.FilterNearest
			st.canvas.DrawImage(sub, op)
		}
		x += scaledTileW
		i++
	}
}

func (g *Game) preAnalyzeFontChanges() {
	g.fontChanges = g.fontChanges[:0]
	glyphPos := 0
	for i := 0; i < len(g.fullText); {
		if size, ok := parseControlCode(g.fullText, i); ok {
			g.fontChanges = append(g.fontChanges, FontChange{glyphPos, size})
			i += 5
			continue
		}
		glyphPos++
		i++
	}
	g.totalGlyphs = glyphPos
}

func (g *Game) updateActSizeFromScroll() {
	if g.totalGlyphs == 0 {
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
	if glyphPos >= g.totalGlyphs {
		glyphPos = g.totalGlyphs - 1
	}

	size := 0
	for _, change := range g.fontChanges {
		if change.position <= glyphPos {
			size = change.newSize
		} else {
			break
		}
	}
	if size != g.actSize {
		g.actSize = size
		g.setSpeed()
	}
}

func (g *Game) Update() error {
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

	g.vbl++

	if g.vbl%2 == 0 {
		g.posY += 4
		if g.posY >= 654 {
			g.posY = 0
		}
		g.posY2 -= 2
		if g.posY2 <= 0 {
			g.posY2 = 200
		}
	}

	for i := 0; i < 8; i++ {
		g.infStars[i][3] += 1 / g.infStars[i][2]
		if g.infStars[i][3] >= 9 {
			g.infStars[i][0] = math.Round(g.rng.Float64() * 9) * 64
			g.infStars[i][1] = math.Round(g.rng.Float64() * 354)
			g.infStars[i][2] = math.Round(g.rng.Float64()*4) + 4
			g.infStars[i][3] = 0
		}
	}

	if g.scrollText1 != nil {
		g.scrollText1.draw()
		baseOffset := g.scrollText1.offset
		baseWidth := float64(g.scrollText1.canvas.Bounds().Dx())
		if g.scrollText2 != nil {
			g.scrollText2.drawAt(baseOffset*g.scrollText2.scaleX + (1-g.scrollText2.scaleX)*baseWidth)
		}
		if g.scrollText3 != nil {
			g.scrollText3.drawAt(baseOffset*g.scrollText3.scaleX + (1-g.scrollText3.scaleX)*baseWidth)
		}
		if g.scrollText4 != nil {
			g.scrollText4.drawAt(baseOffset*g.scrollText4.scaleX + (1-g.scrollText4.scaleX)*baseWidth)
		}
	}
	g.updateActSizeFromScroll()

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.stop > 0 {
		screen.Fill(colornames.Black)

		if g.backRast != nil {
			for j := 0; j < 11; j++ {
				sy := int(g.posY) + j*4
				g.drawPart(screen, g.backRast, 0, 60+2+j*36, 0, sy, 1, 36, 1, 0, 768, 1)
			}
		}

		if g.mergeCanvas != nil {
			g.mergeCanvas.Fill(color.Transparent)
		}

		if g.scrollRast != nil {
			g.drawPart(g.mergeCanvas, g.scrollRast, 0, int(g.posY2)-200, 0, 0, 2, 200, 1, 0, 320, 1)
			g.drawPart(g.mergeCanvas, g.scrollRast, 0, int(g.posY2), 0, 0, 2, 200, 1, 0, 320, 1)
			g.drawPart(g.mergeCanvas, g.scrollRast, 0, int(g.posY2)+200, 0, 0, 2, 200, 1, 0, 320, 1)
		}

		if g.offScroll != nil {
			g.offScroll.Fill(color.Transparent)
		}

		switch g.actSize {
		case 0:
			if g.scrollCanvas1 != nil {
				for j := 0; j < 11; j++ {
					g.drawPart(g.offScroll, g.scrollCanvas1, 0, 2+j*36, 0, 0, 640, 32, 1, 0, 1, 1)
				}
			}
		case 1:
			if g.scrollCanvas2 != nil {
				for j := 0; j < 6; j++ {
					g.drawPart(g.offScroll, g.scrollCanvas2, 0, 2+j*66, 0, 0, 640, 64, 1, 0, 1, 1)
				}
			}
		case 2:
			if g.scrollCanvas3 != nil {
				g.drawPart(g.offScroll, g.scrollCanvas3, 0, 0, 0, 0, 640, 128, 1, 0, 1, 1)
				g.drawPart(g.offScroll, g.scrollCanvas3, 0, 134, 0, 0, 640, 128, 1, 0, 1, 1)
				g.drawPart(g.offScroll, g.scrollCanvas3, 0, 268, 0, 0, 640, 128, 1, 0, 1, 1)
			}
		case 3:
			if g.scrollCanvas4 != nil {
				g.drawPart(g.offScroll, g.scrollCanvas4, 0, 4, 0, 0, 640, 384, 1, 0, 1, 1)
			}
		}

		if g.mergeCanvas != nil && g.offScroll != nil {
			op := &ebiten.DrawImageOptions{}
			op.CompositeMode = ebiten.CompositeModeDestinationIn
			op.GeoM.Translate(0, 2)
			g.mergeCanvas.DrawImage(g.offScroll, op)
			if g.mergeCanvas.Bounds().Dy() >= 2 {
				top := g.mergeCanvas.SubImage(image.Rect(0, 0, g.mergeCanvas.Bounds().Dx(), 2)).(*ebiten.Image)
				top.Clear()
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
				g.drawTile(screen, g.starsImage, tile, 64+int(g.infStars[i][0]), 60+int(g.infStars[i][1]), 64, 46, 1, 0, 1, 1)
			}
		}
	}
}

func (g *Game) drawPart(dest *ebiten.Image, src *ebiten.Image, dx, dy, sx, sy, sw, sh, param8, param9, tileX, tileY int) {
	if src == nil || dest == nil {
		return
	}
	for jy := 0; jy < tileY; jy++ {
		for jx := 0; jx < tileX; jx++ {
			subRect := image.Rect(sx, sy, sx+sw, sy+sh)
			if subRect.Max.X <= src.Bounds().Dx() && subRect.Max.Y <= src.Bounds().Dy() {
				sub := src.SubImage(subRect).(*ebiten.Image)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(dx+jx*sw), float64(dy+jy*sh))
				dest.DrawImage(sub, op)
			}
		}
	}
}

func (g *Game) drawTile(dest *ebiten.Image, src *ebiten.Image, tile int, dx, dy, tileW, tileH int, scale float64, rot float64, flipH, flipV int) {
	if src == nil || dest == nil {
		return
	}
	cols := src.Bounds().Dx() / tileW
	if cols == 0 {
		return
	}
	row := tile / cols
	col := tile % cols
	subRect := image.Rect(col*tileW, row*tileH, (col+1)*tileW, (row+1)*tileH)
	if subRect.Max.X <= src.Bounds().Dx() && subRect.Max.Y <= src.Bounds().Dy() {
		sub := src.SubImage(subRect).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		if flipH == -1 {
			op.GeoM.Scale(-1, 1)
			op.GeoM.Translate(float64(tileW), 0)
		}
		if flipV == -1 {
			op.GeoM.Scale(1, -1)
			op.GeoM.Translate(0, float64(tileH))
		}
		op.GeoM.Scale(scale, scale)
		op.GeoM.Rotate(rot * math.Pi / 180)
		op.GeoM.Translate(float64(dx), float64(dy))
		dest.DrawImage(sub, op)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Remake of the \"Dom intro\" in Golang + Ebiten")
	if err := ebiten.RunGame(NewGame()); err != nil {
		log.Fatal(err)
	}
}
