package commonvo

import "time"

// Timestamps 公共时间戳字段
type Timestamps struct {
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}
