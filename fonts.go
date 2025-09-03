package marquee

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type GlobalFontManager struct {
	fonts         map[string]rl.Font
	refCounts     map[string]int
	mutex         sync.RWMutex
	fontPaths     map[string]string
	monoFontPaths map[string]string
	initialized   bool

	fontStatus map[string]bool
}

var fontManager *GlobalFontManager
var fontManagerOnce sync.Once

func getFontManager() *GlobalFontManager {
	fontManagerOnce.Do(func() {
		fontManager = &GlobalFontManager{
			fonts:         make(map[string]rl.Font),
			refCounts:     make(map[string]int),
			fontPaths:     make(map[string]string),
			monoFontPaths: make(map[string]string),
			fontStatus:    make(map[string]bool),
		}
		fontManager.initializePlatformPaths()
	})
	return fontManager
}

func (fm *GlobalFontManager) initializePlatformPaths() {
	if runtime.GOOS == "darwin" {
		fm.fontPaths["arial"] = "/System/Library/Fonts/Supplemental/Arial.ttf"
		fm.fontPaths["arial-bold"] = "/System/Library/Fonts/Supplemental/Arial Bold.ttf"
		fm.fontPaths["arial-italic"] = "/System/Library/Fonts/Supplemental/Arial Italic.ttf"
		fm.monoFontPaths["monaco"] = "/System/Library/Fonts/Monaco.ttf"
		fm.monoFontPaths["menlo"] = "/System/Library/Fonts/Menlo.ttc"
		fm.monoFontPaths["courier"] = "/System/Library/Fonts/Courier.ttc"
	} else if runtime.GOOS == "windows" {
		fm.fontPaths["arial"] = "C:/Windows/Fonts/arial.ttf"
		fm.fontPaths["arial-bold"] = "C:/Windows/Fonts/arialbd.ttf"
		fm.fontPaths["arial-italic"] = "C:/Windows/Fonts/ariali.ttf"
		fm.monoFontPaths["consolas"] = "C:/Windows/Fonts/consola.ttf"
		fm.monoFontPaths["cascadia"] = "C:/Windows/Fonts/CascadiaCode.ttf"
		fm.monoFontPaths["courier"] = "C:/Windows/Fonts/cour.ttf"
		fm.monoFontPaths["lucida-console"] = "C:/Windows/Fonts/lucon.ttf"
	} else {
		fm.fontPaths["arial"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf"
		fm.fontPaths["arial-bold"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf"
		fm.fontPaths["arial-italic"] = "/usr/share/fonts/truetype/liberation/LiberationSans-Italic.ttf"
		fm.monoFontPaths["dejavu-mono"] = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
		fm.monoFontPaths["liberation-mono"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
		fm.monoFontPaths["ubuntu-mono"] = "/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf"
		fm.monoFontPaths["courier"] = "/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf"
	}
	fm.initialized = true
}

func (fm *GlobalFontManager) GetMonospaceFont(size int32) rl.Font {
	key := fmt.Sprintf("monospace:%d", size)

	fm.mutex.RLock()
	if font, exists := fm.fonts[key]; exists {
		fm.mutex.RUnlock()
		fm.mutex.Lock()
		fm.refCounts[key]++
		fm.mutex.Unlock()
		return font
	}
	fm.mutex.RUnlock()

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	var fontOrder []string
	if runtime.GOOS == "darwin" {
		fontOrder = []string{"monaco", "menlo", "sf-mono", "courier"}
	} else if runtime.GOOS == "windows" {
		fontOrder = []string{"consolas", "cascadia", "courier", "lucida-console"}
	} else {
		fontOrder = []string{"dejavu-mono", "liberation-mono", "ubuntu-mono", "courier"}
	}

	var loadedFont rl.Font

	for _, fontName := range fontOrder {
		if fontPath, exists := fm.monoFontPaths[fontName]; exists {
			testFont := rl.LoadFontEx(fontPath, size, essentialCodepoints)

			if testFont.BaseSize > 0 && testFont.Texture.ID > 0 {
				loadedFont = testFont
				fm.fontStatus[key] = true
				break
			}
		}
	}

	if loadedFont.BaseSize == 0 {
		loadedFont = rl.GetFontDefault()
		fm.fontStatus[key] = false
	}

	fm.fonts[key] = loadedFont
	fm.refCounts[key] = 1

	return loadedFont
}

func (fm *GlobalFontManager) GetFont(fontName string, size int32) rl.Font {
	key := fmt.Sprintf("%s:%d", fontName, size)

	fm.mutex.RLock()
	if font, exists := fm.fonts[key]; exists {
		fm.mutex.RUnlock()
		fm.mutex.Lock()
		fm.refCounts[key]++
		fm.mutex.Unlock()
		return font
	}
	fm.mutex.RUnlock()

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if font, exists := fm.fonts[key]; exists {
		fm.refCounts[key]++
		return font
	}

	fontPath, pathExists := fm.fontPaths[fontName]
	if !pathExists {
		defaultFont := rl.GetFontDefault()
		fm.fonts[key] = defaultFont
		fm.refCounts[key] = 1
		fm.fontStatus[key] = false
		return defaultFont
	}

	font := rl.LoadFontEx(fontPath, size, essentialCodepoints)

	if font.BaseSize > 0 && font.Texture.ID > 0 && font.Texture.Width > 0 {
		fm.fontStatus[key] = true
	} else {
		font = rl.GetFontDefault()
		fm.fontStatus[key] = false
	}

	fm.fonts[key] = font
	fm.refCounts[key] = 1

	return font
}

func (fm *GlobalFontManager) GetFontStatus(fontName string, size int32) bool {
	key := fmt.Sprintf("%s:%d", fontName, size)
	fm.mutex.RLock()
	defer fm.mutex.RUnlock()
	return fm.fontStatus[key]
}

func (fm *GlobalFontManager) ReleaseFont(fontName string, size int32) {
	key := fmt.Sprintf("%s:%d", fontName, size)

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if count, exists := fm.refCounts[key]; exists {
		count--
		if count <= 0 {
			if font, fontExists := fm.fonts[key]; fontExists {
				defaultFont := rl.GetFontDefault()
				if font.BaseSize > 0 && font.Texture.ID != defaultFont.Texture.ID {
					rl.UnloadFont(font)
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
			delete(fm.fontStatus, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

func (fm *GlobalFontManager) ReleaseMonospaceFont(size int32) {
	key := fmt.Sprintf("monospace:%d", size)

	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if count, exists := fm.refCounts[key]; exists {
		count--
		if count <= 0 {
			if font, fontExists := fm.fonts[key]; fontExists {
				defaultFont := rl.GetFontDefault()
				if font.BaseSize > 0 && font.Texture.ID != defaultFont.Texture.ID {
					rl.UnloadFont(font)
				}
			}
			delete(fm.fonts, key)
			delete(fm.refCounts, key)
			delete(fm.fontStatus, key)
		} else {
			fm.refCounts[key] = count
		}
	}
}

type TextMeasureCache struct {
	cache       map[string]rl.Vector2
	accessOrder []string
	maxEntries  int

	fontTextures map[string]uint32
}

func NewTextMeasureCache(maxEntries int) *TextMeasureCache {
	return &TextMeasureCache{
		cache:        make(map[string]rl.Vector2),
		accessOrder:  make([]string, 0),
		maxEntries:   maxEntries,
		fontTextures: make(map[string]uint32),
	}
}

func (tmc *TextMeasureCache) GetTextSize(font rl.Font, text string, fontSize float32) rl.Vector2 {
	key := fmt.Sprintf("%d:%.1f:%s", font.Texture.ID, fontSize, text)
	fontKey := fmt.Sprintf("%d:%.1f", font.Texture.ID, fontSize)

	if cachedTextureID, exists := tmc.fontTextures[fontKey]; exists {
		if cachedTextureID != font.Texture.ID {

			tmc.invalidateFontCache(cachedTextureID)
		}
	}
	tmc.fontTextures[fontKey] = font.Texture.ID

	if size, exists := tmc.cache[key]; exists {
		tmc.updateAccessOrder(key)
		return size
	}

	size := rl.MeasureTextEx(font, text, fontSize, 1)

	tmc.cache[key] = size
	tmc.accessOrder = append(tmc.accessOrder, key)

	if len(tmc.cache) > tmc.maxEntries {
		oldestKey := tmc.accessOrder[0]
		delete(tmc.cache, oldestKey)
		tmc.accessOrder = tmc.accessOrder[1:]
	}

	return size
}

func (tmc *TextMeasureCache) invalidateFontCache(oldTextureID uint32) {
	keysToDelete := make([]string, 0)

	for key := range tmc.cache {

		parts := strings.Split(key, ":")
		if len(parts) >= 1 {
			if texID, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
				if uint32(texID) == oldTextureID {
					keysToDelete = append(keysToDelete, key)
				}
			}
		}
	}

	for _, key := range keysToDelete {
		delete(tmc.cache, key)

		for i, k := range tmc.accessOrder {
			if k == key {
				tmc.accessOrder = append(tmc.accessOrder[:i], tmc.accessOrder[i+1:]...)
				break
			}
		}
	}
}

func (tmc *TextMeasureCache) GetTextWidth(font rl.Font, text string, fontSize float32) float32 {
	return tmc.GetTextSize(font, text, fontSize).X
}

func (tmc *TextMeasureCache) updateAccessOrder(key string) {
	for i, k := range tmc.accessOrder {
		if k == key {
			tmc.accessOrder = append(tmc.accessOrder[:i], tmc.accessOrder[i+1:]...)
			break
		}
	}
	tmc.accessOrder = append(tmc.accessOrder, key)
}

func (tmc *TextMeasureCache) Clear() {
	tmc.cache = make(map[string]rl.Vector2)
	tmc.accessOrder = tmc.accessOrder[:0]
	tmc.fontTextures = make(map[string]uint32)
}

func renderTextWithUnicode(text string, x, y float32, font rl.Font, color rl.Color) {
	fontSize := float32(font.BaseSize)
	if fontSize == 0 {
		fontSize = 16
	}

	hasUnicode := false
	for _, r := range text {
		if r >= 128 {
			hasUnicode = true
			break
		}
	}

	if !hasUnicode {
		rl.DrawTextEx(font, text, rl.NewVector2(x, y), fontSize, 1, color)
		return
	}

	currentX := x
	runes := []rune(text)

	for _, r := range runes {
		if r < 128 {
			charStr := string(r)
			charWidth := rl.MeasureTextEx(font, charStr, fontSize, 1).X
			rl.DrawTextEx(font, charStr, rl.NewVector2(currentX, y), fontSize, 1, color)
			currentX += charWidth
		} else {

			charWidth := calculateUnicodeCharWidth(r, fontSize)
			rl.DrawTextCodepoint(font, r, rl.NewVector2(currentX, y), fontSize, color)
			currentX += charWidth
		}
	}
}

func calculateUnicodeCharWidth(r rune, fontSize float32) float32 {

	switch {

	case r >= 0x00C0 && r <= 0x00FF:
		return fontSize * 0.55

	case r >= 0x0100 && r <= 0x017F:
		return fontSize * 0.58

	case r == 0x2013 || r == 0x2014:
		return fontSize * 0.5
	case r == 0x2018 || r == 0x2019 || r == 0x201C || r == 0x201D:
		return fontSize * 0.3
	case r == 0x2026:
		return fontSize * 0.8
	case r == 0x00AB || r == 0x00BB:
		return fontSize * 0.45

	case r == 0x2022 || r == 0x25CF:
		return fontSize * 0.4

	default:
		return fontSize * 0.6
	}
}
