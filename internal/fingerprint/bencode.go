package fingerprint

import (
	"bytes"
	"fmt"
	"sort"
)

// DecodeBencode bencode 解码公共入口（§59.541 附七导出·原消费方
// extractAnnounces 已随 §59.573 撤除——保留为通用工具·当前仅基准
// 测试消费·未来 bencode 需求复用点）。
func DecodeBencode(data []byte) (any, error) {
	return decodeBencode(data)
}

func decodeBencode(data []byte) (any, error) {
	v, _, err := decodeValue(data, 0)
	return v, err
}

func decodeValue(data []byte, pos int) (any, int, error) {
	if pos >= len(data) {
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("unexpected end of data at pos %d", pos), nil)
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
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("unexpected byte %q at pos %d", data[pos], pos), nil)
	}
}

func decodeDict(data []byte, pos int) (any, int, error) {
	if data[pos] != 'd' {
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("expected 'd' at pos %d", pos), nil)
	}
	pos++

	result := make(map[string]any)
	for pos < len(data) && data[pos] != 'e' {
		key, newPos, err := decodeString(data, pos)
		if err != nil {
			return nil, pos, fpError(ErrFPBencode, "dict key", err)
		}
		keyStr, ok := key.(string)
		if !ok {
			return nil, pos, fpError(ErrFPBencode, "dict key is not a string", nil)
		}

		value, newPos, err := decodeValue(data, newPos)
		if err != nil {
			return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("dict value for key %q", keyStr), err)
		}
		result[keyStr] = value
		pos = newPos
	}

	if pos >= len(data) {
		return nil, pos, fpError(ErrFPBencode, "unterminated dict", nil)
	}
	pos++

	return result, pos, nil
}

func decodeList(data []byte, pos int) (any, int, error) {
	if data[pos] != 'l' {
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("expected 'l' at pos %d", pos), nil)
	}
	pos++

	var result []any
	for pos < len(data) && data[pos] != 'e' {
		value, newPos, err := decodeValue(data, pos)
		if err != nil {
			return nil, pos, fpError(ErrFPBencode, "list item", err)
		}
		result = append(result, value)
		pos = newPos
	}

	if pos >= len(data) {
		return nil, pos, fpError(ErrFPBencode, "unterminated list", nil)
	}
	pos++

	return result, pos, nil
}

func decodeInt(data []byte, pos int) (any, int, error) {
	if data[pos] != 'i' {
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("expected 'i' at pos %d", pos), nil)
	}
	pos++

	end := bytes.IndexByte(data[pos:], 'e')
	if end < 0 {
		return nil, pos, fpError(ErrFPBencode, "unterminated int", nil)
	}
	end += pos

	var val int64
	if _, err := fmt.Sscanf(string(data[pos:end]), "%d", &val); err != nil {
		return nil, pos, fpError(ErrFPBencode, "parse int", err)
	}

	return val, end + 1, nil
}

func decodeString(data []byte, pos int) (any, int, error) {
	colon := bytes.IndexByte(data[pos:], ':')
	if colon < 0 {
		return nil, pos, fpError(ErrFPBencode, fmt.Sprintf("missing colon in string at pos %d", pos), nil)
	}
	colon += pos

	var length int
	if _, err := fmt.Sscanf(string(data[pos:colon]), "%d", &length); err != nil || length < 0 {
		return nil, pos, fpError(ErrFPBencode, "parse string length", err)
	}

	// 11 刀2（v0.0.1652）: 先判长度上界再加法——减法形式无回绕。
	// 原实现 start+length 对 MaxInt 回绕为负·end > len(data) 失效
	// → data[start:end] 负索引 panic（V-02·离线驱动实测 [:-...789]）。
	// Sscanf("%d") 对 MaxInt64 解析成功不报 overflow——前置守卫必须
	// 在加法前（同 V-01 findBencodeEnd 同批范式）。
	start := colon + 1
	if length > len(data)-start {
		return nil, pos, fpError(ErrFPBencode, "string extends beyond data", nil)
	}
	end := start + length
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
		return fpError(ErrFPEncode, fmt.Sprintf("unsupported type: %T", v), nil)
	}
	return nil
}
