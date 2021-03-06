package aoebot

var toAesthetic = map[rune]rune{
	' ':  '　',
	'`':  '`',
	'1':  '１',
	'2':  '２',
	'3':  '３',
	'4':  '４',
	'5':  '５',
	'6':  '６',
	'7':  '７',
	'8':  '８',
	'9':  '９',
	'0':  '０',
	'-':  '－',
	'=':  '＝',
	'~':  '~',
	'!':  '！',
	'@':  '＠',
	'#':  '＃',
	'$':  '＄',
	'%':  '％',
	'^':  '^',
	'&':  '＆',
	'*':  '＊',
	'(':  '（',
	')':  '）',
	'_':  '_',
	'+':  '＋',
	'q':  'ｑ',
	'w':  'ｗ',
	'e':  'ｅ',
	'r':  'ｒ',
	't':  'ｔ',
	'y':  'ｙ',
	'u':  'ｕ',
	'i':  'ｉ',
	'o':  'ｏ',
	'p':  'ｐ',
	'[':  '[',
	']':  ']',
	'\\': '\\',
	'Q':  'Ｑ',
	'W':  'Ｗ',
	'E':  'Ｅ',
	'R':  'Ｒ',
	'T':  'Ｔ',
	'Y':  'Ｙ',
	'U':  'Ｕ',
	'I':  'Ｉ',
	'O':  'Ｏ',
	'P':  'Ｐ',
	'{':  '{',
	'}':  '}',
	'|':  '|',
	'a':  'ａ',
	's':  'ｓ',
	'd':  'ｄ',
	'f':  'ｆ',
	'g':  'ｇ',
	'h':  'ｈ',
	'j':  'ｊ',
	'k':  'ｋ',
	'l':  'ｌ',
	';':  '；',
	'\'': '＇',
	'A':  'Ａ',
	'S':  'Ｓ',
	'D':  'Ｄ',
	'F':  'Ｆ',
	'G':  'Ｇ',
	'H':  'Ｈ',
	'J':  'Ｊ',
	'K':  'Ｋ',
	'L':  'Ｌ',
	':':  '：',
	'"':  '"',
	'z':  'ｚ',
	'x':  'ｘ',
	'c':  'ｃ',
	'v':  'ｖ',
	'b':  'ｂ',
	'n':  'ｎ',
	'm':  'ｍ',
	',':  '，',
	'.':  '．',
	'/':  '／',
	'Z':  'Ｚ',
	'X':  'Ｘ',
	'C':  'Ｃ',
	'V':  'Ｖ',
	'B':  'Ｂ',
	'N':  'Ｎ',
	'M':  'Ｍ',
	'<':  '<',
	'>':  '>',
	'?':  '？',
}

var fromAesthetic = map[rune]rune{
	'　': ' ',
	'ａ': 'a',
	'ｂ': 'b',
	'ｃ': 'c',
	'ｄ': 'd',
	'ｅ': 'e',
	'ｆ': 'f',
	'ｇ': 'g',
	'ｈ': 'h',
	'ｉ': 'i',
	'ｊ': 'j',
	'ｋ': 'k',
	'ｌ': 'l',
	'ｍ': 'm',
	'ｎ': 'n',
	'ｏ': 'o',
	'ｐ': 'p',
	'ｑ': 'q',
	'ｒ': 'r',
	'ｓ': 's',
	'ｔ': 't',
	'ｕ': 'u',
	'ｖ': 'v',
	'ｗ': 'w',
	'ｘ': 'x',
	'ｙ': 'y',
	'ｚ': 'z',
	'Ａ': 'A',
	'Ｂ': 'B',
	'Ｃ': 'C',
	'Ｄ': 'D',
	'Ｅ': 'E',
	'Ｆ': 'F',
	'Ｇ': 'G',
	'Ｈ': 'H',
	'Ｉ': 'I',
	'Ｊ': 'J',
	'Ｋ': 'K',
	'Ｌ': 'L',
	'Ｍ': 'M',
	'Ｎ': 'N',
	'Ｏ': 'O',
	'Ｐ': 'P',
	'Ｑ': 'Q',
	'Ｒ': 'R',
	'Ｓ': 'S',
	'Ｔ': 'T',
	'Ｕ': 'U',
	'Ｖ': 'V',
	'Ｗ': 'W',
	'Ｘ': 'X',
	'Ｙ': 'Y',
	'Ｚ': 'Z',
	'０': '0',
	'１': '1',
	'２': '2',
	'３': '3',
	'４': '4',
	'５': '5',
	'６': '6',
	'７': '7',
	'８': '8',
	'９': '9',
	'`': '`',
	'~': '~',
	'！': '!',
	'＠': '@',
	'＃': '#',
	'＄': '$',
	'％': '%',
	'^': '^',
	'＆': '&',
	'＊': '*',
	'（': '(',
	'）': ')',
	'_': '_',
	'＋': '+',
	'－': '-',
	'＝': '=',
	'[': '[',
	']': ']',
	'{': '{',
	'}': '}',
	'|': '|',
	'；': ';',
	'＇': '\'',
	'：': ':',
	'"': '"',
	'，': ',',
	'．': '.',
	'／': '/',
	'<': '<',
	'>': '>',
	'？': '?',
}
