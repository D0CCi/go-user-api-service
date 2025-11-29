package models

import "time"

type TeamMember struct {
	UserID   string `json:"user_id" db:"user_id"`
	Username string `json:"username" db:"username"`
	IsActive bool   `json:"is_active" db:"is_active"`
}

type Team struct {
	TeamName string       `json:"team_name" db:"team_name"`
	Members  []TeamMember `json:"members"`
}

type User struct {
	UserID   string `json:"user_id" db:"user_id"`
	Username string `json:"username" db:"username"`
	TeamName string `json:"team_name" db:"team_name"`
	IsActive bool   `json:"is_active" db:"is_active"`
}

type PullRequestStatus string

const (
	StatusOpen   PullRequestStatus = "OPEN"
	StatusMerged PullRequestStatus = "MERGED"
)

type PullRequest struct {
	PullRequestID     string            `json:"pull_request_id" db:"pull_request_id"`
	PullRequestName   string            `json:"pull_request_name" db:"pull_request_name"`
	AuthorID          string            `json:"author_id" db:"author_id"`
	Status            PullRequestStatus `json:"status" db:"status"`
	AssignedReviewers []string          `json:"assigned_reviewers"`
	NeedMoreReviewers bool              `json:"needMoreReviewers" db:"need_more_reviewers"`
	CreatedAt         *time.Time        `json:"createdAt,omitempty" db:"created_at"`
	MergedAt          *time.Time        `json:"mergedAt,omitempty" db:"merged_at"`
}

type PullRequestShort struct {
	PullRequestID   string            `json:"pull_request_id"`
	PullRequestName string            `json:"pull_request_name"`
	AuthorID        string            `json:"author_id"`
	Status          PullRequestStatus `json:"status"`
}

type ErrorCode string

const (
	ErrorTeamExists  ErrorCode = "TEAM_EXISTS"
	ErrorPRExists    ErrorCode = "PR_EXISTS"
	ErrorPRMerged    ErrorCode = "PR_MERGED"
	ErrorNotAssigned ErrorCode = "NOT_ASSIGNED"
	ErrorNoCandidate ErrorCode = "NO_CANDIDATE"
	ErrorNotFound    ErrorCode = "NOT_FOUND"
)

type ErrorResponse struct {
	Error struct {
		Code    ErrorCode `json:"code"`
		Message string    `json:"message"`
	} `json:"error"`
}

// Statistics models
type UserReviewStats struct {
	UserID           string `json:"user_id" db:"user_id"`
	Username         string `json:"username" db:"username"`
	TotalAssignments int    `json:"total_assignments" db:"total_assignments"`
	OpenAssignments  int    `json:"open_assignments" db:"open_assignments"`
	MergedAssignments int   `json:"merged_assignments" db:"merged_assignments"`
}

type PRStats struct {
	TotalPRs        int `json:"total_prs" db:"total_prs"`
	OpenPRs         int `json:"open_prs" db:"open_prs"`
	MergedPRs       int `json:"merged_prs" db:"merged_prs"`
	TotalAssignments int `json:"total_assignments" db:"total_assignments"`
}

type StatisticsResponse struct {
	UserStats []UserReviewStats `json:"user_stats"`
	PRStats   PRStats           `json:"pr_stats"`
}