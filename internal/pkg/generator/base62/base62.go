package base62

import (
	"errors"
	"math"
	"strings"
)

const (
	// alphabet 是我们定义的 Base62 字符集。
	// 顺序可以自定义，但一旦确定，就不能再更改，否则已生成的短链接将无法解码。
	alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	// base 是字符集的长度，即62。
	base = uint64(len(alphabet))
)

var (
	// charMap 用于快速查找字符在 alphabet 中的索引，以优化解码速度。
	// 使用 init() 函数在程序启动时构建这个 map。
	charMap = make(map[rune]uint64)
	
	ErrInvalidCharacter = errors.New("base62: invalid character in input string") 	// 解码时遇到了不在 alphabet 中的无效字符
	ErrOverflow = errors.New("base62: decoded value exceeds uint64 max") 			// 解码结果超出uint64范围
	ErrEmptyInput  = errors.New("base62: input string is empty") 					// 输入字符串为空

)

func init() {
	for i, char := range alphabet {
		charMap[char] = uint64(i)
	}
}

// Encode 将一个 uint64 类型的正整数转换为 Base62 编码的字符串。
// 这是从 ID 到短码字符串的关键步骤之一。
func Encode(number uint64) string {
	if number == 0 {
		return string(alphabet[0]) // 处理特殊情况：输入为0
	}

	var sb strings.Builder
	// 预估字符串长度，稍微减少内存分配次数
	// Ceil(Log62(number)) -> Ceil(Log(number) / Log(62))
	estimatedLen := int(math.Ceil(math.Log(float64(number)) / math.Log(float64(base))))
	sb.Grow(estimatedLen)

	// 使用经典的 "除基取余法"
	for number > 0 {
		remainder := number % base
		sb.WriteByte(alphabet[remainder])
		number /= base
	}

	// 因为取余法得到的是反向的字符串（例如：123 -> "321"），所以需要反转。
	runes := []rune(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// Decode 将一个 Base62 编码的字符串解析回 uint64 类型的正整数。
// 这个函数在需要从 short_code 反查 ID 时非常有用。
func Decode(encoded string) (uint64, error) {
	// 处理空字符串输入
	if encoded == "" {
		return 0, ErrEmptyInput
	}

	var number uint64 = 0
	power := uint64(1) // 代表 62 的 n 次方

	// 从字符串的末尾开始向前遍历
	for i := len(encoded) - 1; i >= 0; i-- {
		char := rune(encoded[i])
		value, ok := charMap[char]
		if !ok {
			// 如果在 charMap 中找不到字符，说明输入了无效字符
			return 0, ErrInvalidCharacter
		}

		// 检查溢出：如果 (math.MaxUint64 - value*power) / base < number，则会溢出
        // 为简化，我们这里假设生成的ID不会导致解码溢出，在大型系统中需要更严谨的检查
        // if number > (math.MaxUint64 - value) / base { ... }
		if value > math.MaxUint64 / power || number > math.MaxUint64 - value*power {
			return 0, ErrOverflow
		}
        
		number += value * power
		power *= base
	}

	return number, nil
}
