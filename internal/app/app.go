package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func Run(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown option: %s", args[0])
	}

	scanner := NewScanner()
	m := newModel(scanner, 2*time.Second)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

