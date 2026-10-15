// Package js reproduces the JavaScript/Node primitives upstream's subscription
// code relies on, so Go output matches it byte for byte.
package js

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// EncodeURIComponent escapes everything except A-Z a-z 0-9 - _ . ! ~ * ' ( ).
func EncodeURIComponent(s string) string {
	const unreserved = "-_.!~*'()"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

// Base64 is Buffer.from(s).toString('base64').
func Base64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Base64Bytes is Buffer.from(b).toString('base64').
func Base64Bytes(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// Base64URL is Buffer#toString('base64url') (unpadded).
func Base64URL(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// DecodeBase64 is Buffer.from(s, 'base64'): it accepts both alphabets,
// ignores padding and stops at the first invalid character.
func DecodeBase64(s string) []byte {
	var clean strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			clean.WriteRune(r)
		case r == '+' || r == '-':
			clean.WriteByte('+')
		case r == '/' || r == '_':
			clean.WriteByte('/')
		case r == '=':
			goto done
		}
		// Node skips any other character.
	}
done:
	text := clean.String()
	if len(text)%4 == 1 {
		text = text[:len(text)-1]
	}
	decoded, err := base64.RawStdEncoding.DecodeString(text)
	if err != nil {
		return nil
	}
	return decoded
}

// DecodeBase64String is Buffer.from(s, 'base64').toString() (UTF-8, invalid
// sequences replaced like Node).
func DecodeBase64String(s string) string {
	return strings.ToValidUTF8(string(DecodeBase64(s)), "�")
}

// PrettyBytes ports prettyBytesUtil (xbytes, IEC, two decimals); sticky fixes
// the unit at prefixIndex (3 = GiB). Zero renders as "0".
func PrettyBytes(bytes float64, sticky bool, prefixIndex int) string {
	if bytes == 0 {
		return "0"
	}
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}
	index := 0
	if sticky {
		index = prefixIndex
	} else {
		for index < len(units)-1 && math.Abs(bytes) >= math.Pow(1024, float64(index+1)) {
			index++
		}
	}
	return strconv.FormatFloat(bytes/math.Pow(1024, float64(index)), 'f', 2, 64) + " " + units[index]
}

var (
	monthNames   = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// DayjsFormat ports dayjs#format (no plugins) for a UTC process.
func DayjsFormat(t time.Time, format string) string {
	t = t.UTC()
	if format == "" {
		format = "YYYY-MM-DDTHH:mm:ssZ"
	}
	tokens := []string{"YYYY", "YY", "MMMM", "MMM", "MM", "M", "DD", "D", "dddd", "ddd", "dd", "d", "HH", "H", "hh", "h", "mm", "m", "ss", "s", "SSS", "ZZ", "Z", "A", "a"}
	var b strings.Builder
	for i := 0; i < len(format); {
		if format[i] == '[' {
			if end := strings.IndexByte(format[i:], ']'); end > 0 {
				b.WriteString(format[i+1 : i+end])
				i += end + 1
				continue
			}
		}
		matched := false
		for _, token := range tokens {
			if strings.HasPrefix(format[i:], token) {
				b.WriteString(dayjsToken(t, token))
				i += len(token)
				matched = true
				break
			}
		}
		if !matched {
			r, size := utf8.DecodeRuneInString(format[i:])
			b.WriteRune(r)
			i += size
		}
	}
	return b.String()
}

func dayjsToken(t time.Time, token string) string {
	hour12 := t.Hour() % 12
	if hour12 == 0 {
		hour12 = 12
	}
	switch token {
	case "YYYY":
		return fmt.Sprintf("%04d", t.Year())
	case "YY":
		return fmt.Sprintf("%04d", t.Year())[2:]
	case "MMMM":
		return monthNames[t.Month()-1]
	case "MMM":
		return monthNames[t.Month()-1][:3]
	case "MM":
		return fmt.Sprintf("%02d", int(t.Month()))
	case "M":
		return strconv.Itoa(int(t.Month()))
	case "DD":
		return fmt.Sprintf("%02d", t.Day())
	case "D":
		return strconv.Itoa(t.Day())
	case "dddd":
		return weekdayNames[t.Weekday()]
	case "ddd":
		return weekdayNames[t.Weekday()][:3]
	case "dd":
		return weekdayNames[t.Weekday()][:2]
	case "d":
		return strconv.Itoa(int(t.Weekday()))
	case "HH":
		return fmt.Sprintf("%02d", t.Hour())
	case "H":
		return strconv.Itoa(t.Hour())
	case "hh":
		return fmt.Sprintf("%02d", hour12)
	case "h":
		return strconv.Itoa(hour12)
	case "mm":
		return fmt.Sprintf("%02d", t.Minute())
	case "m":
		return strconv.Itoa(t.Minute())
	case "ss":
		return fmt.Sprintf("%02d", t.Second())
	case "s":
		return strconv.Itoa(t.Second())
	case "SSS":
		return fmt.Sprintf("%03d", t.Nanosecond()/1e6)
	case "ZZ":
		return "+0000"
	case "Z":
		return "+00:00"
	case "A":
		if t.Hour() < 12 {
			return "AM"
		}
		return "PM"
	case "a":
		if t.Hour() < 12 {
			return "am"
		}
		return "pm"
	}
	return token
}

// ISOString is Date#toISOString.
func ISOString(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
