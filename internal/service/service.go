package service

import (
	"fmt"
	"math/rand"
	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/repository"
	"time"
)

// Service - тут вся основная логика работы с PR и ревьюерами
type Service struct {
	repo *repository.Repository
}

func NewService(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

// Teams
func (s *Service) CreateTeam(team *models.Team) error {
	exists, err := s.repo.TeamExists(team.TeamName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("TEAM_EXISTS")
	}

	if err := s.repo.CreateTeam(team.TeamName); err != nil {
		return err
	}

	// Создаю или обновляю пользователей в команде
	for _, member := range team.Members {
		user := &models.User{
			UserID:   member.UserID,
			Username: member.Username,
			TeamName: team.TeamName,
			IsActive: member.IsActive,
		}
		if err := s.repo.CreateOrUpdateUser(user); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) GetTeam(teamName string) (*models.Team, error) {
	team, err := s.repo.GetTeam(teamName)
	if err != nil {
		return nil, err
	}
	return team, nil
}

// Users
func (s *Service) SetUserActive(userID string, isActive bool) (*models.User, error) {
	if err := s.repo.UpdateUserActive(userID, isActive); err != nil {
		return nil, err
	}
	return s.repo.GetUser(userID)
}

// Pull Requests

// CreatePullRequest - логика создания PR и назначения ревьюеров.
func (s *Service) CreatePullRequest(prID, prName, authorID string) (*models.PullRequest, error) {
	// Сначала проверяю, нет ли уже PR с таким ID.
	exists, err := s.repo.PullRequestExists(prID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("PR_EXISTS")
	}

	// Нахожу автора и его команду.
	author, err := s.repo.GetUser(authorID)
	if err != nil {
		return nil, fmt.Errorf("author not found")
	}

	// Ищу всех активных ребят из его команды, кроме него самого.
	candidates, err := s.repo.GetActiveUsersByTeam(author.TeamName, authorID)
	if err != nil {
		return nil, err
	}

	// Выбираю до 2-х случайных ревьюеров из списка кандидатов.
	reviewers := s.selectRandomReviewers(candidates, 2)
	needMoreReviewers := len(reviewers) < 2 // Если нашлось меньше двух, ставлю флаг.

	pr := &models.PullRequest{
		PullRequestID:     prID,
		PullRequestName:   prName,
		AuthorID:          authorID,
		Status:            models.StatusOpen,
		AssignedReviewers: reviewers,
		NeedMoreReviewers: needMoreReviewers,
	}

	// Сохраняю всё в базу.
	if err := s.repo.CreatePullRequest(pr); err != nil {
		return nil, err
	}

	// Возвращаю полный объект PR, чтобы в ответе были все поля.
	return s.repo.GetPullRequest(prID)
}

func (s *Service) MergePullRequest(prID string) (*models.PullRequest, error) {
	pr, err := s.repo.GetPullRequest(prID)
	if err != nil {
		return nil, err
	}

	// Если он уже смержен, ничего не делаю, просто возвращаю его. Это для идемпотентности.
	if pr.Status == models.StatusMerged {
		return pr, nil
	}

	if err := s.repo.MergePullRequest(prID); err != nil {
		return nil, err
	}

	return s.repo.GetPullRequest(prID)
}

// ReassignReviewer - логика переназначения ревьюера.
func (s *Service) ReassignReviewer(prID, oldReviewerID string) (*models.PullRequest, string, error) {
	pr, err := s.repo.GetPullRequest(prID)
	if err != nil {
		return nil, "", err
	}

	// Нельзя переназначать ревьюеров на смерженном PR.
	if pr.Status == models.StatusMerged {
		return nil, "", fmt.Errorf("PR_MERGED")
	}

	// Проверяю, а был ли вообще такой ревьюер на этом PR.
	found := false
	for _, reviewerID := range pr.AssignedReviewers {
		if reviewerID == oldReviewerID {
			found = true
			break
		}
	}
	if !found {
		return nil, "", fmt.Errorf("NOT_ASSIGNED")
	}

	// Нахожу команду старого ревьюера, чтобы искать замену в ней же.
	oldReviewerTeam, err := s.repo.GetUserTeam(oldReviewerID)
	if err != nil {
		return nil, "", fmt.Errorf("old reviewer not found")
	}

	// Ищу кандидатов на замену.
	candidates, err := s.repo.GetActiveUsersByTeam(oldReviewerTeam, oldReviewerID)
	if err != nil {
		return nil, "", err
	}

	// Убираю из кандидатов тех, кто уже назначен на этот PR.
	availableCandidates := s.filterAssignedReviewers(candidates, pr.AssignedReviewers, pr.AuthorID)
	if len(availableCandidates) == 0 {
		// Если некого назначить.
		return nil, "", fmt.Errorf("NO_CANDIDATE")
	}
	// Выбираю случайного.
	newReviewerID := s.selectRandomReviewer(availableCandidates)

	// Обновляю инфу в базе.
	if err := s.repo.ReassignReviewer(prID, oldReviewerID, newReviewerID); err != nil {
		if err.Error() == "reviewer is not assigned to this PR" {
			return nil, "", fmt.Errorf("NOT_ASSIGNED")
		}
		return nil, "", err
	}

	updatedPR, err := s.repo.GetPullRequest(prID)
	if err != nil {
		return nil, "", err
	}

	return updatedPR, newReviewerID, nil
}

func (s *Service) GetPullRequestsByReviewer(reviewerID string) ([]*models.PullRequestShort, error) {
	// Просто проверяю, что такой юзер есть, перед тем как искать его ревью.
	_, err := s.repo.GetUser(reviewerID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	return s.repo.GetPullRequestsByReviewer(reviewerID)
}

// Statistics - просто собираю статистику из репозитория
func (s *Service) GetStatistics() (*models.StatisticsResponse, error) {
	userStats, err := s.repo.GetUserReviewStats()
	if err != nil {
		return nil, err
	}

	prStats, err := s.repo.GetPRStats()
	if err != nil {
		return nil, err
	}

	// Преобразую указатели в значения для ответа
	userStatsValues := make([]models.UserReviewStats, len(userStats))
	for i, stat := range userStats {
		userStatsValues[i] = *stat
	}

	return &models.StatisticsResponse{
		UserStats: userStatsValues,
		PRStats:   *prStats,
	}, nil
}

// BulkDeactivateTeam - массовая деактивация пользователей команды с безопасной переназначаемостью открытых PR
func (s *Service) BulkDeactivateTeam(teamName string) ([]string, []string, error) {
	// Проверяю, что команда существует
	_, err := s.repo.GetTeam(teamName)
	if err != nil {
		return nil, nil, fmt.Errorf("team not found")
	}

	// Сначала получаю список пользователей, которых нужно деактивировать
	// Делаю это до деактивации, чтобы потом найти открытые PR
	deactivatedUserIDs, err := s.repo.GetUsersByTeamForDeactivation(teamName)
	if err != nil {
		return nil, nil, err
	}

	if len(deactivatedUserIDs) == 0 {
		return []string{}, []string{}, nil
	}

	// Ищу все открытые PR, где эти пользователи назначены ревьюерами
	// Делаю это до деактивации, чтобы переназначение сработало
	affectedPRIDs, err := s.repo.GetOpenPRsWithReviewers(deactivatedUserIDs)
	if err != nil {
		return nil, nil, err
	}

	// Для каждого открытого PR переназначаю ревьюеров до деактивации
	reassignedPRs := make([]string, 0)
	reassignedPRsMap := make(map[string]bool) // Чтобы не дублировать PR в списке

	for _, prID := range affectedPRIDs {
		pr, err := s.repo.GetPullRequest(prID)
		if err != nil {
			continue
		}

		// Получаю автора, чтобы знать его команду для поиска замены
		author, err := s.repo.GetUser(pr.AuthorID)
		if err != nil {
			continue
		}

		// Прохожу по всем ревьюерам и переназначаю тех, кого нужно деактивировать
		for _, reviewerID := range pr.AssignedReviewers {
			for _, deactivatedID := range deactivatedUserIDs {
				if reviewerID == deactivatedID {
					// При массовой деактивации ищу замену в команде автора, а не заменяемого ревьюера
					// Потому что если деактивируем всю команду, то в ней не будет активных для замены
					newReviewerID, err := s.reassignReviewerForBulkDeactivation(prID, reviewerID, author.TeamName, pr.AssignedReviewers, pr.AuthorID, deactivatedUserIDs)
					if err == nil && newReviewerID != "" {
						// Добавляю PR в список только один раз, даже если там несколько ревьюеров переназначил
						if !reassignedPRsMap[prID] {
							reassignedPRs = append(reassignedPRs, prID)
							reassignedPRsMap[prID] = true
						}
					}
					break
				}
			}
		}
	}

	// Теперь можно деактивировать пользователей
	_, err = s.repo.BulkDeactivateUsersByTeam(teamName)
	if err != nil {
		return nil, nil, err
	}

	return deactivatedUserIDs, reassignedPRs, nil
}

// reassignReviewerForBulkDeactivation - переназначение при массовой деактивации
// Ищу замену в команде автора, потому что в команде заменяемого ревьюера все будут деактивированы
func (s *Service) reassignReviewerForBulkDeactivation(prID, oldReviewerID, authorTeamName string, currentReviewers []string, authorID string, deactivatedUserIDs []string) (string, error) {
	// Ищу кандидатов в команде автора, как при создании PR
	candidates, err := s.repo.GetActiveUsersByTeam(authorTeamName, authorID)
	if err != nil {
		return "", err
	}

	// Убираю тех, кто уже назначен на этот PR
	availableCandidates := s.filterAssignedReviewers(candidates, currentReviewers, authorID)
	
	// И еще убираю тех, кого собираюсь деактивировать
	deactivatedMap := make(map[string]bool)
	for _, id := range deactivatedUserIDs {
		deactivatedMap[id] = true
	}
	
	finalCandidates := make([]*models.User, 0)
	for _, candidate := range availableCandidates {
		if !deactivatedMap[candidate.UserID] {
			finalCandidates = append(finalCandidates, candidate)
		}
	}
	
	if len(finalCandidates) == 0 {
		return "", fmt.Errorf("NO_CANDIDATE")
	}

	// Выбираю случайного из оставшихся
	newReviewerID := s.selectRandomReviewer(finalCandidates)

	// Обновляю в базе
	if err := s.repo.ReassignReviewer(prID, oldReviewerID, newReviewerID); err != nil {
		return "", err
	}

	return newReviewerID, nil
}

// --- Вспомогательные методы ---

// selectRandomReviewers - выбираю случайных ревьюеров из списка кандидатов
func (s *Service) selectRandomReviewers(candidates []*models.User, maxCount int) []string {
	if len(candidates) == 0 {
		return []string{}
	}

	count := maxCount
	if len(candidates) < maxCount {
		count = len(candidates)
	}

	// Перемешиваю кандидатов, чтобы выбор был случайным
	mixedCandidates := make([]*models.User, len(candidates))
	copy(mixedCandidates, candidates)
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(mixedCandidates), func(i, j int) {
		mixedCandidates[i], mixedCandidates[j] = mixedCandidates[j], mixedCandidates[i]
	})

	// Беру первых N из перемешанного списка
	reviewers := make([]string, 0, count)
	for i := 0; i < count; i++ {
		reviewers = append(reviewers, mixedCandidates[i].UserID)
	}

	return reviewers
}

func (s *Service) selectRandomReviewer(candidates []*models.User) string {
	if len(candidates) == 0 {
		return ""
	}
	rand.Seed(time.Now().UnixNano())
	return candidates[rand.Intn(len(candidates))].UserID
}

// filterAssignedReviewers - убираю из кандидатов тех, кто уже назначен на PR, и автора
func (s *Service) filterAssignedReviewers(candidates []*models.User, assigned []string, authorID string) []*models.User {
	assignedMap := make(map[string]bool)
	for _, id := range assigned {
		assignedMap[id] = true
	}

	filtered := make([]*models.User, 0)
	for _, candidate := range candidates {
		if candidate.UserID == authorID {
			continue
		}
		if !assignedMap[candidate.UserID] {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}
