// Package bencode implements just enough of BEP 3 (bencoding) to read
// .torrent files and tracker HTTP announce responses. It is not a general
// purpose codec: values decode to int64, []byte, []any, or map[string]any.
package bencode

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

var errUnexpectedEOF = errors.New("bencode: unexpected end of input")

// Decode parses a single bencoded value from the start of data and returns
// it along with the number of bytes consumed.
func Decode(data []byte) (any, int, error) {
	return decodeValue(data, 0)
}

func decodeValue(data []byte, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, errUnexpectedEOF
	}
	switch {
	case data[pos] == 'i':
		return decodeInt(data, pos)
	case data[pos] == 'l':
		return decodeList(data, pos)
	case data[pos] == 'd':
		return decodeDict(data, pos)
	case data[pos] >= '0' && data[pos] <= '9':
		return decodeString(data, pos)
	default:
		return nil, pos, fmt.Errorf("bencode: invalid token %q at offset %d", data[pos], pos)
	}
}

func decodeInt(data []byte, pos int) (int64, int, error) {
	end := bytes.IndexByte(data[pos:], 'e')
	if end < 0 {
		return 0, pos, errUnexpectedEOF
	}
	end += pos
	n, err := strconv.ParseInt(string(data[pos+1:end]), 10, 64)
	if err != nil {
		return 0, pos, fmt.Errorf("bencode: invalid integer at offset %d: %w", pos, err)
	}
	return n, end + 1, nil
}

func decodeString(data []byte, pos int) ([]byte, int, error) {
	colon := bytes.IndexByte(data[pos:], ':')
	if colon < 0 {
		return nil, pos, errUnexpectedEOF
	}
	colon += pos
	length, err := strconv.Atoi(string(data[pos:colon]))
	if err != nil || length < 0 {
		return nil, pos, fmt.Errorf("bencode: invalid string length at offset %d", pos)
	}
	start := colon + 1
	end := start + length
	if end > len(data) {
		return nil, pos, errUnexpectedEOF
	}
	return data[start:end], end, nil
}

func decodeList(data []byte, pos int) ([]any, int, error) {
	pos++ // consume 'l'
	var list []any
	for {
		if pos >= len(data) {
			return nil, pos, errUnexpectedEOF
		}
		if data[pos] == 'e' {
			return list, pos + 1, nil
		}
		v, next, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, err
		}
		list = append(list, v)
		pos = next
	}
}

func decodeDict(data []byte, pos int) (map[string]any, int, error) {
	pos++ // consume 'd'
	dict := make(map[string]any)
	for {
		if pos >= len(data) {
			return nil, pos, errUnexpectedEOF
		}
		if data[pos] == 'e' {
			return dict, pos + 1, nil
		}
		key, next, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, err
		}
		pos = next
		val, next, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, err
		}
		dict[string(key)] = val
		pos = next
	}
}

// RawInfoDict scans a top-level bencoded dict for the "info" key and returns
// the exact raw bytes of its value, unparsed. This is BEP 3's prescribed way
// to compute an info_hash: hash the info dict's bytes exactly as they
// appeared in the original file rather than a re-encoding, since a
// re-encoding is only guaranteed identical if the source used canonical
// (sorted-key) form, which not every torrent file does.
func RawInfoDict(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 'd' {
		return nil, errors.New("bencode: not a dict")
	}
	pos := 1
	for {
		if pos >= len(data) {
			return nil, errUnexpectedEOF
		}
		if data[pos] == 'e' {
			return nil, errors.New("bencode: no \"info\" key found")
		}
		key, next, err := decodeString(data, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		valStart := pos
		_, valEnd, err := decodeValue(data, pos)
		if err != nil {
			return nil, err
		}
		if string(key) == "info" {
			return data[valStart:valEnd], nil
		}
		pos = valEnd
	}
}

// Encode writes v back to bencoded form. Maps are encoded with sorted keys
// as BEP 3 requires. Supported types: int64/int, []byte/string, []any,
// map[string]any.
func Encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(buf *bytes.Buffer, v any) error {
	switch t := v.(type) {
	case int:
		fmt.Fprintf(buf, "i%de", t)
	case int64:
		fmt.Fprintf(buf, "i%de", t)
	case string:
		fmt.Fprintf(buf, "%d:%s", len(t), t)
	case []byte:
		fmt.Fprintf(buf, "%d:", len(t))
		buf.Write(t)
	case []any:
		buf.WriteByte('l')
		for _, item := range t {
			if err := encodeValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	case map[string]any:
		buf.WriteByte('d')
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(buf, "%d:%s", len(k), k)
			if err := encodeValue(buf, t[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	default:
		return fmt.Errorf("bencode: unsupported type %T", v)
	}
	return nil
}
