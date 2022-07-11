package main

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct{
		file string
		want result
	}{
		{
			file: "testdata/ffprobe_1.out",
			want: result{
				start: "00:00:00:00",
				end: "00:00:03:11",
				duration: "102",
				resolution: "1920*1080",
			},
		},
	}
	cfg := config {
		start: true,
		end: true,
		duration: true,
		resolution: true,
	}
	for _, c := range cases {
		b, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatalf("couldn't read file: %s", c.file)
		}
		out := string(b)
		got, err := parse(out, cfg)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if got != c.want {
			t.Fatalf("got %v, want %v", got, c.want)
		}
	}
}
