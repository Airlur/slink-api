package generator

import (
	"math/bits"

	"short-link/internal/pkg/generator/base62"
	"short-link/internal/pkg/logger"
)

const (
	idMask       uint64 = 0x7FFFFFFFFFFF // 47 位，可保证 Base62 长度 ≤ 8
	minCodeValue uint64 = 916132832      // 62^5，保证至少 6 位
	rangeSize    uint64 = idMask + 1 - minCodeValue
)

// Generate 根据数据库自增 ID 生成 6~8 位的短码。
func Generate(id uint64) string {
	maskedID := id & idMask
	mixedID := mixID(maskedID)
	codeValue := minCodeValue + (mixedID % rangeSize)

	logger.Debug("ID生成过程",
		"originalID", id,
		"mixedID", mixedID,
		"codeValue", codeValue,
		"maskedID", maskedID,
	)

	shortCode := base62.Encode(codeValue)

	logger.Debug("短码生成结果",
		"shortCode", shortCode,
		"length", len(shortCode),
	)
	return shortCode
}

// Reverse 仅用于调试：Base62 解码后返回混排 ID。
func Reverse(shortCode string) (uint64, error) {
	return base62.Decode(shortCode)
}

// mixID 在 47-bit 空间内执行可逆混排，既保证长度又增强离散性。
func mixID(id uint64) uint64 {
	const (
		mulA uint64 = 0x5bd1e995
		addA uint64 = 0x41c64e6d
		mulB uint64 = 0x27d4eb2d
		addB uint64 = 0x9e3779b1
	)

	x := id & idMask
	x = rotateLeft47(x, 17)
	x = (x*mulA + addA) & idMask
	x ^= x >> 23
	x = (x*mulB + addB) & idMask

	if x < minCodeValue {
		x += minCodeValue
	}
	return x
}

func rotateLeft47(x uint64, k uint) uint64 {
	k %= 47
	return bits.RotateLeft64(x, int(k)) & idMask
}
