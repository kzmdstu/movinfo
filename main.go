package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Timecode is timecode system that supports 30fps and (drop) 29.97fps.
// See introduction of drop frame timecode system at http://andrewduncan.net/timecodes/
type Timecode struct {
	drop  bool
	frame int
}

// NewTimecode creates new Timecode.
func NewTimecode(code string, drop bool) (*Timecode, error) {
	t := &Timecode{}
	t.drop = drop
	if len(code) != 11 {
		return nil, fmt.Errorf("invalid timecode: %v", code)
	}
	codes := [4]int{}
	for i := 0; i < len(code); i += 3 {
		n, err := strconv.Atoi(code[i : i+2])
		if err != nil {
			return nil, fmt.Errorf("invalid timecode: %v", code)
		}
		codes[i/3] = n
	}
	h := codes[0]
	m := codes[1]
	s := codes[2]
	f := codes[3]
	frame := 108000*h + 1800*m + 30*s + f
	if t.drop {
		totalMinutes := 60*h + m
		frame -= 2 * (totalMinutes - totalMinutes/10)
	}
	t.frame = frame
	return t, nil
}

// Add adds frames to the Timecode.
func (t *Timecode) Add(n int) {
	t.frame += n
}

// String represents the Timecode as string.
func (t *Timecode) String() string {
	frame := t.frame
	if t.drop {
		D := frame / 17982  // number of "full" 10 minutes chunks in drop frame system
		M := frame % 17982  // remainder frames
		d := (M - 2) / 1798 // number of 1 minute chunks those drop frames; M-2 because the first chunk will not drop frames
		frame += 18*D + 2*d // 10 minutes chunks drop 18 frames; 1 minute chunks drop 2 frames
	}
	h := frame / 30 / 60 / 60 % 24
	m := frame / 30 / 60 % 60
	s := frame / 30 % 60
	f := frame % 30
	codes := [4]int{h, m, s, f}
	timecode := ""
	for i, c := range codes {
		if i == 1 || i == 2 {
			timecode += ":"
		}
		if i == 3 {
			if t.drop {
				timecode += ";"
			} else {
				timecode += ":"
			}
		}
		tc := strconv.Itoa(c)
		if len(tc) == 1 {
			tc = "0" + tc
		}
		timecode += tc
	}
	return timecode
}

func main() {
	log.SetFlags(0)
	var (
		start, end, duration, resolution bool
	)
	flag.BoolVar(&start, "start", false, "get start frame timecode from the mov.")
	flag.BoolVar(&end, "end", false, "get end frame timecode from the mov.")
	flag.BoolVar(&duration, "duration", false, "get duration in frame from the mov.")
	flag.BoolVar(&resolution, "resolution", false, "get resolution of the mov.")
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Print(filepath.Base(os.Args[0]) + " [args...] movfile")
		flag.PrintDefaults()
		log.Println("Results will be printed following order regardless of the flag order given by user: ")
		log.Println("\tstart, end, duration, resolution")
		return
	}
	file := args[0]
	ext := filepath.Ext(file)
	if ext != "" {
		// remove dot(.)
		ext = ext[1:]
	}
	if !start && !end && !duration && !resolution {
		log.Fatalf("need to set at least one of -start, -end, -duration, -resolution flag")
	}

	c := exec.Command("ffprobe", "-show_streams", file)
	out, err := c.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to execute: %v", string(out))
	}

	lines := strings.Split(string(out), "\n")
	timecode := ""
	fps := ""
	frames := 0
	width := ""
	height := ""
	for _, l := range lines {
		if fps != "" && timecode != "" && frames != 0 {
			break
		}
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "TAG:timecode=") {
			timecode = strings.TrimPrefix(l, "TAG:timecode=")
			if len(timecode) != 11 {
				log.Fatal("invalid timecode: %v", l)
			}
		}
		if strings.HasPrefix(l, "Stream #0:") && fps == "" {
			flds := strings.Fields(l)
			if flds[2] != "Video:" {
				continue
			}
			idx := -1
			for i, f := range flds {
				if f == "fps" || f == "fps," {
					idx = i
					break
				}
			}
			if idx == -1 {
				continue
			}
			fps = flds[idx-1]
		}
		if strings.HasPrefix(l, "nb_frames=") && frames == 0 {
			frames, err = strconv.Atoi(strings.TrimPrefix(l, "nb_frames="))
			if err != nil {
				log.Fatal("invalid frames: %v", l)
			}
		}
		if strings.HasPrefix(l, "width=") && width == "" {
			width = strings.TrimPrefix(l, "width=")
		}
		if strings.HasPrefix(l, "height=") && height == "" {
			height = strings.TrimPrefix(l, "height=")
		}
	}
	if start {
		if timecode == "" {
			log.Fatal("missing TAG:timecode information")
		}
		fmt.Println(timecode)
	}
	if end {
		if timecode == "" {
			log.Fatal("missing TAG:timecode information")
		}
		if fps == "" {
			log.Fatal("missing fps information")
		}
		if frames == 0 {
			log.Fatal("missing nb_frames information")
		}
		if fps != "30" && fps != "29.97" {
			log.Fatalf("unsupported fps: %v", fps)
		}
		drop := false
		if fps == "29.97" {
			drop = true
		}
		tc, err := NewTimecode(timecode, drop)
		if err != nil {
			log.Fatal(err)
		}
		tc.Add(frames - 1)
		fmt.Println(tc)
	}
	if duration {
		if frames == 0 {
			log.Fatal("missing nb_frames information")
		}
		fmt.Println(frames)
	}
	if resolution {
		if width == "" {
			log.Fatal("missing width information")
		}
		if height == "" {
			log.Fatal("missing height information")
		}
		fmt.Println(width + "*" + height)
	}
}
