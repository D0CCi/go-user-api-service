package handlers

import (
	"net/http"
	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/service"

	"github.com/gin-gonic/gin"
)

// Handlers - это как бы "контроллеры" из мира MVC.
// Они принимают HTTP-запросы, дергают нужные методы из сервиса и отдают ответ.
type Handlers struct {
	service *service.Service
}

func NewHandlers(service *service.Service) *Handlers {
	return &Handlers{service: service}
}

// CreateTeam - ручка для создания новой команды.
// Принимает JSON с названием команды и списком участников.
func (h *Handlers) CreateTeam(c *gin.Context) {
	var team models.Team
	if err := c.ShouldBindJSON(&team); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	if err := h.service.CreateTeam(&team); err != nil {
		// Если команда с таким именем уже есть, возвращаем специальную ошибку.
		if err.Error() == "TEAM_EXISTS" {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorTeamExists,
					Message: "team_name already exists",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"team": team})
}

func (h *Handlers) GetTeam(c *gin.Context) {
	teamName := c.Query("team_name")
	if teamName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "team_name is required",
			},
		})
		return
	}

	team, err := h.service.GetTeam(teamName)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "team not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, team)
}

// Users
func (h *Handlers) SetUserActive(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		IsActive bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	user, err := h.service.SetUserActive(req.UserID, req.IsActive)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "user not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *Handlers) GetReview(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "user_id is required",
			},
		})
		return
	}

	prs, err := h.service.GetPullRequestsByReviewer(userID)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNotFound,
					Message: "user not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":       userID,
		"pull_requests": prs,
	})
}

// Pull Requests

// CreatePullRequest - самая главная ручка. Создает PR и сразу назначает ревьюеров.
func (h *Handlers) CreatePullRequest(c *gin.Context) {
	var req struct {
		PullRequestID   string `json:"pull_request_id" binding:"required"`
		PullRequestName string `json:"pull_request_name" binding:"required"`
		AuthorID        string `json:"author_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	pr, err := h.service.CreatePullRequest(req.PullRequestID, req.PullRequestName, req.AuthorID)
	if err != nil {
		// Тут обрабатываю разные возможные ошибки от сервисного слоя
		if err.Error() == "PR_EXISTS" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorPRExists,
					Message: "PR id already exists",
				},
			})
			return
		}
		if err.Error() == "author not found" {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNotFound,
					Message: "author or team not found",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"pr": pr})
}

func (h *Handlers) MergePullRequest(c *gin.Context) {
	var req struct {
		PullRequestID string `json:"pull_request_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	pr, err := h.service.MergePullRequest(req.PullRequestID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "pull request not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pr": pr})
}

func (h *Handlers) ReassignReviewer(c *gin.Context) {
	var req struct {
		PullRequestID string `json:"pull_request_id" binding:"required"`
		OldUserID     string `json:"old_user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: err.Error(),
			},
		})
		return
	}

	pr, newReviewerID, err := h.service.ReassignReviewer(req.PullRequestID, req.OldUserID)
	if err != nil {
		if err.Error() == "PR_MERGED" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorPRMerged,
					Message: "cannot reassign on merged PR",
				},
			})
			return
		}
		if err.Error() == "NOT_ASSIGNED" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNotAssigned,
					Message: "reviewer is not assigned to this PR",
				},
			})
			return
		}
		if err.Error() == "NO_CANDIDATE" {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: struct {
					Code    models.ErrorCode `json:"code"`
					Message string           `json:"message"`
				}{
					Code:    models.ErrorNoCandidate,
					Message: "no active replacement candidate in team",
				},
			})
			return
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: struct {
				Code    models.ErrorCode `json:"code"`
				Message string           `json:"message"`
			}{
				Code:    models.ErrorNotFound,
				Message: "pull request or user not found",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pr":         pr,
		"replaced_by": newReviewerID,
	})
}

func (h *Handlers) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}


