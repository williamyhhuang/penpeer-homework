package usecase

// CodeBloom 定義 Bloom Filter 介面（Hexagonal Port）
// 用於在查詢 Redis / DB 之前快速篩掉肯定不存在的短碼，防止快取穿透攻擊
type CodeBloom interface {
	// MightExist 回傳 false 代表此 code 肯定不存在，可直接拒絕
	MightExist(code string) bool
	// Add 將新建立的短碼加入 filter
	Add(code string)
}
