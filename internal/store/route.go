package store

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// RouteFile is this board's own routing overrides (BRE's ROUTE.CFG). A league
// whose Coordinator maintains HOST routing in the roster does not need one.
const RouteFile = "ibroute.cfg"

// ParseRouteFile reads routing overrides. Each rule is "ROUTE <dest> <via>",
// where dest is a node number or "*" for every board, and via is the node to
// hand the packet to. Lines starting with ";" are comments.
//
// BRE's file also carries CRASH / HOLD / NORMAL, which set a FidoNet mailer's
// send priority. Those are read and ignored here: a packet is a file in a
// directory, and what the transport does with it afterwards is configured in
// the transport, not in the game.
func ParseRouteFile(path string) ([]game.RouteRule, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var rules []game.RouteRule
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 || strings.HasPrefix(fields[0], ";") ||
			!strings.EqualFold(fields[0], "ROUTE") {
			continue
		}
		dest := 0
		if fields[1] != "*" {
			n, err := strconv.Atoi(fields[1])
			if err != nil || n <= 0 {
				continue
			}
			dest = n
		}
		via, err := strconv.Atoi(fields[2])
		if err != nil || via <= 0 {
			continue
		}
		rules = append(rules, game.RouteRule{Dest: dest, Via: via})
	}
	return rules, sc.Err()
}
