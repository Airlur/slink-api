package permutation

// Permute 使用简化的 Feistel 网络结构对 uint64 ID 进行可逆的置换（混淆）。
// Feistel 网络是一种对称加密结构，保证了操作的可逆性。
//
// 我们的目标不是真正的加密，而是打乱ID的顺序，使其生成的短码看起来无规律、不可预测。
// 例如，避免出现 id=100 -> code="1c", id=101 -> code="1d" 这种情况。
//
// 这个版本将核心逻辑完全建立在 uint32 类型上，更加清晰和高效。
func Permute(id uint64) uint64 {
	// 1. 【入口转换】: 在函数开始时，将 uint64 拆分并转换为两个 uint32 数据块。
	// 这是最清晰的做法，将类型转换严格限制在函数边界。
	left := uint32(id >> 32)        // 取原始ID的高32位
	right := uint32(id & 0xFFFFFFFF) // 取原始ID的低32位

	// 2. 【核心逻辑】: 进行多轮迭代，增加混淆度。偶数轮次即可，4轮是比较常见的选择。
	// 在 uint32 类型上执行所有轮次的操作。这个循环内部不再有任何类型转换，可读性高且高效。
	for i := 0; i < 4; i++ {
		// roundFunction 是一个 F 函数，它是一个非线性的函数，是混淆的关键。
		fOut := roundFunction(right, uint32(i))
		// 将 F 函数的输出与左半部分进行异或操作，生成新的右半部分。
		newRight := left ^ fOut

		// 交换左右两部分，为下一轮做准备。
		left = right
		right = newRight
	}

	// 3. 【出口转换】: 在函数返回前，将处理完的两个 uint32 数据块合并回一个 uint64。
	// 因为最后一轮也进行了交换，所以这里组合的顺序是 right 在前，left 在后，
	// 这与 Unpermute 的结构完全对称，保证了可逆性。
	return (uint64(right) << 32) | uint64(left)
}

// Unpermute 是 Permute 的逆操作，用于从混淆后的ID恢复原始ID。
// Feistel 网络的优美之处在于解密过程和加密过程的算法结构完全相同，只是轮密钥的使用顺序相反。
// 同样遵循入口和出口转换的原则。
func Unpermute(permutedID uint64) uint64 {
	// 1. 【入口转换】: 与 Permute 完全相同。
	left := uint32(permutedID >> 32)
	right := uint32(permutedID & 0xFFFFFFFF)

	// 2. 【核心逻辑】: 轮次反向，从最后一轮的密钥开始，这正是Feistel网络可逆的关键。
	// 操作本身与 Permute 完全一致。
	for i := 3; i >= 0; i-- {
		fOut := roundFunction(right, uint32(i))
		newRight := left ^ fOut

		left = right
		right = newRight
	}

	// 3. 【出口转换】: 合并的顺序也与 Permute 完全相同。
	return (uint64(right) << 32) | uint64(left)
}

// roundFunction 是 Feistel 网络中的 F 函数。
// 它的设计目标是非线性，并产生雪崩效应 (avalanche effect)，
// 即输入的微小变化能导致输出的巨大变化。
func roundFunction(input uint32, roundIndex uint32) uint32 {
	// const 定义了一些“魔法常数”，它们通常选择无理数的二进制表示以增加随机性。
	// 这里的 0x9E3779B9 来自黄金比例，是密码学中常用的一个常数。
	const (
		keyA uint32 = 0x9E3779B9
		keyB uint32 = 0x61C88647
	)
	
	// 步骤1: 将输入与轮次索引和密钥混合，确保每一轮的操作都不同。
	mixed := input + (keyA * (roundIndex + 1))
	
	// 步骤2: 执行简单的非线性操作（XOR 和循环移位），这是混淆的核心。
	// 循环移位可以保证信息不丢失，而异或提供了非线性。
	mixed = (mixed << 7) | (mixed >> (32 - 7)) // 循环左移7位
	mixed = mixed ^ keyB
	
	return mixed
}
