BEGIN;

-- Reset application status and dates
UPDATE applications 
SET status = 'UNDER_REVIEW',
    final_approval_date = NULL,
    rejection_date = NULL,
    review_completed_at = NULL,
    updated_at = NOW()
WHERE id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641';

-- Reset assignment counters and flags
UPDATE application_group_assignments 
SET ready_for_final_approval = false,
    completed_at = NULL,
    final_decision_at = NULL,
    final_decision_id = NULL,
    final_approver_assigned_at = NULL,
    approved_count = 0,
    rejected_count = 0,
    pending_count = 4, -- Set to total members (4 in your case)
    updated_at = NOW()
WHERE application_id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641' 
AND is_active = true;

-- Reset ALL decisions back to PENDING status
UPDATE member_approval_decisions 
SET status = 'PENDING',
    decided_at = NULL,
    was_revoked = false,
    revoked_by = NULL,
    revoked_at = NULL,
    revoked_reason = NULL,
    updated_at = NOW()
WHERE assignment_id = (
    SELECT id FROM application_group_assignments 
    WHERE application_id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641' 
    AND is_active = true
)
AND deleted_at IS NULL;

-- Remove ALL comments
UPDATE comments 
SET deleted_at = NOW() 
WHERE application_id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641' 
AND deleted_at IS NULL;

-- Remove final approval if exists
UPDATE final_approvals 
SET deleted_at = NOW() 
WHERE application_id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641' 
AND deleted_at IS NULL;

-- Remove issues if any
UPDATE application_issues 
SET deleted_at = NOW() 
WHERE assignment_id = (
    SELECT id FROM application_group_assignments 
    WHERE application_id = '5e58e60a-b42c-4fca-9560-6a6e7fd76641' 
    AND is_active = true
)
AND deleted_at IS NULL;

COMMIT;