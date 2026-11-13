package money

import (
	"strconv"
	"strings"
)

func Format(amount int64, currency string) string {
	if currency == "IDR" {
		return FormatIDR(amount)
	}
	return currency + " " + strconv.FormatInt(amount, 10)
}

func FormatIDR(amount int64) string {
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	return sign + "Rp " + groupThousands(strconv.FormatInt(amount, 10), '.')
}

func groupThousands(digits string, sep byte) string {
	var b strings.Builder
	b.Grow(len(digits) + len(digits)/3)

	for i := 0; i < len(digits); i++ {

		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(sep)
		}
		b.WriteByte(digits[i])
	}
	return b.String()
}
