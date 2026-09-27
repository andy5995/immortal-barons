module github.com/andy5995/immortal-barons/cmd/ib-sysop

go 1.26.4

require (
	gioui.org v0.10.2
	github.com/andy5995/immortal-barons v0.0.0
)

require (
	gioui.org/shader v1.0.9 // indirect
	github.com/go-text/typesetting v0.3.4 // indirect
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/image v0.26.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

// The panel is its own module so the GUI toolkit never enters the game's
// go.mod: the door is built and packaged without it. It builds against the
// game's packages in this checkout.
replace github.com/andy5995/immortal-barons => ../..
