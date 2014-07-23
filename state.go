package main

import "os/exec"

func newState(params []string) *state {
	return &state{
		newRotation(params),
		map[string]*exec.Cmd{},
	}
}

type state struct {
	rotation *rotation
	cmds     map[string]*exec.Cmd
}

type eachFunc func(p string, c *exec.Cmd)

// Functions that keep the state unchanged.

func (s *state) current() (string, *exec.Cmd) {
	p := s.rotation.current()
	return p, s.cmd(p)
}

func (s *state) next() (string, *exec.Cmd) {
	p := s.rotation.next()
	return p, s.cmd(p)
}

func (s *state) cmd(param string) *exec.Cmd {
	if c, ok := s.cmds[param]; ok {
		return c
	}
	return nil
}

func (s *state) empty() bool {
	return len(s.cmds) == 0
}

func (s *state) each(f eachFunc) {
	for p, c := range s.cmds {
		f(p, c)
	}
}

// Functions that change the state.

func (s *state) set(param string, cmd *exec.Cmd) {
	s.cmds[param] = cmd
}

func (s *state) unset(param string) {
	delete(s.cmds, param)
}

func (s *state) rotate() {
	s.rotation.rotate()
}
