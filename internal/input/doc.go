// Package input turns raw mouse and keyboard activity into one immutable
// per-frame snapshot, [State], that every UI and world view of Drifthold reads.
//
// All positions and sizes in a State are expressed in points; the conversion
// from device pixels happens once, in the [Source] that produced the snapshot.
//
// Two sources exist:
//
//   - [Device] (see [NewDevice]) reads the real mouse and keyboard through
//     Ebitengine and must be polled exactly once per game Update.
//   - [ScriptSource] (see [NewScript]) replays a list of [Step] values frame by
//     frame. It never touches Ebitengine's input functions, so it works in unit
//     tests without a window and drives the screenshot tests hosted by package
//     uitest.
//
// Both sources share the same double-click and key-repeat logic, so scripted
// tests exercise exactly the behaviour a player gets from a real device.
//
// # Script reference
//
// A script is a JSON array of step objects. Each step carries exactly one
// action; "button" and "frames" are modifiers of that action. Coordinates are
// in points. The frame counts below say how many frames a step occupies.
//
//	{"wait":30}                               30 idle frames
//	{"move":[640,360]}                        cursor jumps there (1 frame)
//	{"move":[640,360],"frames":12}            cursor glides there over 12 frames
//	{"click":[100,200]}                       move, press, release (3 frames)
//	{"click":[100,200],"button":"right"}      same with the right button ("middle" works too)
//	{"dblclick":[100,200]}                    move, press, release, press, release (5 frames)
//	{"drag":[x0,y0,x1,y1],"frames":20}        move, press, 20 held moves, release (23 frames)
//	{"press":[100,200]}                       move, press and keep the button held (2 frames)
//	{"release":[300,200]}                     move with the button held, release (2 frames)
//	{"wheel":-3}                              wheel delta for one frame (+ = away from user)
//	{"key":"R"}                               key down, key up (2 frames)
//	{"key":"Backspace","frames":40}           key held for 40 frames, then up (41 frames)
//	{"hold":"ShiftLeft","frames":10}          key held during the NEXT 10 frames (0 frames itself)
//	{"text":"hello"}                          one rune per frame, delivered through State.Chars
//	{"shot":"name.png"}                       one idle frame, then the host saves a screenshot
//	{"quit":true}                             ends the script
//	{"note":"free text"}                      ignored; may also accompany any other step
//
// Details worth knowing when writing scripts:
//
//   - "hold" does not block: the steps that follow run while the key is down,
//     which is how modifier-clicks are written. Without "frames" the key stays
//     down until the script ends.
//   - "drag" without "frames" uses 10 frames; "key" and "move" without
//     "frames" use 1.
//   - Inside "text", newline, tab and backspace characters become taps of the
//     Enter, Tab and Backspace keys (2 frames each); everything else arrives as
//     a typed character without any key event.
//   - Key names are Ebitengine's ("R", "Escape", "ShiftLeft", "F1", "Digit1",
//     "ArrowLeft", "Enter", "Space", "Tab", ...), matched case-insensitively;
//     see [ParseKey] for the accepted aliases.
//   - Two clicks on the same spot less than 0.35 s apart are a double click,
//     exactly as on a real device. Put {"wait":22} between them when two
//     separate clicks are meant.
//   - The cursor starts off-screen at (-1000,-1000) until the first step that
//     positions it.
package input
