package menu

import (
	"fmt"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Splash prints the Immortal Barons ANSI title banner, then waits for a
// keypress. The banner is original art (not copied from BRE), built with the
// ansi-artwork skill; it carries its own colors, so it is printed as-is. See
// the banner const below.
func Splash(s session.Session) {
	fmt.Fprint(s, ansi.Clear)
	fmt.Fprint(s, banner)
	pause(s)
}

const banner = "    \x1b[36m+\x1b[0m   \x1b[90m·\x1b[0m       \x1b[93m█████\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m  \x1b[93m███\x1b[0m  \x1b[93m████\x1b[0m  \x1b[93m█████\x1b[0m  \x1b[93m███\x1b[0m  \x1b[93m█\x1b[0m             \x1b[90m·\x1b[0m\n" +
	"  \x1b[90m·\x1b[0m               \x1b[93m█\x1b[0m \x1b[36m+\x1b[0m \x1b[93m██\x1b[0m \x1b[93m██\x1b[0m \x1b[93m██\x1b[0m \x1b[93m██\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m  \x1b[36m+\x1b[0m         \x1b[31m▄\x1b[0m\x1b[91;41m▀\x1b[0m\x1b[91m███▄\x1b[0m\n" +
	"          \x1b[90m·\x1b[0m     \x1b[90m·\x1b[0m \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒▒\x1b[0m \x1b[93;101m▒▒\x1b[0m \x1b[93;101m▒▒\x1b[0m \x1b[93;101m▒▒\x1b[0m \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒\x1b[0m \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒\x1b[0m   \x1b[93;101m▒\x1b[0m \x1b[93;101m▒\x1b[0m          \x1b[31m▄████\x1b[0m\x1b[91;41m▀\x1b[0m\x1b[91m███▄\x1b[0m\n" +
	"     \x1b[97m*\x1b[0m  \x1b[90m·\x1b[0m     \x1b[90m·\x1b[0m \x1b[90m·\x1b[0m \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m \x1b[90m·\x1b[0m \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m  \x1b[37m.\x1b[0m       \x1b[31m███████\x1b[0m\x1b[91;41m▀\x1b[0m\x1b[91m██\x1b[0m\n" +
	"                  \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m█\x1b[0m   \x1b[91m█\x1b[0m \x1b[91m████\x1b[0m    \x1b[91m█\x1b[0m   \x1b[91m█████\x1b[0m \x1b[91m█\x1b[0m       \x1b[97m*\x1b[0m  \x1b[31m▀████████\x1b[0m\x1b[91m▀\x1b[0m\n" +
	"   \x1b[37m.\x1b[0m       \x1b[97m*\x1b[0m      \x1b[91;41m▒\x1b[0m   \x1b[91;41m▒\x1b[0m   \x1b[91;41m▒\x1b[0m \x1b[91;41m▒\x1b[0m   \x1b[91;41m▒\x1b[0m \x1b[91;41m▒\x1b[0m   \x1b[91;41m▒\x1b[0m \x1b[91;41m▒▒▒▒\x1b[0m    \x1b[91;41m▒\x1b[0m   \x1b[91;41m▒▒▒▒▒\x1b[0m \x1b[91;41m▒\x1b[0m            \x1b[31m▀████▀\x1b[0m\n" +
	"           \x1b[90m·\x1b[0m      \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m     \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m  \x1b[90m·\x1b[0m              \x1b[36m+\x1b[0m\n" +
	"               \x1b[90m·\x1b[0m  \x1b[31m█\x1b[0m \x1b[97m*\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m \x1b[37m.\x1b[0m \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m  \x1b[31m█\x1b[0m    \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m         \x1b[97m*\x1b[0m   \x1b[37m.\x1b[0m\n" +
	"  \x1b[97m*\x1b[0m         \x1b[36m+\x1b[0m   \x1b[31m█████\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m  \x1b[31m███\x1b[0m  \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m   \x1b[31m█\x1b[0m \x1b[31m█████\x1b[0m            \x1b[36m+\x1b[0m\n" +
	" \x1b[90m·\x1b[0m      \x1b[36m+\x1b[0m     \x1b[90m·\x1b[0m \x1b[90m▓▓▓▓▓\x1b[0m \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m  \x1b[90m▓▓▓\x1b[0m  \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m \x1b[90m▓▓▓▓▓\x1b[0m        \x1b[90m·\x1b[0m\n" +
	"         \x1b[97m*\x1b[0m      \x1b[90m·\x1b[0m       \x1b[37m.\x1b[0m      \x1b[37m.\x1b[0m \x1b[37m.\x1b[0m     \x1b[36m+\x1b[0m                                 \x1b[90m·\x1b[0m\n" +
	"                 \x1b[37m.\x1b[0m    \x1b[97m████\x1b[0m   \x1b[97m███\x1b[0m  \x1b[97m████\x1b[0m   \x1b[97m███\x1b[0m  \x1b[97m█\x1b[0m   \x1b[97m█\x1b[0m  \x1b[97m████\x1b[0m      \x1b[36m+\x1b[0m\n" +
	"   \x1b[93m▄▄\x1b[0m\x1b[97;103m▀\x1b[0m\x1b[97m███▄▄\x1b[0m  \x1b[97m*\x1b[0m        \x1b[97;103m▒▒▒▒\x1b[0m   \x1b[97;103m▒▒▒\x1b[0m  \x1b[97;103m▒▒▒▒\x1b[0m   \x1b[97;103m▒▒▒\x1b[0m  \x1b[97;103m▒\x1b[0m   \x1b[97;103m▒\x1b[0m  \x1b[97;103m▒▒▒▒\x1b[0m    \x1b[90m·\x1b[0m  \x1b[90m·\x1b[0m \x1b[33m▄▄\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m███▄▄\x1b[0m\n" +
	"  \x1b[93;43m▀\x1b[0m\x1b[93m████\x1b[0m\x1b[97;103m▀\x1b[0m\x1b[97m████\x1b[0m          \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m \x1b[37m.\x1b[0m \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m██\x1b[0m  \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m            \x1b[33;100m▀\x1b[0m\x1b[33m████\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m████\x1b[0m\n" +
	" \x1b[33m███\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m████\x1b[0m\x1b[97;103m▀\x1b[0m\x1b[97m███\x1b[0m    \x1b[37m.\x1b[0m \x1b[36m+\x1b[0m  \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m   \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m \x1b[36m+\x1b[0m \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m \x1b[93m█\x1b[0m  \x1b[37m.\x1b[0m    \x1b[90m·\x1b[0m   \x1b[90m███\x1b[0m\x1b[33;100m▀\x1b[0m\x1b[33m████\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m███\x1b[0m\n" +
	" \x1b[33m█████\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m████\x1b[0m\x1b[97;103m▀\x1b[0m\x1b[97m█\x1b[0m       \x1b[37m.\x1b[0m \x1b[93;43m▒\x1b[0m   \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m   \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[37m.\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[90m·\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m \x1b[93;43m▒\x1b[0m   \x1b[37m.\x1b[0m     \x1b[37m──\x1b[0m\x1b[90m█████\x1b[0m\x1b[33;100m▀\x1b[0m\x1b[33m████\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m█\x1b[0m\x1b[37m──\x1b[0m\n" +
	"  \x1b[33m██████\x1b[0m\x1b[93;43m▀\x1b[0m\x1b[93m███\x1b[0m          \x1b[33m████\x1b[0m  \x1b[33m█████\x1b[0m \x1b[33m████\x1b[0m  \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m  \x1b[33m██\x1b[0m  \x1b[33m███\x1b[0m         \x1b[90m██████\x1b[0m\x1b[33;100m▀\x1b[0m\x1b[33m███\x1b[0m\n" +
	"   \x1b[33m▀▀████▀\x1b[0m\x1b[93m▀\x1b[0m      \x1b[37m.\x1b[0m    \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m  \x1b[97m*\x1b[0m  \x1b[33m█\x1b[0m         \x1b[90m▀▀████▀\x1b[0m\x1b[33m▀\x1b[0m\n" +
	"           \x1b[36m+\x1b[0m     \x1b[90m·\x1b[0m    \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m  \x1b[33m█\x1b[0m  \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[36m+\x1b[0m   \x1b[33m█\x1b[0m  \x1b[37m.\x1b[0m  \x1b[90m·\x1b[0m           \x1b[97m*\x1b[0m \x1b[90m·\x1b[0m\n" +
	"      \x1b[90m·\x1b[0m   \x1b[97m*\x1b[0m           \x1b[33m████\x1b[0m  \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m  \x1b[33m███\x1b[0m  \x1b[33m█\x1b[0m   \x1b[33m█\x1b[0m \x1b[33m████\x1b[0m \x1b[37m.\x1b[0m                \x1b[97m*\x1b[0m\n" +
	"     \x1b[90m·\x1b[0m                \x1b[90m▓▓▓▓\x1b[0m  \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m  \x1b[90m▓▓▓\x1b[0m  \x1b[90m▓\x1b[0m   \x1b[90m▓\x1b[0m \x1b[90m▓▓▓▓\x1b[0m  \x1b[37m.\x1b[0m        \x1b[90m·\x1b[0m\n" +
	"\x1b[90m▂▃▁▄▆\x1b[0m\x1b[31m▂▅▃▁▇\x1b[0m\x1b[90m▂▄▃▆▁\x1b[0m\x1b[31m▃▅▂▄▃\x1b[0m\x1b[90m▇▂▅▃▁\x1b[0m\x1b[31m▄▆▂▃▅\x1b[0m\x1b[90m▁▇▃▂▄\x1b[0m\x1b[31m▆▂▅▃▁\x1b[0m\x1b[90m▂▃▁▄▆\x1b[0m\x1b[31m▂▅▃▁▇\x1b[0m\x1b[90m▂▄▃▆▁\x1b[0m\x1b[31m▃▅▂▄▃\x1b[0m\x1b[90m▇▂▅▃▁\x1b[0m\x1b[31m▄▆▂▃▅\x1b[0m\x1b[90m▁▇▃▂▄\x1b[0m\x1b[31m▆▂▅▃▁\x1b[0m\n" +
	"\x1b[31m░\x1b[0m\x1b[90m░░▒▒░\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m░░░░░▒\x1b[0m\x1b[31m▒\x1b[0m\x1b[90m░░░░░░░\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m▒░░░\x1b[0m\x1b[31m░\x1b[0m\x1b[90m░░░▒▒░░░░░\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m░\x1b[0m\x1b[31m▒\x1b[0m\x1b[90m▒░░░░░░░▒▒░░\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m░░░░▒▒░░░░░░\x1b[0m\x1b[31m░\x1b[0m\x1b[90m▒▒\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m░░░░░\x1b[0m\x1b[93m▓\x1b[0m\x1b[90m▒▒░\x1b[0m\x1b[31m░\x1b[0m\x1b[90m░\x1b[0m" +
	"\x1b[0m"
