package chatwiki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"chatclaw/internal/sqlite"

	"github.com/uptrace/bun"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Binding represents a ChatWiki binding record exposed to the frontend.
type Binding struct {
	ID        int64  `json:"id"`
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	TTL       int64  `json:"ttl"`
	Exp       int64  `json:"exp"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// Robot represents a ChatWiki robot/application item returned to the frontend.
type Robot struct {
	ID                  string `json:"id"`
	RobotKey            string `json:"robot_key"`
	Name                string `json:"name"`
	Intro               string `json:"intro"`
	Type                string `json:"type"`
	Icon                string `json:"icon"`
	SwitchStatus        int    `json:"chat_claw_switch_status"`
	ApplicationTypeCode string `json:"application_type"`
}

// Library represents a ChatWiki knowledge base item returned to the frontend.
type Library struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Intro        string `json:"intro"`
	Type         string `json:"type"`
	TypeName     string `json:"type_name"`
	SwitchStatus int    `json:"chat_claw_switch_status"`
}

// LibraryGroup represents a ChatWiki file group in a knowledge base.
type LibraryGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Total int    `json:"total"`
}

// LibraryFile represents a ChatWiki file item in a knowledge base.
type LibraryFile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
	Status    int    `json:"status"`
	UpdatedAt string `json:"updated_at"`
	ThumbPath string `json:"thumb_path"`
}

// LibraryParagraph represents a QA paragraph item in a knowledge base.
type LibraryParagraph struct {
	ID       string   `json:"id"`
	Question string   `json:"question"`
	Answer   string   `json:"answer"`
	Images   []string `json:"images"`
}

// LibraryParagraphPage represents a page result for QA paragraphs.
// Total is -1 when upstream does not return a reliable total count.
type LibraryParagraphPage struct {
	List  []LibraryParagraph `json:"list"`
	Total int                `json:"total"`
}

type chatWikiRobotRaw struct {
	ID              string `json:"id"`
	RobotKey        string `json:"robot_key"`
	RobotName       string `json:"robot_name"`
	RobotIntro      string `json:"robot_intro"`
	RobotAvatar     string `json:"robot_avatar"`
	ApplicationType string `json:"application_type"`
	SwitchStatus    string `json:"chat_claw_switch_status"`
}

type chatWikiLibraryRaw struct {
	ID           string `json:"id"`
	LibraryName  string `json:"library_name"`
	LibraryIntro string `json:"library_intro"`
	Type         string `json:"type"`
	SwitchStatus string `json:"chat_claw_switch_status"`
}

// bindingModel is the bun ORM model for the chatwiki_bindings table.
type bindingModel struct {
	bun.BaseModel `bun:"table:chatwiki_bindings"`

	ID        int64     `bun:"id,pk,autoincrement"`
	ServerURL string    `bun:"server_url,notnull"`
	Token     string    `bun:"token,notnull"`
	TTL       int64     `bun:"ttl,notnull"`
	Exp       int64     `bun:"exp,notnull"`
	UserID    string    `bun:"user_id,notnull"`
	UserName  string    `bun:"user_name,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
	UpdatedAt time.Time `bun:"updated_at,notnull"`
}

// ChatWikiService exposes ChatWiki binding operations to the frontend via Wails.
type ChatWikiService struct {
	app *application.App
}

func NewChatWikiService(app *application.App) *ChatWikiService {
	return &ChatWikiService{app: app}
}

// GetBinding returns the current binding, or nil if none exists.
func (s *ChatWikiService) GetBinding() (*Binding, error) {
	db := sqlite.DB()
	if db == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m := new(bindingModel)
	err := db.NewSelect().Model(m).OrderExpr("id DESC").Limit(1).Scan(ctx)
	if err != nil {
		return nil, nil
	}
	return toBinding(m), nil
}

// SaveBinding creates or replaces the binding. Called from deeplink handler.
func SaveBinding(app *application.App, serverURL, token, ttl, exp, userID, userName string) error {
	db := sqlite.DB()
	if db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ttlInt, _ := strconv.ParseInt(ttl, 10, 64)
	expInt, _ := strconv.ParseInt(exp, 10, 64)
	now := time.Now().UTC()

	// Delete old bindings, keep only latest
	if _, err := db.NewDelete().Model((*bindingModel)(nil)).Where("1=1").Exec(ctx); err != nil {
		app.Logger.Warn("Failed to delete old chatwiki bindings", "error", err)
	}

	m := &bindingModel{
		ServerURL: serverURL,
		Token:     token,
		TTL:       ttlInt,
		Exp:       expInt,
		UserID:    userID,
		UserName:  userName,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := db.NewInsert().Model(m).Exec(ctx)
	if err != nil {
		app.Logger.Error("Failed to save chatwiki binding", "error", err)
		return err
	}
	app.Logger.Info("ChatWiki binding saved", "user_id", userID, "user_name", userName)
	return nil
}

// DeleteBinding removes the current binding.
func (s *ChatWikiService) DeleteBinding() error {
	db := sqlite.DB()
	if db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().Model((*bindingModel)(nil)).Where("1=1").Exec(ctx)
	return err
}

// GetRobotList fetches the robot/application list from ChatWiki API.
func (s *ChatWikiService) GetRobotList() ([]Robot, error) {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return nil, fmt.Errorf("no binding found")
	}

	baseURL := strings.TrimRight(binding.ServerURL, "/")
	q := url.Values{}
	q.Set("application_type", "-1")
	q.Set("only_open", "0")
	apiURL := baseURL + "/manage/chatclaw/getRobotList?" + q.Encode()

	s.app.Logger.Info("[ChatWiki] GetRobotList request",
		"url", apiURL,
		"token_length", len(binding.Token),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Token", binding.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.app.Logger.Error("[ChatWiki] GetRobotList request failed", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	s.app.Logger.Info("[ChatWiki] GetRobotList response",
		"status", resp.StatusCode,
		"body", string(body),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Res  int                `json:"res"`
		Code int                `json:"code"`
		Msg  string             `json:"msg"`
		Data []chatWikiRobotRaw `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	resultCode := apiResp.Res
	if resultCode == 0 && apiResp.Code != 0 {
		resultCode = apiResp.Code
	}
	if resultCode != 0 {
		return nil, fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
	}

	robots := make([]Robot, 0, len(apiResp.Data))
	for _, item := range apiResp.Data {
		robotType := "chat"
		if item.ApplicationType == "1" {
			robotType = "workflow"
		}
		switchStatus := parseSwitchStatus(item.SwitchStatus)
		fullAvatar := normalizeAssetURL(binding.ServerURL, item.RobotAvatar)
		s.app.Logger.Info("[ChatWiki] robot avatar resolved",
			"robot_id", item.ID,
			"robot_name", item.RobotName,
			"raw_robot_avatar", item.RobotAvatar,
			"full_robot_avatar", fullAvatar,
			"switch_status", switchStatus,
		)
		robots = append(robots, Robot{
			ID:                  item.ID,
			RobotKey:            item.RobotKey,
			Name:                item.RobotName,
			Intro:               item.RobotIntro,
			Type:                robotType,
			Icon:                fullAvatar,
			SwitchStatus:        switchStatus,
			ApplicationTypeCode: item.ApplicationType,
		})
	}
	return robots, nil
}

// GetLibraryList fetches the full knowledge base list from ChatWiki API.
// libType: 0=normal, 2=QA, 3=wechat-official-account
func (s *ChatWikiService) GetLibraryList(libType int) ([]Library, error) {
	return s.getLibraryList(libType, 0)
}

// GetLibraryListOnlyOpen fetches only enabled knowledge bases from ChatWiki API.
// libType: 0=normal, 2=QA, 3=wechat-official-account
func (s *ChatWikiService) GetLibraryListOnlyOpen(libType int) ([]Library, error) {
	return s.getLibraryList(libType, 1)
}

func (s *ChatWikiService) getLibraryList(libType int, onlyOpen int) ([]Library, error) {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return nil, fmt.Errorf("no binding found")
	}

	baseURL := strings.TrimRight(binding.ServerURL, "/")
	q := url.Values{}
	q.Set("type", strconv.Itoa(libType))
	q.Set("only_open", strconv.Itoa(onlyOpen))
	apiURL := baseURL + "/manage/chatclaw/getLibraryList?" + q.Encode()

	s.app.Logger.Info("[ChatWiki] GetLibraryList request",
		"url", apiURL,
		"type", libType,
		"only_open", onlyOpen,
		"token_length", len(binding.Token),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Token", binding.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.app.Logger.Error("[ChatWiki] GetLibraryList request failed", "error", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	s.app.Logger.Info("[ChatWiki] GetLibraryList response",
		"status", resp.StatusCode,
		"type", libType,
		"only_open", onlyOpen,
		"body", string(body),
	)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Res  int                  `json:"res"`
		Code int                  `json:"code"`
		Msg  string               `json:"msg"`
		Data []chatWikiLibraryRaw `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	resultCode := apiResp.Res
	if resultCode == 0 && apiResp.Code != 0 {
		resultCode = apiResp.Code
	}
	if resultCode != 0 {
		return nil, fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
	}

	libraries := make([]Library, 0, len(apiResp.Data))
	for _, item := range apiResp.Data {
		typeName := "normal"
		switch item.Type {
		case "2":
			typeName = "qa"
		case "3":
			typeName = "wechat"
		}
		libraries = append(libraries, Library{
			ID:           item.ID,
			Name:         item.LibraryName,
			Intro:        item.LibraryIntro,
			Type:         item.Type,
			TypeName:     typeName,
			SwitchStatus: parseSwitchStatus(item.SwitchStatus),
		})
	}
	return libraries, nil
}

// GetLibraryGroup fetches folder groups for the specified knowledge base.
// groupType is fixed to 1 for chatclaw integration.
func (s *ChatWikiService) GetLibraryGroup(libraryID string, groupType int) ([]LibraryGroup, error) {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return nil, fmt.Errorf("no binding found")
	}
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return nil, fmt.Errorf("library_id is required")
	}
	if groupType <= 0 {
		groupType = 1
	}

	baseURL := strings.TrimRight(binding.ServerURL, "/")
	q := url.Values{}
	q.Set("library_id", libraryID)
	q.Set("group_type", strconv.Itoa(groupType))
	apiURL := baseURL + "/manage/chatclaw/getLibraryGroup?" + q.Encode()

	s.app.Logger.Info("[ChatWiki] GetLibraryGroup request",
		"url", apiURL,
		"library_id", libraryID,
		"group_type", groupType,
	)

	body, err := s.chatWikiGET(binding.Token, apiURL)
	if err != nil {
		return nil, err
	}

	items, err := decodeAPIDataObjectArray(body)
	if err != nil {
		return nil, err
	}

	result := make([]LibraryGroup, 0, len(items)+1)
	allGroupTotal := 0
	for _, item := range items {
		id := getStringByKeys(item, "group_id", "id")
		if id == "" {
			continue
		}
		name := getStringByKeys(item, "group_name", "name")
		total := getIntByKeys(item, "total", "count", "total_count", "total_num", "file_count")
		allGroupTotal += total
		result = append(result, LibraryGroup{
			ID:    id,
			Name:  name,
			Total: total,
		})
	}
	result = append([]LibraryGroup{{
		ID:    "-1",
		Name:  "全部分组",
		Total: allGroupTotal,
	}}, result...)
	return result, nil
}

// GetLibFileList fetches the file list in a knowledge base.
// groupID is optional; pass an empty string to query all files.
func (s *ChatWikiService) GetLibFileList(
	libraryID string,
	status string,
	page int,
	size int,
	sortField string,
	sortType string,
	groupID string,
	fileName string,
) ([]LibraryFile, error) {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return nil, fmt.Errorf("no binding found")
	}
	libraryID = strings.TrimSpace(libraryID)
	if libraryID == "" {
		return nil, fmt.Errorf("library_id is required")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	baseURL := strings.TrimRight(binding.ServerURL, "/")
	q := url.Values{}
	q.Set("library_id", libraryID)
	if strings.TrimSpace(status) != "" {
		q.Set("status", strings.TrimSpace(status))
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	if strings.TrimSpace(sortField) != "" {
		q.Set("sort_field", strings.TrimSpace(sortField))
	}
	if strings.TrimSpace(sortType) != "" {
		q.Set("sort_type", strings.TrimSpace(sortType))
	}
	if strings.TrimSpace(groupID) != "" {
		q.Set("group_id", strings.TrimSpace(groupID))
	}
	if strings.TrimSpace(fileName) != "" {
		q.Set("file_name", strings.TrimSpace(fileName))
	}
	apiURL := baseURL + "/manage/chatclaw/getLibFileList?" + q.Encode()

	s.app.Logger.Info("[ChatWiki] GetLibFileList request",
		"url", apiURL,
		"library_id", libraryID,
		"status", strings.TrimSpace(status),
		"page", page,
		"size", size,
		"sort_field", strings.TrimSpace(sortField),
		"sort_type", strings.TrimSpace(sortType),
		"group_id", strings.TrimSpace(groupID),
		"file_name", strings.TrimSpace(fileName),
	)

	body, err := s.chatWikiGET(binding.Token, apiURL)
	if err != nil {
		return nil, err
	}

	items, err := decodeAPIDataObjectArray(body)
	if err != nil {
		return nil, err
	}

	result := make([]LibraryFile, 0, len(items))
	for _, item := range items {
		id := getStringByKeys(item, "id", "file_id")
		if id == "" {
			continue
		}
		name := getStringByKeys(item, "file_name", "name", "origin_name")
		if name == "" {
			name = id
		}
		ext := strings.TrimPrefix(getStringByKeys(item, "extension", "ext"), ".")
		statusInt := getIntByKeys(item, "status", "parse_status")
		updatedAt := getStringByKeys(item, "updated_at", "create_time", "created_at")
		thumbPath := normalizeAssetURL(binding.ServerURL, getStringByKeys(item, "thumb_path"))

		result = append(result, LibraryFile{
			ID:        id,
			Name:      name,
			Extension: ext,
			Status:    statusInt,
			UpdatedAt: updatedAt,
			ThumbPath: thumbPath,
		})
	}
	return result, nil
}

// GetParagraphList fetches paragraph list for QA knowledge base.
// libraryID and fileID are mutually optional, but at least one must be provided.
func (s *ChatWikiService) GetParagraphList(
	libraryID string,
	fileID string,
	page int,
	size int,
	status int,
	graphStatus int,
	categoryID int,
	groupID int,
	sortField string,
	sortType string,
	search string,
) (LibraryParagraphPage, error) {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return LibraryParagraphPage{}, fmt.Errorf("no binding found")
	}
	libraryID = strings.TrimSpace(libraryID)
	fileID = strings.TrimSpace(fileID)
	if libraryID == "" && fileID == "" {
		return LibraryParagraphPage{}, fmt.Errorf("library_id or file_id is required")
	}
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}

	baseURL := strings.TrimRight(binding.ServerURL, "/")
	q := url.Values{}
	if libraryID != "" {
		q.Set("library_id", libraryID)
	}
	if fileID != "" {
		q.Set("file_id", fileID)
	}
	q.Set("page", strconv.Itoa(page))
	q.Set("size", strconv.Itoa(size))
	q.Set("status", strconv.Itoa(status))
	q.Set("graph_status", strconv.Itoa(graphStatus))
	q.Set("category_id", strconv.Itoa(categoryID))
	q.Set("group_id", strconv.Itoa(groupID))
	if strings.TrimSpace(sortField) != "" {
		q.Set("sort_field", strings.TrimSpace(sortField))
	}
	if strings.TrimSpace(sortType) != "" {
		q.Set("sort_type", strings.TrimSpace(sortType))
	}
	if strings.TrimSpace(search) != "" {
		q.Set("search", strings.TrimSpace(search))
	}
	apiURL := baseURL + "/manage/chatclaw/getParagraphList?" + q.Encode()

	s.app.Logger.Info("[ChatWiki] GetParagraphList request",
		"url", apiURL,
		"library_id", libraryID,
		"file_id", fileID,
		"page", page,
		"size", size,
		"status", status,
		"graph_status", graphStatus,
		"category_id", categoryID,
		"group_id", groupID,
		"sort_field", strings.TrimSpace(sortField),
		"sort_type", strings.TrimSpace(sortType),
		"search", strings.TrimSpace(search),
	)

	body, err := s.chatWikiGETLoose(binding.Token, apiURL)
	if err != nil {
		return LibraryParagraphPage{}, err
	}

	items, total, hasTotal, err := decodeAPIDataObjectArrayWithTotal(body)
	if err != nil {
		return LibraryParagraphPage{}, err
	}

	result := make([]LibraryParagraph, 0, len(items))
	for _, item := range items {
		id := getStringByKeys(item, "id", "paragraph_id", "qa_id")
		question := getStringByKeys(item, "question", "title", "q", "query")
		answer := getStringByKeys(item, "answer", "content", "a")
		images := getStringSliceByKeys(item, "images", "image_list", "pics", "pic_list", "attachments", "files")
		normalizedImages := make([]string, 0, len(images))
		for _, rawURL := range images {
			fullURL := normalizeAssetURL(binding.ServerURL, rawURL)
			if strings.TrimSpace(fullURL) == "" {
				continue
			}
			normalizedImages = append(normalizedImages, fullURL)
		}
		result = append(result, LibraryParagraph{
			ID:       id,
			Question: question,
			Answer:   answer,
			Images:   normalizedImages,
		})
	}
	if !hasTotal {
		total = -1
	}
	return LibraryParagraphPage{
		List:  result,
		Total: total,
	}, nil
}

// UpdateRobotSwitchStatus updates the robot switch status.
func (s *ChatWikiService) UpdateRobotSwitchStatus(id string, switchStatus int) error {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return fmt.Errorf("no binding found")
	}
	baseURL := strings.TrimRight(binding.ServerURL, "/")
	apiURL := baseURL + "/manage/chatclaw/updateRobotSwitchStatus"

	idInt, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid robot id %q: %w", id, err)
	}
	payload := map[string]any{
		"id":            idInt,
		"switch_status": switchStatus,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	s.app.Logger.Info("[ChatWiki] UpdateRobotSwitchStatus request",
		"url", apiURL,
		"payload", string(bodyBytes),
		"token_length", len(binding.Token),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", binding.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	s.app.Logger.Info("[ChatWiki] UpdateRobotSwitchStatus response",
		"status", resp.StatusCode,
		"body", string(respBody),
	)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Res  int    `json:"res"`
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	resultCode := apiResp.Res
	if resultCode == 0 && apiResp.Code != 0 {
		resultCode = apiResp.Code
	}
	if resultCode != 0 {
		return fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
	}
	return nil
}

// UpdateLibrarySwitchStatus updates the knowledge base switch status.
func (s *ChatWikiService) UpdateLibrarySwitchStatus(id string, switchStatus int) error {
	binding, err := s.GetBinding()
	if err != nil || binding == nil {
		return fmt.Errorf("no binding found")
	}
	baseURL := strings.TrimRight(binding.ServerURL, "/")
	apiURL := baseURL + "/manage/chatclaw/updateLibrarySwitchStatus"

	idInt, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return fmt.Errorf("invalid library id %q: %w", id, err)
	}
	payload := map[string]any{
		"id":            idInt,
		"switch_status": switchStatus,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	s.app.Logger.Info("[ChatWiki] UpdateLibrarySwitchStatus request",
		"url", apiURL,
		"payload", string(bodyBytes),
		"token_length", len(binding.Token),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", binding.Token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	s.app.Logger.Info("[ChatWiki] UpdateLibrarySwitchStatus response",
		"status", resp.StatusCode,
		"body", string(respBody),
	)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp struct {
		Res  int    `json:"res"`
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	resultCode := apiResp.Res
	if resultCode == 0 && apiResp.Code != 0 {
		resultCode = apiResp.Code
	}
	if resultCode != 0 {
		return fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
	}
	return nil
}

func toBinding(m *bindingModel) *Binding {
	return &Binding{
		ID:        m.ID,
		ServerURL: m.ServerURL,
		Token:     m.Token,
		TTL:       m.TTL,
		Exp:       m.Exp,
		UserID:    m.UserID,
		UserName:  m.UserName,
		CreatedAt: m.CreatedAt.Format(sqlite.DateTimeFormat),
		UpdatedAt: m.UpdatedAt.Format(sqlite.DateTimeFormat),
	}
}

func normalizeAssetURL(serverURL, assetPath string) string {
	assetPath = strings.TrimSpace(assetPath)
	if assetPath == "" {
		return ""
	}
	if strings.HasPrefix(assetPath, "http://") || strings.HasPrefix(assetPath, "https://") {
		return assetPath
	}
	base := normalizeAssetBase(serverURL)
	if base == "" {
		return assetPath
	}
	return base + "/" + strings.TrimLeft(assetPath, "/")
}

func normalizeAssetBase(serverURL string) string {
	serverURL = strings.TrimSpace(serverURL)
	if serverURL == "" {
		return ""
	}

	parsed, err := url.Parse(serverURL)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host
	}

	// Fallback for non-standard inputs.
	return strings.TrimRight(serverURL, "/")
}

func parseSwitchStatus(v string) int {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0
	}
	if n == 1 {
		return 1
	}
	return 0
}

func (s *ChatWikiService) chatWikiGET(token string, apiURL string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Res  int             `json:"res"`
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	resultCode := apiResp.Res
	if resultCode == 0 && apiResp.Code != 0 {
		resultCode = apiResp.Code
	}
	if resultCode != 0 {
		return nil, fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
	}

	return apiResp.Data, nil
}

// chatWikiGETLoose accepts both standard API envelopes and raw JSON payloads.
// Some ChatWiki endpoints may return direct arrays/objects or even "NULL".
func (s *ChatWikiService) chatWikiGETLoose(token string, apiURL string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return json.RawMessage("[]"), nil
	}

	var apiResp struct {
		Res  int             `json:"res"`
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &apiResp); err == nil {
		resultCode := apiResp.Res
		if resultCode == 0 && apiResp.Code != 0 {
			resultCode = apiResp.Code
		}
		if resultCode != 0 {
			return nil, fmt.Errorf("API error code=%d msg=%s", resultCode, apiResp.Msg)
		}
		dataTrimmed := strings.TrimSpace(string(apiResp.Data))
		if dataTrimmed == "" || strings.EqualFold(dataTrimmed, "null") {
			return json.RawMessage("[]"), nil
		}
		return apiResp.Data, nil
	}

	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed), nil
	}

	// Some deployments may return plain "NULL" as empty payload.
	if strings.EqualFold(trimmed, "NULL") {
		return json.RawMessage("[]"), nil
	}

	if len(trimmed) > 240 {
		trimmed = trimmed[:240] + "..."
	}
	return nil, fmt.Errorf("decode response: non-JSON body: %s", trimmed)
}

func decodeAPIDataObjectArray(data json.RawMessage) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("decode data: %w", err)
	}
	for _, key := range []string{"list", "rows", "items", "data"} {
		raw, ok := obj[key]
		if !ok || raw == nil {
			continue
		}
		items, ok := raw.([]any)
		if !ok {
			continue
		}
		result := make([]map[string]any, 0, len(items))
		for _, item := range items {
			asMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, asMap)
		}
		return result, nil
	}

	return []map[string]any{}, nil
}

func decodeAPIDataObjectArrayWithTotal(data json.RawMessage) ([]map[string]any, int, bool, error) {
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, 0, false, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, 0, false, fmt.Errorf("decode data: %w", err)
	}

	items := []map[string]any{}
	for _, key := range []string{"list", "rows", "items", "data"} {
		raw, ok := obj[key]
		if !ok || raw == nil {
			continue
		}
		list, ok := raw.([]any)
		if !ok {
			continue
		}
		result := make([]map[string]any, 0, len(list))
		for _, item := range list {
			asMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, asMap)
		}
		items = result
		break
	}

	total, hasTotal := getIntWithPresenceByKeys(obj, "total", "count", "total_count", "total_num", "records")
	return items, total, hasTotal, nil
}

func getStringByKeys(data map[string]any, keys ...string) string {
	for _, key := range keys {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		case float64:
			return strconv.FormatInt(int64(v), 10)
		case int:
			return strconv.Itoa(v)
		case int64:
			return strconv.FormatInt(v, 10)
		case json.Number:
			return v.String()
		default:
			value := strings.TrimSpace(fmt.Sprintf("%v", v))
			if value != "" && value != "<nil>" {
				return value
			}
		}
	}
	return ""
}

func getIntByKeys(data map[string]any, keys ...string) int {
	for _, key := range keys {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil {
				return n
			}
		case json.Number:
			n, err := v.Int64()
			if err == nil {
				return int(n)
			}
		}
	}
	return 0
}

func getIntWithPresenceByKeys(data map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return v, true
		case int64:
			return int(v), true
		case float64:
			return int(v), true
		case string:
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err == nil {
				return n, true
			}
		case json.Number:
			n, err := v.Int64()
			if err == nil {
				return int(n), true
			}
		}
	}
	return 0, false
}

func getStringSliceByKeys(data map[string]any, keys ...string) []string {
	for _, key := range keys {
		raw, ok := data[key]
		if !ok || raw == nil {
			continue
		}
		items, ok := raw.([]any)
		if !ok {
			continue
		}
		result := make([]string, 0, len(items))
		for _, item := range items {
			if item == nil {
				continue
			}
			if m, ok := item.(map[string]any); ok {
				s := getStringByKeys(m, "url", "path", "src", "thumb_path", "image", "file_url")
				if s != "" {
					result = append(result, s)
				}
				continue
			}
			s := strings.TrimSpace(fmt.Sprintf("%v", item))
			if s == "" || s == "<nil>" {
				continue
			}
			result = append(result, s)
		}
		return result
	}
	return []string{}
}
