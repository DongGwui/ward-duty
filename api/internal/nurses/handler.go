package nurses

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/levels"
	"ward-duty-api/internal/notifications"
	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct {
	Repo      *Repo
	LevelRepo *levels.Repo
	Notif     *notifications.Repo // Phase B — level_changed / fixed_pattern_changed 트리거
}

func New(pg *pgxpool.Pool) *Handler {
	return &Handler{Repo: NewRepo(pg), LevelRepo: levels.NewRepo(pg)}
}

// WithNotif — main.go에서 주입. nil이면 알림 발사 안 함.
func (h *Handler) WithNotif(n *notifications.Repo) *Handler { h.Notif = n; return h }

// GET /api/nurses?include_inactive=1
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("include_inactive") == "1"
	ns, err := h.Repo.List(r.Context(), includeInactive)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	// resolved_level 자동 채움
	lvs, err := h.LevelRepo.List(r.Context())
	if err == nil {
		today := time.Now()
		for i := range ns {
			ns[i].ResolvedLevel = ClassifyLevel(&ns[i], lvs, today)
		}
	}
	httpx.JSON(w, http.StatusOK, ns)
}

// POST /api/nurses (head_nurse)
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.Name == "" {
		// Stage 1: email은 옵션 (계정 없이 명단만 추가 가능)
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "name required")
		return
	}
	n, err := h.Repo.Create(r.Context(), in)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, n)
}

// PATCH /api/nurses/{id}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	var in UpdateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	// 알림 발송용으로 변경 전 상태 스냅샷 (실패해도 Update 진행).
	before, _ := h.Repo.Get(r.Context(), id)
	n, err := h.Repo.Update(r.Context(), id, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "nurse not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	notifyChanges(r.Context(), h.Notif, before, n)
	httpx.JSON(w, http.StatusOK, n)
}
