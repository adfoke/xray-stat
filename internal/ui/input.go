package ui

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strings"
)

type InputAction int

const (
	ActionScrollOlder InputAction = iota + 1
	ActionScrollNewer
	ActionPageOlder
	ActionPageNewer
	ActionToNewest
	ActionToOldest
)

func StartInput(ctx context.Context) (<-chan InputAction, func(), error) {
	if !isCharDevice(os.Stdin) {
		return nil, func() {}, nil
	}
	restore, err := enableImmediateInput()
	if err != nil {
		return nil, func() {}, err
	}
	ch := make(chan InputAction, 16)
	go readInput(ctx, ch)
	return ch, restore, nil
}

func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func enableImmediateInput() (func(), error) {
	stateCmd := exec.Command("stty", "-g")
	stateCmd.Stdin = os.Stdin
	state, err := stateCmd.Output()
	if err != nil {
		return func() {}, err
	}

	rawCmd := exec.Command("stty", "cbreak", "-echo")
	rawCmd.Stdin = os.Stdin
	if err := rawCmd.Run(); err != nil {
		return func() {}, err
	}

	restore := strings.TrimSpace(string(state))
	return func() {
		cmd := exec.Command("stty", restore)
		cmd.Stdin = os.Stdin
		_ = cmd.Run()
	}, nil
}

func readInput(ctx context.Context, ch chan<- InputAction) {
	defer close(ch)
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		b, err := reader.ReadByte()
		if err != nil {
			return
		}

		switch b {
		case 'j', 'J':
			sendAction(ctx, ch, ActionScrollOlder)
		case 'k', 'K':
			sendAction(ctx, ch, ActionScrollNewer)
		case 'g':
			sendAction(ctx, ch, ActionToNewest)
		case 'G':
			sendAction(ctx, ch, ActionToOldest)
		case 27:
			parseEscape(ctx, reader, ch)
		}
	}
}

func parseEscape(ctx context.Context, reader *bufio.Reader, ch chan<- InputAction) {
	next, err := reader.ReadByte()
	if err != nil || next != '[' {
		return
	}
	b, err := reader.ReadByte()
	if err != nil {
		return
	}
	switch b {
	case 'A':
		sendAction(ctx, ch, ActionScrollNewer)
	case 'B':
		sendAction(ctx, ch, ActionScrollOlder)
	case '5':
		if tail, err := reader.ReadByte(); err == nil && tail == '~' {
			sendAction(ctx, ch, ActionPageNewer)
		}
	case '6':
		if tail, err := reader.ReadByte(); err == nil && tail == '~' {
			sendAction(ctx, ch, ActionPageOlder)
		}
	case 'H':
		sendAction(ctx, ch, ActionToNewest)
	case 'F':
		sendAction(ctx, ch, ActionToOldest)
	}
}

func sendAction(ctx context.Context, ch chan<- InputAction, action InputAction) {
	select {
	case <-ctx.Done():
	case ch <- action:
	default:
	}
}
