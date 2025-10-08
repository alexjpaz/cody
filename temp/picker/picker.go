package picker

import (
	"errors"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Hard-coded list
var items = []string{
	"alpha",
	"bravo",
	"charlie",
	"delta",
	"echo",
	"foxtrot",
	"golf",
}

type model struct {
	all      []string
	filtered []string
	input    textinput.Model
	quit     bool
	selected string
	err      error
}

func initialModel() model {
	ti := textinput.New()
	ti.Prompt = "filter> "
	ti.Placeholder = "type to filter; Enter prints"
	ti.Focus()

	return model{
		all:      append([]string(nil), items...),
		filtered: append([]string(nil), items...),
		input:    ti,
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quit = true
			m.err = errors.New("canceled")
			return m, tea.Quit
		case "enter":
			if len(m.filtered) == 0 {
				m.err = errors.New("no match")
				return m, tea.Quit
			}
			m.selected = m.filtered[0]
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)

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
	if m.quit {
		return ""
	}
	title := lipgloss.NewStyle().Bold(true).Render("Pick an item (Enter prints & exits)")
	var b strings.Builder
	b.WriteString(title + "\n\n")
	b.WriteString(m.input.View() + "\n\n")

	max := 20
	for i, s := range m.filtered {
		if i == max {
			b.WriteString(lipgloss.NewStyle().Faint(true).Render("...more\n"))
			break
		}
		b.WriteString("  " + s + "\n")
	}
	return b.String()
}

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
