package domain

import (
	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// CommentVisibility says who can read a comment. Public comments are read by everyone who can browse the
// project; internal comments only by the project roles at or above Comment.InternalRole.
type CommentVisibility string

const (
	CommentVisibilityPublic   CommentVisibility = "public"
	CommentVisibilityInternal CommentVisibility = "internal"
)

// CommentLevel ranks the project role of a user for comment visibility. The zero value means no access.
type CommentLevel int

const (
	CommentLevelNone CommentLevel = iota
	CommentLevelViewer
	CommentLevelContributor
	CommentLevelLead
)

// Name returns the project role name of the level, or an empty string for no access.
func (l CommentLevel) Name() string {
	switch l {
	case CommentLevelViewer:
		return "viewer"
	case CommentLevelContributor:
		return "contributor"
	case CommentLevelLead:
		return "lead"
	default:
		return ""
	}
}

// ParseInternalRole reads the minimum role of an internal comment. Only contributor and lead are accepted: a
// viewer minimum would make the comment public.
func ParseInternalRole(raw string) (CommentLevel, error) {
	switch raw {
	case "contributor":
		return CommentLevelContributor, nil
	case "lead":
		return CommentLevelLead, nil
	default:
		return CommentLevelNone, ErrCommentVisibilityInvalid
	}
}

// CanReadComment reports whether a viewer at the given level can read the comment.
func CanReadComment(viewer CommentLevel, visibility CommentVisibility, internalRole CommentLevel) bool {
	if visibility != CommentVisibilityInternal {
		return viewer >= CommentLevelViewer
	}
	return viewer >= internalRole && internalRole != CommentLevelNone
}

// CommentReactions is the fixed set of reaction emojis, in display order.
var CommentReactions = []string{"thumbs_up", "heart", "tada", "eyes", "rocket", "check"}

// IsCommentReaction reports whether emoji is in the reaction set.
func IsCommentReaction(emoji string) bool {
	for _, known := range CommentReactions {
		if emoji == known {
			return true
		}
	}
	return false
}

// CommentDeletedPlaceholder is the body shown for a soft-deleted comment.
const CommentDeletedPlaceholder = "Comentario eliminado"

// CommentMaxMentions bounds the distinct mentions a single body may carry.
const CommentMaxMentions = 25

var (
	ErrCommentTooLong           = apperror.WithCode(apperror.KindValidation, "issue.comment_too_long", "el comentario supera los 20000 caracteres")
	ErrCommentNotFound          = apperror.WithCode(apperror.KindNotFound, "issue.comment_not_found", "comentario no encontrado")
	ErrCommentForbidden         = apperror.WithCode(apperror.KindForbidden, "issue.comment_forbidden", "no tienes permiso para esta accion sobre el comentario")
	ErrCommentParentInvalid     = apperror.WithCode(apperror.KindValidation, "issue.comment_parent_invalid", "el comentario padre no existe, no pertenece a la incidencia o no admite respuestas")
	ErrCommentVisibilityInvalid = apperror.WithCode(apperror.KindValidation, "issue.comment_visibility_invalid", "la visibilidad del comentario no es valida")
	ErrCommentReplyVisibility   = apperror.WithCode(apperror.KindValidation, "issue.comment_reply_visibility", "la respuesta no puede ser publica si el comentario padre es interno")
	ErrCommentTooManyMentions   = apperror.WithCode(apperror.KindValidation, "issue.comment_too_many_mentions", "el comentario menciona demasiadas personas")
	ErrCommentSortInvalid       = apperror.WithCode(apperror.KindValidation, "issue.comment_sort_invalid", "el orden debe ser asc o desc")
	ErrCommentCursorInvalid     = apperror.WithCode(apperror.KindValidation, "issue.comment_cursor_invalid", "el cursor de comentarios no es valido")
	ErrCommentLimitInvalid      = apperror.WithCode(apperror.KindValidation, "issue.comment_limit_invalid", "el limite debe estar entre 1 y 100")
	ErrCommentReactionInvalid   = apperror.WithCode(apperror.KindValidation, "issue.comment_reaction_invalid", "la reaccion no es valida")
)
