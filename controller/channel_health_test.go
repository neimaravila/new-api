package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseHealthFilter(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "slow", input: "slow", want: "slow"},
		{name: "slow uppercase", input: "SLOW", want: "slow"},
		{name: "untested", input: "untested", want: "untested"},
		{name: "empty", input: "", want: ""},
		{name: "unknown", input: "broken", want: ""},
		{name: "not trimmed", input: " slow", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, parseHealthFilter(tc.input))
		})
	}
}
