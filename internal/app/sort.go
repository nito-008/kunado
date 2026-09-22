package app

import (
	"cmp"
	"slices"
	"strings"
)

type sortColumn int

const (
	sortPort sortColumn = iota
	sortIP
	sortPID
	sortProcess
	sortCWD
)

func sortPorts(rows []Port, column sortColumn, ascending bool) {
	slices.SortStableFunc(rows, func(a, b Port) int {
		var n int
		switch column {
		case sortPort:
			n = cmp.Compare(a.Port, b.Port)
		case sortIP:
			n = strings.Compare(a.IP, b.IP)
		case sortPID:
			n = cmp.Compare(a.PID, b.PID)
		case sortProcess:
			n = strings.Compare(strings.ToLower(a.Process), strings.ToLower(b.Process))
		case sortCWD:
			n = strings.Compare(strings.ToLower(a.CWD), strings.ToLower(b.CWD))
		}
		if !ascending {
			return -n
		}
		return n
	})
}
