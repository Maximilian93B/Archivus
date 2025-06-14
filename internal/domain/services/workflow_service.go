package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

var (
	ErrWorkflowNotFound     = errors.New("workflow not found")
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskAlreadyCompleted = errors.New("task already completed")
	ErrInvalidTaskStatus    = errors.New("invalid task status")
	ErrUnauthorizedTask     = errors.New("unauthorized to complete task")
	ErrWorkflowNotActive    = errors.New("workflow is not active")
)

// WorkflowService handles business process automation and document approval workflows
type WorkflowService struct {
	workflowRepo     repositories.WorkflowRepository
	taskRepo         repositories.WorkflowTaskRepository
	documentRepo     repositories.DocumentRepository
	userRepo         repositories.UserRepository
	tenantRepo       repositories.TenantRepository
	auditRepo        repositories.AuditLogRepository
	notificationRepo repositories.NotificationRepository

	notificationService NotificationService
}

// NewWorkflowService creates a new workflow service
func NewWorkflowService(
	workflowRepo repositories.WorkflowRepository,
	taskRepo repositories.WorkflowTaskRepository,
	documentRepo repositories.DocumentRepository,
	userRepo repositories.UserRepository,
	tenantRepo repositories.TenantRepository,
	auditRepo repositories.AuditLogRepository,
	notificationRepo repositories.NotificationRepository,
	notificationService NotificationService,
) *WorkflowService {
	return &WorkflowService{
		workflowRepo:        workflowRepo,
		taskRepo:            taskRepo,
		documentRepo:        documentRepo,
		userRepo:            userRepo,
		tenantRepo:          tenantRepo,
		auditRepo:           auditRepo,
		notificationRepo:    notificationRepo,
		notificationService: notificationService,
	}
}

// CreateWorkflowParams contains parameters for creating a workflow
type CreateWorkflowParams struct {
	TenantID     uuid.UUID           `json:"tenant_id"`
	CreatedBy    uuid.UUID           `json:"created_by"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	DocumentType models.DocumentType `json:"document_type"`
	Rules        WorkflowRules       `json:"rules"`
	IsActive     bool                `json:"is_active"`
}

// WorkflowRules defines the business rules for workflow execution
type WorkflowRules struct {
	// Trigger conditions
	TriggerConditions []TriggerCondition `json:"trigger_conditions"`

	// Approval steps
	ApprovalSteps []ApprovalStep `json:"approval_steps"`

	// Escalation rules
	EscalationRules []EscalationRule `json:"escalation_rules"`

	// Auto-complete conditions
	AutoCompleteConditions []AutoCompleteCondition `json:"auto_complete_conditions"`

	// Notification settings
	NotificationSettings NotificationSettings `json:"notification_settings"`
}

type TriggerCondition struct {
	Type      string      `json:"type"`     // "amount_threshold", "document_type", "vendor", etc.
	Operator  string      `json:"operator"` // "gt", "lt", "eq", "contains", etc.
	Value     interface{} `json:"value"`
	Mandatory bool        `json:"mandatory"`
}

type ApprovalStep struct {
	StepNumber    int    `json:"step_number"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	AssigneeType  string `json:"assignee_type"` // "user", "role", "department"
	AssigneeValue string `json:"assignee_value"`
	RequiredVotes int    `json:"required_votes"` // For multi-approver steps
	DueDays       int    `json:"due_days"`       // Days from creation
	CanDelegate   bool   `json:"can_delegate"`
	IsOptional    bool   `json:"is_optional"`
}

type EscalationRule struct {
	StepNumber      int    `json:"step_number"`      // Which step to escalate
	EscalationDays  int    `json:"escalation_days"`  // Days before escalation
	EscalateToType  string `json:"escalate_to_type"` // "user", "role", "manager"
	EscalateToValue string `json:"escalate_to_value"`
	NotifyOriginal  bool   `json:"notify_original"` // Keep original assignee notified
}

type AutoCompleteCondition struct {
	Type   string      `json:"type"` // "time_passed", "external_approval", etc.
	Value  interface{} `json:"value"`
	Action string      `json:"action"` // "approve", "reject"
}

type NotificationSettings struct {
	NotifyOnAssignment bool  `json:"notify_on_assignment"`
	NotifyOnCompletion bool  `json:"notify_on_completion"`
	NotifyOnEscalation bool  `json:"notify_on_escalation"`
	NotifyOnRejection  bool  `json:"notify_on_rejection"`
	ReminderDays       []int `json:"reminder_days"` // Days to send reminders
}

// CreateWorkflow creates a new workflow template
func (s *WorkflowService) CreateWorkflow(ctx context.Context, params CreateWorkflowParams) (*models.Workflow, error) {
	// Validate rules
	if err := s.validateWorkflowRules(params.Rules); err != nil {
		return nil, fmt.Errorf("invalid workflow rules: %w", err)
	}

	workflow := &models.Workflow{
		ID:          uuid.New(),
		TenantID:    params.TenantID,
		Name:        params.Name,
		Description: params.Description,
		DocType:     params.DocumentType,
		Rules:       models.JSONB(params.Rules),
		IsActive:    params.IsActive,
		CreatedBy:   params.CreatedBy,
	}

	if err := s.workflowRepo.Create(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, params.TenantID, params.CreatedBy, workflow.ID, models.AuditCreate, "Workflow created")

	return workflow, nil
}

// TriggerWorkflow initiates a workflow for a document
func (s *WorkflowService) TriggerWorkflow(ctx context.Context, documentID uuid.UUID, triggeredBy uuid.UUID) error {
	// Get document
	document, err := s.documentRepo.GetByID(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Find applicable workflows
	workflows, err := s.workflowRepo.GetByDocumentType(ctx, document.TenantID, document.DocumentType)
	if err != nil {
		return fmt.Errorf("failed to get workflows: %w", err)
	}

	// Check which workflows should be triggered
	for _, workflow := range workflows {
		if !workflow.IsActive {
			continue
		}

		var rules WorkflowRules
		if err := s.unmarshalRules(workflow.Rules, &rules); err != nil {
			continue // Skip workflow with invalid rules
		}

		// Check trigger conditions
		if s.shouldTriggerWorkflow(document, rules.TriggerConditions) {
			if err := s.initiateWorkflowExecution(ctx, &workflow, document, triggeredBy); err != nil {
				// Log error but don't fail - other workflows might still work
				continue
			}
		}
	}

	return nil
}

// CompleteTask marks a workflow task as completed
func (s *WorkflowService) CompleteTask(ctx context.Context, taskID uuid.UUID, completedBy uuid.UUID, action string, comments string) error {
	// Get task
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ErrTaskNotFound
	}

	// Verify authorization
	if task.AssignedTo != completedBy {
		// Check if user has admin role or can delegate
		user, err := s.userRepo.GetByID(ctx, completedBy)
		if err != nil || (user.Role != models.UserRoleAdmin && user.Role != models.UserRoleManager) {
			return ErrUnauthorizedTask
		}
	}

	// Check if task is already completed
	if task.Status != models.WorkflowPending {
		return ErrTaskAlreadyCompleted
	}

	// Update task status
	var newStatus models.WorkflowStatus
	switch action {
	case "approve":
		newStatus = models.WorkflowApproved
	case "reject":
		newStatus = models.WorkflowRejected
	default:
		return ErrInvalidTaskStatus
	}

	// Complete the task
	if err := s.taskRepo.Complete(ctx, taskID, completedBy, comments); err != nil {
		return fmt.Errorf("failed to complete task: %w", err)
	}

	// Update task status
	task.Status = newStatus
	task.Comments = comments
	now := time.Now()
	task.CompletedAt = &now

	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Handle workflow progression
	if err := s.handleWorkflowProgression(ctx, task, action); err != nil {
		return fmt.Errorf("failed to progress workflow: %w", err)
	}

	// Send notifications
	s.sendTaskCompletionNotifications(ctx, task, completedBy, action)

	// Create audit log
	s.createAuditLog(ctx, task.Document.TenantID, completedBy, task.DocumentID, models.AuditApprove,
		fmt.Sprintf("Workflow task %s: %s", action, comments))

	return nil
}

// GetUserTasks retrieves tasks assigned to a user
func (s *WorkflowService) GetUserTasks(ctx context.Context, userID uuid.UUID, status models.WorkflowStatus) ([]models.WorkflowTask, error) {
	return s.taskRepo.ListByAssignee(ctx, userID, status)
}

// GetPendingTasks gets all pending tasks for a tenant
func (s *WorkflowService) GetPendingTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error) {
	return s.taskRepo.GetPendingTasks(ctx, tenantID)
}

// GetOverdueTasks gets all overdue tasks for a tenant
func (s *WorkflowService) GetOverdueTasks(ctx context.Context, tenantID uuid.UUID) ([]models.WorkflowTask, error) {
	return s.taskRepo.GetOverdueTasks(ctx, tenantID)
}

// GetDocumentWorkflow gets the workflow status for a document
func (s *WorkflowService) GetDocumentWorkflow(ctx context.Context, documentID uuid.UUID) ([]models.WorkflowTask, error) {
	return s.taskRepo.ListByDocument(ctx, documentID)
}

// DelegateTask allows a user to delegate their task to another user
func (s *WorkflowService) DelegateTask(ctx context.Context, taskID, fromUserID, toUserID uuid.UUID, reason string) error {
	// Get task
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ErrTaskNotFound
	}

	// Verify authorization
	if task.AssignedTo != fromUserID {
		return ErrUnauthorizedTask
	}

	// Check if delegation is allowed (would need to check workflow rules)
	workflow, err := s.workflowRepo.GetByID(ctx, task.WorkflowID)
	if err != nil {
		return ErrWorkflowNotFound
	}

	var rules WorkflowRules
	if err := s.unmarshalRules(workflow.Rules, &rules); err != nil {
		return fmt.Errorf("invalid workflow rules: %w", err)
	}

	// Find the step and check if delegation is allowed
	canDelegate := false
	for _, step := range rules.ApprovalSteps {
		if step.StepNumber == task.Priority { // Assuming priority maps to step number
			canDelegate = step.CanDelegate
			break
		}
	}

	if !canDelegate {
		return errors.New("delegation not allowed for this task")
	}

	// Update task assignment
	task.AssignedTo = toUserID
	if err := s.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to delegate task: %w", err)
	}

	// Send notification to new assignee
	s.sendTaskAssignmentNotification(ctx, task, toUserID)

	// Create audit log
	s.createAuditLog(ctx, task.Document.TenantID, fromUserID, task.DocumentID, models.AuditUpdate,
		fmt.Sprintf("Task delegated to user %s: %s", toUserID.String(), reason))

	return nil
}

// ProcessAutomation handles automated workflow processing
func (s *WorkflowService) ProcessAutomation(ctx context.Context) error {
	// This would be called periodically to handle:
	// 1. Escalations
	// 2. Auto-completions
	// 3. Reminders
	// Implementation would depend on your scheduler (cron, etc.)

	// Handle escalations
	if err := s.processEscalations(ctx); err != nil {
		// Log error but continue
	}

	// Handle auto-completions
	if err := s.processAutoCompletions(ctx); err != nil {
		// Log error but continue
	}

	// Send reminders
	if err := s.sendReminders(ctx); err != nil {
		// Log error but continue
	}

	return nil
}

// Helper methods

func (s *WorkflowService) validateWorkflowRules(rules WorkflowRules) error {
	// Validate approval steps
	if len(rules.ApprovalSteps) == 0 {
		return errors.New("at least one approval step is required")
	}

	// Check step numbering
	stepNumbers := make(map[int]bool)
	for _, step := range rules.ApprovalSteps {
		if step.StepNumber <= 0 {
			return errors.New("step numbers must be positive")
		}
		if stepNumbers[step.StepNumber] {
			return errors.New("duplicate step numbers not allowed")
		}
		stepNumbers[step.StepNumber] = true
	}

	return nil
}

func (s *WorkflowService) shouldTriggerWorkflow(document *models.Document, conditions []TriggerCondition) bool {
	for _, condition := range conditions {
		if !s.evaluateCondition(document, condition) {
			if condition.Mandatory {
				return false // Mandatory condition not met
			}
		}
	}
	return true
}

func (s *WorkflowService) evaluateCondition(document *models.Document, condition TriggerCondition) bool {
	switch condition.Type {
	case "amount_threshold":
		if document.Amount == nil {
			return false
		}
		threshold, ok := condition.Value.(float64)
		if !ok {
			return false
		}

		switch condition.Operator {
		case "gt":
			return *document.Amount > threshold
		case "gte":
			return *document.Amount >= threshold
		case "lt":
			return *document.Amount < threshold
		case "lte":
			return *document.Amount <= threshold
		}

	case "vendor_name":
		vendorName, ok := condition.Value.(string)
		if !ok {
			return false
		}

		switch condition.Operator {
		case "eq":
			return document.VendorName == vendorName
		case "contains":
			return fmt.Sprintf("%s", document.VendorName) != "" &&
				fmt.Sprintf("%s", vendorName) != "" &&
				fmt.Sprintf("%s", document.VendorName) == vendorName
		}

	case "document_type":
		docType, ok := condition.Value.(string)
		if !ok {
			return false
		}
		return string(document.DocumentType) == docType
	}

	return false
}

func (s *WorkflowService) initiateWorkflowExecution(ctx context.Context, workflow *models.Workflow, document *models.Document, triggeredBy uuid.UUID) error {
	var rules WorkflowRules
	if err := s.unmarshalRules(workflow.Rules, &rules); err != nil {
		return err
	}

	// Create tasks for the first step
	firstSteps := s.getFirstSteps(rules.ApprovalSteps)

	for _, step := range firstSteps {
		// Resolve assignee
		assigneeID, err := s.resolveAssignee(ctx, document.TenantID, step.AssigneeType, step.AssigneeValue)
		if err != nil {
			continue // Skip this step if assignee can't be resolved
		}

		// Calculate due date
		dueDate := time.Now().AddDate(0, 0, step.DueDays)

		// Create task
		task := &models.WorkflowTask{
			ID:         uuid.New(),
			WorkflowID: workflow.ID,
			DocumentID: document.ID,
			AssignedTo: assigneeID,
			TaskType:   step.Name,
			Status:     models.WorkflowPending,
			Priority:   step.StepNumber,
			DueDate:    &dueDate,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			return fmt.Errorf("failed to create workflow task: %w", err)
		}

		// Send assignment notification
		s.sendTaskAssignmentNotification(ctx, task, assigneeID)
	}

	return nil
}

func (s *WorkflowService) getFirstSteps(steps []ApprovalStep) []ApprovalStep {
	if len(steps) == 0 {
		return nil
	}

	// Find minimum step number
	minStep := steps[0].StepNumber
	for _, step := range steps {
		if step.StepNumber < minStep {
			minStep = step.StepNumber
		}
	}

	// Return all steps with minimum step number
	var firstSteps []ApprovalStep
	for _, step := range steps {
		if step.StepNumber == minStep {
			firstSteps = append(firstSteps, step)
		}
	}

	return firstSteps
}

func (s *WorkflowService) resolveAssignee(ctx context.Context, tenantID uuid.UUID, assigneeType, assigneeValue string) (uuid.UUID, error) {
	switch assigneeType {
	case "user":
		// assigneeValue should be user email or ID
		if userID, err := uuid.Parse(assigneeValue); err == nil {
			return userID, nil
		}

		// Try to find by email
		user, err := s.userRepo.GetByEmail(ctx, tenantID, assigneeValue)
		if err != nil {
			return uuid.Nil, err
		}
		return user.ID, nil

	case "role":
		// Find first user with the specified role
		users, _, err := s.userRepo.ListByTenant(ctx, tenantID, repositories.ListParams{PageSize: 1})
		if err != nil {
			return uuid.Nil, err
		}

		for _, user := range users {
			if string(user.Role) == assigneeValue {
				return user.ID, nil
			}
		}

	case "department":
		// Find first user in the specified department
		users, _, err := s.userRepo.ListByTenant(ctx, tenantID, repositories.ListParams{})
		if err != nil {
			return uuid.Nil, err
		}

		for _, user := range users {
			if user.Department == assigneeValue {
				return user.ID, nil
			}
		}
	}

	return uuid.Nil, errors.New("assignee not found")
}

func (s *WorkflowService) handleWorkflowProgression(ctx context.Context, completedTask *models.WorkflowTask, action string) error {
	if action == "reject" {
		// Workflow is rejected, no further steps
		return s.completeWorkflow(ctx, completedTask.DocumentID, "rejected")
	}

	// Get workflow
	workflow, err := s.workflowRepo.GetByID(ctx, completedTask.WorkflowID)
	if err != nil {
		return err
	}

	var rules WorkflowRules
	if err := s.unmarshalRules(workflow.Rules, &rules); err != nil {
		return err
	}

	// Check if there are next steps
	nextSteps := s.getNextSteps(rules.ApprovalSteps, completedTask.Priority)
	if len(nextSteps) == 0 {
		// No more steps, workflow is completed
		return s.completeWorkflow(ctx, completedTask.DocumentID, "approved")
	}

	// Create tasks for next steps
	for _, step := range nextSteps {
		assigneeID, err := s.resolveAssignee(ctx, completedTask.Document.TenantID, step.AssigneeType, step.AssigneeValue)
		if err != nil {
			continue
		}

		dueDate := time.Now().AddDate(0, 0, step.DueDays)

		task := &models.WorkflowTask{
			ID:         uuid.New(),
			WorkflowID: workflow.ID,
			DocumentID: completedTask.DocumentID,
			AssignedTo: assigneeID,
			TaskType:   step.Name,
			Status:     models.WorkflowPending,
			Priority:   step.StepNumber,
			DueDate:    &dueDate,
		}

		if err := s.taskRepo.Create(ctx, task); err != nil {
			continue
		}

		s.sendTaskAssignmentNotification(ctx, task, assigneeID)
	}

	return nil
}

func (s *WorkflowService) getNextSteps(steps []ApprovalStep, currentStep int) []ApprovalStep {
	// Find the next step number
	nextStepNumber := -1
	for _, step := range steps {
		if step.StepNumber > currentStep {
			if nextStepNumber == -1 || step.StepNumber < nextStepNumber {
				nextStepNumber = step.StepNumber
			}
		}
	}

	if nextStepNumber == -1 {
		return nil // No next steps
	}

	// Return all steps with the next step number
	var nextSteps []ApprovalStep
	for _, step := range steps {
		if step.StepNumber == nextStepNumber {
			nextSteps = append(nextSteps, step)
		}
	}

	return nextSteps
}

func (s *WorkflowService) completeWorkflow(ctx context.Context, documentID uuid.UUID, result string) error {
	// Update document status based on workflow result
	var newStatus models.DocStatus
	switch result {
	case "approved":
		newStatus = models.DocStatusCompleted
	case "rejected":
		newStatus = models.DocStatusError // or a specific "rejected" status
	default:
		newStatus = models.DocStatusCompleted
	}

	return s.documentRepo.UpdateStatus(ctx, documentID, newStatus)
}

func (s *WorkflowService) unmarshalRules(jsonRules models.JSONB, rules *WorkflowRules) error {
	// This would unmarshal the JSONB rules into the WorkflowRules struct
	// Implementation depends on your JSON marshaling approach
	return nil // Placeholder
}

func (s *WorkflowService) processEscalations(ctx context.Context) error {
	// Get overdue tasks and escalate them according to workflow rules
	// Implementation would query overdue tasks and apply escalation rules
	return nil
}

func (s *WorkflowService) processAutoCompletions(ctx context.Context) error {
	// Check for tasks that meet auto-completion conditions
	return nil
}

func (s *WorkflowService) sendReminders(ctx context.Context) error {
	// Send reminder notifications for tasks approaching due dates
	return nil
}

func (s *WorkflowService) sendTaskAssignmentNotification(ctx context.Context, task *models.WorkflowTask, userID uuid.UUID) {
	// Implementation would send notification to the assigned user
	if s.notificationService != nil {
		s.notificationService.SendTaskAssignment(ctx, task, userID)
	}
}

func (s *WorkflowService) sendTaskCompletionNotifications(ctx context.Context, task *models.WorkflowTask, completedBy uuid.UUID, action string) {
	// Implementation would send notifications about task completion
	if s.notificationService != nil {
		s.notificationService.SendTaskCompletion(ctx, task, completedBy, action)
	}
}

func (s *WorkflowService) createAuditLog(ctx context.Context, tenantID, userID, resourceID uuid.UUID, action models.AuditAction, details string) {
	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		ResourceID:   resourceID,
		Action:       action,
		ResourceType: "workflow",
		Details:      models.JSONB{"message": details},
	}

	go func() {
		s.auditRepo.Create(context.Background(), log)
	}()
}

// External service interface
type NotificationService interface {
	SendTaskAssignment(ctx context.Context, task *models.WorkflowTask, userID uuid.UUID) error
	SendTaskCompletion(ctx context.Context, task *models.WorkflowTask, completedBy uuid.UUID, action string) error
	SendTaskReminder(ctx context.Context, task *models.WorkflowTask) error
	SendTaskEscalation(ctx context.Context, task *models.WorkflowTask, escalatedTo uuid.UUID) error
}
