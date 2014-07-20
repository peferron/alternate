package main

import (
	"log"
	"os/exec"
)

func newState(params []string) *state {
	return &state{
		-1,
		map[string]*exec.Cmd{},
		params,
	}
}

type state struct {
	i      int
	cmds   map[string]*exec.Cmd
	params []string
}

//
// Functions that keep the state unchanged.
//

func (s *state) currentParam() string {
	if s.i < 0 {
		log.Fatalf("Cannot call state.currentParam when state.i is %d", s.i)
	}
	return s.params[s.i%len(s.params)]
}

func (s *state) nextParam() string {
	return s.params[(s.i+1)%len(s.params)]
}

func (s *state) currentCmd() *exec.Cmd {
	return s.cmd(s.currentParam())
}

func (s *state) nextCmd() *exec.Cmd {
	return s.cmd(s.nextParam())
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

//
// Functions that change the state.
//

func (s *state) rotate() {
	s.i++
}

func (s *state) setCmd(param string, cmd *exec.Cmd) {
	s.cmds[param] = cmd
}

func (s *state) unsetCmd(param string) {
	delete(s.cmds, param)
}
