package alerts

import (
	"context"
	"encoding/json"
	"github.com/balsuraksha/api/internal/config"
	"github.com/balsuraksha/api/internal/db"
	authMW "github.com/balsuraksha/api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

type DraftHandler struct {
	pool *db.Pool
	cfg  *config.Config
}
type ApprovalHandler struct {
	pool *db.Pool
	cfg  *config.Config
}
type ActivationHandler struct {
	pool *db.Pool
	cfg  *config.Config
}
type PublicHandler struct {
	pool *db.Pool
	cfg  *config.Config
}

func NewDraftHandler(p *db.Pool, c *config.Config) *DraftHandler       { return &DraftHandler{p, c} }
func NewApprovalHandler(p *db.Pool, c *config.Config) *ApprovalHandler { return &ApprovalHandler{p, c} }
func NewActivationHandler(p *db.Pool, c *config.Config) *ActivationHandler {
	return &ActivationHandler{p, c}
}
func NewPublicHandler(p *db.Pool, c *config.Config) *PublicHandler { return &PublicHandler{p, c} }
func (h *DraftHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.HandleCreate)
	r.Get("/{revisionID}", h.HandleGet)
	return r
}
func (h *ApprovalHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.HandleListDrafts)
	r.Post("/", h.HandleDecision)
	return r
}
func (h *ActivationHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/{alertID}/activate", h.HandleActivate)
	r.Post("/{alertID}/withdraw", h.HandleWithdraw)
	r.Post("/{alertID}/resolve", h.HandleResolve)
	return r
}
func (h *PublicHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/", h.HandleList)
	r.Get("/{alertID}", h.HandleGet)
	return r
}
func allowed(w http.ResponseWriter, r *http.Request, roles ...string) *authMW.StaffClaims {
	c := authMW.GetStaffClaims(r.Context())
	if c == nil {
		writeError(w, 401, "unauthorized", "Staff sign-in required")
		return nil
	}
	for _, role := range roles {
		if c.Role == role {
			return c
		}
	}
	writeError(w, 403, "insufficient_role", "Your role cannot perform this action")
	return nil
}
func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if json.NewDecoder(r.Body).Decode(v) != nil {
		writeError(w, 400, "invalid_request", "Invalid request")
		return false
	}
	return true
}
func audit(ctx context.Context, tx pgx.Tx, c *authMW.StaffClaims, id, event string) error {
	_, err := tx.Exec(ctx, `INSERT INTO audit_events(event_type,actor_type,actor_id,organization_id,target_type,target_id) VALUES($1,'staff',$2,$3,'alert',$4)`, event, c.StaffID, c.OrganizationID, id)
	return err
}
func (h *DraftHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	c := allowed(w, r, "alert_preparer", "admin")
	if c == nil {
		return
	}
	var req struct {
		CaseID       string `json:"case_id"`
		Description  string `json:"description_text"`
		Issuer       string `json:"issuer_name"`
		Expiry       string `json:"expiry_at"`
		AreaID       string `json:"area_id"`
		Verification string `json:"verification_reference"`
	}
	if !decode(w, r, &req) {
		return
	}
	expiry, err := time.Parse(time.RFC3339, req.Expiry)
	if _, e := uuid.Parse(req.CaseID); e != nil || err != nil || !expiry.After(time.Now()) || expiry.After(time.Now().Add(24*time.Hour)) || strings.TrimSpace(req.Description) == "" || utf8.RuneCountInString(req.Description) > 500 || len(req.Issuer) > 255 || strings.TrimSpace(req.Issuer) == "" || strings.TrimSpace(req.Verification) == "" || len(req.Verification) > 500 {
		writeError(w, 400, "invalid_draft", "Provide a case, verified source reference, area, public description and expiry within 24 hours")
		return
	}
	test := h.cfg.AppMode == config.AppModeDemo || h.cfg.IsTestMode()
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Draft unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var ok bool
	err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cases WHERE id=$1 AND organization_id=$2 AND route_type='missing_child') AND EXISTS(SELECT 1 FROM alert_areas WHERE id=$3 AND active AND is_test=$4)`, req.CaseID, c.OrganizationID, req.AreaID, test).Scan(&ok)
	if err != nil {
		writeError(w, 503, "db_error", "Draft unavailable")
		return
	}
	if !ok {
		writeError(w, 404, "invalid_case_or_area", "An authorized missing-child case and available area are required")
		return
	}
	id, revision := uuid.NewString(), uuid.NewString()
	_, err = tx.Exec(r.Context(), `INSERT INTO alerts(id,case_id,organization_id,is_test) VALUES($1,$2,$3,$4)`, id, req.CaseID, c.OrganizationID, test)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO alert_revisions(id,alert_id,revision_number,description_text,issuer_name,expiry_at,tip_route_email,prepared_by,area_id,verification_reference) VALUES($1,$2,1,$3,$4,$5,'private-in-app',$6,$7,$8)`, revision, id, req.Description, req.Issuer, expiry, c.StaffID, req.AreaID, req.Verification)
	}
	if err == nil {
		err = audit(r.Context(), tx, c, id, "alert_drafted")
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 503, "db_error", "Draft not confirmed; refresh before retrying")
		return
	}
	writeJSON(w, 201, map[string]interface{}{"revision_id": revision, "alert_id": id, "status": "DRAFT", "is_test": test})
}

type draftRow struct {
	ID           string    `json:"id"`
	AlertID      string    `json:"alert_id"`
	Status       string    `json:"status"`
	AlertStatus  string    `json:"alert_status"`
	Description  string    `json:"description_text"`
	Issuer       string    `json:"issuer_name"`
	Expiry       time.Time `json:"expiry_at"`
	PreparedBy   string    `json:"prepared_by"`
	AreaID       string    `json:"area_id"`
	AreaName     string    `json:"area_name"`
	Verification string    `json:"verification_reference"`
	IsTest       bool      `json:"is_test"`
	Queued       int       `json:"queued"`
	Accepted     int       `json:"provider_accepted"`
	Failed       int       `json:"failed"`
}

const draftQuery = `SELECT ar.id,ar.alert_id,ar.status,a.status,ar.description_text,ar.issuer_name,ar.expiry_at,ar.prepared_by,COALESCE(ar.area_id,''),COALESCE(area.name,'Unregistered area'),COALESCE(ar.verification_reference,''),a.is_test,
(SELECT count(*) FROM delivery_jobs WHERE revision_id=ar.id AND status IN ('QUEUED','PROCESSING')),
(SELECT count(*) FROM delivery_jobs WHERE revision_id=ar.id AND status='SENT'),
(SELECT count(*) FROM delivery_jobs WHERE revision_id=ar.id AND status IN ('FAILED','AMBIGUOUS'))
FROM alert_revisions ar JOIN alerts a ON a.id=ar.alert_id LEFT JOIN alert_areas area ON area.id=ar.area_id `

func scanDraft(row pgx.Row, d *draftRow) error {
	return row.Scan(&d.ID, &d.AlertID, &d.Status, &d.AlertStatus, &d.Description, &d.Issuer, &d.Expiry, &d.PreparedBy, &d.AreaID, &d.AreaName, &d.Verification, &d.IsTest, &d.Queued, &d.Accepted, &d.Failed)
}
func (h *DraftHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	c := authMW.GetStaffClaims(r.Context())
	if c == nil {
		writeError(w, 401, "unauthorized", "Staff sign-in required")
		return
	}
	if _, err := uuid.Parse(chi.URLParam(r, "revisionID")); err != nil {
		writeError(w, 400, "invalid_id", "Invalid revision")
		return
	}
	var d draftRow
	err := scanDraft(h.pool.QueryRow(r.Context(), draftQuery+` WHERE ar.id=$1 AND a.organization_id=$2`, chi.URLParam(r, "revisionID"), c.OrganizationID), &d)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "not_found", "Revision not found")
		return
	}
	if err != nil {
		writeError(w, 503, "db_error", "Revision unavailable")
		return
	}
	writeJSON(w, 200, d)
}
func (h *ApprovalHandler) HandleListDrafts(w http.ResponseWriter, r *http.Request) {
	c := authMW.GetStaffClaims(r.Context())
	if c == nil {
		writeError(w, 401, "unauthorized", "Staff sign-in required")
		return
	}
	rows, err := h.pool.Query(r.Context(), draftQuery+` WHERE a.organization_id=$1 ORDER BY ar.created_at DESC LIMIT 50`, c.OrganizationID)
	if err != nil {
		writeError(w, 503, "db_error", "Alert queue unavailable")
		return
	}
	defer rows.Close()
	drafts := []draftRow{}
	for rows.Next() {
		var d draftRow
		if scanDraft(rows, &d) != nil {
			writeError(w, 503, "db_error", "Alert queue unavailable")
			return
		}
		drafts = append(drafts, d)
	}
	if rows.Err() != nil {
		writeError(w, 503, "db_error", "Alert queue unavailable")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"drafts": drafts, "notification_mode": h.cfg.NotificationMode})
}
func (h *ApprovalHandler) HandleDecision(w http.ResponseWriter, r *http.Request) {
	c := allowed(w, r, "alert_approver", "admin")
	if c == nil {
		return
	}
	var req struct {
		RevisionID string `json:"revision_id"`
		Decision   string `json:"decision"`
		Notes      string `json:"notes"`
	}
	if !decode(w, r, &req) {
		return
	}
	if _, err := uuid.Parse(req.RevisionID); err != nil || (req.Decision != "approve" && req.Decision != "reject") || len(req.Notes) > 1000 {
		writeError(w, 400, "invalid_decision", "Choose approve or reject for a valid revision")
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Review unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var id, preparer, status string
	var expiry time.Time
	err = tx.QueryRow(r.Context(), `SELECT a.id,ar.prepared_by,ar.status,ar.expiry_at FROM alerts a JOIN alert_revisions ar ON ar.alert_id=a.id JOIN alert_areas area ON area.id=ar.area_id AND area.active WHERE ar.id=$1 AND a.organization_id=$2 AND ar.verification_reference IS NOT NULL FOR UPDATE OF a,ar`, req.RevisionID, c.OrganizationID).Scan(&id, &preparer, &status, &expiry)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "not_found", "Reviewable revision not found")
		return
	}
	if err != nil {
		writeError(w, 503, "db_error", "Review unavailable")
		return
	}
	if preparer == c.StaffID {
		writeError(w, 403, "self_approval_rejected", "A different staff member must review this alert")
		return
	}
	var decided bool
	if tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM alert_approvals WHERE revision_id=$1)`, req.RevisionID).Scan(&decided) != nil {
		writeError(w, 503, "db_error", "Review unavailable")
		return
	}
	if decided || status != "DRAFT" || !expiry.After(time.Now()) {
		writeError(w, 409, "not_reviewable", "This revision is already decided or expired")
		return
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO alert_approvals(alert_id,revision_id,approver_id,decision,notes) VALUES($1,$2,$3,$4,$5)`, id, req.RevisionID, c.StaffID, req.Decision, req.Notes)
	next := "APPROVED"
	if req.Decision == "reject" {
		next = "WITHDRAWN"
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE alert_revisions SET status=$1 WHERE id=$2`, next, req.RevisionID)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE alerts SET status=$1,updated_at=NOW() WHERE id=$2`, next, id)
	}
	if err == nil {
		err = audit(r.Context(), tx, c, id, "alert_"+req.Decision)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 503, "db_error", "Review not confirmed; refresh")
		return
	}
	writeJSON(w, 200, map[string]string{"status": next})
}
func (h *ActivationHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	c := allowed(w, r, "alert_approver", "admin")
	if c == nil {
		return
	}
	var req struct {
		Revision string `json:"approved_revision_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	id := chi.URLParam(r, "alertID")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, 400, "invalid_id", "Invalid alert")
		return
	}
	if _, err := uuid.Parse(req.Revision); err != nil {
		writeError(w, 400, "invalid_id", "Invalid revision")
		return
	}
	if !h.cfg.IsSendingAllowed() {
		writeError(w, 503, "sending_disabled", "Notifications are disabled; approval remains saved")
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Activation unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var status, revStatus, active string
	var test bool
	var expiry time.Time
	err = tx.QueryRow(r.Context(), `SELECT a.status,ar.status,COALESCE(a.active_revision_id::text,''),a.is_test,ar.expiry_at FROM alerts a JOIN alert_revisions ar ON ar.alert_id=a.id JOIN alert_areas area ON area.id=ar.area_id AND area.active AND area.is_test=a.is_test JOIN alert_approvals aa ON aa.revision_id=ar.id AND aa.decision='approve' WHERE a.id=$1 AND ar.id=$2 AND a.organization_id=$3 AND ar.verification_reference IS NOT NULL FOR UPDATE OF a,ar`, id, req.Revision, c.OrganizationID).Scan(&status, &revStatus, &active, &test, &expiry)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "not_approved", "Authorized approved revision not found")
		return
	}
	if err != nil {
		writeError(w, 503, "db_error", "Activation unavailable")
		return
	}
	if test != h.cfg.IsTestMode() || !expiry.After(time.Now()) || revStatus != "APPROVED" {
		writeError(w, 409, "invalid_activation", "Check notification mode, approval and expiry")
		return
	}
	if status == "ACTIVE" && active == req.Revision {
		writeJSON(w, 200, map[string]string{"status": "ACTIVE"})
		return
	}
	if status != "APPROVED" {
		writeError(w, 409, "invalid_transition", "This alert cannot be reactivated")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE alerts SET status='ACTIVE',active_revision_id=$1,updated_at=NOW() WHERE id=$2`, req.Revision, id)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO outbox_events(event_type,alert_id,revision_id,payload) VALUES('alert_activated',$1,$2,jsonb_build_object('is_test',$3::boolean))`, id, req.Revision, test)
	}
	if err == nil {
		err = audit(r.Context(), tx, c, id, "alert_activated")
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 503, "db_error", "Activation not confirmed; refresh before retrying")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ACTIVE"})
}
func (h *ActivationHandler) HandleWithdraw(w http.ResponseWriter, r *http.Request) {
	h.terminate(w, r, "WITHDRAWN")
}
func (h *ActivationHandler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	h.terminate(w, r, "RESOLVED")
}
func (h *ActivationHandler) terminate(w http.ResponseWriter, r *http.Request, next string) {
	c := allowed(w, r, "alert_approver", "supervisor", "admin")
	if c == nil {
		return
	}
	id := chi.URLParam(r, "alertID")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, 400, "invalid_id", "Invalid alert")
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Reason) == "" || len(req.Reason) > 500 {
		writeError(w, 400, "reason_required", "Give a brief non-identifying reason")
		return
	}
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeError(w, 503, "db_error", "Update unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	var status string
	err = tx.QueryRow(r.Context(), `SELECT status FROM alerts WHERE id=$1 AND organization_id=$2 FOR UPDATE`, id, c.OrganizationID).Scan(&status)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "not_found", "Alert not found")
		return
	}
	if err != nil {
		writeError(w, 503, "db_error", "Update unavailable")
		return
	}
	if status == next {
		writeJSON(w, 200, map[string]string{"status": next})
		return
	}
	if status != "ACTIVE" {
		writeError(w, 409, "invalid_transition", "Only active alerts can be closed")
		return
	}
	_, err = tx.Exec(r.Context(), `UPDATE alerts SET status=$1,updated_at=NOW() WHERE id=$2`, next, id)
	if err == nil {
		_, err = tx.Exec(r.Context(), `UPDATE delivery_jobs SET status='CANCELLED' WHERE alert_id=$1 AND status IN ('QUEUED','PROCESSING')`, id)
	}
	if err == nil {
		err = audit(r.Context(), tx, c, id, "alert_"+strings.ToLower(next))
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeError(w, 503, "db_error", "Update not confirmed; refresh")
		return
	}
	writeJSON(w, 200, map[string]string{"status": next})
}

type publicAlert struct {
	ID          string    `json:"alert_id"`
	Revision    string    `json:"revision_id"`
	Status      string    `json:"status"`
	Description string    `json:"description_text"`
	Issuer      string    `json:"issuer_name"`
	Expiry      time.Time `json:"expiry_at"`
	Issued      time.Time `json:"issued_at"`
	AreaID      string    `json:"area_id"`
	AreaName    string    `json:"area_name"`
	IsTest      bool      `json:"is_test"`
	Tips        bool      `json:"tip_route_available"`
	URL         string    `json:"current_status_url"`
}

const publicQuery = `SELECT a.id,ar.id,a.status,ar.description_text,ar.issuer_name,ar.expiry_at,a.updated_at,ar.area_id,area.name,a.is_test FROM alerts a JOIN alert_revisions ar ON ar.id=a.active_revision_id JOIN alert_areas area ON area.id=ar.area_id JOIN alert_approvals aa ON aa.revision_id=ar.id AND aa.decision='approve' `

func scanPublic(row pgx.Row, a *publicAlert) error {
	err := row.Scan(&a.ID, &a.Revision, &a.Status, &a.Description, &a.Issuer, &a.Expiry, &a.Issued, &a.AreaID, &a.AreaName, &a.IsTest)
	if a.Status == "ACTIVE" && !a.Expiry.After(time.Now()) {
		a.Status = "EXPIRED"
	}
	a.Tips = a.Status == "ACTIVE"
	a.URL = "/alerts/" + a.ID
	if !a.Tips {
		a.Description = ""
	}
	return err
}
func (h *PublicHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "alertID")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, 404, "not_found", "Alert not found")
		return
	}
	var a publicAlert
	err := scanPublic(h.pool.QueryRow(r.Context(), publicQuery+` WHERE a.id=$1 AND a.status IN ('ACTIVE','RESOLVED','WITHDRAWN','EXPIRED')`, id), &a)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "not_found", "Alert not found")
		return
	}
	if err != nil {
		writeError(w, 503, "unavailable", "Current status unavailable")
		return
	}
	writeJSON(w, 200, a)
}
func (h *PublicHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	test := h.cfg.AppMode == config.AppModeDemo || h.cfg.IsTestMode()
	rows, err := h.pool.Query(r.Context(), publicQuery+` WHERE a.status='ACTIVE' AND ar.expiry_at>NOW() AND area.active AND a.is_test=$1 AND ($2='' OR ar.area_id=$2) ORDER BY a.updated_at DESC LIMIT 50`, test, r.URL.Query().Get("area_id"))
	if err != nil {
		writeError(w, 503, "unavailable", "Alerts unavailable")
		return
	}
	defer rows.Close()
	items := []publicAlert{}
	for rows.Next() {
		var a publicAlert
		if scanPublic(rows, &a) != nil {
			writeError(w, 503, "unavailable", "Alerts unavailable")
			return
		}
		items = append(items, a)
	}
	if rows.Err() != nil {
		writeError(w, 503, "unavailable", "Alerts unavailable")
		return
	}
	writeJSON(w, 200, map[string]interface{}{"alerts": items, "is_test": test})
}
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
