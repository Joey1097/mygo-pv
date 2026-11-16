// Command pv renders a forty-five second 3:4 introduction to MyGo's render
// plugin, in the language the band's own teaser PV uses: the picture comes
// up out of black into a blown white field, the name is set in slabs that
// climb, and the whole thing leans.
//
//	go run ./pv                      # renders/mygo-pv.mp4
//	go run ./pv -o /tmp/pv.mp4
//	go run ./pv -stills 30,120,300   # PNGs of those frames
//
// Everything in it is drawn by mygo's ui package on the GPU: no browser, no
// image files, no footage. The soundtrack is synthesised in Go by
// mygomotion/sound, and the waveform the last section draws is that
// soundtrack's own samples.
package main

import (
	"context"
	"flag"
	"fmt"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/render"
	"github.com/egoist/mygo/ui"

	"mygomotion/sound"
)

const (
	pixelW = 1080
	pixelH = 1440
	scale  = 2
	fps    = 30
	frames = 45 * fps
	dipW   = pixelW / scale
	dipH   = pixelH / scale

	// The piece runs at 120 BPM: a beat is fifteen frames, a bar sixty.
	beat = 15
	bars = 60
)

// Where each section starts. Every one cuts in on a bar line.
const (
	atBloom  = 0    // 0.0–2.4   black, into a blown white field
	atTitle  = 72   // 2.4–8.0   the name, set in slabs
	atFive   = 240  // 8.0–18.0  five cuts, one per "!"
	atUI     = 540  // 18.0–26.0 the widgets
	atFrames = 780  // 26.0–33.0 the strip of frames
	atSound  = 990  // 33.0–38.0 the soundtrack, drawn
	atEnd    = 1140 // 38.0–45.0 the name again, and where to get it
)

// The palette is the teaser's: a cool white, the band's blue, the other
// band's crimson, and nothing else.
var (
	paper   = ui.Hex("#E9ECEF")
	ink     = ui.Hex("#0A0A0C")
	blue    = ui.Hex("#2F7CAC")
	crimson = ui.Hex("#960A4C")
	steel   = ui.Hex("#6E7783")
	silver  = ui.Hex("#B9C0C8")
	hairOn  = ui.Hex("#C6CBD1") // a hairline on white
	hairOff = ui.Hex("#23252B") // a hairline on black
	faint   = ui.Hex("#33373F") // a dim rule on black
	surface = ui.Hex("#111318")
	edge    = ui.Hex("#272B33")
)

// slabCols are the five "!" of MyGO!!!!!. The teaser's lockup runs them
// #206488 -> #5B3B73, a cool blue into a dim violet; these are the same
// trajectory lifted, so that the same five colours read on paper and on
// ink without changing identity between the mark and the cuts.
var slabCols = [5]ui.Color{
	ui.Hex("#2E86B4"),
	ui.Hex("#3A7FB4"),
	ui.Hex("#4F6FB0"),
	ui.Hex("#665BAE"),
	ui.Hex("#7E46AC"),
}

// bangRot are the angles the framework author's own site sets the five
// bangs at (-rotate-9, rotate-6, -rotate-4, rotate-8, -rotate-6): each one
// its own way, which is what makes the mark read as five people and not
// as five slabs.
var bangRot = [5]float32{-9, 6, -4, 8, -6}

// onPaper is whether the piece is a white frame at this point: the title,
// and the end.
func onPaper(i int) bool {
	return (i >= atTitle && i < atFive) || i >= atEnd
}

func main() {
	out := flag.String("o", "renders/mygo-pv.mp4", "output file")
	stills := flag.String("stills", "", "comma separated frames to write as PNGs instead")
	progress := flag.Bool("progress", true, "print render progress")
	flag.Parse()

	loadFonts()
	snd, env := score()
	comp := composition(snd, env)

	o := render.Options{}
	if *progress {
		o.Progress = func(p render.Progress) {
			if p.Done%60 == 0 || p.Done == p.Frames {
				fmt.Printf("\r%s: %d/%d frames, %.0f fps", p.Renderer, p.Done, p.Frames, float64(p.Done)/p.Elapsed.Seconds())
			}
		}
	}

	start := time.Now()
	var err error
	if *stills != "" {
		err = stillsTo(comp, *stills)
	} else {
		if dir := filepath.Dir(*out); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				log.Fatal(err)
			}
		}
		err = render.Video(context.Background(), comp, *out, o)
	}
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}
	if *stills == "" {
		abs, _ := filepath.Abs(*out)
		fmt.Printf("%s in %v\n", abs, time.Since(start).Round(time.Millisecond))
	}
}

func stillsTo(comp render.Composition, list string) error {
	if err := os.MkdirAll("renders", 0o755); err != nil {
		return err
	}
	for _, s := range strings.Split(list, ",") {
		var n int
		if _, err := fmt.Sscan(strings.TrimSpace(s), &n); err != nil {
			return err
		}
		img, err := render.Still(comp, n, render.Options{})
		if err != nil {
			return err
		}
		name := fmt.Sprintf("renders/pv-still-%04d.png", n)
		f, err := os.Create(name)
		if err != nil {
			return err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		fmt.Println(name)
	}
	return nil
}

func loadFonts() {
	for _, f := range []struct{ path, family string }{
		{"assets/Geist.ttf", "Geist"},
		{"assets/GeistMono.ttf", "GeistMono"},
		{"assets/Anton-Regular.ttf", "Anton"},
	} {
		data, err := os.ReadFile(f.path)
		if err != nil {
			log.Fatalf("font %s: %v", f.path, err)
		}
		if err := ui.RegisterFont(data, f.family); err != nil {
			log.Fatalf("font %s: %v", f.path, err)
		}
	}
}

func composition(snd *render.Sound, env []float32) render.Composition {
	return render.Composition{
		Width: pixelW, Height: pixelH, FPS: fps, Frames: frames, Scale: scale,
		Dark: true, Background: ink,
		MotionBlur: render.MotionBlur{Samples: 5, Shutter: 320},
		View: func(c *ui.Context, f render.Frame) {
			f.Audio(snd)
			backdrop(c, f)
			f.Sequence(c, atBloom, atTitle-atBloom, func(f render.Frame) { bloom(c, f) })
			f.Sequence(c, atTitle, atFive-atTitle, func(f render.Frame) { title(c, f) })
			f.Sequence(c, atFive, atUI-atFive, func(f render.Frame) { five(c, f) })
			f.Sequence(c, atUI, atFrames-atUI, func(f render.Frame) { widgets(c, f) })
			f.Sequence(c, atFrames, atSound-atFrames, func(f render.Frame) { strip(c, f) })
			f.Sequence(c, atSound, atEnd-atSound, func(f render.Frame) { wave(c, f, env) })
			f.Sequence(c, atEnd, 0, func(f render.Frame) { end(c, f) })
			hud(c, f)
		},
	}
}

// ------------------------------------------------------------------ timing

// at is 0 to 1 over dur frames from from.
func at(f render.Frame, from, dur int, e ui.Easing) float32 {
	if dur <= 0 {
		return 1
	}
	return float32(render.Interpolate(f.Exact(),
		[]float64{float64(from), float64(from + dur)},
		[]float64{0, 1},
		render.Interpolation{Easing: e}))
}

// band is 0 to 1 in over inDur frames from from, and back to 0 over
// outDur frames ending at from+life.
func band(f render.Frame, from, life, inDur, outDur int) float32 {
	return at(f, from, inDur, ui.EaseOut) * (1 - at(f, from+life-outDur, outDur, ui.EaseIn))
}

func expoOut(t float32) float32 {
	if t >= 1 {
		return 1
	}
	return 1 - float32(math.Pow(2, -10*float64(t)))
}

func expoIn(t float32) float32 {
	if t <= 0 {
		return 0
	}
	return float32(math.Pow(2, 10*float64(t-1)))
}

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// --------------------------------------------------------------- the mark

// mark draws MyGo and its five "!", the i-th at lit[i] of its slam: from
// large, high and clear of the paper, to its size. lit below 0 means the
// mark has not begun, and is not drawn.
// mark draws MyGo and its five "!", the i-th at lit[i] of its slam: from
// large, high and clear of the paper, onto its baseline. lit below 0 means
// the mark has not begun, and is not drawn.
func mark(c *ui.Context, size float32, lit [5]float64) {
	ui.Row(c).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, "MyGo").Font("Geist").FontWeight(800).FontSize(size).
			LetterSpacing(-size * 0.032).TextColor(ink)
		ui.Row(c).AlignItems(ui.Baseline).Gap(size*0.026).
			Margin(0, 0, 0, size*0.028).Children(func() {
			for i := range bangRot {
				e := lit[i]
				if e <= 0 {
					continue
				}
				s := 1 + 0.9*(1-e)*(1-e)
				rise := -0.66 * (1 - e) * float64(size)
				ui.Text(c, "!").Font("Geist").FontWeight(800).FontSize(size).
					TextColor(slabCols[i]).Opacity(float32(min(e*2.4, 1))).
					Scale(float32(s), float32(s)).
					Rotate(bangRot[i]*float32(e)).
					Translate(0, float32(rise))
			}
		})
	})
}

// lean is the slant the teaser sets its name at.
func lean(c *ui.Context, fn func()) {
	ui.Column(c).Skew(-13, 0).TransformOrigin(0, 0.5).Children(fn)
}

// -------------------------------------------------------------- backdrop

// backdrop paints the frame: paper in the sections that are white, ink in
// the rest, and ink again as the very end goes out. It is the only thing
// in the piece that covers the whole frame, so that no section has to
// fight the layout for it.
func backdrop(c *ui.Context, f render.Frame) {
	render.Layer(c).Children(func() {
		ui.Box(c).Fill().Background(ink)
	})
	if onPaper(f.Index) {
		render.Layer(c).Children(func() {
			ui.Box(c).Fill().Background(paper).
				Opacity(1 - at(f, f.Count-16, 16, ui.EaseIn))
		})
	}
}

// hud is the machine voice: a bar that fills over the whole piece, and the
// timecode, coloured for the section it is in.
func hud(c *ui.Context, f render.Frame) {
	fg, line := silver, hairOff
	if onPaper(f.Index) {
		fg, line = steel, hairOn
	}
	render.Layer(c).Children(func() {
		ui.Column(c).Fill().Padding(30, 40, 26, 40).Justify(ui.SpaceBetween).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, "MYGO / RENDER").Font("GeistMono").FontSize(8).
					LetterSpacing(1.8).TextColor(fg)
				ui.Spacer(c)
				ui.Text(c, timecode(f)).Font("GeistMono").FontSize(8).
					LetterSpacing(1.8).TextColor(fg)
			})
			ui.Column(c).Gap(8).Children(func() {
				ui.Box(c).FillWidth().Height(1).Background(line).Opacity(0.7)
				ui.Row(c).Children(func() {
					ui.Text(c, "3:4 · 45.0 S · 30 FPS").Font("GeistMono").FontSize(7.5).
						LetterSpacing(1.6).TextColor(fg).Opacity(0.75)
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("%04d", f.Index)).Font("GeistMono").
						FontSize(7.5).LetterSpacing(1.6).TextColor(fg).Opacity(0.5)
				})
			})
		})
	})
	// The bar: the whole piece's progress, at the very bottom.
	render.Layer(c).Children(func() {
		ui.Box(c).Absolute().Left(0).Bottom(0).Width(dipW).Height(2).
			Background(line).Opacity(0.35)
		ui.Box(c).Absolute().Left(0).Bottom(0).
			Width(float32(f.Index) / float32(f.Count) * dipW).Height(2).
			Background(blue)
	})
}

// timecode is the frame as an edit suite shows it, MM:SS:FF: the last
// field is the frame within the second, 0 to 29 at this rate. A piece
// that is about counting frames has to count them the way a monitor does.
func timecode(f render.Frame) string {
	per := int(f.FPS + 0.5)
	if per <= 0 {
		per = 30
	}
	i := f.Index
	return fmt.Sprintf("%02d:%02d:%02d", i/per/60, (i/per)%60, i%per)
}

// ---------------------------------------------------------- one: the bloom

// bloom is the picture coming up out of black: a point of light, the room
// it lights, and then everything blown to white.
func bloom(c *ui.Context, f render.Frame) {
	const n = atTitle - atBloom
	// The light itself: a radial wash that spreads until it is the frame.
	pr := math.Pow(float64(clamp01(float32((f.Exact()-8)/float64(n-22)))), 2.4)
	r := lerp(20, 940, float32(pr))
	render.Layer(c).Children(func() {
		ui.Box(c).Fill().RadialGradient(ui.RadialGradient{
			Stops: []ui.GradientStop{
				{Color: ui.RGBA(255, 255, 255, 1), At: 0},
				{Color: ui.RGBA(255, 255, 255, 0.94), At: 0.26},
				{Color: ui.RGBA(240, 244, 248, 0.42), At: 0.58},
				{Color: ui.RGBA(233, 236, 239, 0), At: 1},
			},
			Center: ui.Point{X: 0.5, Y: 0.46},
			Radius: r,
			Oklab:  true,
		})
	})
	// The point it starts from.
	render.Layer(c).Children(func() {
		p := at(f, 2, 9, ui.EaseOut)
		ui.Box(c).Absolute().Left(dipW/2 - 4).Top(dipH*0.46 - 4).
			Width(8).Height(8).Radius(4).
			Background(ui.RGBA(255, 255, 255, 1)).Opacity(p).Blur(3)
	})
	// A streak across the lens, as a bright source does to a camera.
	render.Layer(c).Children(func() {
		sw := lerp(50, 1600, expoOut(clamp01(float32((f.Exact()-18)/40))))
		ui.Box(c).Absolute().Left(dipW/2 - sw/2).Top(dipH*0.46 - 2).
			Width(sw).Height(5).Radius(2.5).
			Background(ui.RGBA(255, 255, 255, 0.9)).
			Opacity(0.95 * (1 - at(f, 46, 22, ui.EaseIn))).Blur(7)
	})
	// And the blowout: the frame going to paper.
	render.Layer(c).Children(func() {
		ui.Box(c).Fill().Background(paper).Opacity(at(f, 42, 28, ui.EaseIn))
	})
}

// --------------------------------------------------------- two: the title

// title sets the name on paper: the slabs slam down one at a time, the
// second line comes up under it, and the small type arrives last.
func title(c *ui.Context, f render.Frame) {
	out := at(f, atFive-atTitle-12, 12, ui.EaseIn)
	// The five "!" land a half second apart, on the beat.
	var lit [5]float64
	for i := range lit {
		p := (f.Exact() - float64(4+float64(i)*8)) / 7
		if p > 0 {
			lit[i] = min(p, 1)
		} else {
			lit[i] = -1
		}
	}
	ui.Column(c).Fill().Padding(0, 46).Justify(ui.Center).
		Opacity(1 - out).Children(func() {
		// The kicker over the name.
		ui.Row(c).Gap(8).AlignItems(ui.Center).
			Opacity(at(f, 0, 10, ui.EaseOut)).Children(func() {
			ui.Box(c).Width(18).Height(2).Background(blue)
			ui.Text(c, "MYGO.RUN / RENDER").Font("GeistMono").FontSize(8.5).
				LetterSpacing(2.6).TextColor(steel)
		})
		ui.Box(c).Height(20)

		lean(c, func() {
			// The name, and the five "!" of MyGo!!!!!
			mark(c, 96, lit)

			// The second line, pushed right and up against the name.
			ui.Row(c).Justify(ui.End).Children(func() {
				ui.Text(c, "render").Font("Geist").FontWeight(800).FontSize(78).
					LetterSpacing(-2.6).TextColor(crimson).
					Opacity(at(f, 26, 12, ui.EaseOut)).
					Translate(float32(40*(1-at(f, 26, 18, expoOut))), 0)
			})
		})

		// A rule, and what this is.
		ui.Row(c).AlignItems(ui.Center).Gap(14).Margin(30, 0, 0, 0).
			Opacity(at(f, 44, 18, ui.EaseOut)).Children(func() {
			ui.Box(c).Width(60 * at(f, 44, 24, expoOut)).Height(2).Background(crimson)
			ui.Text(c, "a Go UI toolkit").Font("GeistMono").FontSize(9.5).
				LetterSpacing(1.2).TextColor(ink)
		})
		ui.Text(c, "native views · the GPU · a video out").Font("GeistMono").
			FontSize(9.5).LetterSpacing(1.2).TextColor(steel).
			Margin(12, 0, 0, 0).Opacity(at(f, 56, 18, ui.EaseOut))

		// The command, typing itself in.
		ui.Row(c).AlignItems(ui.Center).Gap(8).Margin(26, 0, 0, 0).
			Opacity(at(f, 74, 12, ui.EaseOut)).Children(func() {
			ui.Text(c, "$").Font("GeistMono").FontSize(10).TextColor(crimson)
			cmd := "$ go run ./pv -o renders/mygo-pv.mp4"[2:]
			n := int(clamp01(at(f, 78, 42, ui.EaseOut)) * float32(len(cmd)))
			ui.Text(c, string([]rune(cmd)[:n])).Font("GeistMono").FontSize(10).
				TextColor(ink)
			if (f.Index/15)%2 == 0 {
				ui.Box(c).Width(6).Height(12).Background(crimson)
			}
		})
	})
}

// ---------------------------------------------------------- three: the five

// five is five cuts, one for each "!": the slab slams in, and the thing it
// stands for is drawn under it.
func five(c *ui.Context, f render.Frame) {
	const life = 60
	idx := f.Index / life
	if idx > 4 {
		idx = 4
	}
	n := f
	n.Index, n.Count = f.Index-idx*life, life

	var (
		label, sub string
		col        ui.Color
		draw       func(c *ui.Context, f render.Frame)
	)
	switch idx {
	case 0:
		label, sub, col = "NATIVE UI", "flexbox and grid, widgets, text editing — no webview at all", slabCols[0]
		draw = drawRows
	case 1:
		label, sub, col = "GPU DRAWN", "Metal, Direct3D, OpenGL — every frame is a draw call", slabCols[1]
		draw = drawScan
	case 2:
		label, sub, col = "60 A SECOND", "and as many more as the disk will take", slabCols[2]
		draw = drawStreak
	case 3:
		label, sub, col = "MOTION BLUR", "five exposures averaged into one frame", slabCols[3]
		draw = drawGhosts
	case 4:
		label, sub, col = "SEVEN MEGS", "one binary, no cgo, every platform", slabCols[4]
		draw = drawWeight
	}

	// The slab is the cut: it lands with the frame, not after it.
	slam := at(n, 0, 8, expoOut)
	ui.Column(c).Fill().Padding(0, 46).Justify(ui.Center).
		Opacity(1 - at(n, life-6, 6, ui.EaseIn)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
			ui.Box(c).Width(30).Height(96).Radius(3).Background(col).
				TransformOrigin(0.5, 1).Scale(1, 0.15+0.85*slam)
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, fmt.Sprintf("0%d", idx+1)).Font("GeistMono").FontSize(10).
					LetterSpacing(3).TextColor(col)
				ui.Text(c, "OF 05").Font("GeistMono").FontSize(8).
					LetterSpacing(2).TextColor(faint)
			})
		})
		ui.Box(c).Height(26)

		tin := at(n, 2, 10, expoOut)
		lean(c, func() {
			ui.Text(c, label).Font("Geist").FontWeight(800).FontSize(56).
				LetterSpacing(-1.6).TextColor(paper).NoWrap().
				Opacity(float32(min(tin*1.8, 1))).
				Translate(float32(-34*(1-tin)), 0)
		})
		ui.Text(c, sub).Font("GeistMono").FontSize(9).LetterSpacing(0.2).
			TextColor(steel).Margin(16, 0, 0, 0).MaxWidth(430).Wrap().
			Opacity(at(n, 9, 13, ui.EaseOut))

		ui.Box(c).Height(44)
		draw(c, n)
	})
}

// drawRows is the first cut: rows of a layout drawing themselves in.
func drawRows(c *ui.Context, f render.Frame) {
	ui.Column(c).Gap(11).Children(func() {
		for i := 0; i < 6; i++ {
			i := i
			e := at(f, 2+i*3, 12, expoOut)
			col := slabCols[i%5]
			fill := col.Mix(paper, 0.20)
			edge := col.Mix(paper, 0.34)
			if i == 2 || i == 4 {
				// A row of cells sharing the line, as a flex row does.
				ui.Row(c).Gap(9).Children(func() {
					for j := 0; j < 3; j++ {
						flex := float32(0.86 + 0.28*math.Sin(float64(f.Exact())*0.075+float64(j*2+i)))
						ui.Box(c).Width(146*at(f, 2+i*3+j*2, 12, expoOut)*flex).
							Height(20).Radius(3).Background(fill).Opacity(0.46).
							Border(1.5, edge)
					}
				})
				continue
			}
			w := float32(0.46 + 0.54*float64((i*29)%7)/6)
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Box(c).Width(430*w*e).Height(20).Radius(3).
					Background(fill).Opacity(0.46).Border(1.5, edge)
				// A caret at the end of the first line: text editing, shown
				// rather than claimed.
				if i == 0 && e > 0.9 && (f.Index/15)%2 == 0 {
					ui.Box(c).Width(2.5).Height(16).Background(paper).Opacity(0.9)
				}
			})
		}
	})
}

// drawScan is the second: the rows of a frame being drawn, one line at a
// time, with the one being drawn brighter than the rest.
func drawScan(c *ui.Context, f render.Frame) {
	// The frame being drawn, a row at a time: what has been drawn is
	// filled, what has not is an outline, and the row being drawn is
	// white.
	y := at(f, 4, 34, ui.EaseInOut) * 13
	ui.Column(c).Gap(5).Children(func() {
		for i := 0; i < 14; i++ {
			i := i
			p := float32(i)
			done := clamp01((y - p) / 1.0)
			now := clamp01(1 - float32(math.Abs(float64(y-p))))
			col := slabCols[i%5]
			w := float32(0.55 + 0.45*float64((i*17)%9)/8)
			ui.Box(c).Width(430*w).Height(7).Radius(3.5).
				Background(col.Mix(paper, now*0.85)).
				Opacity(0.16+0.84*done).Border(1.2, col).
				Scale(1, 1+0.45*now)
		}
	})
}

// drawStreak is the third: the same word at speed, which is what sixty a
// second looks like.
func drawStreak(c *ui.Context, f render.Frame) {
	// Speed, as speed reads: the number, and the air off it.
	ui.Box(c).FillWidth().Height(150).Children(func() {
		render.Layer(c).Children(func() {
			for i := 0; i < 9; i++ {
				i := i
				sp := at(f, 4+i, 26, expoOut)
				w := float32(0.25+0.75*float64((i*31)%10)/9) * 430
				dir := float32(1)
				if i%2 == 1 {
					dir = -1
				}
				ui.Box(c).Absolute().Left(dir * (430 - w) * (1 - sp)).Top(float32(18 + i*15)).
					Width(w).Height(3).Radius(1.5).
					Background(slabCols[i%5]).Opacity(0.18 + 0.5*sp)
			}
		})
		render.Layer(c).Children(func() {
			ui.Row(c).Justify(ui.Center).AlignItems(ui.Center).Fill().Children(func() {
				ui.Text(c, "60").Font("Geist").FontWeight(800).FontSize(148).
					LetterSpacing(-6).TextColor(paper).TextStroke(2, slabCols[2]).
					Skew(-10, 0).Opacity(at(f, 1, 7, ui.EaseOut)).
					Scale(lerp(1.35, 1, at(f, 1, 18, expoOut)), lerp(1.35, 1, at(f, 1, 18, expoOut)))
			})
		})
	})
}

// drawGhosts is the fourth: five exposures of the same bar, converging to
// the one the frame is.
func drawGhosts(c *ui.Context, f render.Frame) {
	// What five exposures look like before they average: the same word,
	// five times, a little apart, settling onto itself.
	settle := at(f, 8, 38, ui.EaseInOut)
	ui.Box(c).FillWidth().Height(96).Children(func() {
		render.Layer(c).Children(func() {
			for i := 4; i >= 0; i-- {
				i := i
				off := float32(float64(i)) * 26 * (1 - settle)
				ui.Box(c).Absolute().Left(0).Top(0).Children(func() {
					ui.Text(c, "BLUR").Font("Geist").FontWeight(800).FontSize(74).
						LetterSpacing(-3).TextColor(slabCols[i]).
						Skew(-11, 0).Opacity(0.20+0.32*float32(4-i)).
						Translate(off, float32(float64(i))*5*(1-settle))
				})
			}
		})
	})
	ui.Row(c).AlignItems(ui.Center).Gap(10).Margin(10, 0, 0, 0).Children(func() {
		ui.Text(c, "×5 SAMPLES").Font("GeistMono").FontSize(8.5).
			LetterSpacing(2).TextColor(slabCols[3])
		ui.Text(c, "SHUTTER 320°").Font("GeistMono").FontSize(8.5).
			LetterSpacing(2).TextColor(faint)
	})
}

// drawWeight is the fifth: the binary, in megabytes, and what it is not.
func drawWeight(c *ui.Context, f render.Frame) {
	// Seven megabytes is almost nothing: the bar goes in from full width.
	w := at(f, 6, 28, ui.EaseInOut)
	ui.Column(c).Gap(12).Children(func() {
		ui.Box(c).Width(430).Height(26).Radius(3).
			Border(1.2, faint).Background(ui.RGBA(0, 0, 0, 0)).Children(func() {
			ui.Box(c).Fill().Radius(2).Background(slabCols[4]).
				Scale(lerp(1, 0.07, w), 1)
		})
		ui.Row(c).Children(func() {
			ui.Text(c, "7 MB").Font("Geist").FontWeight(700).FontSize(15).
				TextColor(slabCols[4])
			ui.Spacer(c)
			ui.Text(c, "· a webview is another sixty").Font("GeistMono").
				FontSize(8.5).TextColor(faint)
		})
	})
}

// ------------------------------------------------------ four: the widgets

// widgets shows the thing other toolkits cannot show: the frame is the UI,
// so the UI is real. A window of native widgets runs in the frame.
func widgets(c *ui.Context, f render.Frame) {
	const life = atFrames - atUI
	in := at(f, 0, 18, expoOut)
	out := at(f, life-12, 12, ui.EaseIn)

	// What the widgets are doing, as the time goes.
	// The counter counts frames, and this section ends on frame 779: that
	// is the number it shows when it cuts, which is the whole point of
	// calling it "the frame is the app".
	const last = atFrames - 1
	linear := math.Min(float64(f.Index)/float64(atFrames-atUI-1), 1)
	raw := atUI + int(linear*float64(last-atUI))
	num := raw / 5 * 5
	if f.Index > atFrames-atUI-9 {
		num = raw
	}
	prog := linear
	var on = f.Index >= 26
	slide := float64(clamp01(at(f, 40, 90, ui.EaseInOut)))

	ui.Column(c).Fill().Padding(0, 46).Justify(ui.Center).
		Opacity(1 - out).Children(func() {
		lean(c, func() {
			ui.Text(c, "THE FRAME IS THE APP").Font("Geist").FontWeight(800).
				FontSize(34).LetterSpacing(-0.8).TextColor(paper).
				Opacity(float32(min(at(f, 4, 14, ui.EaseOut)*1.5, 1)))
		})
		ui.Box(c).Height(26)

		// The window, turned a little, the way the teaser turns its cards.
		ui.Row(c).FillWidth().Height(330).Justify(ui.Center).
			AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(430).Height(268).Radius(14).
				Background(surface).Border(1, edge).
				RotateY(-26*(1-0.85*float32(prog))).RotateX(5-3*float32(prog)).
				Perspective(760).
				TransformOrigin(0.5, 0.5).
				Opacity(in).Translate(float32(50*(1-in)), 0).
				Shadow(0, 24, 60, 0, ui.RGBA(0, 0, 0, 0.55)).
				Children(func() {
					ui.Column(c).Fill().Padding(20, 22).Gap(16).Children(func() {
						// A title bar.
						ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
							for _, dot := range []ui.Color{slabCols[4], slabCols[3], slabCols[1]} {
								ui.Box(c).Width(7).Height(7).Radius(3.5).Background(dot).Opacity(0.9)
							}
							ui.Spacer(c)
							ui.Text(c, "Counter").Font("GeistMono").FontSize(8).
								LetterSpacing(1.4).TextColor(steel)
						})
						ui.Row(c).AlignItems(ui.End).Children(func() {
							ui.Text(c, fmt.Sprintf("%03d", num)).
								Font("Geist").FontWeight(700).FontSize(56).
								LetterSpacing(-2).TextColor(paper)
							ui.Spacer(c)
							ui.Text(c, "FRAMES").Font("GeistMono").FontSize(8).
								LetterSpacing(1.6).TextColor(blue)
						})
						ui.Themed(c, widgetTheme).Background(ui.Transparent).Gap(14).Children(func() {
							ui.Progress(c, prog)
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "Motion blur").Font("Geist").FontSize(10).
									TextColor(paper)
								ui.Spacer(c)
								ui.Switch(c, &on)
							})
							ui.Row(c).AlignItems(ui.Center).Children(func() {
								ui.Text(c, "Quality").Font("Geist").FontSize(10).
									TextColor(paper)
								ui.Spacer(c)
								ui.Slider(c, &slide, 0, 1).Width(150)
							})
						})
					})
				})
		})

		ui.Text(c, "a real Progress, Switch and Slider, drawn by the same call that films them").
			Font("GeistMono").FontSize(8.5).LetterSpacing(0.3).TextColor(steel).
			Margin(28, 0, 0, 0).MaxWidth(460).Wrap().
			Opacity(at(f, 60, 20, ui.EaseOut))
	})
}

// widgetTheme is the accent the widgets are drawn in.
var widgetTheme = func() *ui.Theme {
	t := ui.DarkTheme()
	t.Accent = blue
	t.AccentHover = ui.Hex("#3E93C6")
	t.AccentPressed = ui.Hex("#26688F")
	t.AccentText = ui.Hex("#FFFFFF")
	t.InactiveAccent = ui.Hex("#5A616B")
	t.Surface = surface
	t.Border = edge
	return t
}()

// ------------------------------------------------------- five: the strip

// strip is the piece laid out as what it is: a thousand frames, one after
// the other, and the playhead on the one you are looking at.
func strip(c *ui.Context, f render.Frame) {
	const life = atSound - atFrames
	in := at(f, 0, 16, expoOut)
	out := at(f, life-12, 12, ui.EaseIn)
	// The strip scrolls one cell a frame, so it is always crossing the
	// playhead.
	scroll := float32(f.Exact()) * 2.2

	ui.Column(c).Fill().Justify(ui.Center).Opacity(1 - out).Children(func() {
		ui.Column(c).Padding(0, 46).Children(func() {
			lean(c, func() {
				ui.Text(c, "1350 FRAMES").Font("Geist").FontWeight(800).FontSize(44).
					LetterSpacing(-1.2).TextColor(paper).
					Opacity(float32(min(at(f, 4, 14, ui.EaseOut)*1.5, 1)))
			})
			ui.Text(c, "forty-five seconds, thirty a second, each one built from the number").
				Font("GeistMono").FontSize(8.5).TextColor(steel).
				Margin(12, 0, 0, 0).MaxWidth(460).Wrap().
				Opacity(at(f, 14, 16, ui.EaseOut))
		})

		// The cells.
		ui.Box(c).FillWidth().Height(120).Margin(34, 0, 0, 0).Opacity(in).Children(func() {
			render.Layer(c).Children(func() {
				for i := -8; i < 76; i++ {
					cell := float32(i)*16.4 - scroll
					if cell < -20 || cell > dipW+20 {
						continue
					}
					// Every cell is the colour of the section it is in.
					col := slabCols[((i%5)+5)%5]
					h := 44 + float32(((i*29)%5)*9)
					ui.Box(c).Absolute().Left(cell).Top(112 - h).Width(12).
						Height(h).Radius(1.5).Background(col).
						Opacity(0.30 + 0.5*float32(min(1, max(0, 1-float32(math.Abs(float64(cell-dipW/2)))/380))))
				}
			})
			render.Layer(c).Children(func() {
				// The playhead, and what it is on.
				ui.Box(c).Absolute().Left(dipW / 2).Top(-9).Width(1).
					Height(125).Background(paper).Opacity(0.85)
				ui.Box(c).Absolute().Left(dipW/2 - 4.5).Top(-17).Width(9).
					Height(9).Radius(4.5).Background(crimson)
			})
		})

		ui.Row(c).Padding(0, 46).Margin(18, 0, 0, 0).Children(func() {
			ui.Text(c, "FRAME "+fmt.Sprintf("%04d", f.Index+atFrames)).Font("GeistMono").
				FontSize(8.5).LetterSpacing(1.6).TextColor(paper)
			ui.Spacer(c)
			ui.Text(c, "1/30 S").Font("GeistMono").FontSize(8.5).
				LetterSpacing(1.6).TextColor(steel)
		}).Opacity(at(f, 20, 14, ui.EaseOut))
	})
}

// ------------------------------------------------------- six: the sound

// wave draws the soundtrack: the envelope of the mix, bar by bar, with the
// playhead crossing it.
func wave(c *ui.Context, f render.Frame, env []float32) {
	const life = atEnd - atSound
	in := at(f, 0, 14, expoOut)
	out := at(f, life-12, 12, ui.EaseIn)
	// The playhead runs a little faster than the frame, so that the shape
	// reads as a whole in five seconds.
	x := clamp01(at(f, 4, life-20, ui.EaseInOut))

	ui.Column(c).Fill().Justify(ui.Center).Opacity(1 - out).Children(func() {
		ui.Column(c).Padding(0, 46).Children(func() {
			lean(c, func() {
				ui.Text(c, "IT HAS A SOUNDTRACK").Font("Geist").FontWeight(800).
					FontSize(32).LetterSpacing(-0.8).TextColor(paper).
					Opacity(float32(min(at(f, 4, 14, ui.EaseOut)*1.5, 1)))
			})
			ui.Text(c, "forty-eight kilohertz, stereo, synthesised in Go — here it is, its own samples").
				Font("GeistMono").FontSize(8.5).TextColor(steel).
				Margin(12, 0, 0, 0).MaxWidth(462).Wrap().
				Opacity(at(f, 12, 16, ui.EaseOut))
		})

		// The bars.
		ui.Row(c).AlignItems(ui.Center).Justify(ui.SpaceBetween).
			FillWidth().Height(190).Padding(0, 46).Margin(30, 0, 0, 0).
			Opacity(in).Children(func() {
			for i := range env {
				i := i
				h := float32(math.Sqrt(float64(env[i]))) * 176
				p := float32(i) / float32(len(env))
				col := blue
				if p > 0.70 {
					col = crimson
				}
				// It is drawn: a bar exists once the playhead has passed
				// it, and the one it is on is bright.
				shown := clamp01((x-p)*float32(len(env))/2 + 0.5)
				near := clamp01(1 - float32(math.Abs(float64(p-x)))*6)
				ui.Box(c).Width(2.6).Height(max(h*shown, 1.2)).Radius(1.3).
					Background(col.Mix(paper, near*0.8)).
					Opacity(0.22 + 0.78*max(shown, near))
			}
		})

		// The playhead, and the clock.
		ui.Column(c).Padding(0, 46).Margin(14, 0, 0, 0).Children(func() {
			ui.Box(c).FillWidth().Height(1).Background(hairOff)
			ui.Row(c).Margin(8, 0, 0, 0).Children(func() {
				ui.Text(c, fmt.Sprintf("%.1f S", x*45)).Font("GeistMono").
					FontSize(8.5).LetterSpacing(1.6).TextColor(paper)
				ui.Spacer(c)
				ui.Text(c, "· 12 VOICES · NO SAMPLES").Font("GeistMono").
					FontSize(8.5).LetterSpacing(1.6).TextColor(steel)
			})
		}).Opacity(at(f, 16, 14, ui.EaseOut))
	})
}

// ---------------------------------------------------------- seven: the end

// end puts the name on paper one more time, whole, and says where to get
// it. Then it goes out.
func end(c *ui.Context, f render.Frame) {
	out := at(f, f.Count-26, 26, ui.EaseIn)
	var lit [5]float64
	for i := range lit {
		p := (f.Exact() - float64(6+float64(i)*5)) / 9
		if p > 0 {
			lit[i] = min(p, 1)
		} else {
			lit[i] = -1
		}
	}
	ui.Column(c).Fill().Padding(0, 46).Justify(ui.Center).
		Opacity(1-out).Translate(0, -10*float32(f.Progress())).Children(func() {
		lean(c, func() {
			mark(c, 96, lit)
			ui.Row(c).Justify(ui.End).Children(func() {
				ui.Text(c, "render").Font("Geist").FontWeight(800).FontSize(78).
					LetterSpacing(-2.6).TextColor(crimson).
					Opacity(at(f, 30, 12, ui.EaseOut)).
					Translate(float32(30*(1-at(f, 30, 18, expoOut))), 0)
			})
		})
		ui.Row(c).AlignItems(ui.Center).Gap(12).Margin(38, 0, 0, 0).
			Opacity(at(f, 52, 18, ui.EaseOut)).Children(func() {
			ui.Box(c).Width(54 * at(f, 52, 24, expoOut)).Height(2).Background(crimson)
			ui.Text(c, "mygo.run").Font("GeistMono").FontSize(10).
				LetterSpacing(2.4).TextColor(ink)
		})
		// The same prompt the intro types, so the two cards are one system.
		ui.Row(c).AlignItems(ui.Center).Gap(8).Margin(16, 0, 0, 0).
			Opacity(at(f, 66, 16, ui.EaseOut)).Children(func() {
			ui.Text(c, "$").Font("GeistMono").FontSize(10).TextColor(crimson)
			ui.Text(c, "go run ./pv").Font("GeistMono").FontSize(10).
				LetterSpacing(0.6).TextColor(ink)
			if (f.Index/15)%2 == 0 {
				ui.Box(c).Width(6).Height(12).Background(crimson)
			}
		})
	})
}

// -------------------------------------------------------------- the score

// score builds the soundtrack and returns it with its own envelope, which
// the sixth section draws.
func score() (*render.Sound, []float32) {
	t := sound.New(sound.Seconds45)

	// Voicings sat where a phone speaker actually puts them, 200-800 Hz,
	// instead of the sub that made the last mix a rumble nobody could hear.
	am := []float64{220.00, 261.63, 329.63, 440.00, 659.25}
	fm := []float64{174.61, 220.00, 349.23, 440.00, 523.25}
	cmj := []float64{196.00, 261.63, 392.00, 523.25, 783.99}
	gm := []float64{196.00, 246.94, 392.00, 493.88, 783.99}
	chords := [][]float64{am, fm, cmj, gm}

	// The riff: sixteenths on a plucked string, 440-880 Hz, which is the
	// one band the reference spends most of its energy in.
	riff := []float64{
		440.00, 523.25, 659.25, 587.33, 523.25, 440.00, 392.00, 440.00,
		440.00, 523.25, 659.25, 783.99, 880.00, 783.99, 659.25, 587.33,
	}

	// --- 0.0-2.4  the light arrives. Impacts alone, and silence between
	// them: the reference opens with stabs and gaps, not with a bed.
	t.Hit(0.02, 0.60, 392.00)
	t.Hit(0.70, 0.28, 261.63)
	t.Hit(1.36, 0.38, 329.63)
	t.Hit(1.84, 0.44, 440.00)
	t.Hit(2.18, 0.32, 220.00)

	// --- 2.4-8.0  one impact per "!", up the pentatonic, each with the
	// note played out; nothing fills the gaps between them.
	for i, hz := range []float64{220.00, 261.63, 329.63, 440.00, 523.25} {
		s := 2.40 + float64(i)*0.50
		t.Hit(s, 0.54, hz)
		t.Pluck(s, hz*2, 0.28, -0.28+0.14*float64(i))
		if i > 0 {
			t.Pluck(s, hz, 0.15, 0.22)
		}
	}
	// Under the last of them the first chord fades up, so the groove has
	// somewhere to come from.
	t.Chord(4.90, 1.60, am, 0.20)
	t.Chord(6.30, 1.70, fm, 0.22)

	// --- 8.0-33.0  bars. One impact per bar carries the cut.
	bar := func(start float64, ci, lvl int) {
		ch := chords[ci%4]
		t.Chord(start, 2.30, ch, 0.16+0.05*float64(lvl))
		if lvl > 0 {
			t.Hit(start, 0.26+0.10*float64(lvl), ch[0]*2)
		}
		// bass
		t.Pluck(start, ch[0], 0.150, -0.05)
		t.Pluck(start+1.20, ch[0], 0.120, -0.05)
		// the riff
		for i := 0; i < 16; i++ {
			g := 0.135
			if lvl == 0 {
				g = 0.105
			}
			if lvl == 2 {
				g = 0.150
			}
			t.Pluck(start+float64(i)*0.15, riff[i], g, 0.26)
		}
		if lvl > 0 {
			for i := 0; i < 4; i++ {
				t.Kick(start+float64(i)*0.60, 0.055+0.02*float64(lvl))
			}
			t.Clap(start+1.20, 0.20, 0.14)
			t.Clap(start+3.00-1.80, 0.21, -0.14)
		}
		for i := 0; i < 8; i++ {
			g := 0.155
			if i%2 == 1 {
				g = 0.098
			}
			if lvl == 0 {
				g *= 0.55
			}
			t.Shaker(start+float64(i)*0.30, g, 0.28-0.13*float64(i%4))
		}
		if lvl > 0 {
			g16 := 0.020
			if lvl == 2 {
				g16 = 0.038
			}
			for i := 0; i < 16; i++ {
				t.Shaker(start+float64(i)*0.15, g16, -0.3+0.06*float64(i%5))
			}
		}
	}

	// the five cuts: full
	for i := 0; i < 4; i++ {
		bar(8.00+float64(i)*2.40, i, 1)
	}
	bar(17.60, 4, 1)
	// the widgets: pulled back to harmony and riff, no drums
	for i := 0; i < 3; i++ {
		bar(20.00+float64(i)*2.40, i+1, 0)
	}
	// the frames: building
	for i := 0; i < 3; i++ {
		bar(26.00+float64(i)*2.40, i+2, 2)
	}
	t.Riser(31.60, 1.50, 0.095)

	// --- 33.0-38.0  the break. The drums go, one chord holds, and the
	// picture draws its own samples in the quiet.
	t.Hit(33.00, 0.30, 220.00)
	t.Chord(33.00, 4.90, []float64{220.00, 261.63, 329.63, 440.00}, 0.20)
	for i, hz := range []float64{880.00, 659.25, 523.25, 440.00, 329.63, 261.63} {
		t.Pluck(33.40+float64(i)*0.70, hz, 0.15, -0.3+0.1*float64(i))
	}
	t.Pluck(36.60, 174.61, 0.16, -0.1)
	t.Swell(38.00, 1.10, 0.185)

	// --- 38.0-45.0  the mark again, and the chord held to the end.
	t.Hit(38.00, 0.62, 440.00)
	for i, hz := range []float64{220.00, 261.63, 329.63, 440.00, 523.25} {
		s := 38.10 + float64(i)*0.10
		t.Pluck(s, hz*2, 0.24, -0.25+0.12*float64(i))
		t.Pluck(s, hz, 0.13, 0.2)
	}
	t.Chord(38.10, 6.60, []float64{220.00, 261.63, 329.63, 440.00, 659.25}, 0.24)
	t.Hit(39.80, 0.22, 261.63)
	for i, hz := range []float64{523.25, 659.25, 880.00} {
		t.Bell(40.60+float64(i)*0.22, hz, 0.09, -0.25+0.25*float64(i))
	}
	t.Pluck(43.20, 440.00, 0.14, 0.0)

	env := t.Envelope(168)
	if len(env) > 0 {
		peak := float32(0)
		for _, v := range env {
			peak = max(peak, v)
		}
		if peak > 0 {
			for i := range env {
				env[i] = env[i] / peak * 0.92
			}
		}
	}
	return t.Mix(2.40), env
}
