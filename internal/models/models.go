package models
import "go.mongodb.org/mongo-driver/v2/bson"

import (
	"time"

)

type Member struct {
	ID                bson.ObjectID `bson:"_id,omitempty"          json:"id"`
	FullName          string             `bson:"fullName"              json:"fullName"`
	Email             string             `bson:"email"                 json:"email"`
	PasswordHash      string             `bson:"passwordHash"          json:"-"`
	Phone             string             `bson:"phone,omitempty"       json:"phone,omitempty"`
	DateOfBirth       string             `bson:"dateOfBirth,omitempty"  json:"dateOfBirth,omitempty"`
	Gender            string             `bson:"gender,omitempty"       json:"gender,omitempty"`
	MaritalStatus     string             `bson:"maritalStatus,omitempty" json:"maritalStatus,omitempty"`
	VoicePart         string             `bson:"voicePart"             json:"voicePart"`
	Role              string             `bson:"role"                  json:"role"`
	Status            string             `bson:"status"                json:"status"`
	IsAdmin           bool               `bson:"isAdmin"               json:"isAdmin"`
	Permissions       []string           `bson:"permissions"           json:"permissions"`
	Bio               string             `bson:"bio,omitempty"         json:"bio,omitempty"`
	AvatarURL         string             `bson:"avatarUrl,omitempty"    json:"avatarUrl,omitempty"`
	Attendance        float64            `bson:"attendance"            json:"attendance"`
	ContributionTotal float64            `bson:"contributionTotal"     json:"contributionTotal"`
	JoinDate          time.Time          `bson:"joinDate"              json:"joinDate"`
	LastSeen          time.Time          `bson:"lastSeen"              json:"lastSeen"`
	Online            bool               `bson:"online"                 json:"online"`
	CreatedAt         time.Time          `bson:"createdAt"             json:"createdAt"`
	UpdatedAt         time.Time          `bson:"updatedAt"             json:"updatedAt"`
}

type Event struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title"           json:"title"`
	Description string             `bson:"description"     json:"description"`
	Type        string             `bson:"type"            json:"type"`
	Date        string             `bson:"date"            json:"date"`
	Time        string             `bson:"time"            json:"time"`
	EndTime     string             `bson:"endTime"          json:"endTime"`
	Location    string             `bson:"location"        json:"location"`
	TargetVoices []string           `bson:"targetVoices,omitempty" json:"targetVoices,omitempty"`
	CreatedBy    string             `bson:"createdBy,omitempty"    json:"createdBy,omitempty"`
	IsMandatory bool               `bson:"isMandatory"     json:"isMandatory"`
	CreatedAt   time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"       json:"updatedAt"`
}

type Contribution struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	MemberID   string             `bson:"memberId"         json:"memberId"`
	Amount     float64            `bson:"amount"           json:"amount"`
	Type       string             `bson:"type"             json:"type"`
	Date       string             `bson:"date"             json:"date"`
	Method     string             `bson:"method"           json:"method"`
	ReceiptNo  string             `bson:"receiptNo"        json:"receiptNo"`
	Verified   bool               `bson:"verified"         json:"verified"`
	VerifiedBy *string            `bson:"verifiedBy,omitempty" json:"verifiedBy,omitempty"`
	VerifiedAt time.Time          `bson:"verifiedAt"       json:"verifiedAt"`
	CreatedBy  *string            `bson:"createdBy,omitempty"  json:"createdBy,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt"        json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"        json:"updatedAt"`
}

type Attendance struct {
	ID       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID  string             `bson:"eventId"         json:"eventId"`
	MemberID string             `bson:"memberId"        json:"memberId"`
	Status   string             `bson:"status"          json:"status"`
	Date     string             `bson:"date"            json:"date"`
	MarkedBy string             `bson:"markedBy"        json:"markedBy"`
	MarkedAt time.Time          `bson:"markedAt"        json:"markedAt"`
	MemberName string `bson:"-" json:"memberName,omitempty"`
	VoicePart  string `bson:"-" json:"voicePart,omitempty"`
}

type Excuse struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID    string             `bson:"eventId"         json:"eventId"`
	MemberID   string             `bson:"memberId"        json:"memberId"`
	Reason     string             `bson:"reason"          json:"reason"`
	Status     string             `bson:"status"          json:"status"`
	ReviewedBy string             `bson:"reviewedBy"      json:"reviewedBy"`
	ReviewedAt time.Time          `bson:"reviewedAt"      json:"reviewedAt"`
	ReviewNote string             `bson:"reviewNote"      json:"reviewNote"`
	CreatedAt  time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt"       json:"updatedAt"`
}

type Message struct {
	ID          bson.ObjectID       `bson:"_id,omitempty" json:"id"`
	SenderID    string                   `bson:"senderId"        json:"senderId"`
	RecipientID string                   `bson:"recipientId"     json:"recipientId,omitempty"`
	IsBroadcast bool                     `bson:"isBroadcast"     json:"isBroadcast"`
	ToVoicePart string                   `bson:"toVoicePart"     json:"toVoicePart,omitempty"`
	Subject     string                   `bson:"subject"         json:"subject"`
	Body        string                   `bson:"body"            json:"body"`
	ReadBy      []string                 `bson:"readBy"          json:"readBy"`
	Reactions   map[string][]string      `bson:"reactions"       json:"reactions"`
	DeletedBy   []string                 `bson:"deletedBy"       json:"deletedBy"`
	SentAt      time.Time                `bson:"sentAt"          json:"sentAt"`
}

type WelfareCase struct {
	ID           bson.ObjectID `bson:"_id,omitempty"          json:"id"`
	MemberID     string             `bson:"memberId"              json:"memberId"`
	Category     string             `bson:"category"              json:"category"`
	Description  string             `bson:"description"           json:"description"`
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

type Post struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	AuthorID  string             `bson:"authorId"        json:"authorId"`
	Title     string             `bson:"title"           json:"title"`
	Content   string             `bson:"content"         json:"content"`
	Tags      []string           `bson:"tags"            json:"tags"`
	Likes     []string           `bson:"likes"           json:"likes"`
	Comments  []interface{}      `bson:"comments"        json:"comments"`
	CreatedAt time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"       json:"updatedAt"`
}

type Song struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string             `bson:"title"           json:"title"`
	Artist      string             `bson:"artist"          json:"artist"`
	Genre       string             `bson:"genre"           json:"genre"`
	Language    string             `bson:"language"        json:"language"`
	Lyrics      string             `bson:"lyrics"          json:"lyrics"`
	FileUrl     string             `bson:"fileUrl"         json:"fileUrl,omitempty"`
	Difficulty  string             `bson:"difficulty"      json:"difficulty"`
	AddedBy     string             `bson:"addedBy"         json:"addedBy"`
	CreatedAt   time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"       json:"updatedAt"`
}

type Notification struct {
	ID          bson.ObjectID `bson:"_id,omitempty"          json:"id"`
	RecipientID string             `bson:"recipientId,omitempty"  json:"recipientId,omitempty"`
	Type        string             `bson:"type"                   json:"type"`
	Title       string             `bson:"title"                  json:"title"`
	Message     string             `bson:"message"                json:"message"`
	IsRead      bool               `bson:"isRead"                 json:"isRead"`
	ActionURL   string             `bson:"actionUrl,omitempty"    json:"actionUrl,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt"              json:"createdAt"`
}

type AppSettings struct {
	ID                 bson.ObjectID `bson:"_id,omitempty"          json:"id"`
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

type AutomationRule struct {
	ID           bson.ObjectID     `bson:"_id,omitempty"  json:"id"`
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

type RefreshToken struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Token     string             `bson:"token"           json:"token"`
	MemberID  string             `bson:"memberId"        json:"memberId"`
	UserAgent string             `bson:"userAgent"       json:"userAgent"`
	IPAddress string             `bson:"ipAddress"       json:"ipAddress"`
	ExpiresAt time.Time          `bson:"expiresAt"       json:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"       json:"createdAt"`
}

type OTP struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string             `bson:"email"           json:"email"`
	Code      string             `bson:"code"            json:"code"`
	Purpose   string             `bson:"purpose"         json:"purpose"`
	Attempts  int                `bson:"attempts"        json:"attempts"`
	ExpiresAt time.Time          `bson:"expiresAt"       json:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt"       json:"createdAt"`
}

type AuditLog struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"userId"          json:"userId"`
	UserEmail string             `bson:"userEmail"       json:"userEmail"`
	Action    string             `bson:"action"          json:"action"`
	Resource  string             `bson:"resource"        json:"resource"`
	Details   string             `bson:"details"         json:"details"`
	IPAddress string             `bson:"ipAddress"       json:"ipAddress"`
	UserAgent string             `bson:"userAgent"       json:"userAgent"`
	Timestamp time.Time          `bson:"timestamp"       json:"timestamp"`
	NewValues map[string]interface{} `bson:"newValues,omitempty"   json:"newValues,omitempty"`
}

type Setlist struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID   string             `bson:"eventId,omitempty" json:"eventId,omitempty"`
	Title     string             `bson:"title" json:"title"`
	Items     []interface{}      `bson:"items" json:"items"`
	Notes     string             `bson:"notes,omitempty" json:"notes,omitempty"`
	CreatedBy string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type BudgetGoal struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Year      int                `bson:"year" json:"year"`
	Type      string             `bson:"type" json:"type"`
	Target    float64            `bson:"target" json:"target"`
	CreatedBy string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type Pledge struct {
	ID         bson.ObjectID `bson:"_id,omitempty" json:"id"`
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
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
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
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
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
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name" json:"name"`
	URL          string             `bson:"url" json:"url"`
	Events       []string           `bson:"events" json:"events"`
	Secret       string             `bson:"secret,omitempty" json:"-"`
	Active       bool               `bson:"active" json:"active"`
	FailureCount int                `bson:"failureCount" json:"failureCount"`
	LastSentAt   *time.Time         `bson:"lastSentAt,omitempty" json:"lastSentAt,omitempty"`
	CreatedBy    string             `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt"       json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt"       json:"updatedAt"`
}

type UploadAsset struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	FileName    string             `bson:"fileName" json:"fileName"`
	ContentType string             `bson:"contentType" json:"contentType"`
	Size        int64              `bson:"size" json:"size"`
	URL         string             `bson:"url" json:"url"`
	OwnerID     string             `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	Scope       string             `bson:"scope,omitempty" json:"scope,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
}

type PrayerRequest struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
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
	ID          bson.ObjectID       `bson:"_id,omitempty" json:"id"`
	ChannelID   string                   `bson:"channelId" json:"channelId"`
	SenderID    string                   `bson:"senderId" json:"senderId"`
	Text        string                   `bson:"text" json:"text"`
	Attachments []string                 `bson:"attachments" json:"attachments"`
	ReadBy      []string                 `bson:"readBy"          json:"readBy"`
	Reactions   map[string][]string      `bson:"reactions"       json:"reactions"`
	CreatedAt   time.Time                `bson:"createdAt"       json:"createdAt"`
	SenderName  string                   `bson:"-" json:"senderName,omitempty"`
}
