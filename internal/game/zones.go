package game

// zones.go — the zones the Preferences picker offers, grouped the way a player
// looks for their own. Data, not code: the list is a convenience over the full
// IANA database, which has some six hundred names and cannot be enumerated from
// Go at all, so a player whose zone is not here types its name instead.
//
// One name per populated offset, preferring the city a reader is likeliest to
// recognize. A name that is only an alias of another (Europe/Kiev, US/Eastern)
// is left out: it would read as a second choice that does the same thing.

// ZoneRegion is one heading in the picker and the zones under it.
type ZoneRegion struct {
	Name  string
	Zones []string
}

// ZoneRegions is the picker's two-level list.
var ZoneRegions = []ZoneRegion{
	{"North America", []string{
		"America/St_Johns", "America/Halifax", "America/New_York", "America/Toronto",
		"America/Chicago", "America/Mexico_City", "America/Denver", "America/Phoenix",
		"America/Los_Angeles", "America/Vancouver", "America/Anchorage", "Pacific/Honolulu",
	}},
	{"Central & South America", []string{
		"America/Panama", "America/Bogota", "America/Lima", "America/Caracas",
		"America/Santiago", "America/Argentina/Buenos_Aires", "America/Sao_Paulo",
	}},
	{"Europe", []string{
		"Atlantic/Reykjavik", "Europe/Lisbon", "Europe/Dublin", "Europe/London",
		"Europe/Madrid", "Europe/Paris", "Europe/Brussels", "Europe/Amsterdam",
		"Europe/Berlin", "Europe/Zurich", "Europe/Rome", "Europe/Vienna",
		"Europe/Prague", "Europe/Warsaw", "Europe/Copenhagen", "Europe/Oslo",
		"Europe/Stockholm", "Europe/Helsinki", "Europe/Athens", "Europe/Bucharest",
		"Europe/Kyiv", "Europe/Moscow",
	}},
	{"Africa & Middle East", []string{
		"Africa/Casablanca", "Africa/Lagos", "Africa/Cairo", "Africa/Nairobi",
		"Africa/Johannesburg", "Asia/Jerusalem", "Europe/Istanbul", "Asia/Baghdad",
		"Asia/Tehran", "Asia/Dubai",
	}},
	{"Asia", []string{
		"Asia/Karachi", "Asia/Kolkata", "Asia/Kathmandu", "Asia/Dhaka",
		"Asia/Bangkok", "Asia/Jakarta", "Asia/Singapore", "Asia/Hong_Kong",
		"Asia/Shanghai", "Asia/Taipei", "Asia/Manila", "Asia/Seoul", "Asia/Tokyo",
	}},
	{"Australia & Pacific", []string{
		"Australia/Perth", "Australia/Darwin", "Australia/Adelaide", "Australia/Brisbane",
		"Australia/Sydney", "Australia/Hobart", "Pacific/Guam", "Pacific/Auckland",
		"Pacific/Fiji",
	}},
}
