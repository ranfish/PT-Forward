package fingerprint

import (
	"bytes"
	"fmt"
	"sort"
)

func decodeBencode(data []byte) (any, error) {
	v, _, err := decodeValue(data, 0)
	return v, err
}

func decodeValue(data []byte, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unexpected end of data at pos %d", pos)
	}

	switch data[pos] {
	case 'd':
		return decodeDict(data, pos)
	case 'l':
		return decodeList(data, pos)
	case 'i':
		return decodeInt(data, pos)
	default:
		if data[pos] >= '0' && data[pos] <= '9' {
			return decodeString(data, pos)
		}
		return nil, pos, fmt.Errorf("unexpected byte %q at pos %d", data[pos], pos)
	}
}

func decodeDict(data []byte, pos int) (any, int, error) {
	if data[pos] != 'd' {
		return nil, pos, fmt.Errorf("expected 'd' at pos %d", pos)
	}
	pos++

	result := make(map[string]any)
	for pos < len(data) && data[pos] != 'e' {
		key, newPos, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict key: %w", err)
		}
		keyStr, ok := key.(string)
		if !ok {
			return nil, pos, fmt.Errorf("dict key is not a string")
		}

		value, newPos, err := decodeValue(data, newPos)
		if err != nil {
			return nil, pos, fmt.Errorf("dict value for key %q: %w", keyStr, err)
		}
		result[keyStr] = value
		pos = newPos
	}

	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated dict")
	}
	pos++

	return result, pos, nil
}

func decodeList(data []byte, pos int) (any, int, error) {
	if data[pos] != 'l' {
		return nil, pos, fmt.Errorf("expected 'l' at pos %d", pos)
	}
	pos++

	var result []any
	for pos < len(data) && data[pos] != 'e' {
		value, newPos, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, fmt.Errorf("list item: %w", err)
		}
		result = append(result, value)
		pos = newPos
	}

	if pos >= len(data) {
		return nil, pos, fmt.Errorf("unterminated list")
	}
	pos++

	return result, pos, nil
}

func decodeInt(data []byte, pos int) (any, int, error) {
	if data[pos] != 'i' {
		return nil, pos, fmt.Errorf("expected 'i' at pos %d", pos)
	}
	pos++

	end := bytes.IndexByte(data[pos:], 'e')
	if end < 0 {
		return nil, pos, fmt.Errorf("unterminated int")
	}
	end += pos

	var val int64
	if _, err := fmt.Sscanf(string(data[pos:end]), "%d", &val); err != nil {
		return nil, pos, fmt.Errorf("parse int: %w", err)
	}

	return val, end + 1, nil
}

func decodeString(data []byte, pos int) (any, int, error) {
	colon := bytes.IndexByte(data[pos:], ':')
	if colon < 0 {
		return nil, pos, fmt.Errorf("missing colon in string at pos %d", pos)
	}
	colon += pos

	var length int
	if _, err := fmt.Sscanf(string(data[pos:colon]), "%d", &length); err != nil {
		return nil, pos, fmt.Errorf("parse string length: %w", err)
	}

	start := colon + 1
	end := start + length
	if end > len(data) {
		return nil, pos, fmt.Errorf("string extends beyond data")
	}

	return string(data[start:end]), end, nil
}

func encodeBencode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := encodeValue(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeValue(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case string:
		fmt.Fprintf(buf, "%d:%s", len(val), val)
	case int:
		fmt.Fprintf(buf, "i%de", val)
	case int64:
		fmt.Fprintf(buf, "i%de", val)
	case []any:
		buf.WriteByte('l')
		for _, item := range val {
			if err := encodeValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	case map[string]any:
		buf.WriteByte('d')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(buf, "%d:%s", len(k), k)
			if err := encodeValue(buf, val[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('e')
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}
