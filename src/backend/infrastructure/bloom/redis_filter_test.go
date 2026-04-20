package bloom

import (
	"math"
	"testing"
)

func TestBloomParams(t *testing.T) {
	m, k := bloomParams(1_000_000, 0.01)

	// m 應約等於 9_585_059，允許浮點誤差
	if m < 9_000_000 || m > 10_000_000 {
		t.Errorf("bloomParams m=%d，超出預期範圍 [9_000_000, 10_000_000]", m)
	}
	// k 最優值 ln(2) * m/n ≈ 6.64 → 7
	if k != 7 {
		t.Errorf("bloomParams k=%d，期望 7", k)
	}
}

func TestBloomParamsSmall(t *testing.T) {
	m, k := bloomParams(100, 0.01)
	if m == 0 {
		t.Error("m 不應為 0")
	}
	if k < 1 {
		t.Error("k 至少為 1")
	}
}

func TestHashOffsets_Deterministic(t *testing.T) {
	f := &RedisBloomFilter{m: 1_000_000, k: 7}
	offsets1 := f.hashOffsets("abc123")
	offsets2 := f.hashOffsets("abc123")

	if len(offsets1) != 7 {
		t.Fatalf("期望 7 個 offset，got %d", len(offsets1))
	}
	for i := range offsets1 {
		if offsets1[i] != offsets2[i] {
			t.Errorf("offset[%d] 不一致：%d vs %d", i, offsets1[i], offsets2[i])
		}
	}
}

func TestHashOffsets_WithinBounds(t *testing.T) {
	m := uint64(9_585_059)
	f := &RedisBloomFilter{m: m, k: 7}
	codes := []string{"abc", "xyz", "8Qu3wxx", "hello-world", ""}
	for _, code := range codes {
		for i, offset := range f.hashOffsets(code) {
			if offset < 0 || uint64(offset) >= m {
				t.Errorf("code=%q offset[%d]=%d 超出 [0, %d)", code, i, offset, m)
			}
		}
	}
}

func TestHashOffsets_Distinct(t *testing.T) {
	f := &RedisBloomFilter{m: 9_585_059, k: 7}
	offsets := f.hashOffsets("test_code")
	seen := map[int64]bool{}
	// 7 個 offset 不一定全部不同（理論上可能碰撞），但碰撞率應極低
	// 此測試只驗證沒有全部相同的極端情況
	for _, o := range offsets {
		seen[o] = true
	}
	if len(seen) < 2 {
		t.Errorf("7 個 offset 幾乎全部相同，hash 分布可能有問題：%v", offsets)
	}
}

func TestHashOffsets_DifferentCodes(t *testing.T) {
	f := &RedisBloomFilter{m: 9_585_059, k: 7}
	o1 := f.hashOffsets("code_A")
	o2 := f.hashOffsets("code_B")

	// 不同 code 的 offset 集合應不相同
	allSame := true
	for i := range o1 {
		if o1[i] != o2[i] {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("不同 code 產生了完全相同的 offset，hash 碰撞")
	}
}

func TestBloomParams_FPRateApprox(t *testing.T) {
	// 驗證計算出的 m, k 組合理論誤判率接近目標 fpRate
	capacity := uint(1_000_000)
	targetFP := 0.01
	m, k := bloomParams(capacity, targetFP)

	// 理論 FP rate = (1 - e^(-k*n/m))^k
	n := float64(capacity)
	actualFP := math.Pow(1-math.Exp(-float64(k)*n/float64(m)), float64(k))

	// 允許 ±0.5% 誤差
	if math.Abs(actualFP-targetFP) > 0.005 {
		t.Errorf("理論誤判率 %.4f 偏離目標 %.4f", actualFP, targetFP)
	}
}
