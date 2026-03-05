package generator_test

import (
	"fmt"
	"os"
	"slink-api/internal/pkg/config"
	"slink-api/internal/pkg/generator"
	"slink-api/internal/pkg/logger"
	"testing"
)

// TestMain 在所有测试用例执行前初始化环境
func TestMain(m *testing.M) {
	// 初始化日志为"无操作"模式（测试环境禁用日志输出，避免干扰）
	logger.InitLogger(&config.GlobalConfig.Logger) // 假设你的日志包有 InitLogger 方法接受 zap.Logger

	// 运行所有测试
	os.Exit(m.Run())
}

func TestGenerate_LengthAndUniqueness(t *testing.T) {
	tests := []uint64{1, 10, 100, 1000000, 1234567890, 0xFFFFFFFFFFFF}

	// 使用 map 检查唯一性
	shortCodeMap := make(map[string]uint64)

	for _, id := range tests {
		t.Run(fmt.Sprintf("ID=%d", id), func(t *testing.T) {
			gotShortCode := generator.Generate(id)

			// 验证1：短码非空
			if gotShortCode == "" {
				t.Fatalf("Generate(%d) 生成了空短码", id)
			}

			// 验证2：短码长度合规 (<= 8)
			const maxLen = 8
			if len(gotShortCode) > maxLen {
				t.Errorf("Generate(%d) 短码长度为 %d，超过了最大长度 %d",
					id, len(gotShortCode), maxLen)
			}
			t.Logf("ID[ %d ] --> Code[ %s (len %d) ]", id, gotShortCode, len(gotShortCode))

			// 验证3：短码唯一性
			if existID, ok := shortCodeMap[gotShortCode]; ok {
				t.Fatalf("短码重复：短码=%s 同时对应 ID=%d 和 ID=%d", gotShortCode, existID, id)
			}
			shortCodeMap[gotShortCode] = id
		})
	}
}
