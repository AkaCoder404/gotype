package main

import (
	// fmt包提供了I/O函数

	"flag" // flag包实现了命令行参数的解析
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell"
)

type result struct {
	Wpm       int       `json:"wpm"`
	Cpm       int       `json:"cpm"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp int64     `json:"timestamp"`
	Mistakes  []mistake `json:"mistakes"`

	Wpms []int `json:"wpms"`
}

var usage = `usage: gotype [options] [file]

Modes
  	-words		string 		Specify the words file to use
  	-quotes 	string 		Specify the quotes file to use

Play
	- numwords	int			Number of words to use in the test
	- numsegments	int		Number of segments to use in the test (number of tests)

Display
	- showwpm		bool		Show words per minute
	- blockcursor	bool		Show block cursor
	- theme 		string		The theme to use
 
Misc
	- version		bool		Show the version
`

// Global variables
var scr tcell.Screen // scr是一个tcell.Screen
var err error
var results []result

func main() {
	// Flags
	var themeName string
	var noTheme bool
	var boldFlag bool
	var versionFlag bool
	var helpFlag bool
	var listFlag string

	// GoType flags
	var noSkip bool
	var noBackspace bool
	var normalCursor bool
	var showWpm bool
	var timeout int
	var oneShotMode bool
	var numWords int
	var numSegments int

	var wordFile string  // -words flag
	var quoteFile string // -quotes flag
	var wordLlm string   //
	var quoteLlm string  //

	// var typingTestWordsFile string        // 单词文件
	// var typingTestQuotesFile string       // 引用文件
	var typingTestGetter func() []segment // 函数返回单词的数组

	// Set flags
	flag.StringVar(&themeName, "theme", "default", "The theme to use")
	flag.BoolVar(&noTheme, "notheme", false, "Don't use a theme")
	flag.BoolVar(&boldFlag, "bold", false, "Use bold text")
	flag.BoolVar(&versionFlag, "version", false, "Show the version")
	flag.BoolVar(&helpFlag, "help", false, "Show the help")
	flag.StringVar(&listFlag, "list", "", "List available themes and word files")

	flag.StringVar(&wordFile, "words", "", "Specify the words file to use")
	flag.StringVar(&quoteFile, "quotes", "", "Specify the quotes file to use")
	flag.StringVar(&wordLlm, "wllm", "", "Specify the language model to use")
	flag.StringVar(&quoteLlm, "qllm", "", "Specify the language model to use")

	flag.BoolVar(&noSkip, "noskip", false, "Don't skip words")
	flag.BoolVar(&noBackspace, "nobackspace", false, "Don't allow backspace")
	flag.BoolVar(&normalCursor, "blockcursor", false, "Use a normal cursor")
	flag.BoolVar(&showWpm, "showwpm", false, "Show words per minute")
	flag.IntVar(&timeout, "timeout", -1, "Timeout in seconds")
	flag.BoolVar(&oneShotMode, "oneshot", false, "Exit after one test")
	flag.IntVar(&numWords, "numwords", 50, "Number of words to use in the test")
	flag.IntVar(&numSegments, "numsegments", 1, "Number of segments to use in the test")

	flag.Usage = func() { os.Stdout.Write([]byte(usage)) } // flag.Usage是一个函数，用于打印使用信息
	flag.Parse()                                           // 解析命令行参数

	// List flag
	prefix := ""
	if listFlag != "" {
		if listFlag == "words" || listFlag == "quotes" {
			prefix = "./data//" + listFlag + "/"
		} else {
			prefix = "./" + listFlag + "/"
		}

		// List files located at the prefix
		fmt.Println("Files in " + prefix)
		files, err := os.ReadDir(prefix)
		if err != nil {
			exit("Error listing files: %s", err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}
			fmt.Println(strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
		}

		os.Exit(0)
	}

	// Version flag
	if versionFlag {
		fmt.Println("gotype v0.1")
		os.Exit(1)
	}

	// Set the typing test getter
	switch {
	case wordFile != "":
		typingTestGetter = generateWordsTestFromFile(wordFile, numWords, numSegments)
	case quoteFile != "":
		typingTestGetter = generateQuoteTestFromFile(quoteFile)
	case wordLlm != "":
		typingTestGetter = generateTestFromLLM("words", wordLlm, numWords)
	case quoteLlm != "":
		typingTestGetter = generateTestFromLLM("quote", quoteLlm, numWords)
	default:
		typingTestGetter = generateWordsTestFromFile("english_1k", numWords, numSegments)
	}

	// Set up screen
	scr, err = tcell.NewScreen()
	if err != nil { // 如果err不为空
		panic(err)
	}

	if err := scr.Init(); err != nil { // 初始化屏幕
		panic(err)
	}

	defer func() {
		if err := recover(); err != nil {
			scr.Fini()
			panic(err)
		}
	}()

	//	TODO Set up theme

	// Set up gotype object
	var gotype *gotype = createGoType(scr, boldFlag, themeName) // TODO

	gotype.SkipWord = !noSkip
	gotype.DisableBackspace = noBackspace
	gotype.BlockCursor = normalCursor
	gotype.ShowWpm = showWpm

	if timeout != -1 { // 如果timeout不为0, 则
		timeout *= 1e9
	}

	var tests [][]segment  // 测试数组
	var currentTestIdx int // 当前测试

	// Logic loop
	for {
		if currentTestIdx >= len(tests) { // 如果当前测试索引大于等于测试数组的长度
			tests = append(tests, typingTestGetter()) // 将新的测试添加到测试数组
		}

		if tests[currentTestIdx] == nil { // 如果当前测试为空
			exit("No tests available")
		}

		// wrap the text
		for idx, _ := range tests[currentTestIdx] {
			tests[currentTestIdx][idx].Text = wrapText(tests[currentTestIdx][idx].Text, 80)
		}

		numerrors, numcorrect, dur, rc, mistakes, wpms := gotype.StartTest(tests[currentTestIdx], time.Duration(timeout)) // 开始测试

		switch rc {
		case GoTypeNext:
			currentTestIdx++
		case GoTypePrevious:
			currentTestIdx--
		case GoTypeComplete:
			cpm := int(float64(numcorrect) / (float64(dur) / 60e9))
			wpm := cpm / 5
			accuracy := float64(numcorrect) / float64(numerrors+numcorrect) * 100

			results = append(results, result{wpm, cpm, accuracy, time.Now().Unix(), mistakes, wpms})
			// if !noReport {
			attribution := ""
			if len(tests[currentTestIdx]) == 1 {
				attribution = tests[currentTestIdx][0].Attribution
			}
			showReport(scr, cpm, wpm, accuracy, attribution, mistakes)

			// }
			if oneShotMode {
				exit_program(0)
			}

			currentTestIdx++
		case GoTypeSigInt:
			exit_program(1)
		case GoTypeResize:
			//Resize events restart the test, this shouldn't be a problem in the vast majority of cases and allows us to avoid baking rewrapping logic into the typer.
			//TODO:
		}

	}
}
