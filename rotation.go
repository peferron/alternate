package main

import "log"

func newRotation(s []string) *rotation {
	return &rotation{0, s}
}

type rotation struct {
	i int
	s []string
}

func (r *rotation) current() string {
	if r.i < 0 {
		log.Fatalf("Cannot call rotation.current() when rotation.i is %d", r.i)
	}
	return r.s[r.i%len(r.s)]
}

func (r *rotation) next() string {
	if r.i < -1 {
		log.Fatalf("Cannot call rotation.next() when rotation.i is %d", r.i)
	}
	return r.s[(r.i+1)%len(r.s)]
}

func (r *rotation) rotate() {
	r.i++
}
