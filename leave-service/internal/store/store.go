package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/asdlc-repos/mxcz308/leave-service/internal/models"
)

// Store holds all in-memory application state.
type Store struct {
	mu sync.RWMutex

	// leaveRequests maps request ID -> LeaveRequest
	leaveRequests map[string]*models.LeaveRequest

	// balances maps employeeID -> leaveType -> BalanceEntry
	balances map[string]map[models.LeaveType]*models.BalanceEntry

	// managers maps employeeID -> managerID
	managers map[string]string

	// employees is the set of known employee IDs
	employees map[string]bool

	// counter for generating IDs
	counter int
}

// New creates and seeds a new Store.
func New() *Store {
	s := &Store{
		leaveRequests: make(map[string]*models.LeaveRequest),
		balances:      make(map[string]map[models.LeaveType]*models.BalanceEntry),
		managers:      make(map[string]string),
		employees:     make(map[string]bool),
	}
	s.seed()
	return s
}

// seed populates initial data.
func (s *Store) seed() {
	// Employees: emp1, emp2, emp3 each with 20 annual + 10 sick + 5 personal + 0 unpaid
	for _, empID := range []string{"emp1", "emp2", "emp3"} {
		s.employees[empID] = true
		s.balances[empID] = map[models.LeaveType]*models.BalanceEntry{
			models.LeaveTypeAnnual:   {Allocated: 20, Used: 0},
			models.LeaveTypeSick:     {Allocated: 10, Used: 0},
			models.LeaveTypePersonal: {Allocated: 5, Used: 0},
			models.LeaveTypeUnpaid:   {Allocated: 0, Used: 0},
		}
	}

	// Manager relationships: mgr1 manages emp1 and emp2
	s.managers["emp1"] = "mgr1"
	s.managers["emp2"] = "mgr1"
}

// nextID generates a unique ID (must be called with write lock held).
func (s *Store) nextID(prefix string) string {
	s.counter++
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), s.counter)
}

// EmployeeExists returns true if the given employee ID is known.
func (s *Store) EmployeeExists(employeeID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.employees[employeeID]
}

// GetBalance returns the leave balances for an employee.
// Returns (nil, false) if the employee does not exist.
func (s *Store) GetBalance(employeeID string) (*models.EmployeeBalance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.employees[employeeID] {
		return nil, false
	}

	entries := s.balances[employeeID]
	result := &models.EmployeeBalance{
		EmployeeID: employeeID,
		Balances:   make([]models.LeaveBalance, 0, len(models.AllLeaveTypes)),
	}
	// Return all leave types even if some have zero allocation
	for _, lt := range models.AllLeaveTypes {
		entry, ok := entries[lt]
		if !ok {
			entry = &models.BalanceEntry{}
		}
		result.Balances = append(result.Balances, models.LeaveBalance{
			LeaveType: lt,
			Allocated: entry.Allocated,
			Used:      entry.Used,
			Remaining: entry.Allocated - entry.Used,
		})
	}
	return result, true
}

// SetAllocation updates the Allocated days for a specific leave type for an employee.
// Only the Allocated field is modified; Used is left unchanged to preserve accrued usage.
// Returns false if the employee does not exist.
func (s *Store) SetAllocation(employeeID string, leaveType models.LeaveType, allocated float64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.employees[employeeID] {
		return false
	}

	bmap, ok := s.balances[employeeID]
	if !ok {
		bmap = make(map[models.LeaveType]*models.BalanceEntry)
		s.balances[employeeID] = bmap
	}
	if entry, ok := bmap[leaveType]; ok {
		// Only update Allocated; never touch Used to avoid corrupting accrued usage.
		entry.Allocated = allocated
	} else {
		bmap[leaveType] = &models.BalanceEntry{Allocated: allocated, Used: 0}
	}
	return true
}

// CreateLeaveRequest creates a new leave request after validating overlap and balance.
// Returns the created request, or an error string and HTTP status code.
func (s *Store) CreateLeaveRequest(req *models.LeaveRequest) (*models.LeaveRequest, string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate employee
	if !s.employees[req.EmployeeID] {
		return nil, "employee not found", 404
	}

	// Parse dates
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, "invalid startDate format, expected YYYY-MM-DD", 400
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, "invalid endDate format, expected YYYY-MM-DD", 400
	}
	if endDate.Before(startDate) {
		return nil, "endDate must be on or after startDate", 400
	}

	// Check for overlapping requests (non-denied)
	for _, existing := range s.leaveRequests {
		if existing.EmployeeID != req.EmployeeID || existing.Status == models.LeaveStatusDenied {
			continue
		}
		existStart, _ := time.Parse("2006-01-02", existing.StartDate)
		existEnd, _ := time.Parse("2006-01-02", existing.EndDate)
		if startDate.Before(existEnd.AddDate(0, 0, 1)) && endDate.After(existStart.AddDate(0, 0, -1)) {
			return nil, "overlapping leave request exists", 400
		}
	}

	// Calculate days requested (inclusive)
	days := endDate.Sub(startDate).Hours()/24 + 1

	// For non-unpaid leave types, check balance
	if req.LeaveType != models.LeaveTypeUnpaid {
		empBalances, ok := s.balances[req.EmployeeID]
		if !ok {
			return nil, "no balance record for employee", 400
		}
		entry, ok := empBalances[req.LeaveType]
		if !ok || (entry.Allocated-entry.Used) < days {
			return nil, "insufficient leave balance", 400
		}
	}

	now := time.Now()
	req.ID = s.nextID("req")
	req.Status = models.LeaveStatusPending
	req.CreatedAt = now
	req.UpdatedAt = now

	s.leaveRequests[req.ID] = req
	return req, "", 0
}

// GetLeaveRequest returns a single leave request by ID.
func (s *Store) GetLeaveRequest(id string) (*models.LeaveRequest, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	req, ok := s.leaveRequests[id]
	return req, ok
}

// ListLeaveRequests returns requests filtered by optional employeeID, managerId, or status.
func (s *Store) ListLeaveRequests(employeeID, managerID, status string) []*models.LeaveRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*models.LeaveRequest
	for _, req := range s.leaveRequests {
		if employeeID != "" && req.EmployeeID != employeeID {
			continue
		}
		if managerID != "" {
			mgr, ok := s.managers[req.EmployeeID]
			if !ok || mgr != managerID {
				continue
			}
		}
		if status != "" && string(req.Status) != status {
			continue
		}
		result = append(result, req)
	}
	if result == nil {
		result = []*models.LeaveRequest{}
	}
	return result
}

// UpdateLeaveRequestStatus approves or denies a leave request.
// Returns the updated request, or an error string and HTTP status code.
func (s *Store) UpdateLeaveRequestStatus(id, reviewerID, newStatus, comments string) (*models.LeaveRequest, string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.leaveRequests[id]
	if !ok {
		return nil, "leave request not found", 404
	}

	// Verify manager authorization
	mgr, hasMgr := s.managers[req.EmployeeID]
	if !hasMgr || mgr != reviewerID {
		return nil, "reviewer is not the employee's manager", 403
	}

	// Validate status value
	if newStatus != string(models.LeaveStatusApproved) && newStatus != string(models.LeaveStatusDenied) {
		return nil, "status must be 'approved' or 'denied'", 400
	}

	// Only deduct balance once when transitioning to approved
	if newStatus == string(models.LeaveStatusApproved) && req.Status != models.LeaveStatusApproved {
		if req.LeaveType != models.LeaveTypeUnpaid {
			startDate, _ := time.Parse("2006-01-02", req.StartDate)
			endDate, _ := time.Parse("2006-01-02", req.EndDate)
			days := endDate.Sub(startDate).Hours()/24 + 1

			empBalances := s.balances[req.EmployeeID]
			if entry, ok := empBalances[req.LeaveType]; ok {
				entry.Used += days
			}
		}
	}

	req.Status = models.LeaveStatus(newStatus)
	req.ReviewerID = reviewerID
	req.ReviewerComments = comments
	req.UpdatedAt = time.Now()

	return req, "", 0
}
