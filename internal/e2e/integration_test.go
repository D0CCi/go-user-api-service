package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// TestFullIntegrationFlow проверяет полный сценарий работы сервиса
func TestFullIntegrationFlow(t *testing.T) {
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	teamName := fmt.Sprintf("team-%s", suffix)
	authorID := fmt.Sprintf("author-%s", suffix)
	reviewer1ID := fmt.Sprintf("reviewer1-%s", suffix)
	reviewer2ID := fmt.Sprintf("reviewer2-%s", suffix)
	reviewer3ID := fmt.Sprintf("reviewer3-%s", suffix)
	prID := fmt.Sprintf("pr-%s", suffix)

	// Создаю команду
	teamData := map[string]interface{}{
		"team_name": teamName,
		"members": []map[string]interface{}{
			{"user_id": authorID, "username": "Author", "is_active": true},
			{"user_id": reviewer1ID, "username": "Reviewer 1", "is_active": true},
			{"user_id": reviewer2ID, "username": "Reviewer 2", "is_active": true},
			{"user_id": reviewer3ID, "username": "Reviewer 3", "is_active": true},
		},
	}
	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Ожидался статус 201, получен %d", resp.StatusCode)
	}

	// Проверяю что команда создана
	req, _ = http.NewRequest("GET", baseURL+"/team/get?team_name="+teamName, nil)
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	// Создаю PR
	prData := map[string]string{
		"pull_request_id":   prID,
		"pull_request_name": "Test PR",
		"author_id":         authorID,
	}
	body, _ = json.Marshal(prData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Ожидался статус 201, получен %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(bodyBytes, &result)
	pr := result["pr"].(map[string]interface{})
	reviewers := pr["assigned_reviewers"].([]interface{})

	// Проверяю что автор не в списке ревьюеров
	for _, r := range reviewers {
		if r.(string) == authorID {
			t.Error("Автор не должен быть ревьюером")
		}
	}

	if pr["status"].(string) != "OPEN" {
		t.Error("Статус должен быть OPEN")
	}

	oldReviewerID := reviewers[0].(string)

	// Получаю список PR для ревьюера
	req, _ = http.NewRequest("GET", baseURL+"/users/getReview?user_id="+oldReviewerID, nil)
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	// Переназначаю ревьюера
	reassignData := map[string]string{
		"pull_request_id": prID,
		"old_user_id":     oldReviewerID,
	}
	body, _ = json.Marshal(reassignData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/reassign", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	// Деактивирую пользователя
	setActiveData := map[string]interface{}{
		"user_id":   reviewer3ID,
		"is_active": false,
	}
	body, _ = json.Marshal(setActiveData)
	req, _ = http.NewRequest("POST", baseURL+"/users/setIsActive", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	// Мержу PR
	mergeData := map[string]string{"pull_request_id": prID}
	body, _ = json.Marshal(mergeData)
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	json.Unmarshal(bodyBytes, &result)
	mergedPR := result["pr"].(map[string]interface{})
	if mergedPR["status"].(string) != "MERGED" {
		t.Error("Статус должен быть MERGED")
	}

	// Проверяю идемпотентность merge
	req, _ = http.NewRequest("POST", baseURL+"/pullRequest/merge", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = httpClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Error("Повторный merge должен быть идемпотентным")
	}

	// Пытаюсь переназначить на смерженном PR (должна быть ошибка)
	mergedReviewers := mergedPR["assigned_reviewers"].([]interface{})
	if len(mergedReviewers) > 0 {
		reassignData = map[string]string{
			"pull_request_id": prID,
			"old_user_id":     mergedReviewers[0].(string),
		}
		body, _ = json.Marshal(reassignData)
		req, _ = http.NewRequest("POST", baseURL+"/pullRequest/reassign", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ = httpClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Ожидалась ошибка 409, получен %d", resp.StatusCode)
		}
	}

	// Получаю статистику
	req, _ = http.NewRequest("GET", baseURL+"/statistics", nil)
	resp, _ = httpClient.Do(req)
	bodyBytes, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Ожидался статус 200, получен %d", resp.StatusCode)
	}

	json.Unmarshal(bodyBytes, &result)
	if _, ok := result["user_stats"]; !ok {
		t.Error("Должна быть user_stats")
	}
	if _, ok := result["pr_stats"]; !ok {
		t.Error("Должна быть pr_stats")
	}
}
