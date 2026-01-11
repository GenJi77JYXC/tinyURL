package util

import "strings"

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func Base62Encode(num int64) string {
	if num == 0 {
		return "0"
	}
	var result strings.Builder
	for num > 0 {
		result.WriteByte(base62Chars[num%62])
		num /= 62
	}
	// 反转字符串（因为我们是从低位开始的）
	str := result.String()
	runes := []rune(str)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}
