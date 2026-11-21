package wire

import "unicode/utf8"

// ServerTrimCodePoints are the code points the panel API (/api/v1/pbl/*) trims
// from both ends of every request string before it verifies the checksum
// (Laravel 13 global TrimStrings middleware, Str::trim): PCRE2 \s in UTF/UCP
// mode (tab, LF, VT, FF, CR, space, NEL, the Z category), NUL and Laravel's
// Str::INVISIBLE_CHARACTERS. strings.TrimSpace is not equivalent: it keeps
// U+0000, U+200B, U+FEFF and others, and trims U+001C-U+001F.
var ServerTrimCodePoints = [...]rune{
	0x0000, 0x0009, 0x000A, 0x000B, 0x000C, 0x000D, 0x0020, 0x0085, 0x00A0, 0x00AD, 0x034F, 0x061C, 0x115F,
	0x1160, 0x1680, 0x17B4, 0x17B5, 0x180E, 0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007,
	0x2008, 0x2009, 0x200A, 0x200B, 0x200C, 0x200D, 0x200E, 0x200F, 0x2028, 0x2029, 0x202F, 0x205F, 0x2060,
	0x2061, 0x2062, 0x2063, 0x2064, 0x2065, 0x206A, 0x206B, 0x206C, 0x206D, 0x206E, 0x206F, 0x2800, 0x3000,
	0x3164, 0xFEFF, 0xFFA0, 0x1D159, 0x1D173, 0x1D174, 0x1D175, 0x1D176, 0x1D177, 0x1D178, 0x1D179, 0x1D17A,
	0xE0020,
}

var serverTrimSet = func() map[rune]struct{} {
	set := make(map[rune]struct{}, len(ServerTrimCodePoints))
	for _, codePoint := range ServerTrimCodePoints {
		set[codePoint] = struct{}{}
	}
	return set
}()

func isServerTrimmed(codePoint rune) bool {
	_, trimmed := serverTrimSet[codePoint]
	return trimmed
}

// ServerTrim removes ServerTrimCodePoints from both ends of value, by code
// point, exactly as the panel API does; characters inside are kept. Invalid
// UTF-8 is returned unchanged (it cannot be encoded as JSON anyway).
func ServerTrim(value string) string {
	if !utf8.ValidString(value) {
		return value
	}
	start := 0
	for start < len(value) {
		codePoint, size := utf8.DecodeRuneInString(value[start:])
		if !isServerTrimmed(codePoint) {
			break
		}
		start += size
	}
	end := len(value)
	for end > start {
		codePoint, size := utf8.DecodeLastRuneInString(value[start:end])
		if !isServerTrimmed(codePoint) {
			break
		}
		end -= size
	}
	return value[start:end]
}

// TrimStrings applies ServerTrim to every string in value, nested bodies, maps
// and lists included; keys, their order and every other value are kept. A
// *Body or container is copied, never modified in place.
func TrimStrings(value any) any {
	switch typed := value.(type) {
	case string:
		return ServerTrim(typed)
	case *Body:
		if typed == nil {
			return typed
		}
		trimmed := NewBody()
		for _, key := range typed.keys {
			trimmed.Set(key, TrimStrings(typed.values[key]))
		}
		return trimmed
	case []any:
		trimmed := make([]any, len(typed))
		for index, item := range typed {
			trimmed[index] = TrimStrings(item)
		}
		return trimmed
	case []string:
		trimmed := make([]string, len(typed))
		for index, item := range typed {
			trimmed[index] = ServerTrim(item)
		}
		return trimmed
	case map[string]any:
		trimmed := make(map[string]any, len(typed))
		for key, item := range typed {
			trimmed[key] = TrimStrings(item)
		}
		return trimmed
	}
	return value
}

// TrimBody returns a copy of body with every string trimmed by ServerTrim.
func TrimBody(body *Body) *Body {
	return TrimStrings(body).(*Body)
}
