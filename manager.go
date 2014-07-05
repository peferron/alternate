package main

import (
	"errors"
	"os/exec"
)

type manager struct {
	i      int
	cmds   map[string]*exec.Cmd
	params []string
}

func newManager(params []string) *manager {
	return &manager{-1, map[string]*exec.Cmd{}, params}
}

func (m *manager) first() bool {
	return m.i == -1
}

func (m *manager) next() {
	m.i++
}

func (m *manager) currentParam() string {
	if m.i < 0 {
		panic(errors.New("Cannot call manager.currentParam when i is 0"))
	}
	return m.params[m.i%len(m.params)]
}

func (m *manager) nextParam() string {
	return m.params[(m.i+1)%len(m.params)]
}

func (m *manager) currentCmd() *exec.Cmd {
	return m.cmd(m.currentParam())
}

func (m *manager) nextCmd() *exec.Cmd {
	return m.cmd(m.nextParam())
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
