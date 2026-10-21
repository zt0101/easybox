package model

// EasyboxSubscription pins an EasyBox subscription user. Upstream drops the
// ClientRecord with its last inbound; this row keeps the SubID (URL) stable.
type EasyboxSubscription struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string `json:"email" gorm:"uniqueIndex;not null"`
	SubID     string `json:"subId" gorm:"uniqueIndex;not null;column:sub_id"`
	UUID      string `json:"uuid" gorm:"not null;column:uuid"`
	Auth      string `json:"auth" gorm:"column:auth"`
	Password  string `json:"password" gorm:"column:password"`
	IsDefault bool   `json:"isDefault" gorm:"index;not null;default:false"`
	CreatedAt int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
}

func (EasyboxSubscription) TableName() string { return "easybox_subscriptions" }

// EasyboxEndpoint is an EasyBox line: product state over one upstream Inbound,
// which stays the source of truth for port, protocol and traffic.
type EasyboxEndpoint struct {
	Id             int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name           string `json:"name" gorm:"not null"`
	Template       string `json:"template" gorm:"not null"`
	InboundId      int    `json:"inboundId" gorm:"index"`
	SubscriptionId int    `json:"subscriptionId" gorm:"index"`
	Status         string `json:"status" gorm:"not null"`
	FailedStep     string `json:"failedStep"`
	LastError      string `json:"lastError"`
	CreatedAt      int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt      int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (EasyboxEndpoint) TableName() string { return "easybox_endpoints" }

// EasyboxAuditLog records EasyBox actions (EasyBox.md §53); never credentials.
type EasyboxAuditLog struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Timestamp int64  `json:"timestamp" gorm:"index;not null"`
	Action    string `json:"action" gorm:"index;not null"`
	Object    string `json:"object" gorm:"not null"`
	ObjectId  int    `json:"objectId"`
	Result    string `json:"result" gorm:"not null"`
	Error     string `json:"error"`
}

func (EasyboxAuditLog) TableName() string { return "easybox_audit_logs" }
