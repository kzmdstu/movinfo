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
type Timecode struct {
	drop  bool
	codes [4]int
}

// NewTimecode creates new Timecode.
func NewTimecode(code string, drop bool) (*Timecode, error) {
	t := &Timecode{}
	if len(code) != 11 {
		return nil, fmt.Errorf("invalid timecode: %v", code)
	}
	for i := 0; i < len(code); i += 3 {
		n, err := strconv.Atoi(code[i : i+2])
		if err != nil {
			return nil, fmt.Errorf("invalid timecode: %v", code)
		}
		t.codes[i/3] = n
	}
	t.drop = drop
	return t, nil
}

// Add adds frames to the Timecode.
func (t *Timecode) Add(n int) {
	h := t.codes[0]
	m := t.codes[1]
	s := t.codes[2]
	f := t.codes[3]
	// see http://andrewduncan.net/timecodes/
	frame := 108000*h + 1800*m + 30*s + f
	if t.drop {
		totalMinutes := 60*h + m
		frame -= 2 * (totalMinutes - totalMinutes/10)
	}
	frame += n
	if t.drop {
		D := frame / 17982  // number of "full" 10 minutes chunks in drop frame system
		M := frame % 17982  // remainder frames
		d := (M - 2) / 1798 // number of 1 minute chunks those drop frames; M-2 because the first chunk will not drop frames
		frame += 18*D + 2*d // 10 minutes chunks drop 18 frames; 1 minute chunks drop 2 frames
	}
	t.codes[0] = frame / 30 / 60 / 60 % 24
	t.codes[1] = frame / 30 / 60 % 60
	t.codes[2] = frame / 30 % 60
	t.codes[3] = frame % 30
}

// String represents the Timecode as string.
func (t *Timecode) String() string {
	timecode := ""
	for i, c := range t.codes {
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
		start, end bool
	)
	flag.BoolVar(&start, "start", false, "get start frame timecode from the mov. no-op when the input is an image")
	flag.BoolVar(&end, "end", false, "get end frame timecode from the mov. no-op when the input is an image")
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Print(filepath.Base(os.Args[0]) + " [args...] movfile")
		flag.PrintDefaults()
		return
	}
	file := args[0]
	ext := filepath.Ext(file)
	if ext != "" {
		// remove dot(.)
		ext = ext[1:]
	}
	if start && end {
		log.Fatalf("cannot set both -start and -end flags")
	}
	if !start && !end {
		log.Fatalf("need to set either -start or -end flag for mov input")
	}

	c := exec.Command("ffprobe", "-show_streams", file)
	out, err := c.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to execute: %v", string(out))
	}

	lines := strings.Split(string(out), "\n")
	timecode := ""
	fps := ""
	if start {
		// no need to find fps
		fps = "n/a"
	}
	frames := 0
	if start {
		// no need to find frames
		frames = -1
	}
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
		if strings.HasPrefix(l, "Stream #0:0") && fps == "" {
			flds := strings.Fields(l)
			idx := -1
			for i, f := range flds {
				if f == "fps" || f == "fps," {
					idx = i
					break
				}
			}
			if idx == -1 {
				log.Fatalf("cannot get fps from mov: %v", file)
			}
			fps = flds[idx-1]
		}
		if strings.HasPrefix(l, "nb_frames=") && frames == 0 {
			frames, err = strconv.Atoi(strings.TrimPrefix(l, "nb_frames="))
			if err != nil {
				log.Fatal("invalid frames: %v", l)
			}
		}
	}
	if timecode == "" {
		log.Fatal("missing timecode information")
	}
	if fps == "" {
		log.Fatal("missing fps information")
	}
	if frames == 0 {
		log.Fatal("missing frames information")
	}
	if start {
		fmt.Println(timecode)
		os.Exit(0)
	}
	// end
	if fps != "30" && fps != "29.97" {
		log.Fatal("unsupported fps: %v", fps)
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
