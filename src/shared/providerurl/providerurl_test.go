package providerurl

import "testing"

func TestEffectiveBinance(t *testing.T) {
	cases := []struct {
		name, saved, env, want string
	}{
		{"saved wins", "https://mirror.example", "https://env.example", "https://mirror.example"},
		{"env when nothing saved", "", "https://env.example", "https://env.example"},
		{"default when neither", "", "", DefaultBinanceBaseURL},
		{"blank everywhere is the default", " ", " ", DefaultBinanceBaseURL},
		{"values are trimmed", "  https://mirror.example ", "", "https://mirror.example"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EffectiveBinance(c.saved, c.env); got != c.want {
				t.Fatalf("got %q want %q", got, c.want)
			}
		})
	}
}
