package handler

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"biodays/server/internal/service"
	"github.com/gin-gonic/gin"
)

const historyCookie = "biodays_read_v1"
const historyMaxAge = 90 * 24 * 60 * 60
const historyMaxBytes = 3500
const historyMaxDecoded = 32 * 1024
const historyMaxIDs = 2048

// History godoc
// @Summary 记录已浏览故事
// @Description 将已读 ID 保存到当前浏览器的 HttpOnly Cookie，90 天过期；不会写入 SQLite。
// @Tags stories
// @Accept json
// @Param body body HistoryRequest true "已展示给用户的故事"
// @Success 204 "已记录"
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/history [post]
func (h *StoryHandler) History(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var body HistoryRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidParam, "valid story id required")
		return
	}
	_, err := h.service.Get(c.Request.Context(), body.ID)
	if errors.Is(err, service.ErrNotFound) {
		fail(c, http.StatusNotFound, CodeStoryNotFound, "story not found")
		return
	}
	if err != nil {
		internalError(c, h.logger, err)
		return
	}
	writeHistory(c, appendHistory(readHistory(c.Request), body.ID))
	c.Status(http.StatusNoContent)
}

type HistoryRequest struct {
	ID string `json:"id" binding:"required,max=16"`
}

func readHistory(request *http.Request) []string {
	cookie, err := request.Cookie(historyCookie)
	if err != nil || len(cookie.Value) > historyMaxBytes {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}
	reader, err := zlib.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil
	}
	defer reader.Close()
	decoded, err := io.ReadAll(io.LimitReader(reader, historyMaxDecoded+1))
	if err != nil || len(decoded) > historyMaxDecoded {
		return nil
	}
	var ids []string
	if json.Unmarshal(decoded, &ids) != nil || len(ids) > historyMaxIDs {
		return nil
	}
	for _, id := range ids {
		if len(id) == 0 || len(id) > 16 || strings.TrimSpace(id) != id {
			return nil
		}
	}
	return ids
}

func appendHistory(ids []string, id string) []string {
	result := make([]string, 0, len(ids)+1)
	for _, existing := range ids {
		if existing != id {
			result = append(result, existing)
		}
	}
	result = append(result, id)
	if len(result) > historyMaxIDs {
		result = result[len(result)-historyMaxIDs:]
	}
	return result
}

func writeHistory(c *gin.Context, ids []string) {
	// Compress IDs rather than truncate at ~100 stories: all 408 current stories fit
	// comfortably in one cookie. Bound both encoded and decoded sizes for future growth.
	for {
		raw, _ := json.Marshal(ids)
		var buffer bytes.Buffer
		writer := zlib.NewWriter(&buffer)
		_, _ = writer.Write(raw)
		_ = writer.Close()
		value := base64.RawURLEncoding.EncodeToString(buffer.Bytes())
		if len(value) <= historyMaxBytes && len(raw) <= historyMaxDecoded {
			http.SetCookie(c.Writer, &http.Cookie{
				Name: historyCookie, Value: value, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Secure:   c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"),
				MaxAge:   historyMaxAge, Expires: time.Now().Add(historyMaxAge * time.Second),
			})
			return
		}
		ids = ids[len(ids)/4+1:]
	}
}
