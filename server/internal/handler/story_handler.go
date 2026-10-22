package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"biodays/server/internal/service"

	"github.com/gin-gonic/gin"
)

type StoryHandler struct {
	service *service.StoryService
	logger  *slog.Logger
}

func NewStoryHandler(svc *service.StoryService, logger *slog.Logger) *StoryHandler {
	return &StoryHandler{service: svc, logger: logger}
}

type todayQuery struct {
	TZ       string `form:"tz"`
	Fallback string `form:"fallback"`
}

// Today godoc
// @Summary      获取今日故事
// @Description  按指定时区返回"今天"对应的故事；当日无数据时按 fallback 策略返回随机一篇并标记 is_fallback。
// @Tags         stories
// @Produce      json
// @Param        tz        query     string  false  "时区偏移，例如 +08:00"  default(+08:00)
// @Param        fallback  query     string  false  "random 或 none"        Enums(random, none)  default(random)
// @Success      200       {object}  TodayResponse
// @Failure      400       {object}  ErrorResponse
// @Failure      404       {object}  ErrorResponse
// @Failure      429       {object}  ErrorResponse
// @Failure      500       {object}  ErrorResponse
// @Router       /api/v1/today [get]
func (h *StoryHandler) Today(c *gin.Context) {
	var query todayQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "invalid query parameters: "+err.Error())
		return
	}
	if query.TZ == "" {
		query.TZ = "+08:00"
	}
	if query.Fallback == "" {
		query.Fallback = service.FallbackRandom
	}
	if query.Fallback != service.FallbackRandom && query.Fallback != service.FallbackNone {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "fallback must be one of: random, none")
		return
	}

	story, isFallback, err := h.service.Today(c.Request.Context(), query.TZ, query.Fallback)
	switch {
	case errors.Is(err, service.ErrInvalidOffset):
		fail(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, CodeStoryNotFound, "no story for today")
		return
	case err != nil:
		internalError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, TodayResponse{
		Data: story,
		Meta: TodayMeta{IsFallback: isFallback},
	})
}

type listQuery struct {
	Month    *int   `form:"month" binding:"omitempty,min=1,max=12"`
	Day      *int   `form:"day" binding:"omitempty,min=1,max=31"`
	Tag      string `form:"tag" binding:"omitempty,max=64"`
	Q        string `form:"q" binding:"omitempty,max=100"`
	Page     *int   `form:"page" binding:"omitempty,min=1"`
	PageSize *int   `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// List godoc
// @Summary      故事列表
// @Description  支持按月/日、标签、关键词筛选，固定按 month, day, year 排序。
// @Tags         stories
// @Produce      json
// @Param        month      query     int     false  "月份 1-12"
// @Param        day        query     int     false  "日期 1-31，需与 month 同时使用"
// @Param        tag        query     string  false  "标签精确匹配"
// @Param        q          query     string  false  "关键词，匹配 title/summary/lab_item"
// @Param        page       query     int     false  "页码，默认 1"
// @Param        page_size  query     int     false  "每页数量，默认 20，最大 100"
// @Success      200        {object}  StoryListResponse
// @Failure      400        {object}  ErrorResponse
// @Failure      429        {object}  ErrorResponse
// @Failure      500        {object}  ErrorResponse
// @Router       /api/v1/stories [get]
func (h *StoryHandler) List(c *gin.Context) {
	var query listQuery
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

	stories, total, err := h.service.List(c.Request.Context(), service.ListParams{
		Month:    query.Month,
		Day:      query.Day,
		Tag:      strings.TrimSpace(query.Tag),
		Query:    strings.TrimSpace(query.Q),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		internalError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, StoryListResponse{
		Data: stories,
		Meta: ListMeta{Total: total, Page: page, PageSize: pageSize},
	})
}

// Get godoc
// @Summary      故事详情
// @Description  按业务 id 返回单条故事。
// @Tags         stories
// @Produce      json
// @Param        id   path      string  true  "故事 id，例如 Q1-01"
// @Success      200  {object}  model.StoryDTO
// @Failure      404  {object}  ErrorResponse
// @Failure      429  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/stories/{id} [get]
func (h *StoryHandler) Get(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	story, err := h.service.Get(c.Request.Context(), id)
	switch {
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, CodeStoryNotFound, "story "+id+" not found")
		return
	case err != nil:
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, story)
}

// Random godoc
// @Summary      随机故事
// @Description  随机返回一条故事，可通过 exclude 排除当前正在展示的 id。
// @Tags         stories
// @Produce      json
// @Param        exclude  query     string  false  "逗号分隔的 id 列表，最多 100 个"
// @Success      200      {object}  model.StoryDTO
// @Failure      400      {object}  ErrorResponse
// @Failure      404      {object}  ErrorResponse
// @Failure      429      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /api/v1/stories/random [get]
func (h *StoryHandler) Random(c *gin.Context) {
	exclude, err := parseExclude(c.Query("exclude"))
	if err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
		return
	}

	story, err := h.service.Random(c.Request.Context(), exclude)
	switch {
	case errors.Is(err, service.ErrNotFound):
		fail(c, http.StatusNotFound, CodeStoryNotFound, "no stories available")
		return
	case err != nil:
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, story)
}

func parseExclude(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	if len(raw) > 2000 {
		return nil, errors.New("exclude is too long")
	}
	parts := strings.Split(raw, ",")
	ids := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) > 100 {
		return nil, errors.New("exclude accepts at most 100 ids")
	}
	return ids, nil
}

// Tags godoc
// @Summary      标签列表
// @Description  返回全部标签及对应故事数量，按数量倒序。
// @Tags         meta
// @Produce      json
// @Success      200  {object}  TagsResponse
// @Failure      429  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/tags [get]
func (h *StoryHandler) Tags(c *gin.Context) {
	counts, err := h.service.Tags(c.Request.Context())
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, TagsResponse{Data: counts})
}

// Meta godoc
// @Summary      数据集概况
// @Description  返回故事总数、覆盖季度与月份范围。
// @Tags         meta
// @Produce      json
// @Success      200  {object}  MetaResponse
// @Failure      429  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /api/v1/meta [get]
func (h *StoryHandler) Meta(c *gin.Context) {
	meta, err := h.service.Meta(c.Request.Context())
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	c.JSON(http.StatusOK, MetaResponse{Data: meta})
}
