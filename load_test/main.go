package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL        = "http://localhost:8080"
	targetRPS      = 5
	maxLatencyMS   = 300
	minSuccessRate = 99.9
	duration       = 60
	requestDelay   = 200 * time.Millisecond
)

var (
	successCount   int64
	errorCount     int64
	totalLatency   int64
	requestCount   int64
	latencySamples []int64
	latencyMutex   sync.Mutex
)

func main() {
	log.Println("Начинаю нагрузочное тестирование")

	if !checkServiceHealth() {
		log.Fatal("Сервис не отвечает, запустите docker-compose up")
	}

	log.Println("Создаю тестовые данные...")
	if err := setupTestData(); err != nil {
		log.Fatalf("Не получилось создать данные: %v", err)
	}

	log.Println("Запускаю тест на 60 секунд...")
	startTime := time.Now()
	endTime := startTime.Add(duration * time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(endTime) {
				makeRequest()
				atomic.AddInt64(&requestCount, 1)
				time.Sleep(requestDelay)
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	printResults(elapsed)
}

func checkServiceHealth() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func setupTestData() error {
	// создаю команду для тестов
	teamData := map[string]interface{}{
		"team_name": "test-team",
		"members": []map[string]interface{}{
			{"user_id": "user1", "username": "User 1", "is_active": true},
			{"user_id": "user2", "username": "User 2", "is_active": true},
			{"user_id": "user3", "username": "User 3", "is_active": true},
			{"user_id": "user4", "username": "User 4", "is_active": true},
		},
	}

	body, _ := json.Marshal(teamData)
	req, _ := http.NewRequest("POST", baseURL+"/team/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// создаю несколько PR для тестов
	for i := 0; i < 5; i++ {
		prData := map[string]string{
			"pull_request_id":   fmt.Sprintf("pr-%d", i+1),
			"pull_request_name": fmt.Sprintf("Test PR %d", i+1),
			"author_id":         "user1",
		}
		body, _ := json.Marshal(prData)
		req, _ := http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		client.Do(req)
	}

	return nil
}

func makeRequest() {
	// случайно выбираю эндпоинт для теста
	endpoints := []func() (int64, bool){
		testHealthCheck,
		testGetTeam,
		testGetStatistics,
		testGetReview,
		testCreatePR,
	}

	fn := endpoints[rand.Intn(len(endpoints))]
	latency, success := fn()

	if success {
		atomic.AddInt64(&successCount, 1)
		atomic.AddInt64(&totalLatency, latency)
		latencyMutex.Lock()
		latencySamples = append(latencySamples, latency)
		latencyMutex.Unlock()
	} else {
		atomic.AddInt64(&errorCount, 1)
	}
}

func testHealthCheck() (int64, bool) {
	start := time.Now()
	resp, err := http.Get(baseURL + "/health")
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()
	return latency, resp.StatusCode == http.StatusOK
}

func testGetTeam() (int64, bool) {
	url := baseURL + "/team/get?team_name=test-team"
	start := time.Now()
	resp, err := http.Get(url)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()
	return latency, resp.StatusCode == http.StatusOK
}

func testGetStatistics() (int64, bool) {
	start := time.Now()
	resp, err := http.Get(baseURL + "/statistics")
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()
	return latency, resp.StatusCode == http.StatusOK
}

func testGetReview() (int64, bool) {
	url := baseURL + "/users/getReview?user_id=user1"
	start := time.Now()
	resp, err := http.Get(url)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()
	return latency, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound
}

func testCreatePR() (int64, bool) {
	prData := map[string]string{
		"pull_request_id":   fmt.Sprintf("pr-load-%d", time.Now().UnixNano()),
		"pull_request_name": "Load Test PR",
		"author_id":         "user1",
	}
	body, _ := json.Marshal(prData)
	req, _ := http.NewRequest("POST", baseURL+"/pullRequest/create", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return latency, resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict
}

func printResults(elapsed time.Duration) {
	total := atomic.LoadInt64(&requestCount)
	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)
	totalLat := atomic.LoadInt64(&totalLatency)

	var avgLatency float64
	if success > 0 {
		avgLatency = float64(totalLat) / float64(success)
	}

	successRate := float64(success) / float64(total) * 100
	rps := float64(total) / elapsed.Seconds()

	// считаю перцентили
	latencyMutex.Lock()
	samples := make([]int64, len(latencySamples))
	copy(samples, latencySamples)
	latencyMutex.Unlock()

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	var p50, p95, p99 float64
	var minLat, maxLat int64 = 999999999, 0

	if len(samples) > 0 {
		minLat = samples[0]
		maxLat = samples[len(samples)-1]
		p50 = float64(samples[len(samples)*50/100])
		p95 = float64(samples[len(samples)*95/100])
		p99 = float64(samples[len(samples)*99/100])
	}

	fmt.Println("\n==========================================")
	fmt.Println("Результаты тестирования")
	fmt.Println("==========================================")
	fmt.Printf("Время: %v\n", elapsed.Round(time.Second))
	fmt.Printf("Всего запросов: %d\n", total)
	fmt.Printf("Успешных: %d\n", success)
	fmt.Printf("Ошибок: %d\n", errors)
	fmt.Printf("Процент успеха: %.2f%%\n", successRate)
	fmt.Printf("RPS: %.2f\n", rps)
	fmt.Printf("\nЗадержка:\n")
	fmt.Printf("  Средняя: %.2f ms\n", avgLatency)
	fmt.Printf("  P50: %.2f ms\n", p50)
	fmt.Printf("  P95: %.2f ms\n", p95)
	fmt.Printf("  P99: %.2f ms\n", p99)
	fmt.Printf("  Min: %d ms, Max: %d ms\n", minLat, maxLat)

	fmt.Printf("\nПроверка требований:\n")
	latencyOK := avgLatency <= float64(maxLatencyMS)
	successOK := successRate >= minSuccessRate
	rpsOK := rps >= float64(targetRPS)*0.8

	if latencyOK {
		fmt.Printf("  OK Задержка: %.2f ms <= %d ms\n", avgLatency, maxLatencyMS)
	} else {
		fmt.Printf("  FAIL Задержка: %.2f ms > %d ms\n", avgLatency, maxLatencyMS)
	}

	if successOK {
		fmt.Printf("  OK Успешность: %.2f%% >= %.1f%%\n", successRate, minSuccessRate)
	} else {
		fmt.Printf("  FAIL Успешность: %.2f%% < %.1f%%\n", successRate, minSuccessRate)
	}

	if rpsOK {
		fmt.Printf("  OK RPS: %.2f >= %.1f\n", rps, float64(targetRPS)*0.8)
	} else {
		fmt.Printf("  FAIL RPS: %.2f < %.1f\n", rps, float64(targetRPS)*0.8)
	}

	allOK := latencyOK && successOK && rpsOK
	fmt.Println("\n==========================================")
	if allOK {
		fmt.Println("Все требования выполнены")
	} else {
		fmt.Println("Есть проблемы")
	}
	fmt.Println("==========================================")
}
