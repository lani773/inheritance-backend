// Package utils provides shared formatting and helper utilities.
package utils

import (
	"github.com/inheritance-choir/backend/internal/models"
)

// MemberPublic is the safe public representation of a member.
type MemberPublic struct {
	ID               string   `json:"id"`
	FullName         string   `json:"fullName"`
	Email            string   `json:"email"`
	Phone            string   `json:"phone,omitempty"`
	DateOfBirth      string   `json:"dateOfBirth,omitempty"`
	Gender           string   `json:"gender,omitempty"`
	MaritalStatus    string   `json:"maritalStatus,omitempty"`
	Bio              string   `json:"bio,omitempty"`
	AvatarURL        string   `json:"avatarUrl,omitempty"`
	VoicePart        string   `json:"voicePart"`
	Role             string   `json:"role"`
	Status           string   `json:"status"`
	IsAdmin          bool     `json:"isAdmin"`
	Permissions      []string `json:"permissions"`
	Attendance       float64  `json:"attendance"`
	ContribTotal     float64  `json:"contributionTotal"`
	JoinDate         string   `json:"joinDate"`
	Online           bool     `json:"online"`
}

// FormatMember converts a Member document to a safe public DTO.
func FormatMember(m *models.Member) MemberPublic {
	perms := m.Permissions
	if perms == nil { perms = []string{} }
	return MemberPublic{
		ID:            m.ID.Hex(),
		FullName:      m.FullName,
		Email:         m.Email,
		Phone:         m.Phone,
		DateOfBirth:   m.DateOfBirth,
		Gender:        m.Gender,
		MaritalStatus: m.MaritalStatus,
		Bio:           m.Bio,
		AvatarURL:     m.AvatarURL,
		VoicePart:     m.VoicePart,
		Role:          m.Role,
		Status:        m.Status,
		IsAdmin:       m.IsAdmin,
		Permissions:   perms,
		Attendance:    m.Attendance,
		ContribTotal:  m.ContributionTotal,
		JoinDate:      m.JoinDate.Format("2006-01-02"),
		Online:        m.Online,
	}
}
