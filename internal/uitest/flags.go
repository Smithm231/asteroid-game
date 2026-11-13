package uitest

import (
	"flag"
	"fmt"
	"os"

	"drifthold/internal/input"
)

// Flags parses the command line of a demo program and returns the Options it
// describes. The flags are
//
//	-out dir      directory for screenshots (default "shots")
//	-scale f      pixels per point (default 1)
//	-w n          window width in points (default 1600)
//	-h n          window height in points (default 900)
//	-script path  JSON input script to play instead of the demo's built-in one
//	-frames n     stop after n frames even if the script has not finished
//
// Options.Script is nil unless -script was given, so a demo falls back to
// its own script with
//
//	o := uitest.Flags()
//	if o.Script == nil {
//		o.Script = builtin
//	}
//
// A demo that needs flags of its own defines them with package flag before
// calling Flags, which parses them all; the six names above are taken. On a
// bad command line or an unreadable script, Flags prints the problem and
// exits with status 2.
func Flags() Options {
	o, err := ParseFlags(flag.CommandLine, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	return o
}

// ParseFlags is Flags for an explicit flag set and argument list: it defines
// the common demo flags on fs, parses args and returns the resulting Options.
func ParseFlags(fs *flag.FlagSet, args []string) (Options, error) {
	var (
		out    = fs.String("out", DefaultOutDir, "directory for screenshots")
		scale  = fs.Float64("scale", DefaultScale, "pixels per point")
		w      = fs.Int("w", DefaultW, "window width in points")
		h      = fs.Int("h", DefaultH, "window height in points")
		script = fs.String("script", "", "JSON input script to play instead of the built-in one")
		frames = fs.Int("frames", 0, "stop after this many frames (0 = when the script ends)")
	)
	if err := fs.Parse(args); err != nil {
		return Options{}, err
	}
	if fs.NArg() > 0 {
		return Options{}, fmt.Errorf("uitest: unexpected argument %q", fs.Arg(0))
	}
	if !(*scale > 0) || *scale > 16 {
		return Options{}, fmt.Errorf("uitest: -scale must be in (0, 16], got %v", *scale)
	}
	if *w <= 0 || *h <= 0 {
		return Options{}, fmt.Errorf("uitest: -w and -h must be positive, got %d×%d", *w, *h)
	}
	if *frames < 0 {
		return Options{}, fmt.Errorf("uitest: -frames must not be negative, got %d", *frames)
	}
	o := Options{
		W:      *w,
		H:      *h,
		Scale:  float32(*scale),
		OutDir: *out,
		Frames: *frames,
	}
	if *script != "" {
		steps, err := input.LoadScript(*script)
		if err != nil {
			return Options{}, fmt.Errorf("uitest: -script: %w", err)
		}
		// Non-nil even when the file holds no steps: the caller asked for
		// this script, not for the built-in one.
		o.Script = append([]input.Step{}, steps...)
	}
	return o, nil
}
