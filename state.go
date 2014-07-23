package main

import "os/exec"

func newState(params []string) *state {
	return &state{
		newRotation(params),
		map[string]*exec.Cmd{},
	}
}

type state struct {
	r    *rotation
	cmds map[string]*exec.Cmd
}

// Functions that keep the state unchanged.

func (s *state) currentParam() string {
	return s.r.current()
}

func (s *state) nextParam() string {
	return s.r.next()
}

func (s *state) currentCmd() *exec.Cmd {
	return s.cmd(s.r.current())
}

func (s *state) nextCmd() *exec.Cmd {
	return s.cmd(s.r.next())
}

func (s *state) cmd(param string) *exec.Cmd {
	if c, ok := s.cmds[param]; ok {
		return c
	}
	return nil
}

func (s *state) hasCmds() bool {
	return len(s.cmds) > 0
}

func (s *state) rotate() {
	s.r.rotate()
}

func (s *state) setCmd(param string, cmd *exec.Cmd) {
	s.cmds[param] = cmd
}

func (s *state) unsetCmd(param string) {
	delete(s.cmds, param)
}
