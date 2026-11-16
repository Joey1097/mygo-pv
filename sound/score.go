// Package sound synthesizes the soundtrack for the motion piece in Go:
// pads, ticks, whooshes and a sub boom, mixed into one 48 kHz stereo track
// that the render plugin plays. No audio files are read.
package sound

import (
	"math"
	"math/rand"

	"github.com/egoist/mygo/plugins/render"
)

// rate is the sample rate of the track. The render plugin resamples
// anything else to 48 kHz, so stay there.
const rate = 48000

// Seconds of the composition.
const Seconds = 15.0

// Seconds12 is the length of the 3:4 piece.
const Seconds12 = 12.0

// Seconds45 is the length of the teaser.
const Seconds45 = 45.0

// Track is a stereo buffer being mixed.
type Track struct {
	l, r []float32
	rng  *rand.Rand
}

// New makes an empty track of n seconds at 48 kHz.
func New(seconds float64) *Track {
	n := int(seconds * rate)
	return &Track{
		l:   make([]float32, n),
		r:   make([]float32, n),
		rng: rand.New(rand.NewSource(7)),
	}
}

// add mixes v into both channels at sample i and the pan p (-1 left, 1
// right), scaled by g.
func (t *Track) add(i int, g, pan float64, l, r float32) {
	if i < 0 || i >= len(t.l) || g == 0 {
		return
	}
	gl := g * (1 - math.Max(0, pan)) * 1.0
	gr := g * (1 + math.Min(0, pan)) * 1.0
	t.l[i] += float32(float64(l) * gl)
	t.r[i] += float32(float64(r) * gr)
}

// additive uses a one-pole low pass to take the top off noise.
type onepole struct{ a, y float64 }

func (f *onepole) low(x, cutoff float64) float64 {
	a := math.Exp(-2 * math.Pi * cutoff / rate)
	f.y = a*f.y + (1-a)*x
	return f.y
}

func (f *onepole) high(x, cutoff float64) float64 { return x - f.low(x, cutoff) }

// Pad adds a slow chord of sines with a soft attack and release.
func (t *Track) Pad(start, dur float64, freqs []float64, gain float64) {
	i0 := int(start * rate)
	n := int(dur * rate)
	atk := 0.9 * dur
	rel := 0.55 * dur
	for i := 0; i < n; i++ {
		t0 := float64(i) / rate
		if i0+i >= len(t.l) {
			break
		}
		// envelope: raised cosine attack, slow decay, cosine release
		var env float64
		switch {
		case t0 < atk:
			x := t0 / atk
			env = 0.5 - 0.5*math.Cos(math.Pi*x)
		default:
			env = math.Exp(-1.1 * (t0 - atk))
		}
		if rem := dur - t0; rem < rel {
			env *= 0.5 - 0.5*math.Cos(math.Pi*max(rem, 0)/rel)
		}
		// slow vibrato keeps it from sounding synthetic
		vib := 1 + 0.0015*math.Sin(2*math.Pi*0.17*t0)
		st := (t0 - atk) * 0.02 // gentle chorus width
		var l, r float64
		for k, f := range freqs {
			ph := 2 * math.Pi * f * vib * t0
			amp := math.Cos(float64(k) * 0.7)
			l += math.Sin(ph) * (1 + 0.12*amp)
			r += math.Sin(ph+st) * (1 - 0.12*amp)
		}
		l /= float64(len(freqs))
		r /= float64(len(freqs))
		t.add(i0+i, gain*env, 0, float32(l), float32(r))
	}
}

// Sub adds a low sine swell: the body under a transition.
func (t *Track) Sub(start, dur, f0, f1, gain float64) {
	i0 := int(start * rate)
	n := int(dur * rate)
	ph := 0.0
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		x := float64(i) / float64(n)
		f := f0 + (f1-f0)*x
		ph += 2 * math.Pi * f / rate
		env := math.Pow(math.Sin(math.Pi*math.Min(x*1.15, 1)), 1.4)
		v := math.Sin(ph) * env
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Tick adds a short pluck: a sine with two harmonics, dying fast.
func (t *Track) Tick(start, f, gain, pan float64) {
	i0 := int(start * rate)
	n := int(0.45 * rate)
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		env := math.Exp(-16*t0) * min(t0/0.002, 1)
		v := math.Sin(2*math.Pi*f*t0) + 0.35*math.Sin(2*math.Pi*f*2*t0) + 0.12*math.Sin(2*math.Pi*f*3.01*t0)
		t.add(i0+i, gain*env, pan, float32(v), float32(v))
	}
}

// Whoosh adds filtered noise opening and closing, for a cut.
func (t *Track) Whoosh(start, dur, gain float64) {
	i0 := int(start * rate)
	n := int(dur * rate)
	lp := &onepole{}
	hp := &onepole{}
	ph := 0.0
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		x := float64(i) / float64(n)
		// noise, swept from dark to bright and back
		cut := 300 + 5200*math.Pow(math.Sin(math.Pi*math.Min(x, 1)), 1.6)
		w := lp.low(t.rng.NormFloat64()*0.5, cut)
		w = hp.high(w, 90)
		env := math.Pow(math.Sin(math.Pi*math.Min(x*1.02, 1)), 2.2)
		// a little pitched tone underneath gives it a direction
		ph += 2 * math.Pi * (140 + 260*x) / rate
		v := w*1.5 + 0.15*math.Sin(ph)
		env *= 1
		t.add(i0+i, gain*env, 0, float32(v), float32(v*0.98))
	}
}

// Boom adds a soft kick: a falling sine with a click on top.
func (t *Track) Boom(start, gain float64) {
	i0 := int(start * rate)
	n := int(1.4 * rate)
	ph := 0.0
	lp := &onepole{}
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		f := 52 + 70*math.Exp(-9*t0)
		ph += 2 * math.Pi * f / rate
		env := math.Exp(-3.2 * t0)
		click := lp.high(t.rng.NormFloat64()*0.4, 1800) * math.Exp(-45*t0) * 0.5
		v := math.Sin(ph)*env*0.9 + click
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Shaker adds a short high noise burst, for a grid tick.
func (t *Track) Shaker(start, gain, pan float64) {
	i0 := int(start * rate)
	n := int(0.12 * rate)
	hp := &onepole{}
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		env := math.Exp(-38 * t0)
		v := hp.high(t.rng.NormFloat64()*0.35, 4200) * env
		t.add(i0+i, gain, pan, float32(v), float32(v))
	}
}

// Mix normalizes to peak and returns the track as a stereo sound.
func (t *Track) Mix(peak float64) *render.Sound {
	var m float64
	for i := range t.l {
		m = math.Max(m, math.Abs(float64(t.l[i])))
		m = math.Max(m, math.Abs(float64(t.r[i])))
	}
	if m == 0 {
		m = 1
	}
	g := peak / m
	n := len(t.l)

	// A lookahead limiter, on the way out. A piece built of impacts has a
	// crest factor of twenty-odd dB: the loudest two milliseconds decide
	// the file's peak while the body sits far below it, so a follower
	// without lookahead cannot catch the transient, and a wave shaper
	// flattens the body to pay for it. This looks 3 ms ahead, pulls the
	// gain down before the peak arrives, and lets it back up slowly.
	const ceil = 0.82
	const look = 0.003
	const release = 0.070
	ahead := int(look * rate)
	if ahead < 1 {
		ahead = 1
	}
	pk := make([]float64, n)
	for i := 0; i < n; i++ {
		pk[i] = math.Max(math.Abs(float64(t.l[i])), math.Abs(float64(t.r[i]))) * g
	}
	aRel := math.Exp(-1 / (release * rate))
	cur := 1.0
	out := make([]float32, 0, n*2)
	for i := 0; i < n; i++ {
		mx := 0.0
		for j := i; j < i+ahead && j < n; j++ {
			if pk[j] > mx {
				mx = pk[j]
			}
		}
		need := 1.0
		if mx > ceil {
			need = ceil / mx
		}
		if need < cur {
			cur = need
		} else {
			cur = aRel*cur + (1-aRel)*need
		}
		out = append(out, soft(float32(float64(t.l[i])*g*cur)), soft(float32(float64(t.r[i])*g*cur)))
	}
	return render.SoundOf(rate, 2, out)
}

// soft rounds off anything that still reaches the top, and is the identity
// below -1 dBFS so that the limiter, not this, decides the level.
func soft(x float32) float32 {
	v := math.Abs(float64(x))
	if v <= 0.90 {
		return x
	}
	return float32(math.Copysign(0.90+0.10*math.Tanh((v-0.90)/0.10), float64(x)))
}

// soft rounds off the last bit of headroom, so peaks do not clip.

// Bell adds a struck bell: a carrier bent by a fast-falling modulator, so
// that it rings bright and then plain, with one inharmonic partial for the
// metal.
func (t *Track) Bell(start, f, gain, pan float64) {
	i0 := int(start * rate)
	n := int(1.7 * rate)
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		idx := 3.4 * math.Exp(-5.2*t0)
		mod := math.Sin(2*math.Pi*f*2.01*t0) * idx
		car := math.Sin(2*math.Pi*f*t0 + mod)
		metal := 0.24 * math.Sin(2*math.Pi*f*3.02*t0) * math.Exp(-8.5*t0)
		env := math.Exp(-3.0*t0) * min(t0/0.0015, 1)
		v := (car*0.86 + metal) * env
		t.add(i0+i, gain, pan, float32(v), float32(v))
	}
}

// Kick adds a tight kick: a sine falling from a click to its body.
func (t *Track) Kick(start, gain float64) {
	i0 := int(start * rate)
	n := int(0.6 * rate)
	ph := 0.0
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		f := 44 + 128*math.Exp(-26*t0)
		ph += 2 * math.Pi * f / rate
		env := math.Exp(-7.2 * t0)
		v := math.Sin(ph) * env
		v += 0.32 * math.Exp(-95*t0) * math.Sin(2*math.Pi*1750*t0)
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Clap adds a clap: three short bursts of noise and the room after them.
func (t *Track) Clap(start, gain, pan float64) {
	i0 := int(start * rate)
	n := int(0.34 * rate)
	hp := &onepole{}
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		var env float64
		switch {
		case t0 < 0.013:
			env = math.Exp(-150 * t0)
		case t0 < 0.026:
			env = 0.72 * math.Exp(-140*(t0-0.013))
		case t0 < 0.039:
			env = 0.58 * math.Exp(-130*(t0-0.026))
		default:
			env = 0.42 * math.Exp(-15*(t0-0.039))
		}
		v := hp.high(t.rng.NormFloat64()*0.42, 1000) * env
		t.add(i0+i, gain, pan, float32(v), float32(v*0.94))
	}
}

// Riser adds the noise that climbs into a cut: it darkens to bright, and
// a low tone climbs with it.
func (t *Track) Riser(start, dur, gain float64) {
	i0 := int(start * rate)
	n := int(dur * rate)
	lp := &onepole{}
	ph := 0.0
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		x := float64(i) / float64(n)
		cut := 380 + 7200*x*x
		w := lp.low(t.rng.NormFloat64()*0.45, cut)
		ph += 2 * math.Pi * (170 + 940*x*x) / rate
		env := math.Pow(x, 1.7)
		v := w*1.15 + 0.17*math.Sin(ph)
		t.add(i0+i, gain*env, 0, float32(v), float32(v))
	}
}

// Pluck adds a plucked string: a delay line the length of one period,
// filled with noise and fed back through an averaging lowpass, so that the
// noise turns into a pitch and dies away the way a string does.
func (t *Track) Pluck(start, f, gain, pan float64) {
	period := int(float64(rate) / f)
	if period < 3 {
		return
	}
	buf := make([]float32, period)
	// The excitation: noise, softened, with a little of the fundamental so
	// that low notes have a body to them.
	for i := range buf {
		x := float64(i) / float64(period)
		buf[i] = float32(t.rng.NormFloat64()*0.30 + 0.16*math.Sin(2*math.Pi*x))
	}
	i0 := int(start * rate)
	n := min(int(2.4*rate), len(t.l)-i0)
	if n <= 0 {
		return
	}
	idx := 0
	const decay = 0.9968
	for i := 0; i < n; i++ {
		cur := buf[idx]
		nxt := buf[(idx+1)%period]
		buf[idx] = 0.5 * (cur + nxt) * decay
		idx = (idx + 1) % period
		t.add(i0+i, gain, pan, cur, cur)
	}
}

// Drop adds a pitch falling sub: the bottom of the floor going out from
// under a cut.
func (t *Track) Drop(start, dur, f0, f1, gain float64) {
	i0 := int(start * rate)
	n := min(int(dur*rate), len(t.l)-i0)
	if n <= 0 {
		return
	}
	ph := 0.0
	for i := 0; i < n; i++ {
		x := float64(i) / float64(n)
		f := f0 + (f1-f0)*math.Pow(x, 0.65)
		ph += 2 * math.Pi * f / rate
		env := math.Pow(1-x, 1.5)
		v := math.Tanh(1.6*math.Sin(ph)) * env
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Swell adds a reversed hit: noise rising, dark to bright, that stops dead
// on the cut it is going into.
func (t *Track) Swell(end, dur, gain float64) {
	start := end - dur
	i0 := int(start * rate)
	n := min(int(dur*rate), len(t.l)-i0)
	if n <= 0 {
		return
	}
	lp := &onepole{}
	for i := 0; i < n; i++ {
		x := float64(i) / float64(n)
		w := lp.low(t.rng.NormFloat64()*0.5, 300+9000*x*x*x)
		v := w * math.Pow(x, 2.2)
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Shimmer adds a high ringing tone that fades in and out: air over a
// sustained passage.
func (t *Track) Shimmer(start, dur, f, gain float64) {
	i0 := int(start * rate)
	n := min(int(dur*rate), len(t.l)-i0)
	if n <= 0 {
		return
	}
	for i := 0; i < n; i++ {
		t0 := float64(i) / rate
		x := float64(i) / float64(n)
		env := math.Sin(math.Pi * x)
		v := 0.6*math.Sin(2*math.Pi*f*t0) + 0.4*math.Sin(2*math.Pi*f*2.001*t0)
		v *= 0.55 * env
		t.add(i0+i, gain, 0.35, float32(v), float32(v*-0.35))
	}
}

// Envelope returns n peaks across the mix, for drawing the track.
// Hit is an impact, which is what a trailer is mostly made of: a sub that
// falls from 90 to 40 Hz, a body that knocks at a pitch you can hear, and
// a noise transient no longer than the crack of a stick. The picture cuts
// on these, so they have to be loud, short, and alone.
func (t *Track) Hit(start, gain, body float64) {
	i0 := int(start * rate)
	n := int(0.85 * rate)
	var ph, phb float64
	var noise, bright onepole
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		x := float64(i) / rate
		fs := 40 + 52*math.Exp(-x/0.085)
		ph += 2 * math.Pi * fs / rate
		sub := math.Sin(ph) * math.Exp(-x/0.20)

		fb := body * (1 + 0.30*math.Exp(-x/0.025))
		phb += 2 * math.Pi * fb / rate
		bod := (math.Sin(phb) + 0.30*math.Sin(2*phb) + 0.12*math.Sin(3*phb)) *
			math.Exp(-x/0.115)

		hi := noise.high(t.rng.NormFloat64()*0.7, 1900)
		v := sub*0.42 + bod*0.88 + hi*math.Exp(-x/0.013)*0.62
		// the crack above it: without this the impact has no air, and
		// everything between 1 and 8 kHz stays empty.
		v += bright.high(t.rng.NormFloat64()*0.9, 2600) * math.Exp(-x/0.055) * 0.34
		t.add(i0+i, gain, 0, float32(v), float32(v))
	}
}

// Chord holds a voicing that arrives with the bar rather than behind it:
// a fast attack, no wash, and most of its weight where a phone speaker
// can actually put it, between 400 and 1200 Hz.
func (t *Track) Chord(start, dur float64, freqs []float64, gain float64) {
	i0 := int(start * rate)
	n := int(dur * rate)
	const atk, rel = 0.085, 0.42
	for i := 0; i < n; i++ {
		if i0+i >= len(t.l) {
			break
		}
		t0 := float64(i) / rate
		env := 1.0
		if t0 < atk {
			env = t0 / atk
		}
		if rem := dur - t0; rem < rel {
			env *= max(rem, 0) / rel
		}
		env *= 0.70 + 0.30*math.Min(t0/0.55, 1)
		var l, r float64
		for k, f := range freqs {
			ph := 2 * math.Pi * f * t0
			amp := 1.0 - 0.05*float64(k)
			l += math.Sin(ph) * amp
			r += math.Sin(ph+0.0045) * amp
		}
		l /= float64(len(freqs))
		r /= float64(len(freqs))
		t.add(i0+i, gain*env, 0, float32(l), float32(r))
	}
}

func (t *Track) Envelope(n int) []float32 {
	out := make([]float32, n)
	step := len(t.l) / n
	for i := 0; i < n; i++ {
		peak := float32(0)
		for j := i * step; j < (i+1)*step && j < len(t.l); j++ {
			peak = max(peak, float32(math.Abs(float64(t.l[j]))), float32(math.Abs(float64(t.r[j]))))
		}
		out[i] = peak
	}
	return out
}
