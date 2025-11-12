package config

import "gorm.io/gorm"

// CreateFinalApprovalPartialIndex creates a partial unique index that allows:
// - Multiple soft-deleted final approvals per application (for audit history)
// - Only ONE active final approval per application (enforces business rules)
//
// Why this is needed:
// Without this index, soft-deleted records still violate unique constraints,
// causing "duplicate key" errors when users revoke and re-approve applications.
//
// Example scenario:
// - User approves application → creates final_approval record
// - User revokes approval → soft deletes the final_approval record  
// - User approves again → WITHOUT this index: "duplicate key" error
// - User approves again → WITH this index: new final_approval created successfully
func CreateFinalApprovalPartialIndex(db *gorm.DB) error {
	return db.Exec(`
		DROP INDEX IF EXISTS idx_final_approvals_application_id;
		CREATE UNIQUE INDEX IF NOT EXISTS idx_final_approvals_application_id_active 
		ON final_approvals (application_id) 
		WHERE deleted_at IS NULL;
	`).Error
}