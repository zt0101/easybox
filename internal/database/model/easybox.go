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
	// A CDN line also owns a proxied DNS record and an Origin CA certificate at
	// Cloudflare; deleting the line removes both.
	CdnZoneID   string `json:"-" gorm:"column:cdn_zone_id"`
	CdnHost     string `json:"cdnHost" gorm:"column:cdn_host"`
	CdnRecordID string `json:"-" gorm:"column:cdn_record_id"`
	CdnCertID   string `json:"-" gorm:"column:cdn_cert_id"`
	CreatedAt   int64  `json:"createdAt" gorm:"autoCreateTime:milli"`
	UpdatedAt   int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (EasyboxEndpoint) TableName() string { return "easybox_endpoints" }

// EasyboxCloudflare is the Cloudflare account CDN lines use (one row). The API
// token is a credential: no API response or log carries it.
type EasyboxCloudflare struct {
	Id       int    `json:"-" gorm:"primaryKey"`
	Domain   string `json:"domain" gorm:"not null"`
	ZoneID   string `json:"-" gorm:"column:zone_id;not null"`
	APIToken string `json:"-" gorm:"column:api_token;not null"`
	// PreferredAddresses held the CDN preferred addresses in eb.10-eb.12; they
	// moved to easybox_settings (key cdnPreferredAddresses) and this column is
	// only read once to migrate them.
	PreferredAddresses string `json:"preferredAddresses" gorm:"column:preferred_addresses;type:text"`
	UpdatedAt          int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (EasyboxCloudflare) TableName() string { return "easybox_cloudflare" }

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

// EasyboxSetting is one EasyBox-wide setting that belongs to no other row
// (e.g. the CDN preferred addresses, used with or without a Cloudflare token).
type EasyboxSetting struct {
	Key       string `json:"key" gorm:"primaryKey"`
	Value     string `json:"value" gorm:"type:text"`
	UpdatedAt int64  `json:"updatedAt" gorm:"autoUpdateTime:milli"`
}

func (EasyboxSetting) TableName() string { return "easybox_settings" }
