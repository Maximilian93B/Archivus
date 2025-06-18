package postgresql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type FolderRepository struct {
	db *database.DB
}

func NewFolderRepository(db *database.DB) repositories.FolderRepository {
	return &FolderRepository{db: db}
}

func (r *FolderRepository) Create(ctx context.Context, folder *models.Folder) error {
	if err := r.db.WithContext(ctx).Create(folder).Error; err != nil {
		// Check for duplicate path constraint violation
		if errors.Is(err, gorm.ErrDuplicatedKey) ||
			(err != nil && (strings.Contains(err.Error(), "duplicate key") ||
				strings.Contains(err.Error(), "idx_tenant_folder_path") ||
				strings.Contains(err.Error(), "unique constraint"))) {
			return fmt.Errorf("folder with path '%s' already exists in this tenant", folder.Path)
		}
		return fmt.Errorf("failed to create folder: %w", err)
	}
	return nil
}

func (r *FolderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	// Use selective preloading to optimize performance
	err := r.db.WithContext(ctx).
		Preload("Tenant", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "subdomain")
		}).
		Preload("Parent", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "name", "path")
		}).
		Preload("Creator", func(db *gorm.DB) *gorm.DB {
			return db.Select("id", "first_name", "last_name", "email")
		}).
		Where("id = ?", id).First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("folder not found")
		}
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	return &folder, nil
}

func (r *FolderRepository) GetByPath(ctx context.Context, tenantID uuid.UUID, path string) (*models.Folder, error) {
	var folder models.Folder
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND path = ?", tenantID, path).First(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("folder not found at path: %s", path)
		}
		return nil, fmt.Errorf("failed to get folder by path: %w", err)
	}
	return &folder, nil
}

func (r *FolderRepository) Update(ctx context.Context, folder *models.Folder) error {
	result := r.db.WithContext(ctx).Save(folder)
	if result.Error != nil {
		return fmt.Errorf("failed to update folder: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("folder not found")
	}
	return nil
}

func (r *FolderRepository) GetChildren(ctx context.Context, parentID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).
		Order("name ASC").Find(&folders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get folder children: %w", err)
	}
	return folders, nil
}

func (r *FolderRepository) GetTree(ctx context.Context, tenantID uuid.UUID) ([]repositories.FolderNode, error) {
	var folders []models.Folder
	// Only select fields we need for the tree to optimize performance
	err := r.db.WithContext(ctx).
		Select("id", "parent_id", "name", "path", "level", "color", "icon", "tenant_id").
		Where("tenant_id = ?", tenantID).
		Order("level ASC, name ASC").Find(&folders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get folder tree: %w", err)
	}

	// Optimized O(n) tree building algorithm
	// Create map for O(1) lookup and initialize all nodes
	nodeMap := make(map[uuid.UUID]*repositories.FolderNode)
	var rootNodes []repositories.FolderNode

	// First pass: create all nodes
	for i := range folders {
		node := &repositories.FolderNode{
			Folder:        &folders[i],
			Children:      make([]repositories.FolderNode, 0),
			DocumentCount: 0, // Will be populated separately if needed
		}
		nodeMap[folders[i].ID] = node
	}

	// Second pass: build parent-child relationships - O(n) total complexity
	for _, folder := range folders {
		node := nodeMap[folder.ID]
		if folder.ParentID == nil {
			// Root node
			rootNodes = append(rootNodes, *node)
		} else {
			// Child node - add to parent's children
			if parent, exists := nodeMap[*folder.ParentID]; exists {
				parent.Children = append(parent.Children, *node)
			}
		}
	}

	return rootNodes, nil
}

func (r *FolderRepository) GetDocumentCount(ctx context.Context, folderID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("folder_id = ?", folderID).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count documents in folder: %w", err)
	}
	return count, nil
}

func (r *FolderRepository) Move(ctx context.Context, folderID, newParentID uuid.UUID) error {
	// Start transaction
	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get the folder to move
	var folder models.Folder
	if err := tx.Where("id = ?", folderID).First(&folder).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("folder not found")
		}
		return fmt.Errorf("failed to get folder: %w", err)
	}

	// Check if new parent exists and belongs to same tenant
	var newParent models.Folder
	if err := tx.Where("id = ? AND tenant_id = ?", newParentID, folder.TenantID).First(&newParent).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("new parent folder not found")
		}
		return fmt.Errorf("failed to get new parent folder: %w", err)
	}

	// Prevent circular references
	if folderID == newParentID {
		tx.Rollback()
		return fmt.Errorf("cannot move folder to itself")
	}

	// Check if new parent is a descendant of the folder being moved
	currentPath := folder.Path
	if strings.HasPrefix(newParent.Path, currentPath+"/") {
		tx.Rollback()
		return fmt.Errorf("cannot move folder to its own descendant")
	}

	// Calculate new path and level
	oldPath := folder.Path
	newPath := fmt.Sprintf("%s/%s", newParent.Path, folder.Name)
	newLevel := newParent.Level + 1

	// Update the folder
	result := tx.Model(&folder).Updates(map[string]interface{}{
		"parent_id": newParentID,
		"path":      newPath,
		"level":     newLevel,
	})

	if result.Error != nil {
		tx.Rollback()
		return fmt.Errorf("failed to move folder: %w", result.Error)
	}

	// Update all subfolders' paths recursively
	if err := tx.Exec(`
		UPDATE folders 
		SET path = REPLACE(path, ?, ?), 
		    level = level + ?
		WHERE tenant_id = ? AND path LIKE ?`,
		oldPath, newPath, newLevel-folder.Level, folder.TenantID, oldPath+"/%").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update subfolder paths: %w", err)
	}

	return tx.Commit().Error
}

func (r *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if folder has children or documents
	var childCount int64
	if err := r.db.WithContext(ctx).Model(&models.Folder{}).
		Where("parent_id = ?", id).Count(&childCount).Error; err != nil {
		return fmt.Errorf("failed to check for child folders: %w", err)
	}

	if childCount > 0 {
		return fmt.Errorf("cannot delete folder with child folders")
	}

	var docCount int64
	if err := r.db.WithContext(ctx).Model(&models.Document{}).
		Where("folder_id = ?", id).Count(&docCount).Error; err != nil {
		return fmt.Errorf("failed to check for documents in folder: %w", err)
	}

	if docCount > 0 {
		return fmt.Errorf("cannot delete folder containing documents")
	}

	// Delete the folder
	result := r.db.WithContext(ctx).Delete(&models.Folder{}, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete folder: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("folder not found")
	}
	return nil
}
