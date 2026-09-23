package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type (
	tickMsg time.Time
	scanMsg struct {
		rows []Port
		err  error
	}
)

type model struct {
	scanner       *Scanner
	interval      time.Duration
	rows          []Port
	selected      int
	sort          sortColumn
	ascending     bool
	width, height int
	popup         bool
	err           error
}

func newModel(scanner *Scanner, interval time.Duration) model {
	if interval < 250*time.Millisecond {
		interval = 250 * time.Millisecond
	}
	return model{scanner: scanner, interval: interval, ascending: true}
}

func (m model) Init() tea.Cmd { return m.scanCmd() }

func (m model) scanCmd() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.scanner.Scan(context.Background())
		return scanMsg{rows: rows, err: err}
	}
}

func (m model) tickCmd() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case scanMsg:
		m.err = msg.err
		m.rows = msg.rows
		sortPorts(m.rows, m.sort, m.ascending)
		if m.selected >= len(m.rows) {
			m.selected = max(0, len(m.rows)-1)
		}
		return m, m.tickCmd()
	case tickMsg:
		if !m.popup {
			return m, m.scanCmd()
		}
		return m, m.tickCmd()
	case tea.KeyPressMsg:
		key := msg.String()
		if m.popup {
			switch key {
			case "q", "enter":
				m.popup = false
			}
			return m, nil
		}
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected+1 < len(m.rows) {
				m.selected++
			}
		case "left", "h":
			m.sort = (m.sort + 4) % 5
			sortPorts(m.rows, m.sort, m.ascending)
		case "right", "l":
			m.sort = (m.sort + 1) % 5
			sortPorts(m.rows, m.sort, m.ascending)
		case "r":
			m.ascending = !m.ascending
			sortPorts(m.rows, m.sort, m.ascending)
		case "enter":
			if len(m.rows) > 0 {
				m.popup = true
			}
		case "ctrl+r":
			return m, m.scanCmd()
		}
	}
	return m, nil
}

var (
	titleStyle            = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	headerStyle           = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	sectionStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	selectedBackdropStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("23"))
	dimStyle              = lipgloss.NewStyle().Faint(true)
	hintKeyStyle          = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	hintTextStyle         = lipgloss.NewStyle().Faint(true).Foreground(lipgloss.Color("7"))
	borderStyle           = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(1, 2)
)

func (m model) View() tea.View {
	base := m.listView()
	if m.popup && len(m.rows) > 0 {
		panel := m.detailView(m.rows[m.selected])
		layerBase := lipgloss.NewLayer(dimANSI(base)).X(0).Y(0).Z(0)
		pw, ph := lipgloss.Size(panel)
		x, y := max(0, (m.width-pw)/2), max(0, (m.height-ph)/2)
		modal := lipgloss.NewLayer(panel).X(x).Y(y).Z(1)
		base = lipgloss.NewCompositor(layerBase, modal).Render()
	}
	v := tea.NewView(base)
	v.AltScreen = true
	return v
}

func dimANSI(value string) string {
	const faint = "\x1b[2m"
	value = strings.ReplaceAll(value, "\x1b[0m", "\x1b[0m"+faint)
	value = strings.ReplaceAll(value, "\x1b[m", "\x1b[m"+faint)
	return faint + value + "\x1b[0m"
}

func (m model) listView() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\n\n", titleStyle.Render("kunado"), dimStyle.Render(time.Now().Format("15:04:05")))
	b.WriteString(m.tableHeader())
	b.WriteByte('\n')
	available := max(0, m.height-6)
	start := 0
	if m.selected >= available && available > 0 {
		start = m.selected - available + 1
	}
	end := min(len(m.rows), start+available)
	for i := start; i < end; i++ {
		line := renderRow(m.rows[i], i == m.selected, m.popup, m.width)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render(m.err.Error()))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(strings.Join([]string{
		hint("↑/↓", "select"),
		hint("←/→/r", "sort"),
		hint("Enter", "details"),
		hint("q", "quit"),
	}, "  "))
	return clipLines(b.String(), m.width, m.height)
}

func (m model) tableHeader() string {
	labels := []string{"PORT", "ADDR", "PID", "", "PROC", "CWD"}
	indices := []int{0, 1, 2, -1, 3, 4}
	for i, index := range indices {
		if index == int(m.sort) {
			if m.ascending {
				labels[i] += " ▲"
			} else {
				labels[i] += " ▼"
			}
		}
	}
	for i := range labels {
		labels[i] = headerStyle.Render(labels[i])
	}
	return renderColumns(labels)
}

func renderRow(row Port, selected, popup bool, screenWidth int) string {
	ipColor := "2"
	if strings.Contains(row.IP, ":") {
		ipColor = "5"
	}
	ip := row.IP
	icon, color := processIcon(row.Process)
	if !selected {
		ip = lipgloss.NewStyle().Foreground(lipgloss.Color(ipColor)).Render(ip)
		icon = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(icon)
	}
	pid := "-"
	if row.PID > 0 {
		pid = strconv.Itoa(row.PID)
	}
	cwd := row.CWD
	if cwd == "" {
		cwd = "-"
	}
	process := truncateMiddle(row.Process, columnWidths[4])
	if cwdWidth := screenWidth - fixedColumnsWidth(); cwdWidth > 1 {
		cwd = truncateMiddle(cwd, cwdWidth)
	}
	line := renderColumns([]string{":" + strconv.Itoa(row.Port), ip, pid, icon, process, cwd})
	if selected {
		if screenWidth > 0 {
			line = fixedWidth(line, screenWidth)
		}
		if popup {
			return selectedBackdropStyle.Render(line)
		}
		return selectedStyle.Render(line)
	}
	return line
}

var columnWidths = []int{6, 16, 8, 2, 28}

func renderColumns(values []string) string {
	cells := make([]string, 0, len(values))
	for i, value := range values {
		if i < len(columnWidths) {
			value = fixedWidth(value, columnWidths[i])
		}
		cells = append(cells, value)
	}
	return strings.Join(cells, " ")
}

func fixedWidth(value string, width int) string {
	value = ansi.Truncate(value, width, "")
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func fixedColumnsWidth() int {
	width := len(columnWidths)
	for _, columnWidth := range columnWidths {
		width += columnWidth
	}
	return width
}

func truncateMiddle(value string, width int) string {
	valueWidth := ansi.StringWidth(value)
	if valueWidth <= width || width < 2 {
		return value
	}
	left := width / 2
	right := width - 1 - left
	return ansi.Cut(value, 0, left) + "…" + ansi.Cut(value, valueWidth-right, valueWidth)
}

func (m model) detailView(row Port) string {
	value := func(v string) string {
		if v == "" {
			return "-"
		}
		return v
	}
	pid := "-"
	if row.PID > 0 {
		pid = strconv.Itoa(row.PID)
	}
	lines := []string{
		sectionStyle.Render("Detail:"),
		field("Address", fmt.Sprintf("%s:%d", row.IP, row.Port)),
		field("Protocol", row.Protocol), field("PID", pid), field("Process", row.Process),
		field("User", value(row.User)), field("Command", value(row.Command)), field("CWD", value(row.CWD)),
	}
	if row.Container != "" {
		lines = append(lines, "", sectionStyle.Render("Docker:"),
			field("Container", row.Container), field("Project", value(row.ComposeProject)),
			field("Service", value(row.ComposeService)), field("Container port", value(row.ContainerPort)))
	}
	lines = append(lines, "", hint("Enter/q", "close"))
	width := min(max(44, m.width-8), 88)
	return borderStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func field(label, value string) string {
	return lipgloss.NewStyle().Width(16).Render(headerStyle.Render(label)) + value
}

func hint(keys, action string) string {
	return hintKeyStyle.Render(keys) + " " + hintTextStyle.Render(action)
}

func processIcon(process string) (string, string) {
	p := strings.ToLower(process)
	switch {
	case strings.HasPrefix(p, "docker:"), strings.Contains(p, "docker"):
		return "󰡨", "4"
	case strings.HasPrefix(p, "python"), strings.Contains(p, "uvicorn"), strings.Contains(p, "gunicorn"):
		return "", "3"
	case strings.Contains(p, "code"), strings.Contains(p, "vscode"):
		return "󰨞", "4"
	case strings.HasPrefix(p, "node"), p == "npm", p == "npx", strings.Contains(p, "vite"), strings.HasPrefix(p, "bun"):
		return "", "2"
	case strings.Contains(p, "postgres"):
		return "", "4"
	case strings.Contains(p, "mysql"), strings.Contains(p, "mariadb"):
		return "", "4"
	case strings.Contains(p, "redis"):
		return "󰆼", "1"
	case strings.Contains(p, "nginx"):
		return "󰰓", "6"
	case p == "go" || strings.HasPrefix(p, "go-"):
		return "", "6"
	case strings.Contains(p, "qemu"):
		return "󰍹", "5"
	case strings.Contains(p, "sshd"):
		return "󰣀", "4"
	default:
		return "", "8"
	}
}

func clipLines(value string, width, height int) string {
	if width <= 0 {
		return value
	}
	lines := strings.Split(value, "\n")
	if height > 0 && len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], width, "")
	}
	return strings.Join(lines, "\n")
}

