package repository

import (
	"database/sql"
	"fmt"
	"pr-reviewer-service/internal/models"
)

// Repository - тут вся работа с базой данных
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Teams
func (r *Repository) CreateTeam(teamName string) error {
	_, err := r.db.Exec("INSERT INTO teams (team_name) VALUES ($1)", teamName)
	if err != nil {
		return err
	}
	return nil
}

func (r *Repository) TeamExists(teamName string) (bool, error) {
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM teams WHERE team_name = $1)", teamName).Scan(&exists)
	return exists, err
}

func (r *Repository) GetTeam(teamName string) (*models.Team, error) {
	team := &models.Team{TeamName: teamName}

	rows, err := r.db.Query(`
		SELECT user_id, username, is_active 
		FROM users 
		WHERE team_name = $1 
		ORDER BY user_id
	`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member models.TeamMember
		if err := rows.Scan(&member.UserID, &member.Username, &member.IsActive); err != nil {
			return nil, err
		}
		team.Members = append(team.Members, member)
	}

	if len(team.Members) == 0 {
		// Проверяю, существует ли команда, даже если в ней нет участников
		exists, err := r.TeamExists(teamName)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("team not found")
		}
	}

	return team, nil
}

// Users

// CreateOrUpdateUser - хитрый запрос.
// Он пытается вставить нового юзера, а если юзер с таким user_id уже есть (ON CONFLICT),
// то он просто обновляет его данные. Удобно, чтобы не делать два запроса (SELECT, а потом INSERT/UPDATE).
func (r *Repository) CreateOrUpdateUser(user *models.User) error {
	_, err := r.db.Exec(`
		INSERT INTO users (user_id, username, team_name, is_active, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) 
		DO UPDATE SET 
			username = EXCLUDED.username,
			team_name = EXCLUDED.team_name,
			is_active = EXCLUDED.is_active,
			updated_at = CURRENT_TIMESTAMP
	`, user.UserID, user.Username, user.TeamName, user.IsActive)
	return err
}

func (r *Repository) GetUser(userID string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(`
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE user_id = $1
	`, userID).Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *Repository) UpdateUserActive(userID string, isActive bool) error {
	result, err := r.db.Exec(`
		UPDATE users 
		SET is_active = $1, updated_at = CURRENT_TIMESTAMP 
		WHERE user_id = $2
	`, isActive, userID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// GetActiveUsersByTeam - получает список активных пользователей из команды,
// не включая одного конкретного пользователя (обычно это автор PR).
func (r *Repository) GetActiveUsersByTeam(teamName string, excludeUserID string) ([]*models.User, error) {
	rows, err := r.db.Query(`
		SELECT user_id, username, team_name, is_active 
		FROM users 
		WHERE team_name = $1 AND is_active = true AND user_id != $2
		ORDER BY user_id
	`, teamName, excludeUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		if err := rows.Scan(&user.UserID, &user.Username, &user.TeamName, &user.IsActive); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (r *Repository) GetUserTeam(userID string) (string, error) {
	var teamName string
	err := r.db.QueryRow("SELECT team_name FROM users WHERE user_id = $1", userID).Scan(&teamName)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found")
	}
	return teamName, err
}

// Pull Requests
func (r *Repository) CreatePullRequest(pr *models.PullRequest) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pull_requests (pull_request_id, pull_request_name, author_id, status, need_more_reviewers, created_at)
		VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
	`, pr.PullRequestID, pr.PullRequestName, pr.AuthorID, pr.Status, pr.NeedMoreReviewers)
	if err != nil {
		return err
	}

	for _, reviewerID := range pr.AssignedReviewers {
		_, err = tx.Exec(`
			INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id)
			VALUES ($1, $2)
		`, pr.PullRequestID, reviewerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) PullRequestExists(pullRequestID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM pull_requests WHERE pull_request_id = $1)", pullRequestID).Scan(&exists)
	return exists, err
}

func (r *Repository) GetPullRequest(pullRequestID string) (*models.PullRequest, error) {
	pr := &models.PullRequest{}
	var createdAt, mergedAt sql.NullTime
	var needMoreReviewers bool

	err := r.db.QueryRow(`
		SELECT pull_request_id, pull_request_name, author_id, status, need_more_reviewers, created_at, merged_at
		FROM pull_requests
		WHERE pull_request_id = $1
	`, pullRequestID).Scan(
		&pr.PullRequestID,
		&pr.PullRequestName,
		&pr.AuthorID,
		&pr.Status,
		&needMoreReviewers,
		&createdAt,
		&mergedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("pull request not found")
	}
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		pr.CreatedAt = &createdAt.Time
	}
	if mergedAt.Valid {
		pr.MergedAt = &mergedAt.Time
	}
	pr.NeedMoreReviewers = needMoreReviewers

	rows, err := r.db.Query(`
		SELECT reviewer_id 
		FROM pull_request_reviewers 
		WHERE pull_request_id = $1
		ORDER BY reviewer_id
	`, pullRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var reviewerID string
		if err := rows.Scan(&reviewerID); err != nil {
			return nil, err
		}
		pr.AssignedReviewers = append(pr.AssignedReviewers, reviewerID)
	}

	return pr, nil
}

func (r *Repository) MergePullRequest(pullRequestID string) error {
	result, err := r.db.Exec(`
		UPDATE pull_requests 
		SET status = 'MERGED', merged_at = CURRENT_TIMESTAMP 
		WHERE pull_request_id = $1 AND status = 'OPEN'
	`, pullRequestID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		// Проверяю, существует ли PR
		exists, err := r.PullRequestExists(pullRequestID)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("pull request not found")
		}
		// Если существует, но уже MERGED - это нормально, идемпотентность работает
	}
	return nil
}

func (r *Repository) ReassignReviewer(pullRequestID string, oldReviewerID string, newReviewerID string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Проверяю, что старый ревьювер действительно назначен на этот PR
	var exists bool
	err = tx.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pull_request_reviewers 
			WHERE pull_request_id = $1 AND reviewer_id = $2
		)
	`, pullRequestID, oldReviewerID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("reviewer is not assigned to this PR")
	}

	// Удаляю старого ревьювера
	_, err = tx.Exec(`
		DELETE FROM pull_request_reviewers 
		WHERE pull_request_id = $1 AND reviewer_id = $2
	`, pullRequestID, oldReviewerID)
	if err != nil {
		return err
	}

	// Добавляю нового ревьювера
	_, err = tx.Exec(`
		INSERT INTO pull_request_reviewers (pull_request_id, reviewer_id)
		VALUES ($1, $2)
	`, pullRequestID, newReviewerID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) GetPullRequestsByReviewer(reviewerID string) ([]*models.PullRequestShort, error) {
	rows, err := r.db.Query(`
		SELECT pr.pull_request_id, pr.pull_request_name, pr.author_id, pr.status
		FROM pull_requests pr
		INNER JOIN pull_request_reviewers prr ON pr.pull_request_id = prr.pull_request_id
		WHERE prr.reviewer_id = $1
		ORDER BY pr.created_at DESC
	`, reviewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []*models.PullRequestShort
	for rows.Next() {
		pr := &models.PullRequestShort{}
		if err := rows.Scan(&pr.PullRequestID, &pr.PullRequestName, &pr.AuthorID, &pr.Status); err != nil {
			return nil, err
		}
		prs = append(prs, pr)
	}
	return prs, nil
}

// Statistics - статистика по пользователям
func (r *Repository) GetUserReviewStats() ([]*models.UserReviewStats, error) {
	rows, err := r.db.Query(`
		SELECT 
			u.user_id,
			u.username,
			COUNT(prr.reviewer_id) as total_assignments,
			COUNT(CASE WHEN pr.status = 'OPEN' THEN 1 END) as open_assignments,
			COUNT(CASE WHEN pr.status = 'MERGED' THEN 1 END) as merged_assignments
		FROM users u
		LEFT JOIN pull_request_reviewers prr ON u.user_id = prr.reviewer_id
		LEFT JOIN pull_requests pr ON prr.pull_request_id = pr.pull_request_id
		GROUP BY u.user_id, u.username
		ORDER BY total_assignments DESC, u.user_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*models.UserReviewStats
	for rows.Next() {
		stat := &models.UserReviewStats{}
		if err := rows.Scan(&stat.UserID, &stat.Username, &stat.TotalAssignments, &stat.OpenAssignments, &stat.MergedAssignments); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}
	return stats, nil
}

func (r *Repository) GetPRStats() (*models.PRStats, error) {
	stats := &models.PRStats{}
	err := r.db.QueryRow(`
		SELECT 
			COUNT(*) as total_prs,
			COUNT(CASE WHEN status = 'OPEN' THEN 1 END) as open_prs,
			COUNT(CASE WHEN status = 'MERGED' THEN 1 END) as merged_prs,
			(SELECT COUNT(*) FROM pull_request_reviewers) as total_assignments
		FROM pull_requests
	`).Scan(&stats.TotalPRs, &stats.OpenPRs, &stats.MergedPRs, &stats.TotalAssignments)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// Bulk deactivation - получаю список пользователей без деактивации
func (r *Repository) GetUsersByTeamForDeactivation(teamName string) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT user_id FROM users WHERE team_name = $1 AND is_active = true
	`, teamName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

func (r *Repository) BulkDeactivateUsersByTeam(teamName string) ([]string, error) {
	// Получаю список пользователей перед деактивацией
	userIDs, err := r.GetUsersByTeamForDeactivation(teamName)
	if err != nil {
		return nil, err
	}

	if len(userIDs) == 0 {
		return []string{}, nil
	}

	// Деактивирую всех активных пользователей команды
	_, err = r.db.Exec(`
		UPDATE users 
		SET is_active = false, updated_at = CURRENT_TIMESTAMP 
		WHERE team_name = $1 AND is_active = true
	`, teamName)
	if err != nil {
		return nil, err
	}

	return userIDs, nil
}

func (r *Repository) GetOpenPRsWithReviewers(reviewerIDs []string) ([]string, error) {
	if len(reviewerIDs) == 0 {
		return []string{}, nil
	}

	placeholders := ""
	args := make([]interface{}, len(reviewerIDs)+1)
	args[0] = "OPEN"
	for i, id := range reviewerIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf(`
		SELECT DISTINCT pr.pull_request_id
		FROM pull_requests pr
		INNER JOIN pull_request_reviewers prr ON pr.pull_request_id = prr.pull_request_id
		WHERE pr.status = $1 AND prr.reviewer_id IN (%s)
	`, placeholders)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prIDs []string
	for rows.Next() {
		var prID string
		if err := rows.Scan(&prID); err != nil {
			return nil, err
		}
		prIDs = append(prIDs, prID)
	}
	return prIDs, nil
}