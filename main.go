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

type config struct {
	start bool
	end bool
	duration bool
	resolution bool
}

type result struct {
	start string
	end string
	duration string
	resolution string
}

func main() {
	log.SetFlags(0)
	cfg := config{}
	flag.BoolVar(&cfg.start, "start", false, "get start frame timecode from the mov.")
	flag.BoolVar(&cfg.end, "end", false, "get end frame timecode from the mov.")
	flag.BoolVar(&cfg.duration, "duration", false, "get duration in frame from the mov.")
	flag.BoolVar(&cfg.resolution, "resolution", false, "get resolution of the mov.")
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
	if !cfg.start && !cfg.end && !cfg.duration && !cfg.resolution {
		log.Fatalf("need to set at least one of -start, -end, -duration, -resolution flag")
	}

	c := exec.Command("ffprobe", "-show_streams", file)
	b, err := c.CombinedOutput()
	if err != nil {
		log.Fatalf("failed to execute: %s", b)
	}
	out := string(b)
	res, err := parse(out, cfg)
	if err != nil {
		log.Fatal(err)
	}
	if res.start != "" {
		fmt.Println(res.start)
	}
	if res.end != "" {
		fmt.Println(res.end)
	}
	if res.duration != "" {
		fmt.Println(res.duration)
	}
	if res.resolution != "" {
		fmt.Println(res.resolution)
	}
}

// parse parses ffprobe output data for a mov.
func parse(data string, cfg config) (res result, err error) {
	idx := strings.Index(data, "[STREAM]")
	if idx == -1 {
		return res, fmt.Errorf("cannot find [STREAM] lines")
	}
	overview := data[:idx]
	streamData := data[idx:]
	fps := ""
	videoIdx := -1
	for _, l := range strings.Split(overview, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Stream #0:") && fps == "" {
			flds := strings.Fields(l)
			if flds[2] != "Video:" {
				continue
			}
			rest := strings.TrimPrefix(l, "Stream #0:")
			if len(rest) == 0 {
				return res, fmt.Errorf("unexpected stream line")
			}
			trimDigits := strings.TrimLeft(rest, "0123456789")
			n := rest[:len(rest)-len(trimDigits)]
			videoIdx, err = strconv.Atoi(n)
			if err != nil {
				return res, fmt.Errorf("unexpected stream line")
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
	}
	if videoIdx == -1 {
		return res, fmt.Errorf("not found video stream")
	}
	streams := strings.SplitAfter(streamData, "[/STREAM]")
	if videoIdx >= len(streams) {
		return res, fmt.Errorf("unmatched video stream")
	}
	frames := 0
	width := ""
	height := ""
	timecode := ""
	videoStream := streams[videoIdx]
	for _, l := range strings.Split(videoStream, "\n") {
		if fps != "" && timecode != "" && frames != 0 {
			break
		}
		if strings.HasPrefix(l, "nb_frames=") && frames == 0 {
			frames, err = strconv.Atoi(strings.TrimPrefix(l, "nb_frames="))
			if err != nil {
				return res, fmt.Errorf("invalid frames: %v", l)
			}
		}
		if strings.HasPrefix(l, "width=") && width == "" {
			width = strings.TrimPrefix(l, "width=")
		}
		if strings.HasPrefix(l, "height=") && height == "" {
			height = strings.TrimPrefix(l, "height=")
		}
		if strings.HasPrefix(l, "TAG:timecode=") {
			timecode = strings.TrimPrefix(l, "TAG:timecode=")
			if len(timecode) != 11 {
				return res, fmt.Errorf("invalid timecode: %v", l)
			}
		}
	}
	if cfg.start {
		if timecode == "" {
			return res, fmt.Errorf("missing TAG:timecode information")
		}
		res.start = timecode
	}
	if cfg.end {
		if timecode == "" {
			return res, fmt.Errorf("missing TAG:timecode information")
		}
		if fps == "" {
			return res, fmt.Errorf("missing fps information")
		}
		if frames == 0 {
			return res, fmt.Errorf("missing nb_frames information")
		}
		if fps != "30" && fps != "29.97" && fps != "23.98" && fps != "23.976" {
			return res, fmt.Errorf("unsupported fps: %v", fps)
		}
		drop := false
		if fps == "29.97" || fps == "29.98" || fps == "29.976" {
			drop = true
		}
		tc, err := NewTimecode(timecode, drop)
		if err != nil {
			return res, err
		}
		tc.Add(frames - 1)
		res.end = tc.String()
	}
	if cfg.duration {
		if frames == 0 {
			return res, fmt.Errorf("missing nb_frames information")
		}
		res.duration = strconv.Itoa(frames)
	}
	if cfg.resolution {
		if width == "" {
			return res, fmt.Errorf("missing width information")
		}
		if height == "" {
			return res, fmt.Errorf("missing height information")
		}
		res.resolution = width + "*" + height
	}
	return res, nil
}
