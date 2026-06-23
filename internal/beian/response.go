package beian

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func normalizeResponse(data map[string]any) map[string]any {
	code, ok := ResponseCode(data)
	if ok && code == 500 {
		return map[string]any{"code": 122, "message": "工信部服务器异常"}
	}
	return data
}

// ResponseCode returns the numeric API code from a response map.
func ResponseCode(data map[string]any) (int, bool) {
	v, ok := numberValue(data["code"])
	return v, ok
}

// ResponseSuccess returns the success flag from a response map.
func ResponseSuccess(data map[string]any) bool {
	v, ok := boolValue(data["success"])
	return ok && v
}

// ResponseList returns params.list from a response map.
func ResponseList(data map[string]any) ([]any, bool) {
	params, ok := responseParams(data)
	if !ok {
		return nil, false
	}
	list, ok := params["list"].([]any)
	return list, ok
}

// ResponseTotal returns params.total from a response map.
func ResponseTotal(data map[string]any) (int, bool) {
	params, ok := responseParams(data)
	if !ok {
		return 0, false
	}
	return numberValue(params["total"])
}

func responseParams(data map[string]any) (map[string]any, bool) {
	params, ok := data["params"].(map[string]any)
	return params, ok
}

func stringValue(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, x != ""
	case fmt.Stringer:
		s := x.String()
		return s, s != ""
	case json.Number:
		return x.String(), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case int:
		return strconv.Itoa(x), true
	default:
		return "", false
	}
}

func numberValue(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return int(n), true
		}
		f, err := x.Float64()
		return int(f), err == nil
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	default:
		return 0, false
	}
}

func boolValue(v any) (bool, bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		b, err := strconv.ParseBool(x)
		return b, err == nil
	default:
		return false, false
	}
}
