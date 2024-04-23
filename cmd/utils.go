package main

import (
	"bytes" // bytes包实现了操作[]byte的函数
	"fmt"   // fmt包提供了I/O函数
	"os"    // os包提供了操作系统函数
	"regexp"
	"strings"

	"github.com/gdamore/tcell"
)

// Read the theme file of the given name with the following format:
// bgcol: #260346
// fgcol: #DADADA
// hicol: #C9CCCD
// hicol2: #ad363f
// hicol3: #E74C3C
// errcol: #C54133
func readTheme(name string) map[string]string {
	// Read the file
	res, err := os.ReadFile(fmt.Sprintf("themes/%s.txt", name))
	if err != nil {
		panic(err)
	}

	// Parse the file
	theme := make(map[string]string)
	for _, line := range bytes.Split(res, []byte("\n")) {
		parts := bytes.Split(line, []byte(": "))
		if len(parts) != 2 {
			continue
		}
		theme[string(parts[0])] = string(parts[1])
	}

	return theme
}

func newTcellColor(hex string) (tcell.Color, error) {
	// Check if the color is a valid hex color
	if len(hex) != 7 {
		return 0, fmt.Errorf("invalid hex color")
	}

	// Check if the color starts with a #
	if hex[0] != '#' {
		return 0, fmt.Errorf("invalid hex color")
	}

	// Inline function to convert a byte to a number
	toNumber := func(c byte) int32 {
		if c > '9' { // 如果c大于'9'
			if c >= 'a' { // 如果c大于等于'a'
				return int32(c - 'a' + 10)
			} else {
				return int32(c - 'A' + 10)
			}
		} else {
			return int32(c - '0')
		}
	}

	// Convert the hex color to a tcell.Color
	r := toNumber(hex[1])<<4 | toNumber(hex[2]) // hex[1]左移4位或hex[2]
	g := toNumber(hex[3])<<4 | toNumber(hex[4]) //
	b := toNumber(hex[5])<<4 | toNumber(hex[6]) //

	// Example
	// hex = #260346
	// r = 2*16 + 6 = 38
	return tcell.NewRGBColor(r, g, b), nil
}

func wordWrapBytes(s []byte, n int) {
	sp := 0
	sz := 0

	for i := 0; i < len(s); i++ {
		sz++

		if s[i] == '\n' {
			s[i] = ' '
		}

		if s[i] == ' ' {
			sp = i
		}

		if sz > n {
			if sp != 0 {
				s[sp] = '\n'
			}

			sz = i - sp
		}
	}

}

func wordWrap(s string, n int) string {
	r := []byte(s)
	wordWrapBytes(r, n)
	return string(r)
}

func wrapText(text string, width int) string {
	reflow := func(s string) string {
		sw, _ := scr.Size()

		wsz := width
		if wsz > sw {
			wsz = sw - 8
		}

		s = regexp.MustCompile("\\s+").ReplaceAllString(s, " ")
		return strings.Replace(
			wordWrap(strings.Trim(s, " "), wsz),
			"\n", " \n", -1)
	}

	return reflow(text)
}

func calcStringDimensions(s string) (nc, nr int) {
	if s == "" {
		return 0, 0
	}

	c := 0

	for _, x := range s {
		if x == '\n' {
			nr++
			if c > nc {
				nc = c
			}
			c = 0
		} else {
			c++
		}
	}

	nr++
	if c > nc {
		nc = c
	}

	return
}

func extractMistypedWords(text []rune, typed []rune) (mistakes []mistake) {
	var w []rune
	var t []rune
	f := false

	for i := range text {
		if text[i] == ' ' {
			if f {
				mistakes = append(mistakes, mistake{string(w), string(t)})
			}

			w = w[:0]
			t = t[:0]
			f = false
			continue
		}

		if text[i] != typed[i] {
			f = true
		}

		if text[i] == 0 {
			w = append(w, '_')
		} else {
			w = append(w, text[i])
		}

		if typed[i] == 0 {
			t = append(t, '_')
		} else {
			t = append(t, typed[i])
		}
	}

	if f {
		mistakes = append(mistakes, mistake{string(w), string(t)})
	}

	return
}

// drawString draws a string to the screen at the given position with the given style.
func drawString(scr tcell.Screen, x, y int, s string, cursorIdx int, style tcell.Style) {
	sx := x

	for i, c := range s {
		if c == '\n' {
			y++
			x = sx
		} else {
			scr.SetContent(x, y, c, nil, style)
			if i == cursorIdx {
				scr.ShowCursor(x, y)
			}

			x++
		}
	}

	if cursorIdx == len(s) {
		scr.ShowCursor(x, y)
	}
}

func drawStringAtCenter(scr tcell.Screen, s string, style tcell.Style) {
	nc, nr := calcStringDimensions(s)
	sw, sh := scr.Size()

	x := (sw - nc) / 2
	y := (sh - nr) / 2

	drawString(scr, x, y, s, -1, style)
}

// func saveMistakes(mistakes []mistake) {
// 	var db []mistake

// 	if err := readValue(MISTAKE_DB, &db); err != nil {
// 		db = nil
// 	}

// 	db = append(db, mistakes...)
// 	writeValue(MISTAKE_DB, db)
// }

func showReport(scr tcell.Screen, cpm, wpm int, accuracy float64, attribution string, mistakes []mistake) {
	mistakeStr := ""
	if attribution != "" {
		attribution = "\n\nAttribution: " + attribution
	}

	if len(mistakes) > 0 {
		mistakeStr = "\nMistakes:    "
		for i, m := range mistakes {
			mistakeStr += m.Word
			if i != len(mistakes)-1 {
				mistakeStr += ", "
			}
		}
	}

	report := fmt.Sprintf("WPM:         %d\nCPM:         %d\nAccuracy:    %.2f%%%s%s", wpm, cpm, accuracy, mistakeStr, attribution)

	scr.Clear()
	drawStringAtCenter(scr, report, tcell.StyleDefault)
	scr.HideCursor()
	scr.Show()

	for {
		if key, ok := scr.PollEvent().(*tcell.EventKey); ok && key.Key() == tcell.KeyEscape {
			return
		} else if ok && key.Key() == tcell.KeyCtrlC {
			// exit("Interrupted")
			exit_program(1)
		}
	}
}

// Exit program logic
func exit_program(rc int) {
	scr.Fini()
	os.Exit(rc)
}
