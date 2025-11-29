package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	baseURL = "http://localhost:8080"
	timeout = 10 * time.Second
)

var (
	httpClient = &http.Client{Timeout: timeout}
	testSuffix = fmt.Sprintf("%d", time.Now().UnixNano())
)

func generateID(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, testSuffix)
}

func TestMain(m *testing.M) {
	for i := 0; i < 30; i++ {
		resp, err := httpClient.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			os.Exit(m.Run())
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Сервис недоступен. Запустите docker-compose up")
	os.Exit(1)
}

func TestHealthCheck(t *testing.T) {
	resp, err := httpClient.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"].(string) != "ok" {
		t.Error("Статус должен быть ok")
	}
}

func TestCreateTeam(t *testing.T) {
	teamName := generateID("team")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": generateID("user1"), "username": "User 1", "is_active": true},
			{"user_id": generateID("user2"), "username": "User 2", "is_active": true},
			{"user_id": generateID("user3"), "username": "User 3", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	team := result["team"].(map[string]interface{})
	if team["team_name"].(string) != teamName {
		t.Errorf("Ожидался team_name=%s", teamName)
	}
}

func TestGetTeam(t *testing.T) {
	teamName := generateID("team-get")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": generateID("user"), "username": "User", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	req, _ = http.NewRequest("GET", baseURL+"/team/get?team_name="+teamName, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var team map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&team)
	if team["team_name"].(string) != teamName {
		t.Errorf("Ожидался team_name=%s", teamName)
	}
}

func TestCreatePullRequestWithAutoAssignment(t *testing.T) {
	teamName := generateID("team-pr")
	authorID := generateID("author")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": generateID("reviewer1"), "username": "Reviewer 1", "is_active": true},
			{"user_id": generateID("reviewer2"), "username": "Reviewer 2", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prID := generateID("pr")
	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Error("Должны быть назначены ревьюеры")
	}

	for _, reviewer := range reviewers {
		if reviewer.(string) == authorID {
			t.Error("Автор не должен быть ревьюером")
		}
	}

	if pr["status"].(string) != "OPEN" {
		t.Error("Статус должен быть OPEN")
	}
}

func TestMergePullRequest(t *testing.T) {
	teamName := generateID("team-merge")
	authorID := generateID("author-merge")
	prID := generateID("pr-merge")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": generateID("reviewer"), "username": "Reviewer", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR to Merge",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	mergeData := map[string]string{"pull_request_id": prID}
	body, _ = json.Marshal(mergeData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	pr := result["pr"].(map[string]interface{})
	if pr["status"].(string) != "MERGED" {
		t.Error("Статус должен быть MERGED")
	}

	// Проверяю идемпотентность
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Error("Повторный merge должен быть идемпотентным")
	}
}

func TestReassignReviewer(t *testing.T) {
	teamName := generateID("team-reassign")
	authorID := generateID("author-reassign")
	prID := generateID("pr-reassign")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": generateID("reviewer1"), "username": "Reviewer 1", "is_active": true},
			{"user_id": generateID("reviewer2"), "username": "Reviewer 2", "is_active": true},
			{"user_id": generateID("reviewer3"), "username": "Reviewer 3", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR for Reassign",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	pr := createResult["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	if len(reviewers) == 0 {
		t.Fatal("Должен быть хотя бы один ревьюер")
	}

	oldReviewerID := reviewers[0].(string)

	reassignData := map[string]string{
		"pull_request_id": prID,
		"old_user_id":     oldReviewerID,
	}

	body, _ = json.Marshal(reassignData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/reassign", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err = httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидался статус 200, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["pr"].(map[string]interface{}); !ok {
		t.Error("Ответ должен содержать pr")
	}
	if _, ok := result["replaced_by"].(string); !ok {
		t.Error("Ответ должен содержать replaced_by")
	}
}

func TestReassignOnMergedPR(t *testing.T) {
	teamName := generateID("team-merged")
	authorID := generateID("author-merged")
	prID := generateID("pr-merged")
	reviewer1ID := generateID("reviewer-merged")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": reviewer1ID, "username": "Reviewer 1", "is_active": true},
			{"user_id": generateID("reviewer2"), "username": "Reviewer 2", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR to Merge",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	mergeData := map[string]string{"pull_request_id": prID}
	body, _ = json.Marshal(mergeData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	reassignData := map[string]string{
		"pull_request_id": prID,
		"old_user_id":     reviewer1ID,
	}

	body, _ = json.Marshal(reassignData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/reassign", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидалась ошибка 409, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestGetReview(t *testing.T) {
	teamName := generateID("team-get-review")
	authorID := generateID("author-get-review")
	reviewerID := generateID("reviewer-get-review")
	prID := generateID("pr-get-review")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": reviewerID, "username": "Reviewer", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR for Get Review",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	req, _ = http.NewRequest("GET", baseURL+"/users/getReview?user_id="+reviewerID, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["user_id"].(string) != reviewerID {
		t.Errorf("Ожидался user_id=%s", reviewerID)
	}

	prs := result["pull_requests"].([]interface{})
	if len(prs) == 0 {
		t.Error("Должен быть хотя бы один PR")
	}
}

func TestSetUserActive(t *testing.T) {
	teamName := generateID("team-set-active")
	userID := generateID("user-set-active")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": userID, "username": "User", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	setActiveData := map[string]interface{}{
		"user_id":   userID,
		"is_active": false,
	}

	body, _ = json.Marshal(setActiveData)
	req, _ = http.NewRequest("POST", baseURL+"/users/setIsActive", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	user := result["user"].(map[string]interface{})
	if user["is_active"].(bool) != false {
		t.Error("is_active должен быть false")
	}
}

func TestGetStatistics(t *testing.T) {
	req, _ := http.NewRequest("GET", baseURL+"/statistics", nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)
	if _, ok := stats["user_stats"]; !ok {
		t.Error("Должна быть user_stats")
	}
	if _, ok := stats["pr_stats"]; !ok {
		t.Error("Должна быть pr_stats")
	}
}

func TestBulkDeactivate(t *testing.T) {
	teamName := generateID("team-bulk")
	authorID := generateID("author-bulk")
	prID := generateID("pr-bulk")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": generateID("reviewer1"), "username": "Reviewer 1", "is_active": true},
			{"user_id": generateID("reviewer2"), "username": "Reviewer 2", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR for Bulk",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	bulkData := map[string]string{"team_name": teamName}
	body, _ = json.Marshal(bulkData)
	req, _ = http.NewRequest("POST", baseURL+"/team/bulkDeactivate", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if _, ok := result["deactivated_user_ids"].([]interface{}); !ok {
		t.Error("Должны быть deactivated_user_ids")
	}
	if _, ok := result["reassigned_prs"].([]interface{}); !ok {
		t.Error("Должны быть reassigned_prs")
	}
}

func TestInactiveUserNotAssigned(t *testing.T) {
	teamName := generateID("team-inactive")
	authorID := generateID("author-inactive")
	prID := generateID("pr-inactive")
	inactiveReviewerID := generateID("inactive-reviewer")
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": generateID("active-reviewer"), "username": "Active Reviewer", "is_active": true},
			{"user_id": inactiveReviewerID, "username": "Inactive Reviewer", "is_active": false},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	httpClient.Do(req)

	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "PR Inactive Test",
		"author_id":         authorID,
	}

	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Ошибка: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Errorf("Ожидался статус 201, получен %d. Тело: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	for _, reviewer := range reviewers {
		if reviewer.(string) == inactiveReviewerID {
			t.Error("Неактивный пользователь не должен быть ревьюером")
		}
	}
}
