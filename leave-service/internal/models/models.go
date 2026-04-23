package models

import "time"

// LeaveType represents the types of leave available
type LeaveType string

const (
	LeaveTypeAnnual   LeaveType = "annual"
	LeaveTypeSick     LeaveType = "sick"
	LeaveTypePersonal LeaveType = "personal"
	LeaveTypeUnpaid   LeaveType = "unpaid"
)

// AllLeaveTypes lists all supported leave types in a fixed order
var AllLeaveTypes = []LeaveType{
	LeaveTypeAnnual,
	LeaveTypeSick,
	LeaveTypePersonal,
	LeaveTypeUnpaid,
}

// LeaveStatus represents the status of a leave request
type LeaveStatus string

const (
	LeaveStatusPending  LeaveStatus = "pending"
	LeaveStatusApproved LeaveStatus = "approved"
	LeaveStatusDenied   LeaveStatus = "denied"
)

// LeaveRequest represents a single leave request
type LeaveRequest struct {
	ID               string      `json:"id"`
	EmployeeID       string      `json:"employeeId"`
	LeaveType        LeaveType   `json:"leaveType"`
	StartDate        string      `json:"startDate"`
	EndDate          string      `json:"endDate"`
	Reason           string      `json:"reason,omitempty"`
	Status           LeaveStatus `json:"status"`
	ReviewerID       string      `json:"reviewerId,omitempty"`
	ReviewerComments string      `json:"reviewerComments,omitempty"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
}

// LeaveBalance stores the leave balance for one leave type
type LeaveBalance struct {
	LeaveType LeaveType `json:"leaveType"`
	Allocated float64   `json:"allocated"`
	Used      float64   `json:"used"`
	Remaining float64   `json:"remaining"`
}

// EmployeeBalance groups all leave balances for one employee
type EmployeeBalance struct {
	EmployeeID string         `json:"employeeId"`
	Balances   []LeaveBalance `json:"balances"`
}

// BalanceEntry is the internal per-leave-type record in the store
type BalanceEntry struct {
	Allocated float64
	Used      float64
}
