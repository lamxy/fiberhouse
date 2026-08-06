// Package constant 定義 hertz 核心擴展所用的識別常數。
//
// 框架的 BootConfig.CoreType 為普通字串而非列舉，因此外部擴展可自行約定新的核心識別，
// 這是 fiberhouse 支援核心引擎可插拔的關鍵設計。
package constant

const (
	// CoreTypeWithHertz hertz 核心引擎的類型識別，對應 BootConfig.CoreType
	// 及各 Provider 的 Target() 值
	CoreTypeWithHertz = "hertz"

	// StarterName hertz 核心啟動器在日誌中的識別名
	StarterName = "HertzApplication"
)
