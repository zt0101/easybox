package model

// EasyboxSubscription pins an EasyBox subscription user. Upstream drops the
// ClientRecord with its last inbound; this row keeps the SubID (URL) stable.
type EasyboxSubscription struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string `json:"email" gorm:"uniqueIndex;not null"`
	SubID     string `json:"subId" gorm:"uniqueIndex;not null;column:sub_id"`
	UUID      string `json:"uuid" gorm:"not null;column:uuid"`
	IsDefault bool   `json:"isDefault" gorm:"index;not null;default:false"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
}

func (EasyboxSubscription) TableName() string { return "easybox_subscriptions" }
