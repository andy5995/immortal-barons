package ftn

import (
	"fmt"
	"strconv"
	"strings"
)

// Address is a four-dimensional FTN address: zone:net/node.point.
type Address struct {
	Zone  uint16
	Net   uint16
	Node  uint16
	Point uint16
}

func (a Address) String() string {
	if a.Point != 0 {
		return fmt.Sprintf("%d:%d/%d.%d", a.Zone, a.Net, a.Node, a.Point)
	}
	return fmt.Sprintf("%d:%d/%d", a.Zone, a.Net, a.Node)
}

// ParseAddress accepts zone:net/node with an optional .point. A complete zone
// is required: guessing it is unsafe when an INTL kludge crosses zones.
func ParseAddress(s string) (Address, error) {
	original := s
	s = strings.TrimSpace(s)
	zoneText, rest, ok := strings.Cut(s, ":")
	if !ok {
		return Address{}, fmt.Errorf("FTN address %q has no zone (want zone:net/node)", original)
	}
	netText, nodeText, ok := strings.Cut(rest, "/")
	if !ok {
		return Address{}, fmt.Errorf("invalid FTN address %q (want zone:net/node)", original)
	}
	pointText := "0"
	if node, point, found := strings.Cut(nodeText, "."); found {
		nodeText, pointText = node, point
	}
	parts := []struct {
		name string
		text string
		dst  *uint16
	}{
		{"zone", zoneText, nil},
		{"net", netText, nil},
		{"node", nodeText, nil},
		{"point", pointText, nil},
	}
	var a Address
	parts[0].dst = &a.Zone
	parts[1].dst = &a.Net
	parts[2].dst = &a.Node
	parts[3].dst = &a.Point
	for _, p := range parts {
		if p.text == "" {
			return Address{}, fmt.Errorf("invalid %s in FTN address %q", p.name, original)
		}
		n, err := strconv.ParseUint(p.text, 10, 16)
		if err != nil {
			return Address{}, fmt.Errorf("invalid %s in FTN address %q", p.name, original)
		}
		*p.dst = uint16(n)
	}
	return a, nil
}
