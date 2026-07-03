package session

import "fmt"

// ReadLine reads a line of input terminated by Enter, echoing keystrokes
// (the console runs in no-echo mode). Backspace/DEL erase the last rune.
// It returns whatever was typed so far if the stream ends.
func ReadLine(s Session) (string, error) {
	var b []rune
	for {
		r, err := s.ReadKey()
		if err != nil {
			return string(b), err
		}
		switch r {
		case '\r', '\n':
			fmt.Fprint(s, "\n")
			return string(b), nil
		case 127, 8: // DEL / backspace
			if len(b) > 0 {
				b = b[:len(b)-1]
				fmt.Fprint(s, "\b \b")
			}
		default:
			if r >= 32 {
				b = append(b, r)
				fmt.Fprintf(s, "%c", r)
			}
		}
	}
}
