package http

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/ticket/domain"
)

type attachmentResponse struct {
	ID          string    `json:"id"`
	IssueID     string    `json:"issue_id"`
	UploaderID  string    `json:"uploader_id"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

func attachmentDTO(a *domain.Attachment) attachmentResponse {
	return attachmentResponse{a.ID.String(), a.IssueID.String(), a.UploaderID.String(), a.FileName, a.ContentType, a.Size, a.CreatedAt}
}

func (h *Handler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, role, err := h.attachmentRequest(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if _, err = h.service.Get(r.Context(), issueID, actorID, role); err != nil {
		httpresponse.Error(w, err)
		return
	}
	items, err := h.attachments.ListAttachments(r.Context(), issueID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	data := make([]attachmentResponse, 0, len(items))
	for _, item := range items {
		data = append(data, attachmentDTO(item))
	}
	httpresponse.JSON(w, http.StatusOK, data)
}

func (h *Handler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, role, err := h.attachmentRequest(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if err = h.service.AuthorizeIssueEdit(r.Context(), issueID, actorID, role); err != nil {
		httpresponse.Error(w, err)
		return
	}
	if h.attachments == nil || h.files == nil {
		httpresponse.Error(w, apperror.New(apperror.KindInternal, "attachment storage is unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, domain.MaxAttachmentSize+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "attachment exceeds the request limit or multipart data is invalid"))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	f, header, err := r.FormFile("file")
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "file is required"))
		return
	}
	defer f.Close()
	if header.Size < 1 || header.Size > domain.MaxAttachmentSize {
		httpresponse.Error(w, domain.ErrInvalidAttachment)
		return
	}
	buf := make([]byte, 512)
	n, readErr := io.ReadFull(f, buf)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to read attachment", readErr))
		return
	}
	contentType := http.DetectContentType(buf[:n])
	if semi := strings.IndexByte(contentType, ';'); semi >= 0 {
		contentType = contentType[:semi]
	}
	if contentType == "text/plain" && (!utf8.Valid(buf[:n]) || strings.HasPrefix(strings.TrimSpace(string(buf[:n])), "<")) {
		httpresponse.Error(w, domain.ErrInvalidAttachment)
		return
	}
	if contentType == "text/plain" && strings.EqualFold(strings.TrimSpace(header.Filename[strings.LastIndex(header.Filename, ".")+1:]), "csv") {
		contentType = "text/csv"
	}
	if contentType != "application/pdf" && contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/gif" && contentType != "image/webp" && contentType != "text/plain" && contentType != "text/csv" {
		httpresponse.Error(w, domain.ErrInvalidAttachment)
		return
	}
	attachment, err := domain.NewAttachment(issueID, actorID, header.Filename, contentType, header.Size)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	reader := io.MultiReader(strings.NewReader(string(buf[:n])), f)
	size, err := h.files.Save(r.Context(), attachment.StorageKey, reader)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if size != header.Size {
		h.discardAttachmentFile(attachment.StorageKey, r)
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "attachment size does not match uploaded data"))
		return
	}
	if err = h.attachments.SaveAttachment(r.Context(), attachment); err != nil {
		h.discardAttachmentFile(attachment.StorageKey, r)
		httpresponse.Error(w, err)
		return
	}
	httpresponse.JSON(w, http.StatusCreated, attachmentDTO(attachment))
}

func (h *Handler) discardAttachmentFile(key string, r *http.Request) {
	if err := h.files.Delete(context.WithoutCancel(r.Context()), key); err != nil {
		slog.Warn("failed to remove unreferenced attachment file", "storage_key", key, "error", err)
	}
}

func (h *Handler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	issueID, actorID, role, err := h.attachmentRequest(r)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if _, err = h.service.Get(r.Context(), issueID, actorID, role); err != nil {
		httpresponse.Error(w, err)
		return
	}
	attachmentID, err := uuid.Parse(chi.URLParam(r, "attachment_id"))
	if err != nil {
		httpresponse.Error(w, apperror.New(apperror.KindValidation, "invalid attachment id"))
		return
	}
	attachment, err := h.attachments.FindAttachment(r.Context(), issueID, attachmentID)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	file, err := h.files.Open(r.Context(), attachment.StorageKey)
	if err != nil {
		httpresponse.Error(w, apperror.Wrap(apperror.KindInternal, "failed to open attachment", err))
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", attachment.Size))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.FileName}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, io.LimitReader(file, attachment.Size))
}

func (h *Handler) attachmentRequest(r *http.Request) (uuid.UUID, uuid.UUID, string, error) {
	if h.attachments == nil || h.files == nil {
		return uuid.Nil, uuid.Nil, "", apperror.New(apperror.KindInternal, "attachment storage is unavailable")
	}
	actorID, ok := appmw.UserIDFromContext(r.Context())
	if !ok {
		return uuid.Nil, uuid.Nil, "", apperror.New(apperror.KindUnauthorized, "not authenticated")
	}
	issueID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, uuid.Nil, "", apperror.New(apperror.KindValidation, "invalid issue id")
	}
	role, _ := appmw.RoleFromContext(r.Context())
	return issueID, actorID, role, nil
}
