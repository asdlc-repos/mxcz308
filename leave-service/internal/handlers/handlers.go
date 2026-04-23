package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/asdlc-repos/mxcz308/leave-service/internal/models"
	"github.com/asdlc-repos/mxcz308/leave-service/internal/store"
)

// Handler holds dependencies for HTTP handlers.
type Handler struct {
	store *store.Store
}

// New creates a new Handler.
func New(s *store.Store) *Handler {
	return &Handler{store: s}
}

// RegisterRoutes registers all routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/api/v1/leave-requests", h.handleLeaveRequests)
	mux.HandleFunc("/api/v1/leave-requests/", h.handleLeaveRequestByID)
	mux.HandleFunc("/api/v1/employees/", h.handleEmployees)
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR encoding response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ---------- health ----------

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- /api/v1/leave-requests ----------

func (h *Handler) handleLeaveRequests(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listLeaveRequests(w, r)
	case http.MethodPost:
		h.createLeaveRequest(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listLeaveRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	employeeID := q.Get("employeeId")
	managerID := q.Get("managerId")
	status := q.Get("status")

	results := h.store.ListLeaveRequests(employeeID, managerID, status)
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) createLeaveRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		EmployeeID string `json:"employeeId"`
		LeaveType  string `json:"leaveType"`
		StartDate  string `json:"startDate"`
		EndDate    string `json:"endDate"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.EmployeeID == "" || body.LeaveType == "" || body.StartDate == "" || body.EndDate == "" {
		writeError(w, http.StatusBadRequest, "employeeId, leaveType, startDate, endDate are required")
		return
	}

	lt := models.LeaveType(body.LeaveType)
	valid := false
	for _, t := range models.AllLeaveTypes {
		if lt == t {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, "leaveType must be one of: annual, sick, personal, unpaid")
		return
	}

	req := &models.LeaveRequest{
		EmployeeID: body.EmployeeID,
		LeaveType:  lt,
		StartDate:  body.StartDate,
		EndDate:    body.EndDate,
		Reason:     body.Reason,
	}

	created, errMsg, statusCode := h.store.CreateLeaveRequest(req)
	if errMsg != "" {
		writeError(w, statusCode, errMsg)
		return
	}

	log.Printf("INFO created leave request %s for employee %s", created.ID, created.EmployeeID)
	writeJSON(w, http.StatusCreated, created)
}

// ---------- /api/v1/leave-requests/{id} ----------

func (h *Handler) handleLeaveRequestByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path: /api/v1/leave-requests/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/leave-requests/")
	if id == "" {
		h.handleLeaveRequests(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getLeaveRequest(w, r, id)
	case http.MethodPatch:
		h.updateLeaveRequestStatus(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getLeaveRequest(w http.ResponseWriter, r *http.Request, id string) {
	req, ok := h.store.GetLeaveRequest(id)
	if !ok {
		writeError(w, http.StatusNotFound, "leave request not found")
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *Handler) updateLeaveRequestStatus(w http.ResponseWriter, r *http.Request, id string) {
	var body struct {
		Status     string `json:"status"`
		ReviewerID string `json:"reviewerId"`
		Comments   string `json:"comments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Status == "" || body.ReviewerID == "" {
		writeError(w, http.StatusBadRequest, "status and reviewerId are required")
		return
	}

	updated, errMsg, statusCode := h.store.UpdateLeaveRequestStatus(id, body.ReviewerID, body.Status, body.Comments)
	if errMsg != "" {
		writeError(w, statusCode, errMsg)
		return
	}

	log.Printf("INFO updated leave request %s to status %s by reviewer %s", id, body.Status, body.ReviewerID)
	writeJSON(w, http.StatusOK, updated)
}

// ---------- /api/v1/employees/{id}/balance ----------

func (h *Handler) handleEmployees(w http.ResponseWriter, r *http.Request) {
	// Path after prefix: "{id}/balance"
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/employees/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	employeeID := parts[0]
	subPath := parts[1]

	switch subPath {
	case "balance":
		switch r.Method {
		case http.MethodGet:
			h.getLeaveBalance(w, r, employeeID)
		case http.MethodPut:
			h.setLeaveAllocation(w, r, employeeID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// getLeaveBalance handles GET /api/v1/employees/{id}/balance
// Uses a read lock via store.GetBalance for concurrent safety.
func (h *Handler) getLeaveBalance(w http.ResponseWriter, r *http.Request, employeeID string) {
	balance, ok := h.store.GetBalance(employeeID)
	if !ok {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}
	log.Printf("INFO fetched balance for employee %s", employeeID)
	writeJSON(w, http.StatusOK, balance)
}

// setLeaveAllocation handles PUT /api/v1/employees/{id}/balance
//
// Simulates HR admin authorization via the X-User-Role request header:
//   - If the header is absent the request is allowed (open demo access).
//   - If the header is present but not "hradmin", 403 Forbidden is returned.
//
// The handler updates only the Allocated field; Used is never modified.
func (h *Handler) setLeaveAllocation(w http.ResponseWriter, r *http.Request, employeeID string) {
	// Simulate HR admin authorization check.
	if role := r.Header.Get("X-User-Role"); role != "" && role != "hradmin" {
		writeError(w, http.StatusForbidden, "unauthorized: HR admin role required")
		return
	}

	var body struct {
		LeaveType string  `json:"leaveType"`
		Allocated float64 `json:"allocated"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.LeaveType == "" {
		writeError(w, http.StatusBadRequest, "leaveType is required")
		return
	}
	if body.Allocated < 0 {
		writeError(w, http.StatusBadRequest, "allocated must be non-negative")
		return
	}

	lt := models.LeaveType(body.LeaveType)

	// SetAllocation acquires a write lock and only modifies the Allocated field.
	if ok := h.store.SetAllocation(employeeID, lt, body.Allocated); !ok {
		writeError(w, http.StatusNotFound, "employee not found")
		return
	}

	log.Printf("INFO set allocation for employee %s leaveType %s to %.1f days", employeeID, lt, body.Allocated)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
