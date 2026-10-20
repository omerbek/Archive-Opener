package arc

import "strings"

// cp857 is the upper half of IBM code page 857 (Turkish DOS / OEM), which
// older Windows zip tools use for file names. Undefined slots map to '_'.
var cp857 = []rune("ÇüéâäàåçêëèïîıÄÅ" +
	"ÉæÆôöòûùİÖÜø£ØŞş" +
	"áíóúñÑĞğ¿®¬½¼¡«»" +
	"░▒▓│┤ÁÂÀ©╣║╗╝¢¥┐" +
	"└┴┬├─┼ãÃ╚╔╩╦╠═╬¤" +
	"ºªÊËÈ€ÍÎÏ┘┌█▄¦Ì▀" +
	"ÓßÔÒõÕµ_×ÚÛÙìÿ¯´" +
	"­±_¾¶§÷¸°¨·¹³²■ ")

func decodeCP857(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 {
			b.WriteByte(c)
		} else {
			b.WriteRune(cp857[c-0x80])
		}
	}
	return b.String()
}
