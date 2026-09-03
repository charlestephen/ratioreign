package profile

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// randomFromPattern generates a random string matching a small regex subset:
// literal characters, escaped characters, and one character class `[...]`
// (with ranges and escaped chars) optionally followed by a `{n}` repeat
// count. This covers every client profile shipped with Joal — full regex
// support (alternation, groups, backreferences) was never needed there and
// isn't here either.
//
// Output is built byte-by-byte rather than rune-by-rune: some profiles (e.g.
// rtorrent's `[\x01-\xff]{12}` byte-pool peer id) intentionally generate
// arbitrary single bytes up to 0xFF, which must land in the result as that
// raw byte, not as its multi-byte UTF-8 encoding.
func randomFromPattern(pattern string) (string, error) {
	var out strings.Builder
	r := []rune(pattern)
	i := 0
	for i < len(r) {
		switch r[i] {
		case '\\':
			if i+1 >= len(r) {
				return "", fmt.Errorf("profile: dangling escape in pattern %q", pattern)
			}
			writeRuneAsByteIfPossible(&out, r[i+1])
			i += 2
		case '[':
			class, next, err := parseClass(r, i)
			if err != nil {
				return "", err
			}
			i = next
			count := 1
			if i < len(r) && r[i] == '{' {
				n, next, err := parseCount(r, i)
				if err != nil {
					return "", err
				}
				count = n
				i = next
			}
			for range count {
				c, err := randomRune(class)
				if err != nil {
					return "", err
				}
				writeRuneAsByteIfPossible(&out, c)
			}
		default:
			writeRuneAsByteIfPossible(&out, r[i])
			i++
		}
	}
	return out.String(), nil
}

func writeRuneAsByteIfPossible(out *strings.Builder, r rune) {
	if r <= 0xFF {
		out.WriteByte(byte(r))
		return
	}
	out.WriteRune(r)
}

type classItem struct {
	lo, hi rune // lo == hi for a single char
}

func parseClass(r []rune, start int) ([]classItem, int, error) {
	i := start + 1 // skip '['
	var items []classItem
	for i < len(r) && r[i] != ']' {
		var c rune
		if r[i] == '\\' {
			if i+1 >= len(r) {
				return nil, i, fmt.Errorf("profile: dangling escape in character class")
			}
			c = r[i+1]
			i += 2
		} else {
			c = r[i]
			i++
		}
		if i+1 < len(r) && r[i] == '-' && r[i+1] != ']' {
			var hi rune
			j := i + 1
			if r[j] == '\\' {
				hi = r[j+1]
				j += 2
			} else {
				hi = r[j]
				j++
			}
			items = append(items, classItem{lo: c, hi: hi})
			i = j
		} else {
			items = append(items, classItem{lo: c, hi: c})
		}
	}
	if i >= len(r) {
		return nil, i, fmt.Errorf("profile: unterminated character class")
	}
	return items, i + 1, nil // skip ']'
}

func parseCount(r []rune, start int) (int, int, error) {
	i := start + 1 // skip '{'
	j := i
	for j < len(r) && r[j] != '}' {
		j++
	}
	if j >= len(r) {
		return 0, j, fmt.Errorf("profile: unterminated {count}")
	}
	n := 0
	for _, c := range r[i:j] {
		if c < '0' || c > '9' {
			return 0, j, fmt.Errorf("profile: invalid {count}")
		}
		n = n*10 + int(c-'0')
	}
	return n, j + 1, nil
}

func randomRune(class []classItem) (rune, error) {
	total := int64(0)
	for _, it := range class {
		total += int64(it.hi-it.lo) + 1
	}
	if total <= 0 {
		return 0, fmt.Errorf("profile: empty character class")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(total))
	if err != nil {
		return 0, err
	}
	off := n.Int64()
	for _, it := range class {
		span := int64(it.hi-it.lo) + 1
		if off < span {
			return it.lo + rune(off), nil
		}
		off -= span
	}
	return 0, fmt.Errorf("profile: unreachable")
}
