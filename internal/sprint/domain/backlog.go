package domain

import (
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/rank"
)

var (
	ErrInvalidAnchor = apperror.WithCode(apperror.KindValidation, "backlog.invalid_anchor", "La incidencia ancla debe estar en la seccion de destino y no puede ser una de las movidas")
	ErrNoIssues      = apperror.WithCode(apperror.KindValidation, "backlog.no_issues", "Indica al menos una incidencia")
	ErrTooManyIssues = apperror.WithCode(apperror.KindValidation, "backlog.too_many_issues", "Demasiadas incidencias en un solo movimiento (200 como maximo)")
	ErrBothAnchors   = apperror.WithCode(apperror.KindValidation, "backlog.invalid_anchor", "Usa before_issue_id o after_issue_id, no los dos")
)

// MaxMoveIssues limita cuantas incidencias mueve una sola peticion de reordenado.
const MaxMoveIssues = 200

// RankedID es una incidencia de una seccion con su rank.
type RankedID struct {
	ID   uuid.UUID
	Rank string
}

// PlanSection decide los ranks al insertar las incidencias moved, en ese orden, en la posicion pos (0..len)
// de una seccion cuyas demas incidencias son existing (ordenadas). Devuelve los ranks de las movidas y,
// si no cabe un rank intermedio (ranks iguales o demasiado largos), tambien los de toda la seccion
// renumerada, en cuyo caso rebalanced es true.
func PlanSection(existing []RankedID, moved []uuid.UUID, pos int) (ranks map[uuid.UUID]string, rebalanced bool) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(existing) {
		pos = len(existing)
	}
	lo, hi := "", ""
	if pos > 0 {
		lo = existing[pos-1].Rank
	}
	if pos < len(existing) {
		hi = existing[pos].Rank
	}
	ranks = make(map[uuid.UUID]string, len(moved))
	prev := lo
	ok := true
	for _, id := range moved {
		next, err := rank.Between(prev, hi)
		if err != nil || rank.NeedsRebalance(next) {
			ok = false
			break
		}
		ranks[id] = next
		prev = next
	}
	if ok {
		return ranks, false
	}
	order := make([]uuid.UUID, 0, len(existing)+len(moved))
	for _, e := range existing[:pos] {
		order = append(order, e.ID)
	}
	order = append(order, moved...)
	for _, e := range existing[pos:] {
		order = append(order, e.ID)
	}
	fresh := rank.Rebalance(len(order))
	ranks = make(map[uuid.UUID]string, len(order))
	for i, id := range order {
		ranks[id] = fresh[i]
	}
	return ranks, true
}

// PositionFor calcula el indice de insercion a partir del ancla. Sin ancla se inserta al final.
func PositionFor(existing []RankedID, before, after *uuid.UUID) (int, error) {
	if before != nil && after != nil {
		return 0, ErrBothAnchors
	}
	anchor := before
	if after != nil {
		anchor = after
	}
	if anchor == nil {
		return len(existing), nil
	}
	for i, e := range existing {
		if e.ID == *anchor {
			if after != nil {
				return i + 1, nil
			}
			return i, nil
		}
	}
	return 0, ErrInvalidAnchor
}
