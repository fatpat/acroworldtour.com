package admin

import (
	"net/http"

	"awt/internal/domain"
	"awt/internal/service"
	"awt/internal/web"
)

type filesView struct {
	Files  []service.File
	Flash  string
	Errors formErrors
}

func (h *handlers) registerFiles(r *web.Router) {
	get(r, "/manager/files", h.filesList)
	postUpload(r, "/manager/files", h.fileUpload)
}

func (h *handlers) filesPage(w http.ResponseWriter, r *http.Request, status int, fe formErrors) {
	files, err := h.svc.ListFiles(r.Context())
	if err != nil {
		h.fail(w, r, err, nil)
		return
	}
	render(w, r, status, filesPage(filesView{Files: files, Flash: flash(r), Errors: fe}))
}

func (h *handlers) filesList(w http.ResponseWriter, r *http.Request) {
	h.filesPage(w, r, http.StatusOK, formErrors{})
}

func (h *handlers) fileUpload(w http.ResponseWriter, r *http.Request) {
	redo := func(status int, fe formErrors) { h.filesPage(w, r, status, fe) }
	f, hdr, err := r.FormFile("file")
	if err != nil {
		redo(http.StatusUnprocessableEntity, validationErrors([]domain.FieldError{{Field: "file", Msg: "choose a file"}}))
		return
	}
	defer f.Close()
	if _, err := h.svc.UploadFile(r.Context(), hdr.Filename, f); err != nil {
		h.fail(w, r, err, redo)
		return
	}
	redirect(w, r, "/manager/files", "upload")
}

// fileURL is the public URL of an uploaded file.
func fileURL(f service.File) string { return "/files/" + f.ID }
