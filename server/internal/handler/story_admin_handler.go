package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"biodays/server/internal/model"
	"biodays/server/internal/seed"
	"biodays/server/internal/service"

	"github.com/gin-gonic/gin"
)

// maxStoryBodyBytes caps a single publish request. MaxStoryBatch stories with
// long-form content fit comfortably; anything larger is a client bug.
const maxStoryBodyBytes = 16 << 20

type StoryAdminHandler struct {
	admin   *service.StoryAdminService
	stories *service.StoryService
	logger  *slog.Logger
}

func NewStoryAdminHandler(admin *service.StoryAdminService, stories *service.StoryService, logger *slog.Logger) *StoryAdminHandler {
	return &StoryAdminHandler{admin: admin, stories: stories, logger: logger}
}

type AdminStoryResponse struct {
	Data model.StoryDTO `json:"data"`
}

type AdminStoryUpsertResponse struct {
	Data service.StoryUpsertItem `json:"data"`
}

type AdminStoryBatchResponse struct {
	Data service.StoryBatchResult `json:"data"`
}

type adminStoryListQuery struct {
	Month    *int   `form:"month"`
	Day      *int   `form:"day"`
	Tag      string `form:"tag"`
	Q        string `form:"q"`
	Page     *int   `form:"page"`
	PageSize *int   `form:"page_size"`
	View     string `form:"view"`
}

// List godoc
// @Summary      管理端故事列表
// @Description  与公开列表相同的过滤参数，默认 summary 视图；需要管理员会话或 API Token。
// @Tags         admin-stories
// @Produce      json
// @Param        month      query     int     false  "月份 1-12"
// @Param        day        query     int     false  "日期 1-31（需配合 month）"
// @Param        tag        query     string  false  "标签名"
// @Param        q          query     string  false  "标题/摘要/条目关键词"
// @Param        page       query     int     false  "页码，默认 1"
// @Param        page_size  query     int     false  "每页数量，默认 20，最大 100"
// @Param        view       query     string  false  "summary（默认）或 full"
// @Success      200        {object}  StorySummaryListResponse
// @Failure      400        {object}  ErrorResponse
// @Failure      401        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Router       /api/v1/admin/stories [get]
func (h *StoryAdminHandler) List(c *gin.Context) {
	var query adminStoryListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid query parameters: "+err.Error())
		return
	}
	if query.Day != nil && query.Month == nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "day requires month")
		return
	}
	page := 1
	if query.Page != nil {
		page = *query.Page
	}
	pageSize := 20
	if query.PageSize != nil {
		pageSize = *query.PageSize
	}
	view := query.View
	if view == "" {
		view = "summary"
	}
	if view != "summary" && view != "full" {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "view must be summary or full")
		return
	}

	stories, total, err := h.stories.List(c.Request.Context(), service.ListParams{
		Month: query.Month, Day: query.Day,
		Tag: strings.TrimSpace(query.Tag), Query: strings.TrimSpace(query.Q),
		Page: page, PageSize: pageSize, Summary: view == "summary",
	})
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	if view == "summary" {
		summaries := make([]model.StorySummaryDTO, 0, len(stories))
		for _, story := range stories {
			summaries = append(summaries, story.ToSummary())
		}
		c.JSON(http.StatusOK, StorySummaryListResponse{Data: summaries, Meta: ListMeta{Total: total, Page: page, PageSize: pageSize}})
		return
	}
	c.JSON(http.StatusOK, StoryListResponse{Data: stories, Meta: ListMeta{Total: total, Page: page, PageSize: pageSize}})
}

// Get godoc
// @Summary      管理端故事详情
// @Description  返回单条故事的完整字段。
// @Tags         admin-stories
// @Produce      json
// @Param        id  path  string  true  "故事 id"
// @Success      200 {object} AdminStoryResponse
// @Failure      404 {object} ErrorResponse
// @Router       /api/v1/admin/stories/{id} [get]
func (h *StoryAdminHandler) Get(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	story, err := h.stories.Get(c.Request.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, CodeStoryNotFound, "story "+id+" not found")
	case err != nil:
		internalError(c, h.logger, err)
	default:
		c.JSON(http.StatusOK, AdminStoryResponse{Data: story})
	}
}

// Upsert godoc
// @Summary      发布/更新故事
// @Description  请求体为单篇故事对象，或 {"stories":[...]} 批量（≤200 篇）。按 id 幂等 upsert；批量逐条报告，单篇校验失败返回 400。
// @Tags         admin-stories
// @Accept       json
// @Produce      json
// @Success      200 {object} AdminStoryBatchResponse
// @Success      201 {object} AdminStoryUpsertResponse
// @Failure      400 {object} ErrorResponse
// @Failure      413 {object} ErrorResponse
// @Failure      415 {object} ErrorResponse
// @Router       /api/v1/admin/stories [post]
func (h *StoryAdminHandler) Upsert(c *gin.Context) {
	if c.ContentType() != "application/json" {
		fail(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxStoryBodyBytes))
	if err != nil {
		fail(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, "request body is too large")
		return
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "request body is not valid JSON: "+err.Error())
		return
	}

	if batchRaw, isBatch := probe["stories"]; isBatch {
		var body struct {
			Stories []json.RawMessage `json:"stories"`
		}
		if err := json.Unmarshal(batchRaw, &body.Stories); err != nil {
			fail(c, http.StatusBadRequest, CodeInvalidParam, "stories must be an array: "+err.Error())
			return
		}
		if len(body.Stories) == 0 {
			fail(c, http.StatusBadRequest, CodeInvalidParam, "stories must not be empty")
			return
		}
		if len(body.Stories) > service.MaxStoryBatch {
			fail(c, http.StatusBadRequest, CodeInvalidParam, "stories exceeds the batch limit of 200 items")
			return
		}
		entries := make([]seed.StoryInput, 0, len(body.Stories))
		for _, item := range body.Stories {
			var entry seed.StoryInput
			if err := json.Unmarshal(item, &entry); err != nil {
				fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid story entry: "+err.Error())
				return
			}
			entries = append(entries, entry)
		}
		result, err := h.admin.UpsertBatch(c.Request.Context(), entries)
		if err != nil {
			internalError(c, h.logger, err)
			return
		}
		c.JSON(http.StatusOK, AdminStoryBatchResponse{Data: result})
		return
	}

	var entry seed.StoryInput
	if err := json.Unmarshal(raw, &entry); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid story payload: "+err.Error())
		return
	}
	item, err := h.admin.Upsert(c.Request.Context(), entry)
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	if len(item.Problems) > 0 {
		failFields(c, http.StatusBadRequest, CodeInvalidParam, "story validation failed",
			map[string]string{"story": strings.Join(item.Problems, "; ")})
		return
	}
	status := http.StatusOK
	if item.Created {
		status = http.StatusCreated
	}
	c.JSON(status, AdminStoryUpsertResponse{Data: item})
}

// Update godoc
// @Summary      部分更新故事
// @Description  按 id 打补丁；出现的字段覆盖原值，缺席字段保持不变，id 不可改。
// @Tags         admin-stories
// @Accept       json
// @Produce      json
// @Param        id   path  string  true  "故事 id"
// @Success      200  {object} AdminStoryResponse
// @Failure      400  {object} ErrorResponse
// @Failure      404  {object} ErrorResponse
// @Router       /api/v1/admin/stories/{id} [patch]
func (h *StoryAdminHandler) Update(c *gin.Context) {
	if c.ContentType() != "application/json" {
		fail(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	var patch service.StoryPatch
	if err := c.ShouldBindJSON(&patch); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid patch payload: "+err.Error())
		return
	}
	story, problems, err := h.admin.Update(c.Request.Context(), id, patch)
	switch {
	case len(problems) > 0:
		failFields(c, http.StatusBadRequest, CodeInvalidParam, "story validation failed",
			map[string]string{"story": strings.Join(problems, "; ")})
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, CodeStoryNotFound, "story "+id+" not found")
	case err != nil:
		internalError(c, h.logger, err)
	default:
		c.JSON(http.StatusOK, AdminStoryResponse{Data: story})
	}
}

// Delete godoc
// @Summary      删除故事
// @Description  按 id 删除故事并清理孤立标签。
// @Tags         admin-stories
// @Param        id  path  string  true  "故事 id"
// @Success      204
// @Failure      404 {object} ErrorResponse
// @Router       /api/v1/admin/stories/{id} [delete]
func (h *StoryAdminHandler) Delete(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if err := h.admin.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			fail(c, http.StatusNotFound, CodeStoryNotFound, "story "+id+" not found")
			return
		}
		internalError(c, h.logger, err)
		return
	}
	c.Status(http.StatusNoContent)
}
