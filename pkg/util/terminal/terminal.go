package terminal

import (
	"fmt"
	"github.com/fatih/color"
	"os"
	"time"
	"unicode/utf8"
)

type Spinner struct {
	Interval time.Duration
	Frames []string
}

var DotsSpinner = &Spinner{
	Interval: 80 * time.Millisecond,
	Frames:   []string{
		"⠙",
		"⠹",
		"⠸",
		"⠼",
		"⠴",
		"⠦",
		"⠧",
		"⠇",
		"⠏",
	},
}

type Terminal struct {
	Yellow func(format string, a ...interface{}) string
	Red func(format string, a ...interface{}) string
	Green func(format string, a ...interface{}) string
	Spinning func(format string, a ...interface{}) string
	Spinner *Spinner
	lastOneLineLen int
}

func NewTerminalPrint() *Terminal {
	return &Terminal{
		Yellow:  color.New(color.FgYellow).SprintfFunc(),
		Red:     color.New(color.FgRed).SprintfFunc(),
		Green:   color.New(color.FgCyan).SprintfFunc(),
		Spinning:color.New(color.FgHiCyan).SprintfFunc(),
		Spinner: DotsSpinner,
	}
}

func (t *Terminal) PrintOneLine(content string)  {
	for i := 0; i < t.lastOneLineLen; i++ {
		fmt.Print("\b")
	}
	fmt.Print("\033[J")
	fmt.Print(content)
}

func (t *Terminal) successString(content string) string {
	return t.Green("✔ %s", content)
}

func (t *Terminal) errorString(content string) string {
	return t.Red("✗ %s",content)
}

func (t *Terminal) warnString(content string) string  {
	return t.Yellow("! %s", content)
}

func (t *Terminal) spinningString(index int, content string) string  {
	return t.Spinning("%s %s", t.Spinner.Frames[index], content)
}

func (t *Terminal) infoString (content string) string {
	return t.Spinning("■ %s", content)
}

func (t *Terminal) PrintSuccessOneLine (format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t.PrintOneLine(t.successString(str))
	t.lastOneLineLen = utf8.RuneCountInString(str) + 2
}

func (t *Terminal) PrintWarnOneLine(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t.PrintOneLine(t.warnString(str))
	t.lastOneLineLen = utf8.RuneCountInString(str) + 2
}

func (t *Terminal) PrintInfoOneLine(format string, a ...interface{}) {
	str := fmt.Sprintf(format, a...)
	t.PrintOneLine(t.infoString(str))
	t.lastOneLineLen = utf8.RuneCountInString(str) + 2
}

func (t *Terminal) PrintErrorOneLineWithPanic (a ...interface{})  {
	t.PrintOneLine(t.errorString(fmt.Sprint(a...)))
	t.LineEnd()
	panic(a)
}

func (t *Terminal) PrintErrorOneLine (a ...interface{})  {
	t.PrintOneLine(t.errorString(fmt.Sprint(a...)))
	t.lastOneLineLen = 0
	t.LineEnd()
}

func (t *Terminal) PrintErrorOneLineWithExit (a ...interface{})  {
	t.PrintOneLine(t.errorString(fmt.Sprint(a...)))
	t.LineEnd()
	os.Exit(1)
}

func (t *Terminal) PrintLoadingOneLine (done chan bool, format string, a ...interface{})  {
	go func() {
		index := 0
		for {
			select {
			case <-done:
				return
			case <-time.After(t.Spinner.Interval):
				str := fmt.Sprintf(format, a...)
				t.PrintOneLine(t.spinningString(index, str))
				t.lastOneLineLen = utf8.RuneCountInString(str) + 2
				index = (index + 1) % len(t.Spinner.Frames)
			}
		}
	}()
}

func (t *Terminal) LineEnd()  {
	fmt.Print("\n")
}