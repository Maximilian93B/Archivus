package dto

import "time"

// Authentication DTOs
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name" binding:"required"`
	LastName  string `json:"last_name" binding:"required"`
	TenantID  string `json:"tenant_id" binding:"required,uuid"`
}

type LoginResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

type UserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Role          string `json:"role"`
	EmailVerified bool   `json:"email_verified"`
}

// Document DTOs
type DocumentUploadRequest struct {
	FolderID    string   `form:"folder_id" binding:"omitempty,uuid"`
	Title       string   `form:"title" binding:"omitempty,max=255"`
	Description string   `form:"description" binding:"omitempty"`
	Tags        []string `form:"tags" binding:"omitempty"`
	Categories  []string `form:"categories" binding:"omitempty"`
}

type DocumentResponse struct {
	ID                 string                `json:"id"`
	FileName           string                `json:"file_name"`
	OriginalName       string                `json:"original_name"`
	ContentType        string                `json:"content_type"`
	FileSize           int64                 `json:"file_size"`
	Title              string                `json:"title"`
	Description        string                `json:"description"`
	Status             string                `json:"status"`
	Version            int                   `json:"version"`
	Language           string                `json:"language"`
	Summary            string                `json:"summary,omitempty"`
	AIConfidence       float64               `json:"ai_confidence"`
	Author             string                `json:"author,omitempty"`
	Subject            string                `json:"subject,omitempty"`
	Keywords           string                `json:"keywords,omitempty"`
	DocumentCreatedAt  *time.Time            `json:"document_created_at,omitempty"`
	DocumentModifiedAt *time.Time            `json:"document_modified_at,omitempty"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	Folder             *FolderInfo           `json:"folder,omitempty"`
	Tags               []TagInfo             `json:"tags,omitempty"`
	Categories         []CategoryInfo        `json:"categories,omitempty"`
}

type DocumentUpdateRequest struct {
	Title       string   `json:"title" binding:"omitempty,max=255"`
	Description string   `json:"description" binding:"omitempty"`
	FolderID    string   `json:"folder_id" binding:"omitempty,uuid"`
	Tags        []string `json:"tags" binding:"omitempty"`
	Categories  []string `json:"categories" binding:"omitempty"`
}

type DocumentSearchRequest struct {
	Query      string   `json:"query" binding:"required"`
	FolderID   string   `json:"folder_id" binding:"omitempty,uuid"`
	Tags       []string `json:"tags" binding:"omitempty"`
	Categories []string `json:"categories" binding:"omitempty"`
	DateFrom   string   `json:"date_from" binding:"omitempty"`
	DateTo     string   `json:"date_to" binding:"omitempty"`
	Page       int      `json:"page" binding:"omitempty,min=1"`
	PageSize   int      `json:"page_size" binding:"omitempty,min=1,max=100"`
}

type DocumentSearchResponse struct {
	Documents  []DocumentResponse `json:"documents"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

// AI Processing DTOs
type AIAnalyzeRequest struct {
	DocumentID string   `json:"document_id" binding:"required,uuid"`
	Operations []string `json:"operations" binding:"required,min=1"`
}

type AIAnalyzeResponse struct {
	JobID   string                 `json:"job_id"`
	Status  string                 `json:"status"`
	Results map[string]interface{} `json:"results,omitempty"`
}

// Folder DTOs
type FolderCreateRequest struct {
	Name        string `json:"name" binding:"required,max=255"`
	Description string `json:"description" binding:"omitempty"`
	ParentID    string `json:"parent_id" binding:"omitempty,uuid"`
}

type FolderUpdateRequest struct {
	Name        string `json:"name" binding:"omitempty,max=255"`
	Description string `json:"description" binding:"omitempty"`
	ParentID    string `json:"parent_id" binding:"omitempty,uuid"`
}

type FolderInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Level       int    `json:"level"`
	IsSystem    bool   `json:"is_system"`
}

type FolderResponse struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	Path           string            `json:"path"`
	Level          int               `json:"level"`
	IsSystem       bool              `json:"is_system"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	Parent         *FolderInfo       `json:"parent,omitempty"`
	Children       []FolderInfo      `json:"children,omitempty"`
	DocumentCount  int               `json:"document_count"`
}

// Category DTOs
type CategoryCreateRequest struct {
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description" binding:"omitempty"`
	Color       string `json:"color" binding:"omitempty,len=7"`
	Icon        string `json:"icon" binding:"omitempty,max=50"`
}

type CategoryUpdateRequest struct {
	Name        string `json:"name" binding:"omitempty,max=100"`
	Description string `json:"description" binding:"omitempty"`
	Color       string `json:"color" binding:"omitempty,len=7"`
	Icon        string `json:"icon" binding:"omitempty,max=50"`
}

type CategoryInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	IsSystem    bool   `json:"is_system"`
}

type CategoryResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Color         string    `json:"color"`
	Icon          string    `json:"icon"`
	IsSystem      bool      `json:"is_system"`
	DocumentCount int       `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Tag DTOs
type TagCreateRequest struct {
	Name  string `json:"name" binding:"required,max=50"`
	Color string `json:"color" binding:"omitempty,len=7"`
}

type TagUpdateRequest struct {
	Name  string `json:"name" binding:"omitempty,max=50"`
	Color string `json:"color" binding:"omitempty,len=7"`
}

type TagInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	IsAIGenerated bool   `json:"is_ai_generated"`
}

type TagResponse struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	IsAIGenerated bool      `json:"is_ai_generated"`
	UsageCount    int       `json:"usage_count"`
	DocumentCount int       `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Share DTOs
type ShareCreateRequest struct {
	DocumentID   string     `json:"document_id" binding:"required,uuid"`
	Password     string     `json:"password" binding:"omitempty"`
	ExpiresAt    *time.Time `json:"expires_at" binding:"omitempty"`
	MaxDownloads int        `json:"max_downloads" binding:"omitempty,min=0"`
}

type ShareResponse struct {
	ID            string     `json:"id"`
	Token         string     `json:"token"`
	DocumentID    string     `json:"document_id"`
	Document      string     `json:"document_name"`
	ExpiresAt     *time.Time `json:"expires_at"`
	MaxDownloads  int        `json:"max_downloads"`
	DownloadCount int        `json:"download_count"`
	IsActive      bool       `json:"is_active"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Pagination
type PaginationRequest struct {
	Page     int `json:"page" binding:"omitempty,min=1"`
	PageSize int `json:"page_size" binding:"omitempty,min=1,max=100"`
}

type PaginationResponse struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// Error DTOs
type ErrorResponse struct {
	Error   string                 `json:"error"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

// Success DTOs
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
} 