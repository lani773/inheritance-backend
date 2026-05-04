// Package models defines all MongoDB document structures for the Inheritance Choir system.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─── Member ───────────────────────────────────────────────────

// Member represents a choir member stored in MongoDB.
type Member struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"       json:"id"`
	FullName      string             `bson:"fullName"            json:"fullName"         validate:"required,min=2,max=100"`
	Email         string             `bson:"email"               json:"email"             validate:"required,email"`
	PasswordHash  string             `bson:"passwordHash"        json:"-"`
	Phone         string             `bson:"phone,omitempty"     json:"phone,omitempty"`
	DateOfBirth   string             `bson:"dateOfBirth,omitempty" json:"dateOfBirth,omitempty"`
	Gender        string             `bson:"gender,omitempty"    json:"gender,omitempty"`
	MaritalStatus string             `bson:"maritalStatus,omitempty" json:"maritalStatus,omitempty"`
	Bio           string             `bson:"bio,omitempty"       json:"bio,omitempty"`
	AvatarURL     string             `bson:"avatarUrl,omitempty" json:"avatarUrl,omitempty"`
	VoicePart     string             `bson:"voicePart"           json:"voicePart"         validate:"required,oneof=Soprano Alto Tenor Bass"`
	Role          string             `bson:"role"                json:"role"`
	Status        string             `bson:"status"              json:"status"`
	IsAdmin       bool               `bson:"isAdmin"             json:"isAdmin"`
	Permissions   []string           `bson:"permissions"         json:"permissions"`
	Attendance    float64            `bson:"attendance"          json:"attendance"`
	ContribTotal  float64            `bson:"contributionTotal"   json:"contributionTotal"`
	JoinDate      time.Time          `bson:"joinDate"            json:"joinDate"`
	Online        bool               `bson:"online"              json:"online"`
	LastSeen      *time.Time         `bson:"lastSeen,omitempty"  json:"lastSeen,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt"           json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"           json:"updatedAt"`
}

// ─── Event ────────────────────────────────────────────────────

type Event struct {
	ID           primitive.ObjectID  `bson:"_id,omitempty"        json:"id"`
	Title        string              `bson:"title"                json:"title"       validate:"required"`
	Type         string              `bson:"type"                 json:"type"        validate:"required,oneof=rehearsal performance service meeting workshop special"`
	Date         string              `bson:"date"                 json:"date"        validate:"required"`
	Time         string              `bson:"time"                 json:"time"        validate:"required"`
	EndTime      string              `bson:"endTime,omitempty"    json:"endTime,omitempty"`
	Location     string              `bson:"location"             json:"location"    validate:"required"`
	Description  string              `bson:"description,omitempty" json:"description,omitempty"`
	Mandatory    bool                `bson:"mandatory"            json:"mandatory"`
	TargetVoices []string            `bson:"targetVoices"         json:"targetVoices"`
	Recurrence   string              `bson:"recurrence,omitempty" json:"recurrence,omitempty"`
	CreatedBy    *primitive.ObjectID `bson:"createdBy,omitempty"  json:"createdBy,omitempty"`
	CreatedAt    time.Time           `bson:"createdAt"            json:"createdAt"`
	UpdatedAt    time.Time           `bson:"updatedAt"            json:"updatedAt"`
}

// ─── Contribution ─────────────────────────────────────────────

type Contribution struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"         json:"id"`
	MemberID   string             `bson:"memberId"              json:"memberId"  validate:"required"`
	Type       string             `bson:"type"                  json:"type"      validate:"required,oneof=tithe offering special_gift fundraiser welfare_fund other"`
	Amount     float64            `bson:"amount"                json:"amount"    validate:"required,gt=0"`
	Currency   string             `bson:"currency"              json:"currency"`
	Date       string             `bson:"date"                  json:"date"      validate:"required"`
	Method     string             `bson:"method"                json:"method"    validate:"oneof=cash mobile_money bank_transfer cheque online"`
	Reference  string             `bson:"reference,omitempty"   json:"reference,omitempty"`
	ReceiptNo  string             `bson:"receiptNo"             json:"receiptNo"`
	Notes      string             `bson:"notes,omitempty"       json:"notes,omitempty"`
	Verified   bool               `bson:"verified"              json:"verified"`
	VerifiedBy *string            `bson:"verifiedBy,omitempty"  json:"verifiedBy,omitempty"`
	VerifiedAt *time.Time         `bson:"verifiedAt,omitempty"  json:"verifiedAt,omitempty"`
	CreatedBy  *string            `bson:"createdBy,omitempty"   json:"createdBy,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"             json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"             json:"updatedAt"`
	// Populated via aggregation (not stored)
	MemberName string `bson:"-"                     json:"memberName,omitempty"`
}

// ─── Attendance ───────────────────────────────────────────────

type Attendance struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID  string             `bson:"eventId"       json:"eventId"  validate:"required"`
	MemberID string             `bson:"memberId"      json:"memberId" validate:"required"`
	Status   string             `bson:"status"        json:"status"   validate:"required,oneof=present late excused absent"`
	Date     string             `bson:"date,omitempty" json:"date,omitempty"`
	Notes    string             `bson:"notes,omitempty" json:"notes,omitempty"`
	MarkedBy string             `bson:"markedBy,omitempty" json:"markedBy,omitempty"`
	MarkedAt time.Time          `bson:"markedAt"      json:"markedAt"`
	// Populated
	MemberName string `bson:"-" json:"memberName,omitempty"`
	VoicePart  string `bson:"-" json:"voicePart,omitempty"`
}

type Excuse struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"      json:"id"`
	EventID    string             `bson:"eventId"            json:"eventId"`
	MemberID   string             `bson:"memberId"           json:"memberId"`
	Reason     string             `bson:"reason"             json:"reason"     validate:"required"`
	Status     string             `bson:"status"             json:"status"`
	ReviewedBy string             `bson:"reviewedBy,omitempty" json:"reviewedBy,omitempty"`
	ReviewedAt *time.Time         `bson:"reviewedAt,omitempty" json:"reviewedAt,omitempty"`
	ReviewNote string             `bson:"reviewNote,omitempty" json:"reviewNote,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"          json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"          json:"updatedAt"`
	// Populated
	MemberName string `bson:"-" json:"memberName,omitempty"`
	EventTitle string `bson:"-" json:"eventTitle,omitempty"`
}

// ─── Message ──────────────────────────────────────────────────

type Message struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty"         json:"id"`
	SenderID    string              `bson:"senderId"              json:"senderId"`
	RecipientID string              `bson:"recipientId,omitempty" json:"recipientId,omitempty"`
	ToVoicePart string              `bson:"toVoicePart,omitempty" json:"toVoicePart,omitempty"`
	IsBroadcast bool                `bson:"isBroadcast"           json:"isBroadcast"`
	Subject     string              `bson:"subject"               json:"subject"    validate:"required"`
	Body        string              `bson:"body"                  json:"body"       validate:"required"`
	ReadBy      []string            `bson:"readBy"                json:"readBy"`
	Reactions   map[string][]string `bson:"reactions"             json:"reactions"`
	DeletedBy   []string            `bson:"deletedBy"             json:"deletedBy"`
	SentAt      time.Time           `bson:"sentAt"                json:"sentAt"`
	// Populated
	SenderName string `bson:"-" json:"senderName,omitempty"`
}

// ─── Auth ─────────────────────────────────────────────────────

type OTP struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string             `bson:"email"         json:"email"`
	Code      string             `bson:"code"          json:"-"` // bcrypt hashed
	Purpose   string             `bson:"purpose"       json:"purpose"`
	Attempts  int                `bson:"attempts"      json:"attempts"`
	ExpiresAt time.Time          `bson:"expiresAt"     json:"expiresAt"` // TTL index
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
}

type RefreshToken struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MemberID  string             `bson:"memberId"      json:"memberId"`
	Token     string             `bson:"token"         json:"token"`
	ExpiresAt time.Time          `bson:"expiresAt"     json:"expiresAt"` // TTL index
	UserAgent string             `bson:"userAgent,omitempty" json:"userAgent,omitempty"`
	IPAddress string             `bson:"ipAddress,omitempty" json:"ipAddress,omitempty"`
	Revoked   bool               `bson:"revoked"       json:"revoked"`
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
}

type AuditLog struct {
	ID         primitive.ObjectID     `bson:"_id,omitempty"         json:"id"`
	UserID     string                 `bson:"userId,omitempty"      json:"userId,omitempty"`
	UserEmail  string                 `bson:"userEmail,omitempty"   json:"userEmail,omitempty"`
	Action     string                 `bson:"action"                json:"action"`
	Resource   string                 `bson:"resource,omitempty"    json:"resource,omitempty"`
	ResourceID string                 `bson:"resourceId,omitempty"  json:"resourceId,omitempty"`
	OldValues  map[string]interface{} `bson:"oldValues,omitempty"   json:"oldValues,omitempty"`
	NewValues  map[string]interface{} `bson:"newValues,omitempty"   json:"newValues,omitempty"`
	IPAddress  string                 `bson:"ipAddress,omitempty"   json:"ipAddress,omitempty"`
	UserAgent  string                 `bson:"userAgent,omitempty"   json:"userAgent,omitempty"`
	Timestamp  time.Time              `bson:"timestamp"             json:"timestamp"`
}

// ─── Posts / Songs / Welfare / Notifications / Settings ───────

type Post struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"   json:"id"`
	AuthorID   string             `bson:"authorId"        json:"authorId"`
	Title      string             `bson:"title"           json:"title"   validate:"required"`
	Content    string             `bson:"content"         json:"content" validate:"required"`
	Category   string             `bson:"category"        json:"category"`
	Tags       []string           `bson:"tags"            json:"tags"`
	Pinned     bool               `bson:"pinned"          json:"pinned"`
	ViewCount  int64              `bson:"viewCount"       json:"viewCount"`
	Likes      []string           `bson:"likes"           json:"likes"`
	Comments   []interface{}      `bson:"comments"        json:"comments"`
	CreatedAt  time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"       json:"updatedAt"`
	AuthorName string             `bson:"-"               json:"authorName,omitempty"`
}

type Song struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"         json:"id"`
	Title          string             `bson:"title"                 json:"title"   validate:"required"`
	Artist         string             `bson:"artist,omitempty"      json:"artist,omitempty"`
	Language       string             `bson:"language"              json:"language"`
	Genre          string             `bson:"genre"                 json:"genre"`
	Key            string             `bson:"key,omitempty"         json:"key,omitempty"`
	Tempo          string             `bson:"tempo,omitempty"       json:"tempo,omitempty"`
	Difficulty     string             `bson:"difficulty"            json:"difficulty"`
	Lyrics         string             `bson:"lyrics,omitempty"      json:"lyrics,omitempty"`
	Notes          string             `bson:"notes,omitempty"       json:"notes,omitempty"`
	AudioURL       string             `bson:"audioUrl,omitempty"    json:"audioUrl,omitempty"`
	SheetMusicURL  string             `bson:"sheetMusicUrl,omitempty" json:"sheetMusicUrl,omitempty"`
	Tags           []string           `bson:"tags"                  json:"tags"`
	RehearsalCount int                `bson:"rehearsalCount"        json:"rehearsalCount"`
	AddedBy        string             `bson:"addedBy,omitempty"     json:"addedBy,omitempty"`
	CreatedAt      time.Time          `bson:"createdAt"             json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt"             json:"updatedAt"`
}

type WelfareCase struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"         json:"id"`
	MemberID     string             `bson:"memberId"              json:"memberId"   validate:"required"`
	Title        string             `bson:"title"                 json:"title"      validate:"required"`
	Description  string             `bson:"description"           json:"description" validate:"required"`
	Category     string             `bson:"category"              json:"category"`
	Priority     string             `bson:"priority"              json:"priority"`
	Status       string             `bson:"status"                json:"status"`
	AmountNeeded float64            `bson:"amountNeeded"          json:"amountNeeded"`
	AmountRaised float64            `bson:"amountRaised"          json:"amountRaised"`
	Timeline     []interface{}      `bson:"timeline"              json:"timeline"`
	AssignedTo   string             `bson:"assignedTo,omitempty"  json:"assignedTo,omitempty"`
	CreatedBy    string             `bson:"createdBy,omitempty"   json:"createdBy,omitempty"`
	ResolvedAt   *time.Time         `bson:"resolvedAt,omitempty"  json:"resolvedAt,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt"             json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"             json:"updatedAt"`
	MemberName   string             `bson:"-"                     json:"memberName,omitempty"`
}

type Notification struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"          json:"id"`
	RecipientID string             `bson:"recipientId,omitempty"  json:"recipientId,omitempty"`
	Type        string             `bson:"type"                   json:"type"`
	Title       string             `bson:"title"                  json:"title"`
	Message     string             `bson:"message"                json:"message"`
	IsRead      bool               `bson:"isRead"                 json:"isRead"`
	ActionURL   string             `bson:"actionUrl,omitempty"    json:"actionUrl,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt"              json:"createdAt"`
}

type AppSettings struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty"          json:"id"`
	Key                string             `bson:"key"                    json:"key"`
	ChoirName          string             `bson:"choirName"              json:"choirName"`
	Tagline            string             `bson:"tagline"                json:"tagline"`
	ContactEmail       string             `bson:"contactEmail"           json:"contactEmail"`
	Currency           string             `bson:"currency"               json:"currency"`
	Timezone           string             `bson:"timezone"               json:"timezone"`
	AttendanceTarget   float64            `bson:"attendanceTarget"       json:"attendanceTarget"`
	TitheSuggestion    float64            `bson:"titheSuggestion"        json:"titheSuggestion"`
	EmailNotifications bool               `bson:"emailNotifications"     json:"emailNotifications"`
	MaintenanceMode    bool               `bson:"maintenanceMode"        json:"maintenanceMode"`
	UpdatedAt          time.Time          `bson:"updatedAt"              json:"updatedAt"`
}

// ─── AutomationRule ───────────────────────────────────────────

type AutomationRule struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty"  json:"id"`
	Name         string                 `bson:"name"           json:"name"`
	Description  string                 `bson:"description"    json:"description"`
	Category     string                 `bson:"category"       json:"category"`
	Active       bool                   `bson:"active"         json:"active"`
	Trigger      string                 `bson:"trigger"        json:"trigger"`
	Conditions   map[string]interface{} `bson:"conditions"     json:"conditions"`
	Action       string                 `bson:"action"         json:"action"`
	ActionConfig map[string]interface{} `bson:"actionConfig"   json:"actionConfig"`
	RunCount     int64                  `bson:"runCount"       json:"runCount"`
	LastRun      *time.Time             `bson:"lastRun,omitempty" json:"lastRun,omitempty"`
	CreatedAt    time.Time              `bson:"createdAt"      json:"createdAt"`
	UpdatedAt    time.Time              `bson:"updatedAt"      json:"updatedAt"`
}

type SetlistItem struct {
	SongID   string `bson:"songId" json:"songId"`
	Title    string `bson:"title" json:"title"`
	Key      string `bson:"key,omitempty" json:"key,omitempty"`
	Order    int    `bson:"order" json:"order"`
	Duration string `bson:"duration,omitempty" json:"duration,omitempty"`
	Note     string `bson:"note,omitempty" json:"note,omitempty"`
}

type Setlist struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID   string             `bson:"eventId,omitempty" json:"eventId,omitempty"`
	Title     string             `bson:"title" json:"title"`
	Items     []SetlistItem      `bson:"items" json:"items"`
	Notes     string             `bson:"notes,omitempty" json:"notes,omitempty"`
	CreatedBy string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type BudgetGoal struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Year      int                `bson:"year" json:"year"`
	Type      string             `bson:"type" json:"type"`
	Target    float64            `bson:"target" json:"target"`
	CreatedBy string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Pledge struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CampaignID string             `bson:"campaignId" json:"campaignId"`
	MemberID   string             `bson:"memberId" json:"memberId"`
	Amount     float64            `bson:"amount" json:"amount"`
	Paid       float64            `bson:"paid" json:"paid"`
	DueDate    string             `bson:"dueDate,omitempty" json:"dueDate,omitempty"`
	Status     string             `bson:"status" json:"status"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
	MemberName string             `bson:"-" json:"memberName,omitempty"`
}

type PledgeCampaign struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Target      float64            `bson:"target" json:"target"`
	DueDate     string             `bson:"dueDate,omitempty" json:"dueDate,omitempty"`
	Status      string             `bson:"status" json:"status"`
	CreatedBy   string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type APIKey struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	KeyHash     string             `bson:"keyHash" json:"-"`
	Prefix      string             `bson:"prefix" json:"prefix"`
	Permissions []string           `bson:"permissions" json:"permissions"`
	Status      string             `bson:"status" json:"status"`
	LastUsedAt  *time.Time         `bson:"lastUsedAt,omitempty" json:"lastUsedAt,omitempty"`
	CreatedBy   string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	RevokedAt   *time.Time         `bson:"revokedAt,omitempty" json:"revokedAt,omitempty"`
}

type Webhook struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	URL          string             `bson:"url" json:"url"`
	Events       []string           `bson:"events" json:"events"`
	Secret       string             `bson:"secret,omitempty" json:"-"`
	Active       bool               `bson:"active" json:"active"`
	FailureCount int                `bson:"failureCount" json:"failureCount"`
	LastSentAt   *time.Time         `bson:"lastSentAt,omitempty" json:"lastSentAt,omitempty"`
	CreatedBy    string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type UploadAsset struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	FileName    string             `bson:"fileName" json:"fileName"`
	ContentType string             `bson:"contentType" json:"contentType"`
	Size        int64              `bson:"size" json:"size"`
	URL         string             `bson:"url" json:"url"`
	OwnerID     string             `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	Scope       string             `bson:"scope,omitempty" json:"scope,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
}

type PrayerRequest struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MemberID    string             `bson:"memberId" json:"memberId"`
	Title       string             `bson:"title" json:"title"`
	Description string             `bson:"description" json:"description"`
	Visibility  string             `bson:"visibility" json:"visibility"`
	Status      string             `bson:"status" json:"status"`
	PrayedBy    []string           `bson:"prayedBy" json:"prayedBy"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
	MemberName  string             `bson:"-" json:"memberName,omitempty"`
}

type ChatMessage struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	ChannelID   string              `bson:"channelId" json:"channelId"`
	SenderID    string              `bson:"senderId" json:"senderId"`
	Text        string              `bson:"text" json:"text"`
	Attachments []string            `bson:"attachments" json:"attachments"`
	ReadBy      []string            `bson:"readBy" json:"readBy"`
	Reactions   map[string][]string `bson:"reactions" json:"reactions"`
	CreatedAt   time.Time           `bson:"createdAt" json:"createdAt"`
	SenderName  string              `bson:"-" json:"senderName,omitempty"`
}
