// Package drifthold is the root of the Drifthold game module.
//
// The blank imports below pin every third-party package the game uses so that
// go.mod / go.sum stay stable while packages are developed independently.
package drifthold

import (
	_ "github.com/hajimehoshi/ebiten/v2"
	_ "github.com/hajimehoshi/ebiten/v2/audio"
	_ "github.com/hajimehoshi/ebiten/v2/inpututil"
	_ "github.com/hajimehoshi/ebiten/v2/text/v2"
	_ "github.com/hajimehoshi/ebiten/v2/vector"
	_ "golang.org/x/image/font/gofont/gobold"
	_ "golang.org/x/image/font/gofont/gomedium"
	_ "golang.org/x/image/font/gofont/gomono"
	_ "golang.org/x/image/font/gofont/gomonobold"
	_ "golang.org/x/image/font/gofont/goregular"
)
