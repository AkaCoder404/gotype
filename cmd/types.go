package main

import (
	"fmt" // fmt包提供了I/O函数
	"io"  // io包提供了基本的接口
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/gdamore/tcell"
)

type segment struct {
	Text        string // 文本
	Attribution string // 归因
}

type mistake struct {
	Word  string `json:"word"`
	Typed string `json:"typed"`
}

// Represents our gotype object
type gotype struct {
	scr              tcell.Screen // gotype的窗口
	tty              io.Writer    // tty是一个io.Writer接口
	OnStart          func()       // 开始时的回调函数
	SkipWord         bool         // 是否跳过单词
	ShowWpm          bool         // 是否显示每分钟字数
	DisableBackspace bool         // 是否禁用退格键
	BlockCursor      bool         // 是否显示块光标

	defaultStyle        tcell.Style // 默认样式
	currentWordStyle    tcell.Style // 当前单词的样式
	nextWordStyle       tcell.Style // 下一个单词的样式
	incorrectStyle      tcell.Style // 错误的样式
	incorrectSpaceStyle tcell.Style // 错误空格的样式
	incorrectCharStyle  tcell.Style // 错误字符的样式
	incorrectWordStyle  tcell.Style // 错误单词的样式
	correctStyle        tcell.Style // 正确的样式
}

// GoType States
const (
	GoTypeComplete = iota
	GoTypeSigInt
	GoTypeEscape
	GoTypeNext
	GoTypePrevious
	GoTypeResize
)

func exit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func createGoType(scr tcell.Screen, bold bool, themeName string) *gotype {
	var theme map[string]string

	// Read the theme file
	if result := readTheme(themeName); result == nil {
		exit("%s does not appear to be a valid theme, try running `gotype -list themes` to list available themes", themeName)
	} else {
		theme = result
	}

	// bgcol background color
	// fgcol foreground color
	// hicol highlight color
	// hicol2 highlight color 2
	// hicol3 highlight color 3
	var bgcol, fgcol, hicol, hicol2, hicol3, errcol tcell.Color
	var err error

	// Check if the colors are valid hex colors
	if bgcol, err = newTcellColor(theme["bgcol"]); err != nil {
		exit("Background color is not defined and/or a valid hex color")
	}
	if fgcol, err = newTcellColor(theme["fgcol"]); err != nil {
		exit("Foreground color is not defined and/or a valid hex color")
	}
	if hicol, err = newTcellColor(theme["hicol"]); err != nil {
		exit("Highlight color is not defined and/or a valid hex color")
	}
	if hicol2, err = newTcellColor(theme["hicol2"]); err != nil {
		exit("Highlight color 2 is not defined and/or a valid hex color")
	}
	if hicol3, err = newTcellColor(theme["hicol3"]); err != nil {
		exit("Highlight color 3 is not defined and/or a valid hex color")
	}
	if errcol, err = newTcellColor(theme["errcol"]); err != nil {
		exit("Error color is not defined and/or a valid hex color")
	}

	return NewGoType(scr, bold, bgcol, fgcol, hicol, hicol2, hicol3, errcol)
}

// NewGoType creates a new gotype object
func NewGoType(scr tcell.Screen, emboldenTypedText bool, bgcol, fgcol, hicol, hicol2, hicol3, errcol tcell.Color) *gotype {
	var tty io.Writer // tty是一个io.Writer接口

	// Set up screen styles
	scrSetupRes := tcell.StyleDefault.Foreground(fgcol).Background(bgcol)

	// Set up the screen
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		tty = io.Discard
	}

	// Set up the screen
	correctStyle := scrSetupRes.Foreground(hicol)
	if emboldenTypedText {
		correctStyle = correctStyle.Bold(true)
	}

	// Return the gotype object
	return &gotype{
		scr:      scr,
		SkipWord: true,
		tty:      tty,

		defaultStyle:        scrSetupRes,
		correctStyle:        correctStyle,
		currentWordStyle:    scrSetupRes.Foreground(hicol2),
		nextWordStyle:       scrSetupRes.Foreground(hicol3),
		incorrectStyle:      scrSetupRes.Foreground(errcol),
		incorrectSpaceStyle: scrSetupRes.Foreground(errcol),
	}
}

func (t *gotype) StartTest(text []segment, timeout time.Duration) (numerrs, numcorrect int, duration time.Duration, rc int, mistakes []mistake, wpms []int) {
	timeLeft := timeout

	for idx, seg := range text {
		startImmediately := true
		var d time.Duration
		var e, c int
		var m []mistake

		if idx == 0 {
			startImmediately = false
		}

		e, c, rc, d, m = t.play(seg.Text, timeLeft, startImmediately, seg.Attribution)

		numerrs += e                      // 错误数
		numcorrect += c                   // 正确数
		duration += d                     // 持续时间
		mistakes = append(mistakes, m...) // 错误

		if timeout != -1 {
			timeLeft -= d // 剩余时间
			if timeLeft <= 0 {
				return
			}
		}

		if rc != GoTypeComplete { //
			return
		}
	}

	return
}

// play函数进行打字环节, core game logic
func (t *gotype) play(s string, timeLimit time.Duration, startImmediately bool, attribution string) (nerrs int, ncorrect int, rc int, duration time.Duration, mistakes []mistake) {
	var startTime time.Time
	text := []rune(s)
	typed := make([]rune, len(text))

	sw, sh := scr.Size()
	nc, nr := calcStringDimensions(s) // 计算字符串的维度
	x := (sw - nc) / 2
	y := (sh - nr) / 2

	if !t.BlockCursor {
		t.tty.Write([]byte("\033[5 q"))

		//Assumes original cursor shape was a block (the one true cursor shape), there doesn't appear to be a
		//good way to save/restore the shape if the user has changed it from the otcs.
		defer t.tty.Write([]byte("\033[2 q"))
	}

	t.scr.SetStyle(t.defaultStyle)
	idx := 0

	calcStats := func() {
		nerrs = 0
		ncorrect = 0

		mistakes = extractMistypedWords(text[:idx], typed[:idx])

		for i := 0; i < idx; i++ {
			if text[i] != '\n' {
				if text[i] != typed[i] {
					nerrs++
				} else {
					ncorrect++
				}
			}
		}

		rc = GoTypeComplete
		// duration = time.Now().Sub(startTime)
		duration = time.Since(startTime)
	}

	redraw := func() {
		cx := x
		cy := y
		inword := -1

		for i := range text {
			style := t.defaultStyle

			if text[i] == '\n' {
				cy++
				cx = x
				if inword != -1 {
					inword++
				}
				continue
			}

			if i == idx {
				scr.ShowCursor(cx, cy)
				inword = 0
			}

			if i >= idx {
				if text[i] == ' ' {
					inword++
				} else if inword == 0 {
					style = t.currentWordStyle
				} else if inword == 1 {
					style = t.nextWordStyle
				} else {
					style = t.defaultStyle
				}
			} else if text[i] != typed[i] {
				if text[i] == ' ' {
					style = t.incorrectSpaceStyle
				} else {
					style = t.incorrectStyle
				}
			} else {
				style = t.correctStyle
			}

			scr.SetContent(cx, cy, text[i], nil, style)
			cx++
		}

		aw, ah := calcStringDimensions(attribution)
		drawString(t.scr, x+nc-aw, y+nr+1, attribution, -1, t.defaultStyle)

		if timeLimit != -1 && !startTime.IsZero() {
			// remaining := timeLimit - time.Now().Sub(startTime)
			remaining := timeLimit - time.Since(startTime)
			drawString(t.scr, x+nc/2, y+nr+ah+1, "      ", -1, t.defaultStyle)
			drawString(t.scr, x+nc/2, y+nr+ah+1, strconv.Itoa(int(remaining/1e9)+1), -1, t.defaultStyle)
		}

		if t.ShowWpm && !startTime.IsZero() {
			calcStats()
			if duration > 1e7 { //Avoid flashing large numbers on test start.
				wpm := int((float64(ncorrect) / 5) / (float64(duration) / 60e9))
				drawString(t.scr, x+nc/2-4, y-2, fmt.Sprintf("WPM: %-10d\n", wpm), -1, t.defaultStyle)
			}
		}

		//Potentially inefficient, but seems to be good enough
		t.scr.Show()
	}

	deleteWord := func() {
		if idx == 0 {
			return
		}

		idx--

		for idx > 0 && (text[idx] == ' ' || text[idx] == '\n') {
			idx--
		}

		for idx > 0 && text[idx] != ' ' && text[idx] != '\n' {
			idx--
		}

		if text[idx] == ' ' || text[idx] == '\n' {
			typed[idx] = text[idx]
			idx++
		}
	}

	tickerCloser := make(chan bool)

	//Inject nil events into the main event loop at regular invervals to force an update
	ticker := func() {
		for {
			select {
			case <-tickerCloser:
				return
			default:
			}

			time.Sleep(time.Duration(5e8))
			t.scr.PostEventWait(nil)
		}
	}

	go ticker()
	defer close(tickerCloser)

	if startImmediately {
		startTime = time.Now()
	}

	t.scr.Clear()
	for {
		redraw()

		ev := t.scr.PollEvent()

		switch ev := ev.(type) {
		case *tcell.EventResize:
			rc = GoTypeResize
			return
		case *tcell.EventKey:
			if runtime.GOOS != "windows" && ev.Key() == tcell.KeyBackspace { //Control+backspace on unix terms
				if !t.DisableBackspace {
					deleteWord()
				}
				continue
			}

			if startTime.IsZero() {
				startTime = time.Now()
			}

			switch key := ev.Key(); key {
			case tcell.KeyCtrlC:
				rc = GoTypeSigInt

				return
			case tcell.KeyEscape:
				rc = GoTypeEscape

				return
			case tcell.KeyCtrlL:
				t.scr.Sync()

			case tcell.KeyRight:
				rc = GoTypeNext
				return

			case tcell.KeyLeft:
				rc = GoTypePrevious
				return

			case tcell.KeyCtrlW:
				if !t.DisableBackspace {
					deleteWord()
				}

			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if !t.DisableBackspace {
					if ev.Modifiers() == tcell.ModAlt || ev.Modifiers() == tcell.ModCtrl {
						deleteWord()
					} else {
						if idx == 0 {
							break
						}

						idx--

						for idx > 0 && text[idx] == '\n' {
							idx--
						}
					}
				}
			case tcell.KeyRune:
				if idx < len(text) {
					if t.SkipWord && ev.Rune() == ' ' {
						if idx > 0 && text[idx-1] == ' ' && text[idx] != ' ' { //Do nothing on word boundaries.
							break
						}

						for idx < len(text) && text[idx] != ' ' && text[idx] != '\n' {
							typed[idx] = 0
							idx++
						}

						if idx < len(text) {
							typed[idx] = text[idx]
							idx++
						}
					} else {
						typed[idx] = ev.Rune()
						idx++
					}

					for idx < len(text) && text[idx] == '\n' {
						typed[idx] = text[idx]
						idx++
					}
				}

				if idx == len(text) {
					calcStats()
					return
				}
			}
		default: //tick
			// if timeLimit != -1 && !startTime.IsZero() && timeLimit <= time.Now().Sub(startTime) {
			if timeLimit != -1 && !startTime.IsZero() && timeLimit <= time.Since(startTime) {
				calcStats()
				return
			}

			redraw()
		}
	}
}
