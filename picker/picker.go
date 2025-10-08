package picker

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hard-coded values to display/filter
var items = []string{
	"apple",
	"banana",
	"blueberry",
	"blackberry",
	"cherry",
	"grape",
	"mango",
	"orange",
	"peach",
	"strawberry",
}

type model struct {
	all      []string
	filtered []string
	input    textinput.Model
	quitting bool
	selected string
	err      error
}

func initialModel() model {
	ti := textinput.New()
	ti.Prompt = "filter> "
	ti.Placeholder = "type to filter, Enter prints first match"
	ti.Focus()

	m := model{
		all:      append([]string(nil), items...), // copy
		filtered: append([]string(nil), items...),
		input:    ti,
	}
	return m
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			m.err = errors.New("canceled")
			return m, tea.Quit
		case "enter":
			if len(m.filtered) == 0 {
				m.err = errors.New("no match")
				return m, tea.Quit
			}
			// pick the first filtered item
			m.selected = m.filtered[0]
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

	// recalc filter
	q := strings.ToLower(m.input.Value())
	if q == "" {
		m.filtered = m.all
	} else {
		out := make([]string, 0, len(m.all))
		for _, s := range m.all {
			if strings.Contains(strings.ToLower(s), q) {
				out = append(out, s)
			}
		}
		m.filtered = out
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	title := lipgloss.NewStyle().Bold(true).Render("Pick a fruit (Enter prints first match)")
	var b strings.Builder
	b.WriteString(title + "\n\n")
	b.WriteString(m.input.View() + "\n\n")

	max := 20
	for i, s := range m.filtered {
		if i == max {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render(
				"...and "+itoa(len(m.filtered)-max)+" more") + "\n")
			break
		}
		b.WriteString("  " + s + "\n")
	}
	return b.String()
}

// Run starts the TUI and returns the selected line (or an error).
func Run() (string, error) {
	p := tea.NewProgram(initialModel())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	m := final.(model)
	if m.err != nil {
		return "", m.err
	}
	return m.selected, nil
}

// tiny int->string helper to avoid fmt in View()
func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	n := i
	for n > 0 {
		pos--
		buf[pos] = digits[n%10]
		n /= 10
	}
	return string(buf[pos:])
}
