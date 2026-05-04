// Package realtime — typed event taxonomy for the Inheritance Choir WebSocket hub.
package realtime

// Event type constants — all WS events use these names.
// Format: "resource:action"
const (
	// ── Auth / Presence ───────────────────────────────────────────
	EvtMemberOnline  = "member:online"
	EvtMemberOffline = "member:offline"
	EvtPresenceList  = "presence:list"

	// ── Members ───────────────────────────────────────────────────
	EvtMemberCreated = "member:created"
	EvtMemberUpdated = "member:updated"
	EvtMemberDeleted = "member:deleted"
	EvtMemberApproved = "member:approved"

	// ── Contributions ─────────────────────────────────────────────
	EvtContributionCreated  = "contribution:created"
	EvtContributionVerified = "contribution:verified"
	EvtContributionDeleted  = "contribution:deleted"

	// ── Events ────────────────────────────────────────────────────
	EvtEventCreated = "event:created"
	EvtEventUpdated = "event:updated"
	EvtEventDeleted = "event:deleted"

	// ── Attendance ────────────────────────────────────────────────
	EvtAttendanceMarked = "attendance:marked"
	EvtExcuseSubmitted  = "excuse:submitted"
	EvtExcuseReviewed   = "excuse:reviewed"

	// ── Chat ──────────────────────────────────────────────────────
	EvtChatMessage = "chat:message"
	EvtChatTyping  = "chat:typing"
	EvtChatRead    = "chat:read"

	// ── Messages (broadcast/DM) ───────────────────────────────────
	EvtMessageSent    = "message:sent"
	EvtMessageDeleted = "message:deleted"

	// ── Notifications ─────────────────────────────────────────────
	EvtNotificationNew = "notification:new"

	// ── Posts ─────────────────────────────────────────────────────
	EvtPostCreated = "post:created"
	EvtPostLiked   = "post:liked"
	EvtPostComment = "post:comment"

	// ── Songs / Setlists ──────────────────────────────────────────
	EvtSongCreated     = "song:created"
	EvtSetlistUpdated  = "setlist:updated"
	EvtSetlistDeleted  = "setlist:deleted"

	// ── Finance ───────────────────────────────────────────────────
	EvtBudgetUpdated       = "budget:updated"
	EvtPledgeNew           = "pledge:new"
	EvtPledgeCampaignNew   = "pledge:campaign:new"

	// ── Welfare ───────────────────────────────────────────────────
	EvtWelfareCreated = "welfare:created"
	EvtWelfareUpdated = "welfare:updated"

	// ── Prayer ────────────────────────────────────────────────────
	EvtPrayerNew   = "prayer:new"
	EvtPrayerPrayed = "prayer:prayed"

	// ── Automation ────────────────────────────────────────────────
	EvtAutomationRun     = "automation:run"
	EvtAutomationUpdated = "automation:updated"

	// ── Upload ────────────────────────────────────────────────────
	EvtUploadNew = "upload:new"

	// ── Admin ─────────────────────────────────────────────────────
	EvtBroadcast      = "admin:broadcast"
	EvtSystemAlert    = "system:alert"
	EvtLiveStats      = "system:live_stats"

	// ── System ────────────────────────────────────────────────────
	EvtPing      = "ping"
	EvtPong      = "pong"
	EvtSubscribe = "subscribe"
	EvtConnected = "connected"
)

// Channels — named pub/sub rooms clients can subscribe to.
const (
	ChGeneral   = "general"   // all members
	ChAdmin     = "admin"     // admins only
	ChSoprano   = "soprano"
	ChAlto      = "alto"
	ChTenor     = "tenor"
	ChBass      = "bass"
	ChFinance   = "finance"
	ChPrayer    = "prayer"
)

// VoicePartChannel maps a voice part string to its channel name.
func VoicePartChannel(voicePart string) string {
	switch voicePart {
	case "Soprano":
		return ChSoprano
	case "Alto":
		return ChAlto
	case "Tenor":
		return ChTenor
	case "Bass":
		return ChBass
	default:
		return ChGeneral
	}
}
