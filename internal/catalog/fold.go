package catalog

import "strings"

// fold maps letters with diacritics (Latin-1, Latin Extended-A) to ASCII.
var foldMap = map[rune]string{
	'ą': "a", 'ć': "c", 'ę': "e", 'ł': "l", 'ń': "n", 'ó': "o", 'ś': "s", 'ź': "z", 'ż': "z",
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae", 'ç': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ì': "i", 'í': "i", 'î': "i", 'ï': "i",
	'ñ': "n", 'ò': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ù': "u", 'ú': "u",
	'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss", 'č': "c", 'ď': "d", 'ě': "e",
	'ň': "n", 'ř': "r", 'š': "s", 'ť': "t", 'ů': "u", 'ž': "z", 'ő': "o", 'ű': "u",
}

// Fold lower-cases s and replaces common diacritics with plain letters.
func Fold(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if m, ok := foldMap[r]; ok {
			b.WriteString(m)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
