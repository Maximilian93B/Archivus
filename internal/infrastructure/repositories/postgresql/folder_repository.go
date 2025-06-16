package postgresql

import (
	"context"
	"errors"
	"fmt"

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
		return fmt.Errorf("failed to create folder: %w", err)
	}
	return nil
}

func (r *FolderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	err := r.db.WithContext(ctx).Preload("Tenant").Preload("Parent").Preload("Creator").
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
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("level ASC, name ASC").Find(&folders).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get folder tree: %w", err)
	}

	// Build folder tree
	folderMap := make(map[uuid.UUID]*repositories.FolderNode)
	var rootNodes []repositories.FolderNode

	// First pass: create all nodes
	for _, folder := range folders {
		node := repositories.FolderNode{
			Folder:        &folder,
			Children:      make([]repositories.FolderNode, 0),
			DocumentCount: 0, // Will be populated separately if needed
		}
		folderMap[folder.ID] = &node
	}

	// Second pass: build the tree structure
	for _, folder := range folders {
		node := folderMap[folder.ID]
		if folder.ParentID == nil {
			// Root folder
			rootNodes = append(rootNodes, *node)
		} else {
			// Child folder
			if parent, exists := folderMap[*folder.ParentID]; exists {
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

	// Update folder's parent and recalculate path and level
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

	// TODO: Update all subfolders' paths and levels recursively
	// This is a complex operation that would require recursive updates

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
