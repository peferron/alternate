package main

import (
	"errors"
	"os/exec"
)

type manager struct {
	i      int
	cmds   map[string]*exec.Cmd
	values []string
}

func newManager(values []string) *manager {
	return &manager{-1, map[string]*exec.Cmd{}, values}
}

func (m *manager) first() bool {
	return m.i == -1
}

func (m *manager) next() {
	m.i++
}

func (m *manager) currentValue() string {
	if m.i < 0 {
		panic(errors.New("Cannot call manager.currentValue when i is 0"))
	}
	return m.values[m.i%len(m.values)]
}

func (m *manager) nextValue() string {
	return m.values[(m.i+1)%len(m.values)]
}

func (m *manager) currentCmd() *exec.Cmd {
	return m.cmd(m.currentValue())
}

func (m *manager) nextCmd() *exec.Cmd {
	return m.cmd(m.nextValue())
}

func (m *manager) cmd(value string) *exec.Cmd {
	if c, ok := m.cmds[value]; ok {
		return c
	}
	return nil
}

func (m *manager) setCmd(value string, cmd *exec.Cmd) {
	m.cmds[value] = cmd
}

func (m *manager) unsetCmd(value string) {
	delete(m.cmds, value)
}

func (m *manager) hasCmds() bool {
	return len(m.cmds) > 0
}
