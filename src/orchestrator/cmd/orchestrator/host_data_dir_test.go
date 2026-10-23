package main

import "testing"

func TestPlausibleHostDataDir(t *testing.T) {
	cases := []struct {
		path string
		ok   bool
	}{
		{"/home/user/trading-copilot/data", true},
		{"/data", true},
		{"/", false},
		{"rw", false},
		{"/dev/mapper/vg-lv", false},
	}
	for _, c := range cases {
		if plausibleHostDataDir(c.path) != c.ok {
			t.Fatalf("%q want %v", c.path, c.ok)
		}
	}
}
