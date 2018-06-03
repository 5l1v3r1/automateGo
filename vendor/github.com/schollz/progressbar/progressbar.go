package progressbar

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// ProgressBar is a thread-safe, simple
// progress bar
type ProgressBar struct {
	state  state
	config config

	lock sync.RWMutex
}

type state struct {
	currentNum        int
	currentPercent    int
	lastPercent       int
	currentSaucerSize int

	lastShown time.Time
	startTime time.Time
}

type config struct {
	max                  int // max number of the counter
	width                int
	writer               io.Writer
	theme                Theme
	renderWithBlankState bool
}

// Theme defines the elements of the bar
type Theme struct {
	Saucer        string
	SaucerPadding string
	BarStart      string
	BarEnd        string
}

// Option is the type all options need to adhere to
type Option func(p *ProgressBar)

// OptionSetWidth sets the width of the bar
func OptionSetWidth(s int) Option {
	return func(p *ProgressBar) {
		p.config.width = s
	}
}

// OptionSetTheme sets the elements the bar is constructed of
func OptionSetTheme(t Theme) Option {
	return func(p *ProgressBar) {
		p.config.theme = t
	}
}

// OptionSetWriter sets the output writer (defaults to os.StdOut)
func OptionSetWriter(w io.Writer) Option {
	return func(p *ProgressBar) {
		p.config.writer = w
	}
}

// OptionSetRenderBlankState sets whether or not to render a 0% bar on construction
func OptionSetRenderBlankState(r bool) Option {
	return func(p *ProgressBar) {
		p.config.renderWithBlankState = r
	}
}

var defaultTheme = Theme{Saucer: "█", SaucerPadding: " ", BarStart: "|", BarEnd: "|"}

// NewOptions constructs a new instance of ProgressBar, with any options you specify
func NewOptions(max int, options ...Option) *ProgressBar {
	b := ProgressBar{
		state: getBlankState(),
		config: config{
			writer: os.Stdout,
			theme:  defaultTheme,
			width:  40,
			max:    max,
		},
		lock: sync.RWMutex{},
	}

	for _, o := range options {
		o(&b)
	}

	if b.config.renderWithBlankState {
		b.RenderBlank()
	}

	return &b
}

func getBlankState() state {
	now := time.Now()
	return state{
		startTime: now,
		lastShown: now,
	}
}

// New returns a new ProgressBar
// with the specified maximum
func New(max int) *ProgressBar {
	return NewOptions(max)
}

// RenderBlank renders the current bar state, you can use this to render a 0% state
func (p *ProgressBar) RenderBlank() error {
	return renderProgressBar(p.config, p.state)
}

// Reset will reset the clock that is used
// to calculate current time and the time left.
func (p *ProgressBar) Reset() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.state = getBlankState()
}

// Add with increase the current count on the progress bar
func (p *ProgressBar) Add(num int) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.config.max == 0 {
		return errors.New("max must be greater than 0")
	}
	p.state.currentNum += num
	percent := float64(p.state.currentNum) / float64(p.config.max)
	p.state.currentSaucerSize = int(percent * float64(p.config.width))
	p.state.currentPercent = int(percent * 100)
	updateBar := p.state.currentPercent != p.state.lastPercent && p.state.currentPercent > 0

	p.state.lastPercent = p.state.currentPercent
	if p.state.currentNum > p.config.max {
		return errors.New("current number exceeds max")
	}

	if updateBar {
		return renderProgressBar(p.config, p.state)
	}

	return nil
}

func renderProgressBar(c config, s state) error {
	var leftTime float64
	if s.currentNum > 0 {
		leftTime = time.Since(s.startTime).Seconds() / float64(s.currentNum) * (float64(c.max) - float64(s.currentNum))
	}

	str := fmt.Sprintf("\r%4d%% %s%s%s%s [%s:%s]            ",
		s.currentPercent,
		c.theme.BarStart,
		strings.Repeat(c.theme.Saucer, s.currentSaucerSize),
		strings.Repeat(c.theme.SaucerPadding, c.width-s.currentSaucerSize),
		c.theme.BarEnd,
		(time.Duration(time.Since(s.startTime).Seconds()) * time.Second).String(),
		(time.Duration(leftTime) * time.Second).String(),
	)
	_, err := io.WriteString(c.writer, str)
	if err != nil {
		return err
	}

	if f, ok := c.writer.(*os.File); ok {
		return f.Sync()
	}

	return nil
}
