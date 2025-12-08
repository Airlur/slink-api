package generator

import (
	"short-link/internal/pkg/logger"
	"short-link/internal/pkg/generator/base62"
	"short-link/internal/pkg/generator/permutation"
)

// Generate 是生成短码的核心函数。
// 它接收一个数据库自增的 uint64 ID，
// 经过置换和编码后，返回一个唯一的、非连续的、URL友好的短码字符串。
//
// 流程:
// 1. 对原始 ID 进行置换 (Permutation)，打乱其顺序，避免短码被顺序猜测。
//    例如: ID=100 -> PermutedID=987654321
// 2. 对置换后的 ID 进行 Base62 编码，将其转换为短的、由数字和大小写字母组成的字符串。
//    例如: PermutedID=987654321 -> ShortCode="abc123Z"


// idMask 掩码，用于限制ID的有效范围，确保生成的Base62码长度稳定在8位以内。
// 0x7FFFFFFFFFFF 是一个47位的掩码，其最大值小于 62^8，可以保证Base62编码长度不超过8位。提供 140万亿 的ID容量。
const idMask uint64 = 0x7FFFFFFFFFFF // 47位掩码

func Generate(id uint64) string {
	// 步骤1: 先对原始 ID 和掩码进行与操作，限制其最大值和短码长度
	maskedID := id & idMask 
	
	// 步骤2: 再对截取后的 ID 进行置换混淆
	permutedID := permutation.Permute(maskedID)

	logger.Debug("ID生成过程", 
		"originalID", id, 
		"permutedID", permutedID, 
		"maskedID", maskedID,
	)
	
	// 步骤3: 对掩码处理后的 ID 进行 Base62 编码
	shortCode := base62.Encode(permutedID)

	logger.Debug("短码生成结果", 
		"shortCode", shortCode, 
		"length", len(shortCode),
	)
	return shortCode
}

// Reverse 是 Generate 的逆向操作。
// 注意：由于Generate中使用了有损的掩码操作，此函数无法反解出原始ID。
// 它的主要用途是在调试时，从短码反解出被掩码和置换后的ID，以供分析。
func Reverse(shortCode string) (uint64, error) {
	// 步骤 1: Base62解码
	maskedPermutedID, err := base62.Decode(shortCode)
	if err != nil {
		return 0, err
	}

	// 步骤 2: 反向置换
	// 得到的是被掩码之前的原始ID，而不是最最原始的ID
	// 注意：因为 Generate 阶段是用 permutedID 进行 Encode 的，
	// 所以这里 Decode 出来的就是 permutedID，直接 Unpermute 即可。
	originalID := permutation.Unpermute(maskedPermutedID)

	return originalID, nil
}
